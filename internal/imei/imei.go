// Package imei implements an IMEI decoder.
package imei

// NOTE: for more information about IMEI codes and their structure you may
// consult with:
//
// https://en.wikipedia.org/wiki/International_Mobile_Equipment_Identity.

import (
	"errors"
)

var (
	ErrInvalid  = errors.New("imei: invalid ")
	ErrChecksum = errors.New("imei: invalid checksum")
)

const (
	zero = 48
	nine = 57
)

// Decode returns the IMEI code contained in the first 15 bytes of b.
//
// In case b isn't strictly composed of digits, the returned error will be
// ErrInvalid.
//
// In case b's checksum is wrong, the returned error will be ErrChecksum.
//
// Decode does NOT allocate under any condition. Additionally, it panics if b
// isn't at least 15 bytes long.
func Decode(b []byte) (code uint64, err error) {
	if len(b) < 15 {
		panic("b invalid length")
	}

	var sum int64
	for i := range b {
		// byte is a digit
		if b[i] < zero || b[i] > nine {
			return 0, ErrInvalid
		}

		// build code
		code = (code * 10) + uint64(b[i]-zero)

		// sum for luhn digit validation
		// ignore luhn digit in sum
		if i == 14 {
			continue
		}
		if i&1 == 1 {
			if v := int64((b[i] - zero) * 2); v > 9 {
				sum += v - 9
			} else {
				sum += v
			}
		} else {
			sum += int64(b[i] - zero)
		}
	}
	luhnDigit := (10 - (sum % 10)) % 10

	// validate luhn digit
	if luhnDigit != int64(b[14]-zero) {
		return 0, ErrChecksum
	}

	return code, nil
}
