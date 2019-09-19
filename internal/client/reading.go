package client

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
)

// Reading is the set of device readings.
type Reading struct {
	// Temperature denotes the temperature reading of the message.
	Temperature float64

	// Altitude denotes the altitude reading of the message.
	Altitude float64

	// Latitude denotes the latitude reading of the message.
	Latitude float64

	// Longitude denotes the longitude reading of the message.
	Longitude float64

	// BatteryLevel denotes the battery level reading of the message.
	BatteryLevel float64
}

// Decode decodes the reading message payload in the given b into r.
//
// If any of the fields are outside their valid min/max ranges ok will be unset.
//
// Decode does NOT allocate under any condition. Additionally, it panics if b
// isn't at least 40 bytes long.
// TODO: add min and max checks
// TODO: remove existing allocations
func (r *Reading) Decode(b []byte) (ok bool) {
	if len(b) < 40 {
		panic("invalid payload, too short")
	}

	var (
		rdr = bytes.NewReader(b)
		buf = make([]byte, 8)
	)
	for i := 0; i < 5; i++ {
		_, err := io.ReadFull(rdr, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		switch val := math.Float64frombits(binary.BigEndian.Uint64(buf)); i {
		case 0:
			r.Temperature = val
		case 1:
			r.Altitude = val
		case 2:
			r.Latitude = val
		case 3:
			r.Longitude = val
		case 4:
			r.BatteryLevel = val
		}
	}
	return true
}

// Encode encodes r into a slice of Big-Endian IEEE 754 binary representations.
// Each field is stored in sub slice 8 bytes wide. The resulting encoded bytes
// are returned.
func (r Reading) Encode() ([]byte, error) {
	var (
		b     = make([]byte, 0, 40)
		field = make([]byte, 8)
	)
	for i := 0; i < 5; i++ {
		switch i {
		case 0:
			binary.BigEndian.PutUint64(field, math.Float64bits(r.Temperature))
		case 1:
			binary.BigEndian.PutUint64(field, math.Float64bits(r.Altitude))
		case 2:
			binary.BigEndian.PutUint64(field, math.Float64bits(r.Latitude))
		case 3:
			binary.BigEndian.PutUint64(field, math.Float64bits(r.Longitude))
		case 4:
			binary.BigEndian.PutUint64(field, math.Float64bits(r.BatteryLevel))
		}
		b = append(b, field...)
	}
	return b, nil
}
