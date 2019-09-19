// TODO: document package.
package client

import (
	"bytes"
	"errors"
	"io"
	"net"
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
}

// New initializes a Client object with the passed net.Conn. On success, the
// a Client reference, and a nil error is returned. On failure a nil Client
// reference, and an error is returned.
func New(conn net.Conn) (*Client, error) {
	b := make([]byte, 15)
	if _, err := io.ReadFull(conn, b); err != nil {
		return nil, err
	}

	imei, err := imei.Decode(b)
	if err != nil {
		return nil, err
	}

	return &Client{
		Conn:      conn,
		imei:      imei,
		createdAt: time.Now(),
	}, nil
}

// Login authorizes the Client connection by ensuring TCP message following
// Dial, has a "login" payload. On success, a nil error is returned. On
// failure, a non-nil error is returned.
func (c *Client) Login() error {
	b := make([]byte, 5)
	for {
		_, err := io.ReadFull(c.Conn, b)
		if err == io.EOF {
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	if !bytes.Equal([]byte(login), b) {
		return ErrClientUnauthorized
	}
	if time.Since(c.createdAt) > time.Second {
		return ErrClientLoginWindowExpired
	}
	c.authorized = true
	return nil
}

// ProcessReadings process incoming "Reading" TCP messages for the Client.
func (c Client) ProcessReadings() error {
	return nil
}
