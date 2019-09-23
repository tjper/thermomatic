// TODO: document package.
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
	"sync"
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
	sync.RWMutex
	net.Conn

	imei        uint64
	bucket      uint64
	createdAt   time.Time
	lastReadAt  time.Time
	lastReading Reading
	logReading  logReadingFunc

	logInfo  *log.Logger
	logError *log.Logger

	once    sync.Once
	toClose chan struct{}
	done    chan struct{}
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
		imei:       imei,
		createdAt:  time.Now(),
		lastReadAt: time.Now(),
		logReading: LogReadingWithUnixNano,

		logInfo:  log.New(os.Stdout, "", 0),
		logError: log.New(os.Stderr, "", 0),

		toClose: make(chan struct{}, 5),
		done:    make(chan struct{}),
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
	<-c.toClose
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
			c.Lock()
			if c.bucket < max {
				c.bucket++
			}
			c.Unlock()
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
			c.RLock()
			lastReadAt := c.lastReadAt
			c.RUnlock()
			if time.Since(lastReadAt) > (2 * time.Second) {
				c.logError.Printf("[IMEI %d] No Readings for 2 seconds, Closing Client\n", c.IMEI())
				c.Close()
				return
			}
		}
	}
}

// Close releases all Client sub-processes and resources.
func (c *Client) Close() {
	c.once.Do(func() {
		c.toClose <- struct{}{}
	})
}

// IMEI is a getter for the Client.IMEI field
func (c *Client) IMEI() uint64 {
	c.RLock()
	defer c.RUnlock()
	return c.imei
}

// LastReading is a getter for the Client's most recent reading.
func (c *Client) LastReading() Reading {
	c.RLock()
	defer c.RUnlock()
	return c.lastReading
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
				return fmt.Errorf("[IMEI %d] failed to client.Login/ReadFull\tb = % x, err = %s", c.IMEI(), b, err)
			}
			if !bytes.Equal([]byte(login), b) {
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
			c.RLock()
			balance := c.bucket
			c.RUnlock()
			if balance == 0 {
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
				return fmt.Errorf("[IMEI %d] failed to client.ProcessReadings/ReadFull\tb = % x, err = %s", c.IMEI(), b, err)
			}

			c.Lock()
			c.bucket--
			c.Unlock()

			if err := reading.Decode(b); err != nil {
				c.logError.Printf(
					"[IMEI %d] Failed to Client.ProcessReadings/decode\t b = %x, err = %s\n",
					c.IMEI(),
					b,
					err)
				continue
			}

			c.logReading(c.logError, c.IMEI(), reading)
			c.Lock()
			c.lastReadAt = time.Now()
			c.lastReading = reading
			c.Unlock()
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
