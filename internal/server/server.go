// Package server provides a library to initialize and start a
// thermomatic server.
package server

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/tjper/thermomatic/internal/client"
)

// Server is the thermomatic server.
// TODO: XXX improve client event and error logging, and server event logging
// TODO: XXX drop client connections that fail to send a message once every two seconds
// TODO: XXX drop client connections that fail to send login message within 1 second of imei
// TODO: XXX drop client if imei is invalid
// TODO: XXX drop client from ClientMap on client disconnect
// TODO: XXX 2nd pass on bound checks
// TODO: XXX Log client connecting, logging in, disconnecting
// TODO: XXX 2nd pass on Server logging
// TODO: XXX review code documentation and update accordingly
// TODO: XXX add ticker to Reading process to minimize unecessary spinning
// TODO: XXX devise strategy against resource exhaustion attacks
// TODO: XXX devise strategy for duplicate logins
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
		logError:  log.New(os.Stderr, "[Thermomatic ERROR] ", 0),
		logInfo:   log.New(os.Stdout, "[Thermomatic INFO] ", 0),
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

// Shutdown communicates to all thermomatic server processes that shutdown has
// begun. Shutdown logs that shutdown has completed when server has been
// completely shutdown.
func (srv *Server) Shutdown() {
	srv.logInfo.Printf(
		"Shutting down Thermomatic server listening at %s\n",
		srv.listener.Addr())
	close(srv.stop)
	<-srv.exited
	srv.logInfo.Println("Finished shutting down Thermomatic server.")
}

// ListenAndServe accepts incoming TCP connections, creates and manages
// Clients, and processes the clients connection contents in a seperate
// goroutine.
func (srv *Server) ListenAndServe() {
	srv.logInfo.Println("accepting TCP connections...")
	ctx, cancel := context.WithCancel(context.Background())

	for {
		select {
		case <-srv.stop:
			srv.listener.Close()
			cancel()
			close(srv.exited)
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
			go func(ctx context.Context, c net.Conn) {
				defer c.Close()

				client, err := client.New(ctx, conn)
				if err != nil {
					srv.logError.Println(err)
					return
				}
				defer client.Close()

				if srv.clientMap.Exists(client.IMEI()) {
					srv.logError.Printf("Client %d is already connected\n", client.IMEI())
					return
				}
				srv.clientMap.Store(client.IMEI(), *client)
				defer srv.clientMap.Delete(client.IMEI())

				if err := client.ProcessLogin(ctx); err != nil {
					srv.logError.Println(err)
					return
				}

				if err := client.ProcessReadings(ctx); err != nil {
					srv.logError.Println(err)
					return
				}
			}(ctx, conn)
		}
	}
}
