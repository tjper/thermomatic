// Package server provides a library to initialize and start a
// thermomatic server.
package server

import (
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/tjper/thermomatic/internal/client"
)

// Server is the thermomatic server.
// TODO: improve client event and error logging, and server event logging
// TODO: drop client connections that fail to send a message once every two seconds
// TODO: drop client connections that fail to send login message within 1 second of imei
// TODO: drop client if imei is invalid
// TODO: 2nd pass on bound checks
// TODO: Log client connecting, logging in, disconnecting
// TODO: 2nd pass on Server logging
type Server struct {
	listener  *net.TCPListener
	logError  *log.Logger
	logInfo   *log.Logger
	clientMap *client.ClientMap

	stop   chan struct{}
	exited chan struct{}
}

// New initializes a Server object and listens for TCP packets on the port
// specified on localhost. On success, a Server reference is returned, and a
// nil error. On failure, a nil Server reference is returned, and a non-nil
// error.
func New(port int, options ...ServerOption) (*Server, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{
		Port: port,
	})
	if err != nil {
		return nil, err
	}
	if err := l.SetDeadline(time.Now().Add(time.Second)); err != nil {
		return nil, err
	}

	srv := &Server{
		listener:  l,
		clientMap: client.NewClientMap(),
		logError:  log.New(os.Stderr, "", 0),
		logInfo:   log.New(os.Stdout, "", 0),
		stop:      make(chan struct{}),
		exited:    make(chan struct{}),
	}
	for _, option := range options {
		option(srv)
	}

	srv.logInfo.Printf("Initialized Thermomatic Server at localhost:%d\n", port)
	return srv, nil
}

// ServerOption modifies a Server object. Typically used with New to initialize
// a Server object.
type ServerOption func(*Server)

// WithLoggerOutput returns a ServerOption function that configures the Server's
// loggers to write to w.
func WithLoggerOutput(w io.Writer) ServerOption {
	return func(srv *Server) {
		srv.logError.SetOutput(w)
		srv.logInfo.SetOutput(w)
	}
}

// Shutdown shuts down the thermomatic server.
func (srv *Server) Shutdown() {
	srv.logInfo.Printf(
		"Shutting down Thermomatic server listening at %s\n",
		srv.listener.Addr())
	close(srv.stop)
	<-srv.exited
	srv.logInfo.Println("Finished shutting down Thermomatic server.")
}

// Accept accepts incoming TCP connections and processes their contents.
func (srv *Server) Accept() {
	srv.logInfo.Println("accepting TCP connections...")
	for {
		select {
		case <-srv.stop:
			srv.Stop()
			return

		default:
			conn, err := srv.listener.Accept()
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			if err != nil {
				srv.logError.Println(err)
				continue
			}

			go func(c net.Conn) {
				defer c.Close()

				client, err := client.New(c,
					client.WithErrorLogger(srv.logError),
					client.WithInfoLogger(srv.logInfo),
				)
				if err != nil {
					srv.logError.Println(err)
					return
				}
				srv.clientMap.Store(client.IMEI(), *client)

				if err := client.Login(); err != nil {
					srv.logError.Println(err)
					return
				}

				if err := client.ProcessReadings(); err != nil {
					srv.logError.Println(err)
					return
				}
			}(conn)
		}
	}
}

func (srv *Server) Stop() {
	srv.listener.Close()
	srv.logInfo.Println("Closed Listener.")

	srv.clientMap.Range(func(_ uint64, c client.Client) bool {
		c.Shutdown()
		return true
	})
	srv.logInfo.Println("Closed Clients.")

	close(srv.exited)
}
