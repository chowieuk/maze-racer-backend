package main

import "sync"

// CMap is intended to be a thread-safe map implementation.
// Implementations must ensure all methods are safe for mutex access.
type CMap[K comparable, V any] interface {
	// sync.Locker // constrain to implementations using Lock() and Unlock()
	Set(key K, value V)
	Del(key K)
	Get(key K) (V, bool)
	Values() []V
	Keys() []K
	Reset()
}

// mutexMap uses a sync RW Mutex to ensure thread safe read / writes
type mutexMap[K comparable, V any] struct {
	sync.RWMutex
	data map[K]V
}

// NewMutexMap creates a new mutex map
func NewMutexMap[K comparable, V any]() CMap[K, V] {
	return &mutexMap[K, V]{
		data: make(map[K]V),
	}
}

// Set adds or updates a key-value pair
func (m *mutexMap[K, V]) Set(key K, value V) {
	m.Lock()
	defer m.Unlock()
	m.data[key] = value
}

// Delete removes a key-value pair
func (m *mutexMap[K, V]) Del(key K) {
	m.Lock()
	defer m.Unlock()
	delete(m.data, key)
}

// Get retrieves a value by key
func (m *mutexMap[K, V]) Get(key K) (V, bool) {
	m.RLock()
	defer m.RUnlock()
	val, exists := m.data[key]
	return val, exists
}

// Values returns a slice of all values
func (m *mutexMap[K, V]) Values() []V {
	m.RLock()
	defer m.RUnlock()
	values := make([]V, 0, len(m.data))
	for _, v := range m.data {
		values = append(values, v)
	}
	return values
}

// Keys returns a slice of all keys
func (m *mutexMap[K, V]) Keys() []K {
	m.RLock()
	defer m.RUnlock()
	keys := make([]K, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

// Clear deletes all key value pairs in the cmap
func (m *mutexMap[K, V]) Reset() {
	for _, k := range m.Keys() {
		m.Del(k)
	}
}
