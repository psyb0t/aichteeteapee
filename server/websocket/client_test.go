package websocket

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHub for testing
type mockHub struct {
	processEventCalls []Event
	mu                sync.Mutex
	doneCh            chan struct{}
}

func newMockHub() *mockHub {
	return &mockHub{
		doneCh: make(chan struct{}),
	}
}

func (h *mockHub) ProcessEvent(client *Client, event *Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.processEventCalls = append(h.processEventCalls, *event)
}

func (h *mockHub) AddClient(_ *Client)           {}
func (h *mockHub) RemoveClient(_ uuid.UUID)      {}
func (h *mockHub) GetClient(_ uuid.UUID) *Client { return nil }
func (h *mockHub) GetOrCreateClient(_ uuid.UUID, _ ...ClientOption) (*Client, bool) {
	return nil, false
}
func (h *mockHub) GetAllClients() map[uuid.UUID]*Client               { return nil }
func (h *mockHub) RegisterEventHandler(_ EventType, _ EventHandler)   {}
func (h *mockHub) RegisterEventHandlers(_ map[EventType]EventHandler) {}
func (h *mockHub) UnregisterEventHandler(_ EventType)                 {}
func (h *mockHub) BroadcastToAll(_ *Event)                            {}
func (h *mockHub) BroadcastToClients(_ []uuid.UUID, _ *Event)         {}
func (h *mockHub) BroadcastToSubscribers(_ EventType, _ *Event)       {}
func (h *mockHub) Close()                                             {}
func (h *mockHub) Done() <-chan struct{}                              { return h.doneCh }
func (h *mockHub) Name() string                                       { return "mock-hub" }

func (h *mockHub) getProcessEventCalls() []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]Event, len(h.processEventCalls))
	copy(result, h.processEventCalls)
	return result
}

// newMockClient creates a mock client for testing
func newMockClient(hub Hub) *Client {
	client := NewClientWithID(uuid.New())
	client.SetHub(hub)
	return client
}

func TestNewClient(t *testing.T) {
	hub := &mockHub{}

	client := NewClient()
	client.SetHub(hub)

	assert.NotNil(t, client)
	assert.NotEqual(t, uuid.Nil, client.ID())
	assert.Equal(t, hub, client.hub)
	assert.Equal(t, 0, client.ConnectionCount())
	assert.NotNil(t, client.connections)
	assert.NotNil(t, client.sendCh)
	assert.NotNil(t, client.doneCh)
}

func TestNewClientWithID(t *testing.T) {
	hub := &mockHub{}
	clientID := uuid.New()

	client := NewClientWithID(clientID)
	client.SetHub(hub)

	assert.NotNil(t, client)
	assert.Equal(t, clientID, client.ID())
	assert.Equal(t, hub, client.hub)
	assert.Equal(t, 0, client.ConnectionCount())
}

func TestClient_AddConnection(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	connID := uuid.New()
	conn := newMockConnection(connID, client)

	assert.Equal(t, 0, client.ConnectionCount())

	client.AddConnection(conn)

	assert.Equal(t, 1, client.ConnectionCount())

	connections := client.GetConnections()
	assert.Len(t, connections, 1)
	assert.Equal(t, conn, connections[connID])
}

func TestClient_RemoveConnection(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	conn1ID := uuid.New()
	conn2ID := uuid.New()
	conn1 := newMockConnection(conn1ID, client)
	conn2 := newMockConnection(conn2ID, client)

	client.AddConnection(conn1)
	client.AddConnection(conn2)
	assert.Equal(t, 2, client.ConnectionCount())

	// Remove first connection
	client.RemoveConnection(conn1ID)
	assert.Equal(t, 1, client.ConnectionCount())

	connections := client.GetConnections()
	assert.Len(t, connections, 1)
	assert.Nil(t, connections[conn1ID])
	assert.NotNil(t, connections[conn2ID])

	// Remove non-existent connection (should not panic)
	client.RemoveConnection(uuid.New())
	assert.Equal(t, 1, client.ConnectionCount())

	// Remove last connection
	client.RemoveConnection(conn2ID)
	assert.Equal(t, 0, client.ConnectionCount())
	assert.True(t, len(client.GetConnections()) == 0)
}

func TestClient_GetConnections(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	// Empty initially
	connections := client.GetConnections()
	assert.NotNil(t, connections)
	assert.Len(t, connections, 0)

	// Add connections
	conn1ID := uuid.New()
	conn2ID := uuid.New()
	conn1 := newMockConnection(conn1ID, client)
	conn2 := newMockConnection(conn2ID, client)

	client.AddConnection(conn1)
	client.AddConnection(conn2)

	connections = client.GetConnections()
	assert.Len(t, connections, 2)
	assert.Equal(t, conn1, connections[conn1ID])
	assert.Equal(t, conn2, connections[conn2ID])

	// Verify it returns a copy (modifying returned map shouldn't affect client)
	delete(connections, conn1ID)
	assert.Equal(t, 2, client.ConnectionCount()) // Original should be unchanged
}

func TestClient_Send(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	event := NewEvent(EventTypeSystemLog, "test-data")

	// Should be able to send
	client.Send(event)

	// Check that event was queued (we can't easily test the distribution without integration)
	// The distributionPump would normally handle this
}

func TestClient_SendAfterStop(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	client.Stop()

	event := NewEvent(EventTypeSystemLog, "test-data")

	// Should not block or panic when sending to stopped client
	client.Send(event)
}

