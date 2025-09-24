package wshub

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// newMockClientForMap creates a simple mock client for clientsMap testing.
func newMockClientForMap(id uuid.UUID) *Client {
	// Create a minimal client with just the ID for map testing
	client := &Client{}
	client.id = id // Access private field for testing

	return client
}

func TestNewClientsMap(t *testing.T) {
	cm := newClientsMap()

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.clients)
	assert.Equal(t, 0, cm.Count())
}

func TestClientsMap_Add(t *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()

	cm := newClientsMap()
	client1 := newMockClientForMap(client1ID)
	client2 := newMockClientForMap(client2ID)

	cm.Add(client1)
	assert.Equal(t, 1, cm.Count())

	cm.Add(client2)
	assert.Equal(t, 2, cm.Count())

	// Test adding the same client overwrites
	client1Updated := newMockClientForMap(client1ID)
	cm.Add(client1Updated)
	assert.Equal(t, 2, cm.Count())
}

func TestClientsMap_Get(t *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()
	nonExistentID := uuid.New()

	cm := newClientsMap()
	client1 := newMockClientForMap(client1ID)
	client2 := newMockClientForMap(client2ID)

	cm.Add(client1)
	cm.Add(client2)

	// Test getting existing clients
	retrieved1 := cm.Get(client1ID)
	assert.Equal(t, client1, retrieved1)
	assert.Equal(t, client1ID, retrieved1.id)

	retrieved2 := cm.Get(client2ID)
	assert.Equal(t, client2, retrieved2)
	assert.Equal(t, client2ID, retrieved2.id)

	// Test getting non-existent client
	retrieved3 := cm.Get(nonExistentID)
	assert.Nil(t, retrieved3)
}

func TestClientsMap_Remove(t *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()
	nonExistentID := uuid.New()

	cm := newClientsMap()
	client1 := newMockClientForMap(client1ID)
	client2 := newMockClientForMap(client2ID)

	cm.Add(client1)
	cm.Add(client2)
	assert.Equal(t, 2, cm.Count())

	// Test removing existing client
	removed := cm.Remove(client1ID)
	assert.Equal(t, client1, removed)
	assert.Equal(t, 1, cm.Count())
	assert.Nil(t, cm.Get(client1ID))
	assert.NotNil(t, cm.Get(client2ID))

	// Test removing non-existent client
	removed2 := cm.Remove(nonExistentID)
	assert.Nil(t, removed2)
	assert.Equal(t, 1, cm.Count())

	// Test removing last client
	removed3 := cm.Remove(client2ID)
	assert.Equal(t, client2, removed3)
	assert.Equal(t, 0, cm.Count())
}

func TestClientsMap_GetAll(t *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()
	client3ID := uuid.New()

	cm := newClientsMap()
	client1 := newMockClientForMap(client1ID)
	client2 := newMockClientForMap(client2ID)
	client3 := newMockClientForMap(client3ID)

	cm.Add(client1)
	cm.Add(client2)
	cm.Add(client3)

	all := cm.GetAll()
	assert.Len(t, all, 3)
	assert.Equal(t, client1, all[client1ID])
	assert.Equal(t, client2, all[client2ID])
	assert.Equal(t, client3, all[client3ID])

	// Test that GetAll returns a copy, not reference
	delete(all, client1ID)
	assert.Equal(t, 3, cm.Count()) // Original should be unchanged
	assert.NotNil(t, cm.Get(client1ID))

	// Test empty map
	cm2 := newClientsMap()
	empty := cm2.GetAll()
	assert.NotNil(t, empty)
	assert.Len(t, empty, 0)
}

func TestClientsMap_Count(t *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()

	cm := newClientsMap()
	assert.Equal(t, 0, cm.Count())

	client1 := newMockClientForMap(client1ID)
	cm.Add(client1)
	assert.Equal(t, 1, cm.Count())

	client2 := newMockClientForMap(client2ID)
	cm.Add(client2)
	assert.Equal(t, 2, cm.Count())

	cm.Remove(client1ID)
	assert.Equal(t, 1, cm.Count())

	cm.Remove(client2ID)
	assert.Equal(t, 0, cm.Count())
}

func TestClientsMap_ThreadSafety(t *testing.T) {
	cm := newClientsMap()

	var wg sync.WaitGroup

	numGoroutines := 100
	clientsPerGoroutine := 10

	// Store client IDs so we can remove them later
	var (
		clientIDs      []uuid.UUID
		clientIDsMutex sync.Mutex
	)

	// Concurrent adds

	for i := range numGoroutines {
		wg.Add(1)

		go func(_ int) {
			defer wg.Done()

			for range clientsPerGoroutine {
				clientID := uuid.New()
				client := newMockClientForMap(clientID)
				cm.Add(client)

				// Store the client ID for later removal
				clientIDsMutex.Lock()

				clientIDs = append(clientIDs, clientID)

				clientIDsMutex.Unlock()
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
		}()
	}

	wg.Wait()

	// Should have exactly numGoroutines * clientsPerGoroutine clients
	expectedCount := numGoroutines * clientsPerGoroutine
	assert.Equal(t, expectedCount, cm.Count())
	assert.Len(t, clientIDs, expectedCount)

	// Concurrent removes using the actual client IDs
	wg = sync.WaitGroup{}
	for i := range numGoroutines {
		wg.Add(1)

		go func(goroutineID int) {
			defer wg.Done()

			for j := range clientsPerGoroutine {
				// Calculate the index for this goroutine's clients
				idx := goroutineID*clientsPerGoroutine + j
				if idx < len(clientIDs) {
					cm.Remove(clientIDs[idx])
				}
			}
		}(i)
	}

	wg.Wait()

	// Should be empty after all removes
	assert.Equal(t, 0, cm.Count())
}

func TestClientsMap_ConcurrentAddRemove(t *testing.T) {
	cm := newClientsMap()

	var wg sync.WaitGroup

	numOperations := 1000

	// Mix of concurrent adds and removes on the same client IDs
	for i := range numOperations {
		wg.Add(2) // One for add, one for remove

		go func(_ int) {
			defer wg.Done()

			clientID := uuid.New() // Use random UUID
			client := newMockClientForMap(clientID)
			cm.Add(client)
		}(i)

		go func(_ int) {
			defer wg.Done()

			clientID := uuid.New() // Use random UUID
			cm.Remove(clientID)
		}(i)
	}

	wg.Wait()

	// Final state should be consistent (no panics, no race conditions)
	count := cm.Count()
	all := cm.GetAll()
	assert.Equal(t, count, len(all))

	// All remaining clients should be retrievable
	for clientID := range all {
		retrieved := cm.Get(clientID)
		assert.NotNil(t, retrieved)
		assert.Equal(t, clientID, retrieved.id)
	}
}

func TestClientsMap_GetOrAdd(t *testing.T) {
	cm := newClientsMap()
	clientID := uuid.New()
	client := newMockClientForMap(clientID)

	// Test adding a new client
	result, added := cm.GetOrAdd(client)
	assert.True(t, added)
	assert.Equal(t, client, result)
	assert.Equal(t, clientID, result.ID())

	// Test getting an existing client (should not add)
	sameClient := newMockClientForMap(clientID) // Different pointer, same ID
	result2, added2 := cm.GetOrAdd(sameClient)
	assert.False(t, added2)
	assert.Equal(t, client, result2) // Should return original client
	// Should NOT return the new one (different pointers)
	assert.NotSame(t, sameClient, result2)

	// Verify only one client in map
	all := cm.GetAll()
	assert.Equal(t, 1, len(all))
}
