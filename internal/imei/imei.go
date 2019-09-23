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
	zero   = 48
	nine   = 57
	length = 15
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
	if len(b) < length {
		panic("b invalid length")
	}

	var sum uint64
	for i := 0; i < length; i++ {
		// byte is a digit
		digit := uint64(b[i] - zero)

		if digit > 9 {
			return 0, ErrInvalid
		}

		// build code
		code = (code * 10) + digit

		// sum for luhn digit validation
		// ignore luhn digit in sum
		if i == 14 {
			continue
		}
		if i&1 == 1 {
			if v := digit * 2; v > 9 {
				sum += v - 9
			} else {
				sum += v
			}
		} else {
			sum += digit
		}
	}
	luhnDigit := (10 - (sum % 10)) % 10

	// validate luhn digit
	if luhnDigit != uint64(b[14]-zero) {
		return 0, ErrChecksum
	}

	return code, nil
}
