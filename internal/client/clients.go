package client

import "sync"

// ClientMap is a concurrent safe map. Keys are the IMEI for a client, and the
// stored value is a Client object.
type ClientMap struct {
	sync.RWMutex
	m map[uint64]Client
}

// NewClientMap initializes a ClientMap object
func NewClientMap() *ClientMap {
	return &ClientMap{
		m: make(map[uint64]Client),
	}
}

// Load retrieves the existence of the key, and Client if it exist from the
// ClientMap.
func (m *ClientMap) Load(imei uint64) (Client, bool) {
	m.RLock()
	client, ok := m.m[imei]
	m.RUnlock()
	return client, ok
}

// Store stores a key-value pair in the ClientMap.
func (m *ClientMap) Store(key uint64, client Client) {
	m.Lock()
	m.m[key] = client
	m.Unlock()
}

// Delete deletes a key-value pair from the ClientMap.
func (m *ClientMap) Delete(key uint64) {
	m.Lock()
	delete(m.m, key)
	m.Unlock()
}

// Range ranges over the ClientMap and calls f for each key-value pair. If f
// returns false, range stops the iteration.
func (m *ClientMap) Range(f func(uint64, Client) bool) {
	m.RLock()
	for imei, client := range m.m {
		if !f(imei, client) {
			break
		}
	}
	m.RUnlock()
}
