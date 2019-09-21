package imei

import (
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		Name     string
		Imei     []byte
		Expected uint64
	}{
		{
			Name:     "happy path",
			Imei:     []byte("490154203237518"),
			Expected: 490154203237518,
		},
		{
			Name:     "luhn digit is 0",
			Imei:     []byte("355041000729140"),
			Expected: 355041000729140,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			actual, err := Decode(test.Imei)
			if err != nil {
				t.Fatalf("unexpected error = %s\n", err)
			}
			if actual != test.Expected {
				t.Fatalf(
					"expected != actual\nexpected = %v\nactual = %v\n",
					test.Expected,
					actual)
			}
		})
	}
}

func TestDecodeAllocations(t *testing.T) {
	tests := []struct {
		Name     string
		Imei     []byte
		Expected uint64
	}{
		{
			Name:     "happy path",
			Imei:     []byte("490154203237518"),
			Expected: 490154203237518,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			avg := testing.AllocsPerRun(1000, func() {
				if _, err := Decode(test.Imei); err != nil {
					t.Fatalf("unexpected error = %s\n", err)
				}
			})
			if avg > 0 {
				t.Errorf("expected avg # of allocations to be 0, avg = %v", avg)
			}
		})
	}
}

func TestDecodePanics(t *testing.T) {
	tests := []struct {
		Name     string
		Imei     []byte
		Expected error
	}{
		{
			Name:     "invalid length",
			Imei:     []byte("3550410729140"),
			Expected: ErrInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("expected panic")
				}
			}()
			_, _ = Decode(test.Imei)
		})
	}
}

var actual uint64

func benchmarkDecode(b *testing.B, imei []byte) {
	var a uint64
	for i := 0; i < b.N; i++ {
		a, _ = Decode([]byte("490154203237518"))
	}
	actual = a
}

func BenchmarkDecode1(b *testing.B) { benchmarkDecode(b, []byte("490154203237518")) }
func BenchmarkDecode2(b *testing.B) { benchmarkDecode(b, []byte("355041000729140")) }
