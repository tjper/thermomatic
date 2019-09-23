package client

// ReadingHolder stores and controls access to a Reading value.
type ReadingHolder struct {
	setValCh chan Reading
	getValCh chan Reading
}

// NewReadingHolder initializes a ReadingHolder with v.
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

// Get retrieves the Reading value.
func (h ReadingHolder) Get() Reading {
	return <-h.getValCh
}

// Set sets the Reading value to v.
func (h ReadingHolder) Set(v Reading) {
	h.setValCh <- v
}
