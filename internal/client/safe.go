package client

import (
	"sync"
	"time"
)

type safeUint64 struct {
	sync.RWMutex
	val uint64
}

func (u *safeUint64) get() uint64 {
	u.RLock()
	val := u.val
	u.RUnlock()
	return val
}

func (u *safeUint64) set(val uint64) {
	u.Lock()
	u.val = val
	u.Unlock()
}

func (u *safeUint64) decrement() {
	u.Lock()
	u.val--
	u.Unlock()
}

type safeTime struct {
	sync.RWMutex
	val time.Time
}

func (t *safeTime) get() time.Time {
	t.RLock()
	val := t.val
	t.RUnlock()
	return val
}

func (t *safeTime) set(val time.Time) {
	t.Lock()
	t.val = val
	t.Unlock()
}

type safeReading struct {
	sync.RWMutex
	val Reading
}

func (r *safeReading) get() Reading {
	r.RLock()
	val := r.val
	r.RUnlock()
	return val
}

func (r *safeReading) set(reading Reading) {
	r.Lock()
	r.val = reading
	r.Unlock()
}
