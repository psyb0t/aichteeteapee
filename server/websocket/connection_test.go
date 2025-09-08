package websocket

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// newMockConnection creates a mock connection for testing
func newMockConnection(id uuid.UUID, client *Client) *Connection {
	return &Connection{
		id:     id,
		client: client,
		sendCh: make(chan *Event, 10),
		doneCh: make(chan struct{}),
	}
}

// Tests for Connection methods

func TestConnection_Stop(t *testing.T) {
	client := &Client{id: uuid.New()}
	conn := newMockConnection(uuid.New(), client)

	// Should not panic when stopping
	conn.Stop()

	// Channels should be closed
	select {
	case <-conn.doneCh:
		// Good, channel is closed
	default:
		require.Fail(t, "doneCh should be closed")
	}

	select {
	case <-conn.sendCh:
		// Good, channel is closed
	default:
		require.Fail(t, "sendCh should be closed")
	}

	// Multiple stops should not panic
	conn.Stop()
	conn.Stop()
}

func TestConnection_StopIdempotent(t *testing.T) {
	client := &Client{id: uuid.New()}
	conn := newMockConnection(uuid.New(), client)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Multiple concurrent stops should be safe
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn.Stop()
		}()
	}

	wg.Wait()

	// Channels should still be closed
	select {
	case <-conn.doneCh:
		// Good
	default:
		require.Fail(t, "doneCh should be closed")
	}
}

func TestConnection_SendWithNilChannels(t *testing.T) {
	// Create connection without starting pumps (channels will be nil)
	conn := &Connection{
		id:     uuid.New(),
		conn:   nil, // Not needed for this test
		client: nil, // Not needed for this test
		// sendCh and doneCh will be nil
	}
	
	// This should hit the nil channels path and return early
	event := NewEvent(EventTypeSystemLog, "test")
	conn.Send(event) // Should not panic
	
	// Verify connection exists but channels are nil
	require.NotNil(t, conn)
	require.Nil(t, conn.sendCh)
	require.Nil(t, conn.doneCh)
}

func TestConnection_SendAfterDone(t *testing.T) {
	conn := &Connection{
		id:     uuid.New(),
		conn:   nil,
		client: nil,
		sendCh: make(chan *Event, 1),
		doneCh: make(chan struct{}),
	}
	
	// Mark connection as done
	conn.isDone.Store(true)
	
	// This should hit the isDone path and return early
	event := NewEvent(EventTypeSystemLog, "test")
	conn.Send(event) // Should not block or panic
	
	// Channel should be empty since event was not sent
	require.Len(t, conn.sendCh, 0)
}

func TestConnection_SendRaceCondition(t *testing.T) {
	conn := &Connection{
		id:     uuid.New(),
		conn:   nil,
		client: nil,
		sendCh: make(chan *Event, 1),
		doneCh: make(chan struct{}),
	}
	
	// Start a goroutine that will close the connection during send
	go func() {
		time.Sleep(1 * time.Millisecond)
		conn.isDone.Store(true)
		close(conn.doneCh)
	}()
	
	// Try to send while connection is being closed
	event := NewEvent(EventTypeSystemLog, "test")
	conn.Send(event) // Should handle the race condition gracefully
}

func TestConnection_Getters(t *testing.T) {
	hub := &mockHub{}
	client := NewClientWithID(uuid.New())
	client.SetHub(hub)
	
	conn := &Connection{
		id:     uuid.New(),
		conn:   nil,
		client: client,
	}
	
	// Test GetHubName
	hubName := conn.GetHubName()
	require.Equal(t, "mock-hub", hubName)
	
	// Test GetClientID
	clientID := conn.GetClientID()
	require.Equal(t, client.ID(), clientID)
}
