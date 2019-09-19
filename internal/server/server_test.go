// +build integration

package server

import (
	"net"
	"strconv"
	"testing"

	"github.com/tjper/thermomatic/internal/client"
)

func TestLogin(t *testing.T) {
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
				[]byte("490154203237518"),
				[]byte("login"),
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

func TestProcessReadings(t *testing.T) {
	tests := []struct {
		Name     string
		Port     int
		Messages [][]byte
	}{
		{
			Name: "Single Reading",
			Port: 1337,
			Messages: [][]byte{
				[]byte("490154203237518"),
				[]byte("login"),
				reading(t),
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

func reading(t *testing.T) []byte {
	b, err := client.Reading{
		Temperature:  67.77,
		Altitude:     2.63555,
		Latitude:     33.41,
		Longitude:    44.4,
		BatteryLevel: 0.25666,
	}.Encode()
	if err != nil {
		t.Fatalf("unexpected error = %s", err)
	}
	return b
}
