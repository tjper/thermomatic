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

	"github.com/tjper/thermomatic/internal/common"
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

	imei        common.Uint64Holder
	createdAt   common.TimeHolder
	lastReadAt  common.TimeHolder
	lastReading ReadingHolder
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
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		return nil, fmt.Errorf("failed to client.New/SetReadDeadline\terr = %s", err)
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
		Conn:        conn,
		imei:        common.NewUint64Holder(imei),
		createdAt:   common.NewTimeHolder(time.Now()),
		lastReadAt:  common.NewTimeHolder(time.Now()),
		lastReading: NewReadingHolder(Reading{}),
		logReading:  LogReadingWithUnixNano,

		logInfo:  log.New(os.Stdout, "", log.LstdFlags),
		logError: log.New(os.Stderr, "", log.LstdFlags),

		toShutdown: make(chan struct{}, 7),
		done:       make(chan struct{}),
	}

	for _, option := range options {
		option(c)
	}
	go c.moderator()

	c.logInfo.Printf("[IMEI %d] Connection Established\n", c.IMEI())
	return c, nil
}

func (c Client) moderator() {
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

// toClose releases all Client sub-processes and resources.
func (c Client) shutdown() {
	c.toShutdown <- struct{}{}
}

// IMEI is a getter for the client's IMEI.
func (c Client) IMEI() uint64 {
	return c.imei.Get()
}

// LastReading is a getter for the Client's most recent reading.
func (c Client) LastReading() Reading {
	return c.lastReading.Get()
}

// ProcessLogin authorizes the Client connection by ensuring TCP message
// following IMEI message, has a "login" payload. On success, a nil error is
// returned. On failure, a non-nil error is returned.
func (c Client) ProcessLogin(ctx context.Context) error {
	b := make([]byte, 5)
	for {
		select {
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		default:
			_, err := io.ReadFull(c.Conn, b)
			if err, ok := err.(net.Error); ok && err.Timeout() {
				c.logError.Printf("[IMEI %d] Login Window Expired\n", c.IMEI())
				c.shutdown()
				return ErrClientLoginWindowExpired
			}
			if err == io.EOF {
				continue
			}
			if err != nil {
				c.shutdown()
				return fmt.Errorf("[IMEI %d] failed to client.ProcessLogin/ReadFull\tb = % x, err = %s", c.IMEI(), b, err)
			}
			if err := c.Conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				c.shutdown()
				return fmt.Errorf("[IMEI %d] failed to client.ProcessLogin/SetReadDeadline\terr = %s", c.IMEI(), err)
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
func (c Client) ProcessReadings(ctx context.Context) error {
	read := time.NewTicker(time.Duration(25 * time.Millisecond))
	defer read.Stop()

	b := make([]byte, 40)
	var reading Reading
	for {
		select {
		case <-c.done:
			return ErrClientClose
		default:
		}
		select {
		case <-ctx.Done():
			return ErrClientClose
		default:
		}

		select {
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		case <-read.C:
			_, err := io.ReadFull(c.Conn, b)
			if err, ok := err.(net.Error); ok && err.Timeout() {
				c.logError.Printf("[IMEI %d] No Readings for 2 seconds, Closing Client\n", c.IMEI())
				c.shutdown()
				return nil
			}
			if err == io.EOF {
				continue
			}
			if err != nil {
				c.shutdown()
				return fmt.Errorf("[IMEI %d] failed to client.ProcessReadings/ReadFull\tb = % x, err = %s", c.IMEI(), b, err)
			}
			if err := c.Conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				c.shutdown()
				return fmt.Errorf("[IMEI %d] failed to client.ProcessReadings/SetReadDeadline\terr = %s", c.IMEI(), err)
			}

			if err := reading.Decode(b); err != nil {
				c.logError.Printf(
					"[IMEI %d] Failed to Client.ProcessReadings/decode\t b = %x, err = %s\n",
					c.imei.Get(),
					b,
					err)
				continue
			}

			c.logReading(c.logError, c.imei.Get(), reading)
			c.lastReadAt.Set(time.Now())
			c.lastReading.Set(reading)
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

// WithLoggerFlags returns a ClientOption that sets the Client's loggers flags
// to the flags passed.
func WithLoggerFlags(flags int) ClientOption {
	return func(c *Client) {
		c.logError.SetFlags(flags)
		c.logInfo.SetFlags(flags)
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
