package websocket

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Tests for connectionsMap

func TestNewConnectionsMap(t *testing.T) {
	cm := newConnectionsMap()

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.conns)
	assert.Equal(t, 0, cm.Count())
	assert.True(t, cm.IsEmpty())
}

func TestConnectionsMap_Add(t *testing.T) {
	// Generate UUIDs for this test
	conn1ID := uuid.New()
	conn2ID := uuid.New()

	cm := newConnectionsMap()
	client := &Client{id: uuid.New()}
	conn1 := newMockConnection(conn1ID, client)
	conn2 := newMockConnection(conn2ID, client)

	cm.Add(conn1)
	assert.Equal(t, 1, cm.Count())
	assert.False(t, cm.IsEmpty())

	cm.Add(conn2)
	assert.Equal(t, 2, cm.Count())

	// Test adding the same connection overwrites
	conn1Updated := newMockConnection(conn1ID, client)
	cm.Add(conn1Updated)
	assert.Equal(t, 2, cm.Count())
}

func TestConnectionsMap_Get(t *testing.T) {
	// Generate UUIDs for this test
	conn1ID := uuid.New()
	conn2ID := uuid.New()
	nonExistentID := uuid.New()

	cm := newConnectionsMap()
	client := &Client{id: uuid.New()}
	conn1 := newMockConnection(conn1ID, client)
	conn2 := newMockConnection(conn2ID, client)

	cm.Add(conn1)
	cm.Add(conn2)

	// Test getting existing connections
	retrieved1 := cm.Get(conn1ID)
	assert.Equal(t, conn1, retrieved1)
	assert.Equal(t, conn1ID, retrieved1.id)

	retrieved2 := cm.Get(conn2ID)
	assert.Equal(t, conn2, retrieved2)
	assert.Equal(t, conn2ID, retrieved2.id)

	// Test getting non-existent connection
	retrieved3 := cm.Get(nonExistentID)
	assert.Nil(t, retrieved3)
}

func TestConnectionsMap_Remove(t *testing.T) {
	// Generate UUIDs for this test
	conn1ID := uuid.New()
	conn2ID := uuid.New()
	nonExistentID := uuid.New()

	cm := newConnectionsMap()
	client := &Client{id: uuid.New()}
	conn1 := newMockConnection(conn1ID, client)
	conn2 := newMockConnection(conn2ID, client)

	cm.Add(conn1)
	cm.Add(conn2)
	assert.Equal(t, 2, cm.Count())

	// Test removing existing connection
	removed := cm.Remove(conn1ID)
	assert.Equal(t, conn1, removed)
	assert.Equal(t, 1, cm.Count())
	assert.Nil(t, cm.Get(conn1ID))
	assert.NotNil(t, cm.Get(conn2ID))

	// Test removing non-existent connection
	removed2 := cm.Remove(nonExistentID)
	assert.Nil(t, removed2)
	assert.Equal(t, 1, cm.Count())

	// Test removing last connection
	removed3 := cm.Remove(conn2ID)
	assert.Equal(t, conn2, removed3)
	assert.Equal(t, 0, cm.Count())
	assert.True(t, cm.IsEmpty())
}

func TestConnectionsMap_GetAll(t *testing.T) {
	// Generate UUIDs for this test
	conn1ID := uuid.New()
	conn2ID := uuid.New()
	conn3ID := uuid.New()

	cm := newConnectionsMap()
	client := &Client{id: uuid.New()}
	conn1 := newMockConnection(conn1ID, client)
	conn2 := newMockConnection(conn2ID, client)
	conn3 := newMockConnection(conn3ID, client)

	cm.Add(conn1)
	cm.Add(conn2)
	cm.Add(conn3)

	all := cm.GetAll()
	assert.Len(t, all, 3)
	assert.Equal(t, conn1, all[conn1ID])
	assert.Equal(t, conn2, all[conn2ID])
	assert.Equal(t, conn3, all[conn3ID])

	// Test that GetAll returns a copy, not reference
	delete(all, conn1ID)
	assert.Equal(t, 3, cm.Count()) // Original should be unchanged
	assert.NotNil(t, cm.Get(conn1ID))

	// Test empty map
	cm2 := newConnectionsMap()
	empty := cm2.GetAll()
	assert.NotNil(t, empty)
	assert.Len(t, empty, 0)
}

