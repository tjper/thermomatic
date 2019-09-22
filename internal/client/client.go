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
	"strconv"
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
	net.Conn

	imei            uint64
	workloadBalance uint64
	createdAt       time.Time
	lastReadAt      time.Time

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
	b := make([]byte, 15)
	if _, err := io.ReadFull(conn, b); err != nil {
		return nil, fmt.Errorf("failed to client.New/ReadFull\tb = \"%s\" err = %s", b, err)
	}
	imei, err := imei.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("failed to client.New/Decode\tb = \"%s\" err = %s", b, err)
	}

	c := &Client{
		Conn:      conn,
		imei:      imei,
		createdAt: time.Now(),

		logInfo:  log.New(os.Stdout, "[Client "+strconv.Itoa(int(imei))+"] ", 0),
		logError: log.New(os.Stderr, "[Client "+strconv.Itoa(int(imei))+"] ", 0),

		toClose: make(chan struct{}, 5),
		done:    make(chan struct{}),
	}
	for _, option := range options {
		option(c)
	}
	go c.watchReadFrequency(ctx, 500*time.Millisecond)
	go c.workloadBalanceIncrementer(ctx, 20*time.Millisecond, 10)
	go c.moderator()

	c.logInfo.Println("Connection Established")
	return c, nil
}

func (c Client) moderator() {
	<-c.toClose
	close(c.done)
}

// workloadBalanceIncrementer increments the workloadBalance field by 1 at the
// rate passed as long as the balance is below max.
func (c *Client) workloadBalanceIncrementer(ctx context.Context, rate time.Duration, max uint64) {
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if c.workloadBalance <= max {
				c.workloadBalance++
			}
		}
	}
}

// watchReadFrequency ensures that a Reading has occurred within the last 2
// seconds, or the Client connection is closed.
func (c Client) watchReadFrequency(ctx context.Context, checkRate time.Duration) {
	ticker := time.NewTicker(checkRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if time.Since(c.lastReadAt) > (2 * time.Second) {
				c.Close()
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
func (c Client) IMEI() uint64 {
	return c.imei
}

// ProcessLogin authorizes the Client connection by ensuring TCP message
// following IMEI message, has a "login" payload. On success, a nil error is
// returned. On failure, a non-nil error is returned.
func (c *Client) ProcessLogin(ctx context.Context) error {
	b := make([]byte, 5)
	for {
		select {
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		default:
		}

		select {
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		case <-time.After(time.Second):
			return ErrClientLoginWindowExpired
		default:
			_, err := io.ReadFull(c.Conn, b)
			if err == io.EOF {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to client.Login/ReadFull\tb = % x, err = %s", b, err)
			}
			if !bytes.Equal([]byte(login), b) {
				return ErrClientUnauthorized
			}
			c.logInfo.Println("Logged-In")
			return nil
		}
	}
}

// ProcessReadings process incoming "Reading" TCP messages for the Client.
func (c *Client) ProcessReadings(ctx context.Context) error {
	prevprefix := c.logError.Prefix()
	defer c.logError.SetPrefix(prevprefix)
	c.logError.SetPrefix("")

	b := make([]byte, 40)
	var reading Reading
	for {
		select {
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		default:
		}

		select {
		case <-ctx.Done():
			return ErrClientClose
		case <-c.done:
			return ErrClientClose
		default:
			if c.workloadBalance == 0 {
				continue
			}
			c.workloadBalance--

			_, err := io.ReadFull(c.Conn, b)
			if err == io.EOF {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to client.ProcessReadings/ReadFull\tb = % x, err = %s", b, err)
			}
			if err := reading.Decode(b); err != nil {
				c.logError.SetPrefix(prevprefix)
				c.logError.Printf(
					"Failed to Client.ProcessReadings/decode\t b = %x, err = %s\n",
					b,
					err)
				continue
			}
			c.logError.Printf("%d,%d,%s",
				time.Now().UnixNano(),
				c.IMEI(),
				reading)
			c.lastReadAt = time.Now()
		}
	}
}

// ClientOption modifies a Client object. Typically used with New to initialize
// a Client object.
type ClientOption func(*Client)

// WithInfoLogger returns a ClientOption that configures the Client to use the
// passed logger as the info logger.
func WithInfoLogger(logger *log.Logger) ClientOption {
	return func(c *Client) {
		c.logInfo = logger
	}
}

// WithErrorLogger returns a ClientOption that configures the Client to use the
// passed logger as the error logger.
func WithErrorLogger(logger *log.Logger) ClientOption {
	return func(c *Client) {
		c.logError = logger
	}
}
