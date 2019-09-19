// +build integration

package server

import (
	"net"
	"strconv"
	"testing"
)

func TestStart(t *testing.T) {
	tests := []struct {
		Name     string
		Port     int
		Messages [][]byte
	}{
		{
			Name:     "Connect",
			Port:     1337,
			Messages: [][]byte{},
		},
		{
			Name: "Login",
			Port: 1337,
			Messages: [][]byte{
				[]byte("490sdfs154203237518"),
				[]byte("lsdfogin"),
			},
		},
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
			for _, message := range test.Messages {
				_, err := conn.Write(message)
				if err != nil {
					t.Fatalf("unexpected error = %s", err)
				}
			}
		})
	}
}
