package commontypes

import (
	"sync"
)

type Map[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		data: make(map[K]V),
	}
}

func (m *Map[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
}

func (m *Map[K, V]) Get(key K) (V, bool) { //nolint:ireturn
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.data[key]

	return value, exists
}

func (m *Map[K, V]) Delete(key K) (V, bool) { //nolint:ireturn
	m.mu.Lock()
	defer m.mu.Unlock()

	value, exists := m.data[key]
	if exists {
		delete(m.data, key)
	}

	return value, exists
}

func (m *Map[K, V]) Clear() map[K]V {
	m.mu.Lock()
	defer m.mu.Unlock()

	old := m.data
	m.data = make(map[K]V)

	return old
}

func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.data)
}

func (m *Map[K, V]) Has(key K) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[key]

	return exists
}

func (m *Map[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]K, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}

	return keys
}

func (m *Map[K, V]) Values() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()

	values := make([]V, 0, len(m.data))
	for _, v := range m.data {
		values = append(values, v)
	}

	return values
}

// Range iterates over all entries under a read lock.
// Return false from the callback to stop iteration.
func (m *Map[K, V]) Range(fn func(K, V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for k, v := range m.data {
		if !fn(k, v) {
			return
		}
	}
}

// DeleteFunc removes all entries for which the callback returns true.
// Returns the number of deleted entries.
func (m *Map[K, V]) DeleteFunc(fn func(K, V) bool) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	deleted := 0

	for k, v := range m.data {
		if fn(k, v) {
			delete(m.data, k)

			deleted++
		}
	}

	return deleted
}