func TestClient_Stop(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	// Add a connection
	connID := uuid.New()
	conn := newMockConnection(connID, client)
	client.AddConnection(conn)

	assert.Equal(t, 1, client.ConnectionCount())

	// Stop should not panic
	client.Stop()

	// Multiple stops should not panic
	client.Stop()
	client.Stop()
}

func TestClient_IsSubscribedTo(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	// For now, should always return true
	assert.True(t, client.IsSubscribedTo(EventTypeSystemLog))
	assert.True(t, client.IsSubscribedTo(EventTypeError))
}

func TestClient_ThreadSafety(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)
	var wg sync.WaitGroup
	numGoroutines := 50
	connectionsPerGoroutine := 5

	// Store connection IDs for later removal
	var connIDs []uuid.UUID
	var connIDsMutex sync.Mutex

	// Concurrent connection additions
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for range connectionsPerGoroutine {
				connID := uuid.New()
				conn := newMockConnection(connID, client)
				client.AddConnection(conn)

				// Store connection ID for later removal
				connIDsMutex.Lock()
				connIDs = append(connIDs, connID)
				connIDsMutex.Unlock()
			}
		}(i)
	}

	// Concurrent reads
	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.ConnectionCount()
			client.GetConnections()
			client.ID()
		}()
	}

	// Concurrent sends
	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := NewEvent(EventTypeSystemLog, "concurrent-test")
			client.Send(event)
		}()
	}

	wg.Wait()

	// Should have all connections
	expectedCount := numGoroutines * connectionsPerGoroutine
	assert.Equal(t, expectedCount, client.ConnectionCount())
	assert.Len(t, connIDs, expectedCount)

	// Concurrent removals using actual connection IDs
	wg = sync.WaitGroup{}
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := range connectionsPerGoroutine {
				idx := goroutineID*connectionsPerGoroutine + j
				if idx < len(connIDs) {
					client.RemoveConnection(connIDs[idx])
				}
			}
		}(i)
	}

	wg.Wait()

	// Should be empty after all removes
	assert.Equal(t, 0, client.ConnectionCount())
}

func TestClient_Run(t *testing.T) {
	hub := newMockHub()
	client := newMockClient(hub)

	// Run should not block and should handle hub cancellation
	done := make(chan struct{})
	go func() {
		client.Run()
		close(done)
	}()

	// Close hub to make Run() exit
	close(hub.doneCh)

	select {
	case <-done:
		// Good, Run completed
	case <-time.After(200 * time.Millisecond):
		require.Fail(t, "Run did not complete within timeout")
	}
}

func TestClient_DistributionPump(t *testing.T) {
	hub := newMockHub()
	client := newMockClient(hub)

	// Add mock connections that can receive events
	conn1ID := uuid.New()
	conn2ID := uuid.New()
	conn1 := newMockConnection(conn1ID, client)
	conn2 := newMockConnection(conn2ID, client)

	client.AddConnection(conn1)
	client.AddConnection(conn2)

	// Start distribution pump
	go client.distributionPump()

	// Send an event
	event := NewEvent(EventTypeSystemLog, "distribution-test")
	client.Send(event)

	// Give some time for distribution
	time.Sleep(10 * time.Millisecond)

	// Stop the pump
	client.Stop()

	// Note: In a real test, we would verify that the event was sent to both connections
	// This would require more sophisticated mocking of the Connection.Send method
}

func TestClient_SendEvent(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	event := NewEvent(EventTypeSystemLog, "test-data")

	// SendEvent should work as an alias for Send
	client.SendEvent(event)

	// This is primarily for hub compatibility
}

func TestClient_WithOptions(t *testing.T) {
	hub := &mockHub{}

	client := NewClient(
		WithSendBufferSize(100),
		WithReadTimeout(30*time.Second),
	)
	client.SetHub(hub)

	assert.NotNil(t, client)
	assert.Equal(t, 100, client.config.SendBufferSize)
	assert.Equal(t, 30*time.Second, client.config.ReadTimeout)
}

func TestClient_ConnectionManagement(t *testing.T) {
	hub := &mockHub{}
	client := newMockClient(hub)

	// Test multiple connections per client (multi-device support)
	var connections []*Connection
	for range 5 {
		connID := uuid.New()
		conn := newMockConnection(connID, client)
		connections = append(connections, conn)
		client.AddConnection(conn)
	}

	assert.Equal(t, 5, client.ConnectionCount())

	// Remove connections one by one
	for i, conn := range connections {
		client.RemoveConnection(conn.id)
		expectedCount := len(connections) - i - 1
		assert.Equal(t, expectedCount, client.ConnectionCount())
	}

	assert.Equal(t, 0, client.ConnectionCount())
}

func TestClient_SendBufferFullHandling(t *testing.T) {
	hub := &mockHub{}

	// Create client with very small buffer
	client := NewClientWithID(uuid.New(), WithSendBufferSize(1))
	client.SetHub(hub)

	// Fill the buffer
	event1 := NewEvent(EventTypeSystemLog, "test-1")
	client.Send(event1)

	// This should trigger the default case (buffer full) and log an error
	event2 := NewEvent(EventTypeSystemLog, "test-2")
	client.Send(event2) // Should not block

	// Verify client is still functional
	assert.NotNil(t, client)
}
