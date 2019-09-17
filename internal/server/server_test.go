// +build integration

package server

import (
	"net"
	"strconv"
	"testing"
)

func TestStart(t *testing.T) {
	tests := []struct {
		Name string
		Port int
	}{
		{Name: "Connect", Port: 1337},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			svr, err := New(test.Port)
			if err != nil {
				t.Fatal(err)
			}
			defer svr.Shutdown()
			go svr.Accept()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
		})
	}
}
