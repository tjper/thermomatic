// +build integration

package server

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tjper/thermomatic/internal/client"
)

// TODO: XXX golden test login
// TODO: XXX golden test did not login within second if IMEI
// TODO: XXX golden test no message in two seconds, connection closed
// TODO: XXX golden test 10 sent readings
// TODO: XXX golden test 1000 readings
// TODO: XXX golden test 10 sent readings, correct last reading
// TODO: XXX golden test online IMEI
// TODO: XXX golden test offline IMEI
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
			w := newSafeWriter()
			svr, err := New(
				test.Port,
				WithLoggerOutput(w),
			)
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer svr.Shutdown()
			go svr.ListenAndServe()

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
			time.Sleep(time.Second)

			isGolden(t, w.Bytes())
		})
	}
}

func TestLoginWindowExpired(t *testing.T) {
	tests := []struct {
		Name     string
		Port     int
		Messages [][]byte
	}{
		{
			Name: "login never sent",
			Port: 1337,
			Messages: [][]byte{
				[]byte("490154203237518"),
			},
		},
		{
			Name: "login window expired",
			Port: 1337,
			Messages: [][]byte{
				[]byte("490154203237518"),
				[]byte("login"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := newSafeWriter()
			svr, err := New(
				test.Port,
				WithLoggerOutput(w),
			)
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer svr.Shutdown()
			go svr.ListenAndServe()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer conn.Close()
			for i := range test.Messages {
				_, err := conn.Write(test.Messages[i])
				if err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
				time.Sleep(1500 * time.Millisecond)
			}

			isGolden(t, w.Bytes())
		})
	}
}

func TestNoMessageForTwoSeconds(t *testing.T) {
	tests := []struct {
		Name          string
		Port          int
		LoginMessages [][]byte
		Messages      [][]byte
	}{
		{
			Name: "No Message for Two Seconds",
			Port: 1337,
			LoginMessages: [][]byte{
				[]byte("490154203237518"),
				[]byte("login"),
			},
			Messages: [][]byte{
				reading(t),
				reading(t),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := newSafeWriter()
			svr, err := New(
				test.Port,
				WithLoggerOutput(w),
				WithClientOptions(
					client.WithLogReading(client.LogReading),
				),
			)
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer svr.Shutdown()
			go svr.ListenAndServe()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer conn.Close()

			for _, message := range test.LoginMessages {
				if _, err := conn.Write(message); err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
			}
			for _, message := range test.Messages {
				if _, err := conn.Write(message); err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
				time.Sleep(2500 * time.Millisecond)
			}

			isGolden(t, w.Bytes())
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
			Name:     "10 Messages",
			Port:     1337,
			Messages: messagesTen(t),
		},
		{
			Name:     "100 Messages",
			Port:     1337,
			Messages: messagesOneHundred(t),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := newSafeWriter()
			svr, err := New(
				test.Port,
				WithLoggerOutput(w),
				WithClientOptions(
					client.WithLogReading(client.LogReading),
				),
			)
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer svr.Shutdown()
			go svr.ListenAndServe()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer conn.Close()

			for _, message := range test.Messages {
				_, err := conn.Write([]byte(message))
				if err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
			}
			time.Sleep(2 * time.Second)

			isGolden(t, w.Bytes())
		})
	}
}

func TestLastReading(t *testing.T) {
	tests := []struct {
		Name     string
		Port     int
		HttpPort int
		Messages [][]byte
		Imei     int
	}{
		{
			Name:     "10 Messages, check last message",
			Port:     1337,
			HttpPort: 1338,
			Messages: messagesTen(t),
			Imei:     490154203237518,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := newSafeWriter()
			svr, err := New(
				test.Port,
				WithLoggerOutput(w),
				WithHttpServer(test.HttpPort),
			)
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			go svr.ListenAndServe()

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
			time.Sleep(time.Second)
			resp, err := http.Get(
				fmt.Sprintf(
					"http://localhost:%d/readings/%d",
					test.HttpPort,
					test.Imei))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer resp.Body.Close()

			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}

			conn.Close()
			svr.Shutdown()

			isGolden(t, b)
		})
	}
}

func TestImeiStatus(t *testing.T) {
	tests := []struct {
		Name     string
		Port     int
		HttpPort int
		Messages [][]byte
		Imei     int
		Expected int
	}{
		{
			Name:     "10 Messages, ensure IMEI status is online",
			Port:     1337,
			HttpPort: 1338,
			Messages: messagesTen(t),
			Imei:     490154203237518,
			Expected: http.StatusOK,
		},
		{
			Name:     "10 Messages, ensure IMEI status is offline",
			Port:     1337,
			HttpPort: 1338,
			Messages: messagesTen(t),
			Imei:     490224203237518,
			Expected: http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := newSafeWriter()
			svr, err := New(
				test.Port,
				WithLoggerOutput(w),
				WithHttpServer(test.HttpPort),
			)
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer svr.Shutdown()
			go svr.ListenAndServe()

			conn, err := net.Dial("tcp", ":"+strconv.Itoa(test.Port))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer conn.Close()

			for _, message := range test.Messages {
				_, err := conn.Write([]byte(message))
				if err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
			}
			time.Sleep(time.Second)
			resp, err := http.Get(
				fmt.Sprintf(
					"http://localhost:%d/status/%d",
					test.HttpPort,
					test.Imei))
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			defer resp.Body.Close()

			if test.Expected != resp.StatusCode {
				t.Errorf("unexpected Status Code, Status Code = %d", resp.StatusCode)
			}
		})
	}
}

func messagesTen(t *testing.T) [][]byte {
	f, err := os.Open("testdata/TestProcessReadings/messagesTen.json")
	if err != nil {
		t.Errorf("unexpected error = %s\n", err)
	}
	defer f.Close()
	return messages(t, f)
}

func messagesOneHundred(t *testing.T) [][]byte {
	f, err := os.Open("testdata/TestProcessReadings/messagesOneHundred.json")
	if err != nil {
		t.Errorf("unexpected error = %s\n", err)
	}
	defer f.Close()
	return messages(t, f)
}

func messages(t *testing.T, r io.Reader) [][]byte {
	var readings []client.Reading
	if err := json.NewDecoder(r).Decode(&readings); err != nil {
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

type safeWriter struct {
	sync.RWMutex
	*bytes.Buffer
}

func newSafeWriter() *safeWriter {
	return &safeWriter{
		Buffer: new(bytes.Buffer),
	}
}

func (w *safeWriter) Write(b []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	return w.Buffer.Write(b)
}

func (w *safeWriter) Bytes() []byte {
	w.RLock()
	defer w.RUnlock()
	return w.Buffer.Bytes()
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

func isGolden(t *testing.T, actual []byte) {
	file := "testdata/" + t.Name() + ".golden"
	if *golden {
		if err := ioutil.WriteFile(file, actual, 0644); err != nil {
			t.Errorf("unexpected error = %s\n", err)
		}
	}

	expected, err := ioutil.ReadFile(file)
	if err != nil {
		t.Errorf("unexpected error = %s\n", err)
	}

	if !bytes.Equal(expected, actual) {
		t.Errorf("actual != expected\nexpected = %s\nactual = %s\n", expected, actual)
	}
}
