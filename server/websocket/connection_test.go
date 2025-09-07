package websocket

import (
	"sync"
	"testing"

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