func TestConnectionsMap_Count_And_IsEmpty(t *testing.T) {
	// Generate UUIDs for this test
	conn1ID := uuid.New()
	conn2ID := uuid.New()

	cm := newConnectionsMap()
	assert.Equal(t, 0, cm.Count())
	assert.True(t, cm.IsEmpty())

	client := &Client{id: uuid.New()}
	conn1 := newMockConnection(conn1ID, client)
	cm.Add(conn1)
	assert.Equal(t, 1, cm.Count())
	assert.False(t, cm.IsEmpty())

	conn2 := newMockConnection(conn2ID, client)
	cm.Add(conn2)
	assert.Equal(t, 2, cm.Count())
	assert.False(t, cm.IsEmpty())

	cm.Remove(conn1ID)
	assert.Equal(t, 1, cm.Count())
	assert.False(t, cm.IsEmpty())

	cm.Remove(conn2ID)
	assert.Equal(t, 0, cm.Count())
	assert.True(t, cm.IsEmpty())
}

func TestConnectionsMap_ThreadSafety(t *testing.T) {
	cm := newConnectionsMap()
	var wg sync.WaitGroup
	numGoroutines := 100
	connsPerGoroutine := 10

	client := &Client{id: uuid.New()}

	// Store connection IDs so we can remove them later
	var connIDs []uuid.UUID
	var connIDsMutex sync.Mutex

	// Concurrent adds
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for range connsPerGoroutine {
				connID := uuid.New()
				conn := newMockConnection(connID, client)
				cm.Add(conn)

				// Store the connection ID for later removal
				connIDsMutex.Lock()
				connIDs = append(connIDs, connID)
				connIDsMutex.Unlock()
			}
		}(i)
	}

	// Concurrent reads
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.Count()
			cm.GetAll()
			cm.Get(uuid.New()) // Just test with random UUID
			cm.IsEmpty()
		}()
	}

	wg.Wait()

	// Should have exactly numGoroutines * connsPerGoroutine connections
	expectedCount := numGoroutines * connsPerGoroutine
	assert.Equal(t, expectedCount, cm.Count())
	assert.False(t, cm.IsEmpty())
	assert.Len(t, connIDs, expectedCount)

	// Concurrent removes using the actual connection IDs
	wg = sync.WaitGroup{}
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := range connsPerGoroutine {
				// Calculate the index for this goroutine's connections
				idx := goroutineID*connsPerGoroutine + j
				if idx < len(connIDs) {
					cm.Remove(connIDs[idx])
				}
			}
		}(i)
	}

	wg.Wait()

	// Should be empty after all removes
	assert.Equal(t, 0, cm.Count())
	assert.True(t, cm.IsEmpty())
}

func TestConnectionsMap_ConcurrentAddRemove(t *testing.T) {
	cm := newConnectionsMap()
	var wg sync.WaitGroup
	numOperations := 1000
	client := &Client{id: uuid.New()}

	// Mix of concurrent adds and removes on random connection IDs
	for i := range numOperations {
		wg.Add(2) // One for add, one for remove

		go func(id int) {
			defer wg.Done()
			connID := uuid.New() // Use random UUID
			conn := newMockConnection(connID, client)
			cm.Add(conn)
		}(i)

		go func(id int) {
			defer wg.Done()
			connID := uuid.New() // Use random UUID
			cm.Remove(connID)
		}(i)
	}

	wg.Wait()

	// Final state should be consistent (no panics, no race conditions)
	count := cm.Count()
	all := cm.GetAll()
	assert.Equal(t, count, len(all))

	// All remaining connections should be retrievable
	for connID := range all {
		retrieved := cm.Get(connID)
		assert.NotNil(t, retrieved)
		assert.Equal(t, connID, retrieved.id)
	}
}
