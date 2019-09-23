package client

import (
	"time"
)

type Uint64Holder struct {
	setValCh       chan uint64
	getValCh       chan uint64
	decrementValCh chan struct{}
}

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

func (h Uint64Holder) Get() uint64 {
	return <-h.getValCh
}

func (h Uint64Holder) Set(v uint64) {
	h.setValCh <- v
}

func (h Uint64Holder) Decrement() {
	h.decrementValCh <- struct{}{}
}

type TimeHolder struct {
	setValCh chan time.Time
	getValCh chan time.Time
}

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

func (h TimeHolder) Get() time.Time {
	return <-h.getValCh
}

func (h TimeHolder) Set(v time.Time) {
	h.setValCh <- v
}

type ReadingHolder struct {
	setValCh chan Reading
	getValCh chan Reading
}

func NewReadingHolder(v Reading) ReadingHolder {
	h := ReadingHolder{
		setValCh: make(chan Reading),
		getValCh: make(chan Reading),
	}
	go h.mux()
	h.Set(v)
	return h
}

func (h ReadingHolder) mux() {
	var value Reading
	for {
		select {
		case value = <-h.setValCh:
		case h.getValCh <- value:
		}
	}
}

func (h ReadingHolder) Get() Reading {
	return <-h.getValCh
}

func (h ReadingHolder) Set(v Reading) {
	h.setValCh <- v
}
