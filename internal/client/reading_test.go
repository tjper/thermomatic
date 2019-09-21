package client_test

import (
	"bytes"
	"testing"

	"github.com/tjper/thermomatic/internal/client"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		Name    string
		Reading client.Reading
	}{
		{
			Name: "happy path",
			Reading: client.Reading{
				Temperature:  67.77,
				Altitude:     2.63555,
				Latitude:     33.41,
				Longitude:    44.4,
				BatteryLevel: 0.25666,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			b, err := test.Reading.Encode()
			if err != nil {
				t.Errorf("unexpected error = %s", err)
			}
			reading := client.Reading{}
			if err := reading.Decode(b); err != nil {
				t.Errorf("unexpected error = %s", err)
			}

			expected := b
			actual, err := reading.Encode()
			if err != nil {
				t.Errorf("unexpected error = %s", err)
			}
			if !bytes.Equal(expected, actual) {
				t.Errorf(
					"expected = %v\nactual = %v\n",
					test.Reading,
					reading)
			}
		})
	}
}

func TestDecodeAllocations(t *testing.T) {
	tests := []struct {
		Name    string
		Reading client.Reading
	}{
		{
			Name: "happy path",
			Reading: client.Reading{
				Temperature:  67.77,
				Altitude:     2.63555,
				Latitude:     33.41,
				Longitude:    44.4,
				BatteryLevel: 0.25666,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			b, err := test.Reading.Encode()
			if err != nil {
				t.Errorf("unexpected error = %s\n", err)
			}
			avg := testing.AllocsPerRun(1000, func() {
				if err := test.Reading.Decode(b); err != nil {
					t.Errorf("unexpected error = %s\n", err)
				}
			})
			if avg > 0 {
				t.Errorf("expected avg # of allocations to be 0, avg = %v", avg)
			}

		})
	}
}

var reading client.Reading

func benchmarkDecode(b *testing.B, buf []byte) {
	var r client.Reading
	for i := 0; i < b.N; i++ {
		r.Decode(buf)
	}
	reading = r
}

func BenchmarkDecode1(b *testing.B) {
	r := client.Reading{
		Temperature:  67.77,
		Altitude:     2.63555,
		Latitude:     33.41,
		Longitude:    44.4,
		BatteryLevel: 0.25666,
	}
	buf, err := r.Encode()
	if err != nil {
		b.Errorf("unexpected error = %s\n", err)
	}

	b.ResetTimer()
	benchmarkDecode(b, buf)
}
