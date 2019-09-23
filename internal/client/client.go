// Package client provides a library to initialize and manage a Client. A
// Client is a TCP connection managed by the Thermomatic server.
package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/tjper/thermomatic/internal/imei"
)

var (
	// ErrClientUnauthorized indicates the client connection has not logged in.
	ErrClientUnauthorized = errors.New("client unauthorized")

	// ErrClientLoginWindowExpired indicates the client connection login window expired. The default login window is 1 second after the IMEI is received.
	ErrClientLoginWindowExpired = errors.New("client login window expired")

	// ErrClientClose indicates the client was closed.
	ErrClientClose = errors.New("client closed")
)

const (
	login = "login"
)

// Client is a thermomatic client.
type Client struct {
	net.Conn

	imei        safeUint64
	bucket      safeUint64
	createdAt   safeTime
	lastReadAt  safeTime
	lastReading safeReading
	logReading  logReadingFunc

	logInfo  *log.Logger
	logError *log.Logger

	toShutdown chan struct{}
	done       chan struct{}
}

// New initializes a Client object with the passed net.Conn. On success, the
// a Client reference, and a nil error is returned. On failure a nil Client
// reference, and an error is returned.
func New(ctx context.Context, conn net.Conn, options ...ClientOption) (*Client, error) {
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return nil, fmt.Errorf("failed to Client.New/SetDeadline\terr = %s", err)
	}

	b := make([]byte, 15)
	if _, err := io.ReadFull(conn, b); err != nil {
		return nil, fmt.Errorf("failed to client.New/ReadFull\tb = \"%s\" err = %s", b, err)
	}
	imei, err := imei.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("failed to client.New/Decode\tb = \"%s\" err = %s", b, err)
	}

	c := &Client{
		Conn:       conn,
		imei:       safeUint64{val: imei},
		createdAt:  safeTime{val: time.Now()},
		lastReadAt: safeTime{val: time.Now()},
		logReading: LogReadingWithUnixNano,

		logInfo:  log.New(os.Stdout, "", 0),
		logError: log.New(os.Stderr, "", 0),

		toShutdown: make(chan struct{}, 5),
		done:       make(chan struct{}),
	}

	for _, option := range options {
		option(c)
	}
	go c.watchReadFrequency(ctx, 500*time.Millisecond)
	go c.bucketIncrementer(ctx, 20*time.Millisecond, 10)
	go c.moderator()

	c.logInfo.Printf("[IMEI %d] Connection Established\n", c.IMEI())
	return c, nil
}

func (c *Client) moderator() {
	<-c.toShutdown
	close(c.done)
}

// LogReading logs the reading with the reading device's IMEI.
func LogReading(logger *log.Logger, imei uint64, reading Reading) {
	logger.Printf("%d,%s\n", imei, reading)
}

// LogReadingWithUnixNano logs the reading with the current UnixNano time, and
// the reading device's IMEI.
func LogReadingWithUnixNano(logger *log.Logger, imei uint64, reading Reading) {
	logger.Printf("%d,%d,%s\n", time.Now().UnixNano(), imei, reading)
}

// bucketIncrementer increments the workloadBalance field by 1 at the
// rate passed as long as the balance is below max.
func (c *Client) bucketIncrementer(ctx context.Context, rate time.Duration, max uint64) {
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if v := c.bucket.get(); v < max {
				c.bucket.set(v + 1)
			}
		}
	}
}

// watchReadFrequency ensures that a Reading has occurred within the last 2
// seconds, or the Client connection is closed.
func (c *Client) watchReadFrequency(ctx context.Context, checkRate time.Duration) {
	ticker := time.NewTicker(checkRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if time.Since(c.lastReadAt.get()) > (2 * time.Second) {
				c.logError.Printf("[IMEI %d] No Readings for 2 seconds, Closing Client\n", c.IMEI())
				c.shutdown()
				return
			}
		}
	}
}

// toClose releases all Client sub-processes and resources.
func (c *Client) shutdown() {
	c.toShutdown <- struct{}{}
}

// IMEI is a getter for the client's IMEI.
func (c *Client) IMEI() uint64 {
	return c.imei.get()
}

// LastReading is a getter for the Client's most recent reading.
func (c *Client) LastReading() Reading {
	return c.lastReading.get()
}

// ProcessLogin authorizes the Client connection by ensuring TCP message
// following IMEI message, has a "login" payload. On success, a nil error is
// returned. On failure, a non-nil error is returned.
func (c *Client) ProcessLogin(ctx context.Context) error {
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

	b := make([]byte, 5)
	for {
		select {
		case <-timeout.C:
			c.logError.Printf("[IMEI %d] Login Window Expired\n", c.IMEI())
			c.shutdown()
			return ErrClientLoginWindowExpired
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		default:
			_, err := io.ReadFull(c.Conn, b)
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			}
			if err == io.EOF {
				continue
			}
			if err != nil {
				c.shutdown()
				return fmt.Errorf("[IMEI %d] failed to client.Login/ReadFull\tb = % x, err = %s", c.IMEI(), b, err)
			}
			if !bytes.Equal([]byte(login), b) {
				c.shutdown()
				return ErrClientUnauthorized
			}
			c.logInfo.Printf("[IMEI %d] Logged-In\n", c.IMEI())
			return nil
		}
	}
}

// ProcessReadings process incoming "Reading" TCP messages for the Client.
func (c *Client) ProcessReadings(ctx context.Context) error {
	b := make([]byte, 40)
	var reading Reading
	for {
		select {
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		default:
			if c.bucket.get() == 0 {
				continue
			}

			_, err := io.ReadFull(c.Conn, b)
			if err, ok := err.(net.Error); ok && err.Timeout() {
				continue
			}
			if err == io.EOF {
				continue
			}
			if err != nil {
				c.shutdown()
				return fmt.Errorf("[IMEI %d] failed to client.ProcessReadings/ReadFull\tb = % x, err = %s", c.IMEI(), b, err)
			}
			c.bucket.decrement()

			if err := reading.Decode(b); err != nil {
				c.logError.Printf(
					"[IMEI %d] Failed to Client.ProcessReadings/decode\t b = %x, err = %s\n",
					c.imei.get(),
					b,
					err)
				continue
			}

			c.logReading(c.logError, c.imei.get(), reading)
			c.lastReadAt.set(time.Now())
			c.lastReading.set(reading)
		}
	}
}

// ClientOption modifies a Client object. Typically used with New to initialize
// a Client object.
type ClientOption func(*Client)

// WithLoggerOutput returns a ClientOption that sets the client's loggers
// output to the writer passed.
func WithLoggerOutput(w io.Writer) ClientOption {
	return func(c *Client) {
		c.logError.SetOutput(w)
		c.logInfo.SetOutput(w)
	}
}

// logReadingFunc logs a Reading.
type logReadingFunc func(*log.Logger, uint64, Reading)

// WithLogReading returns a ClientOption that sets the client's LogReading
// function to the function specified.
func WithLogReading(f logReadingFunc) ClientOption {
	return func(c *Client) {
		c.logReading = f
	}
}
