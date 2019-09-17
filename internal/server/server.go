// Package server provides a library to initialize and start a
// thermomatic server.
package server

import (
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// Server is the thermomatic server.
type Server struct {
	listener *net.TCPListener
	stdErr   *log.Logger
	stdOut   *log.Logger

	stop   chan struct{}
	exited chan struct{}
}

// New initializes a Server object and listens for TCP packets on the port
// specified on localhost. On success, a Server reference is returned, and a
// nil error. On failure, a nil Server reference is returned, and a non-nil
// error.
func New(port int) (*Server, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{
		Port: port,
	})

	if err != nil {
		return nil, err
	}
	return &Server{
		listener: l,
		stdErr:   log.New(os.Stderr, "[thermomatic ERROR] ", log.LstdFlags),
		stdOut:   log.New(os.Stdout, "[thermomatic INFO] ", log.LstdFlags),
		stop:     make(chan struct{}),
		exited:   make(chan struct{}),
	}, nil
}

// Shutdown shuts down the thermomatic server.
func (srv *Server) Shutdown() {
	srv.stdOut.Println("Stopping Server...")
	close(srv.stop)
	<-srv.exited
	srv.stdOut.Println("Stopped Server...")
}

// Accept accepts incoming TCP connections and processes their contents.
func (srv *Server) Accept() {
	var handlers sync.WaitGroup
	for {
		select {
		case <-srv.stop:
			srv.stdOut.Println("Closing Listener...")
			srv.listener.Close()
			srv.stdOut.Println("Stop Accepting Conn(s)...")
			handlers.Wait()
			srv.stdOut.Println("Closed Listener...")
			close(srv.exited)
			return

		default:
			err := srv.listener.SetDeadline(time.Now().Add(time.Second))
			if err != nil {
				srv.stdErr.Println(err)
				continue
			}
			conn, err := srv.listener.Accept()
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			if err != nil {
				srv.stdErr.Println(err)
				continue
			}

			handlers.Add(1)
			go func(c net.Conn) {
				defer c.Close()
				handlers.Done()
			}(conn)
		}
	}
}
