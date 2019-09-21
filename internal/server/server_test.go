// +build development

package server

import (
	"bytes"
	"encoding/json"
	"flag"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/tjper/thermomatic/internal/client"
)

var golden = flag.Bool("golden", false, "overwrite *.golden files for golden file tests")

func TestLogin(t *testing.T) {
	tests := []struct {
		Name     string
		Port     int
		Messages [][]byte
	}{
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
				t.Errorf("unexpected error = %s\n", err)
			}
			defer svr.Shutdown()
			go svr.Accept()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer conn.Close()
			for _, message := range test.Messages {
				_, err := conn.Write(message)
				if err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
			}
		})
	}
}

func TestProcessReading(t *testing.T) {
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
				t.Errorf("unexpected error = %s\n", err)
			}
			defer svr.Shutdown()
			go svr.Accept()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer conn.Close()
			for _, message := range test.Messages {
				_, err := conn.Write(message)
				if err != nil {
					t.Errorf("unexpected error = %s\n", err)
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
		t.Errorf("unexpected error = %s\n", err)
	}
	return b
}

func TestProcessReadings(t *testing.T) {
	tests := []struct {
		Name     string
		Port     int
		Messages [][]byte
	}{
		{
			Name:     "1000 Messages",
			Port:     1337,
			Messages: messages(t),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := new(bytes.Buffer)
			svr, err := New(test.Port, WithLoggerOutput(w))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			go svr.Accept()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}

			for _, message := range test.Messages {
				_, err := conn.Write([]byte(message))
				if err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
			}
		})
	}
}

func messages(t *testing.T) [][]byte {
	f, err := os.Open("testdata/readings.json")
	if err != nil {
		t.Errorf("unexpected error = %s\n", err)
	}
	defer f.Close()

	var readings []client.Reading
	if err := json.NewDecoder(f).Decode(&readings); err != nil {
		t.Errorf("unexpected error = %s\n", err)
	}

	msgs := [][]byte{
		[]byte("490154203237518"),
		[]byte("login"),
	}
	for _, reading := range readings {
		b, err := reading.Encode()
		if err != nil {
			t.Errorf("unexpected error = %s\n", err)
		}
		msgs = append(msgs, b)
	}
	return msgs
}
