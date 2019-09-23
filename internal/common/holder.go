package common

import "time"

// Uint64Holder stores and controls access to a uint64 value.
type Uint64Holder struct {
	setValCh       chan uint64
	getValCh       chan uint64
	decrementValCh chan struct{}
}

// NewUint64Holder initializes a Uint64Holder with v.
func NewUint64Holder(v uint64) Uint64Holder {
	h := Uint64Holder{
		setValCh:       make(chan uint64),
		getValCh:       make(chan uint64),
		decrementValCh: make(chan struct{}),
	}
	go h.mux()
	h.Set(v)
	return h
}

func (h Uint64Holder) mux() {
	var value uint64
	for {
		select {
		case value = <-h.setValCh:
		case h.getValCh <- value:
		case <-h.decrementValCh:
			value--
		}
	}
}

// Get retrieves the uint64 value.
func (h Uint64Holder) Get() uint64 {
	return <-h.getValCh
}

// Set sets the uint64 value to v.
func (h Uint64Holder) Set(v uint64) {
	h.setValCh <- v
}

// Decrement decrements the uint64 value.
func (h Uint64Holder) Decrement() {
	h.decrementValCh <- struct{}{}
}

// TimeHolder stores and controls access to a time.Time value.
type TimeHolder struct {
	setValCh chan time.Time
	getValCh chan time.Time
}

// NewTimeHolder initializes a TimeHolder with v.
func NewTimeHolder(v time.Time) TimeHolder {
	h := TimeHolder{
		setValCh: make(chan time.Time),
		getValCh: make(chan time.Time),
	}
	go h.mux()
	h.Set(v)
	return h
}

func (h TimeHolder) mux() {
	var value time.Time
	for {
		select {
		case value = <-h.setValCh:
		case h.getValCh <- value:
		}
	}
}

// Get retrieves the time.Time value.
func (h TimeHolder) Get() time.Time {
	return <-h.getValCh
}

// Set sets the time.Time value to v.
func (h TimeHolder) Set(v time.Time) {
	h.setValCh <- v
}
