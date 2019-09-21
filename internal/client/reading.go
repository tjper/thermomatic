package client

import (
	"encoding/binary"
	"fmt"
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
func (r *Reading) Decode(b []byte) error {
	if len(b) < 40 {
		panic("invalid payload, too short")
	}

	temp := math.Float64frombits(binary.BigEndian.Uint64(b[0:8]))
	if temp < -300 || temp > 300 {
		return fmt.Errorf("invalid temperature, temp = %v", temp)
	}
	r.Temperature = temp

	alt := math.Float64frombits(binary.BigEndian.Uint64(b[8:16]))
	if alt < -20000 || alt > 20000 {
		return fmt.Errorf("invalid altitude, alt = %v", alt)
	}
	r.Altitude = alt

	lat := math.Float64frombits(binary.BigEndian.Uint64(b[16:24]))
	if lat < -90 || lat > 90 {
		return fmt.Errorf("invalid latitude, lat = %v", lat)
	}
	r.Latitude = lat

	long := math.Float64frombits(binary.BigEndian.Uint64(b[24:32]))
	if long < -180 || long > 180 {
		return fmt.Errorf("invalid longitude, long = %v", long)
	}
	r.Longitude = long

	batteryLvl := math.Float64frombits(binary.BigEndian.Uint64(b[32:40]))
	if batteryLvl < 0 || batteryLvl > 100 {
		return fmt.Errorf("invalid battery level, batteryLvl = %v", batteryLvl)
	}
	r.BatteryLevel = batteryLvl

	return nil
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

// String satisfies the fmt.Stringer interface, and returns a string
// representation of Reading.
func (r Reading) String() string {
	return fmt.Sprintf("%v,%v,%v,%v,%v",
		r.Temperature,
		r.Altitude,
		r.Latitude,
		r.Longitude,
		r.BatteryLevel)

}
