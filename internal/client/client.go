// TODO: document package.
package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/tjper/thermomatic/internal/imei"
)

var (
	// ErrClientUnauthorized indicates the client connection has not logged in.
	ErrClientUnauthorized = errors.New("client unauthorized")

	// ErrClientLoginWindowExpired indicates the client connection login window expired. The default login window is 1 second after the IMEI is received.
	ErrClientLoginWindowExpired = errors.New("client login window expired")
)

const (
	login = "login"
)

// Client is a thermomatic client.
type Client struct {
	net.Conn

	imei       uint64
	authorized bool
	createdAt  time.Time
	lastReadAt time.Time

	logInfo  *log.Logger
	logError *log.Logger

	stop   chan struct{}
	exited chan struct{}
}

// New initializes a Client object with the passed net.Conn. On success, the
// a Client reference, and a nil error is returned. On failure a nil Client
// reference, and an error is returned.
func New(conn net.Conn, options ...ClientOption) (*Client, error) {
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

		stop:   make(chan struct{}),
		exited: make(chan struct{}),
	}
	for _, option := range options {
		option(c)
	}
	go c.watchReadFrequency(500 * time.Millisecond)
	c.logInfo.Println("Connection Established")
	return c, nil
}

func (c Client) watchReadFrequency(checkRate time.Duration) {
	ticker := time.NewTicker(checkRate)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			c.Stop()
			return
		case <-ticker.C:
			if time.Since(c.lastReadAt) < (2 * time.Second) {
				return
			}
			c.Stop()
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

// IMEI is a getter for the Client.IMEI field
func (c Client) IMEI() uint64 {
	return c.imei
}

// Login authorizes the Client connection by ensuring TCP message following
// Dial, has a "login" payload. On success, a nil error is returned. On
// failure, a non-nil error is returned.
func (c *Client) Login() error {
	b := make([]byte, 5)
	for {
		select {
		case <-time.After(time.Second):
			c.Stop()
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
			c.authorized = true
			c.logInfo.Println("Logged-In")
			return nil
		}
	}
}

// ProcessReadings process incoming "Reading" TCP messages for the Client.
func (c *Client) ProcessReadings() error {
	b := make([]byte, 40)
	var reading Reading
	for {
		select {
		case <-c.stop:
			c.Stop()
			return nil

		default:
			_, err := io.ReadFull(c.Conn, b)
			if err == io.EOF {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to client.ProcessReadings/ReadFull\tb = % x, err = %s", b, err)
			}
			if err := reading.Decode(b); err != nil {
				c.logError.Printf(
					"Failed to Client.ProcessReadings/decode\t b = %x, err = %s\n",
					b,
					err)
				continue
			}
			c.logError.Printf("%d, %d, %s",
				time.Now().UnixNano(),
				c.IMEI(),
				reading)
			c.lastReadAt = time.Now()
		}
	}
}

// Shutdown signals the Client to being shutdown processes.
func (c Client) Shutdown() {
	close(c.stop)
	<-c.exited
}

// Stop ends the Clients processing and executes the appropriate cleanup tasks.
func (c Client) Stop() {
	c.logInfo.Println("Stopping...")
	c.Close()
	close(c.exited)
	c.logInfo.Println("Stopped")
}
