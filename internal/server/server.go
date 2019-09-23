// Package server provides a library to initialize and start a
// thermomatic server.
package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/tjper/thermomatic/internal/client"
)

// Server is the thermomatic server.
type Server struct {
	listener   *net.TCPListener
	httpServer http.Server

	clientMap     *client.ClientMap
	clientOptions []client.ClientOption

	logError *log.Logger
	logInfo  *log.Logger

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
		listener:      l,
		clientMap:     client.NewClientMap(),
		clientOptions: make([]client.ClientOption, 0),
		logError:      log.New(os.Stderr, "[Thermomatic ERROR] ", 0),
		logInfo:       log.New(os.Stdout, "[Thermomatic INFO] ", 0),
		stop:          make(chan struct{}),
		exited:        make(chan struct{}),
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
		srv.clientOptions = append(srv.clientOptions, client.WithLoggerOutput(w))
	}
}

// WithClientOptions returns a ServerOption function that configures the Server
// to utilize a set of ClientOptions for each Client object.
func WithClientOptions(options ...client.ClientOption) ServerOption {
	return func(srv *Server) {
		srv.clientOptions = append(srv.clientOptions, options...)
	}
}

// WithHttpServer returns a ServerOption function that initializes and starts
// an http server.
func WithHttpServer(port int) ServerOption {
	return func(srv *Server) {
		go func() {
			srv.httpServer = http.Server{
				Addr:    fmt.Sprintf(":%d", port),
				Handler: srv.router(),
			}
			srv.logError.Println(srv.httpServer.ListenAndServe())
		}()
	}
}

// Shutdown communicates to all thermomatic server processes that shutdown has
// begun. Shutdown logs that shutdown has completed when server has been
// completely shutdown.
func (srv *Server) Shutdown() {
	srv.logInfo.Printf(
		"Shutting down Thermomatic server listening at %s\n",
		srv.listener.Addr())

	if err := srv.httpServer.Shutdown(context.Background()); err != nil {
		srv.logError.Println(err)
	}

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

	var subProcesses sync.WaitGroup
	for {
		select {
		case <-srv.stop:
			srv.listener.Close()
			cancel()
			subProcesses.Wait()
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
			subProcesses.Add(1)
			go func(ctx context.Context, c net.Conn) {
				defer subProcesses.Done()
				defer c.Close()

				client, err := client.New(ctx, conn, srv.clientOptions...)
				if err != nil {
					srv.logError.Println(err)
					return
				}

				if srv.clientMap.Exists(client.IMEI()) {
					srv.logError.Printf("Client %d is already connected\n", client.IMEI())
					return
				}
				srv.clientMap.Store(client.IMEI(), *client)
				defer srv.clientMap.Delete(client.IMEI())

				if err := client.ProcessLogin(ctx); err != nil {
					srv.logError.Printf("failed to ProcessLogin\terr = %s\n", err)
					return
				}

				if err := client.ProcessReadings(ctx); err != nil {
					srv.logError.Printf("failed to ProcessReadings\terr = %s\n", err)
					return
				}
			}(ctx, conn)
		}
	}
}
