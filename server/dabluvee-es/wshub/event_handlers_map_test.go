package wshub

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	dabluveees "github.com/psyb0t/aichteeteapee/server/dabluvee-es"
	"github.com/stretchr/testify/assert"
)

var (
	errMock             = errors.New("mock error")
	errTest             = errors.New("test error")
	errProcessingFailed = errors.New("processing failed")
	errNoDataProvided   = errors.New("no data provided")
)

// mockHubForHandlers creates a simple mock hub for event handlers testing.
type mockHubForHandlers struct {
	name string
}

func (mh *mockHubForHandlers) Name() string                  { return mh.name }
func (mh *mockHubForHandlers) Close()                        {}
func (mh *mockHubForHandlers) Done() <-chan struct{}         { return nil }
func (mh *mockHubForHandlers) AddClient(_ *Client)           {}
func (mh *mockHubForHandlers) RemoveClient(_ uuid.UUID)      {}
func (mh *mockHubForHandlers) GetClient(_ uuid.UUID) *Client { return nil }
func (mh *mockHubForHandlers) GetOrCreateClient(
	_ uuid.UUID, _ ...ClientOption,
) (*Client, bool) {
	return nil, false
}

func (mh *mockHubForHandlers) GetAllClients() map[uuid.UUID]*Client {
	return nil
}

func (mh *mockHubForHandlers) RegisterEventHandler(
	_ dabluveees.EventType, _ EventHandler,
) {
}

func (mh *mockHubForHandlers) RegisterEventHandlers(
	_ map[dabluveees.EventType]EventHandler,
) {
}
func (mh *mockHubForHandlers) UnregisterEventHandler(_ dabluveees.EventType) {}
func (mh *mockHubForHandlers) ProcessEvent(_ *Client, _ *dabluveees.Event)   {}
func (mh *mockHubForHandlers) BroadcastToAll(_ *dabluveees.Event)            {}
func (mh *mockHubForHandlers) BroadcastToClients(
	_ []uuid.UUID, _ *dabluveees.Event,
) {
}

func (mh *mockHubForHandlers) BroadcastToSubscribers(
	_ dabluveees.EventType, _ *dabluveees.Event,
) {
}

// MockEventHandler creates a mock event handler for testing.
func newMockEventHandler(returnError bool) EventHandler {
	return func(_ Hub, _ *Client, _ *dabluveees.Event) error {
		if returnError {
			return errMock
		}

		return nil
	}
}

func TestNewEventHandlersMap(t *testing.T) {
	ehm := NewEventHandlersMap()

	assert.NotNil(t, ehm)
	assert.NotNil(t, ehm.handlers)
	assert.Len(t, ehm.handlers, 0)
}

func TestEventHandlersMap_Add(t *testing.T) {
	ehm := NewEventHandlersMap()
	handler1 := newMockEventHandler(false)
	handler2 := newMockEventHandler(true)

	// Test adding handlers
	ehm.Add(dabluveees.EventTypeSystemLog, handler1)
	ehm.Add(dabluveees.EventTypeShellExec, handler2)

	// Verify handlers were added
	retrieved1, exists1 := ehm.Get(dabluveees.EventTypeSystemLog)
	assert.True(t, exists1)
	assert.NotNil(t, retrieved1)

	retrieved2, exists2 := ehm.Get(dabluveees.EventTypeShellExec)
	assert.True(t, exists2)
	assert.NotNil(t, retrieved2)

	// Test overwriting handler
	handler3 := newMockEventHandler(false)
	ehm.Add(dabluveees.EventTypeSystemLog, handler3)
	retrieved3, exists3 := ehm.Get(dabluveees.EventTypeSystemLog)
	assert.True(t, exists3)
	assert.NotNil(t, retrieved3)
	// Note: We can't directly compare function pointers, but we can test behavior
}

func TestEventHandlersMap_Get(t *testing.T) {
	ehm := NewEventHandlersMap()
	handler1 := newMockEventHandler(false)
	handler2 := newMockEventHandler(true)

	ehm.Add(dabluveees.EventTypeSystemLog, handler1)
	ehm.Add(dabluveees.EventTypeError, handler2)

	// Test getting existing handlers
	retrieved1, exists1 := ehm.Get(dabluveees.EventTypeSystemLog)
	assert.True(t, exists1)
	assert.NotNil(t, retrieved1)

	retrieved2, exists2 := ehm.Get(dabluveees.EventTypeError)
	assert.True(t, exists2)
	assert.NotNil(t, retrieved2)

	// Test getting non-existent handler
	retrieved3, exists3 := ehm.Get(dabluveees.EventTypeShellExec)
	assert.False(t, exists3)
	assert.Nil(t, retrieved3)
}

func TestEventHandlersMap_Remove(t *testing.T) {
	ehm := NewEventHandlersMap()
	handler1 := newMockEventHandler(false)
	handler2 := newMockEventHandler(true)

	ehm.Add(dabluveees.EventTypeSystemLog, handler1)
	ehm.Add(dabluveees.EventTypeError, handler2)

	// Verify handlers exist
	_, exists1 := ehm.Get(dabluveees.EventTypeSystemLog)
	_, exists2 := ehm.Get(dabluveees.EventTypeError)

	assert.True(t, exists1)
	assert.True(t, exists2)

	// Remove one handler
	ehm.Remove(dabluveees.EventTypeSystemLog)

	// Verify removal
	_, exists1After := ehm.Get(dabluveees.EventTypeSystemLog)
	_, exists2After := ehm.Get(dabluveees.EventTypeError)

	assert.False(t, exists1After)
	assert.True(t, exists2After) // Other handler should remain

	// Test removing non-existent handler (should not panic)
	ehm.Remove(dabluveees.EventTypeShellExec)

	// Remove remaining handler
	ehm.Remove(dabluveees.EventTypeError)
	_, exists2Final := ehm.Get(dabluveees.EventTypeError)
	assert.False(t, exists2Final)
}

func TestEventHandlersMap_HandlerInvocation(t *testing.T) {
	ehm := NewEventHandlersMap()
	hub := &mockHubForHandlers{name: "test-hub"}
	testClient := NewClient()
	event := dabluveees.NewEvent(dabluveees.EventTypeSystemLog, "test data")

	// Test handler that succeeds
	successHandler := func(h Hub, c *Client, e *dabluveees.Event) error {
		assert.Equal(t, hub, h)
		assert.NotNil(t, c)
		assert.Equal(t, event.Type, e.Type)
		assert.Equal(t, event.ID, e.ID)

		return nil
	}

	// Test handler that fails
	errorHandler := func(h Hub, c *Client, e *dabluveees.Event) error {
		assert.Equal(t, hub, h)
		assert.NotNil(t, c)
		assert.Equal(t, dabluveees.EventTypeError, e.Type) // Expect error event type

		return errTest
	}

	ehm.Add(dabluveees.EventTypeSystemLog, successHandler)
	ehm.Add(dabluveees.EventTypeError, errorHandler)

	// Test successful handler invocation
	handler1, exists1 := ehm.Get(dabluveees.EventTypeSystemLog)
	assert.True(t, exists1)

	err1 := handler1(hub, testClient, event)
	assert.NoError(t, err1)

	// Test error handler invocation
	errorEvent := dabluveees.NewEvent(dabluveees.EventTypeError, "error data")
	handler2, exists2 := ehm.Get(dabluveees.EventTypeError)
	assert.True(t, exists2)

	err2 := handler2(hub, testClient, errorEvent)
	assert.Error(t, err2)
	assert.Equal(t, "test error", err2.Error())
}

func TestEventHandlersMap_ThreadSafety(t *testing.T) {
	ehm := NewEventHandlersMap()

	var wg sync.WaitGroup

	numGoroutines := 100

	// Concurrent adds
	for i := range numGoroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			eventType := dabluveees.EventType(fmt.Sprintf("test.event.%d", id))
			handler := func(_ Hub, _ *Client, _ *dabluveees.Event) error {
				return nil
			}
			ehm.Add(eventType, handler)
		}(i)
	}

	// Concurrent reads
	for i := range 50 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			eventType := dabluveees.EventType(
				fmt.Sprintf("test.event.%d", id%numGoroutines),
			)
			ehm.Get(eventType)
		}(i)
	}

	wg.Wait()

	// Verify all handlers were added
	for i := range numGoroutines {
		eventType := dabluveees.EventType(fmt.Sprintf("test.event.%d", i))
		_, exists := ehm.Get(eventType)
		assert.True(t, exists, "Handler for %s should exist", eventType)
	}
}

func TestEventHandlersMap_ConcurrentAddRemove(t *testing.T) {
	ehm := NewEventHandlersMap()

	var wg sync.WaitGroup

	numOperations := 1000

	// Mix of concurrent adds and removes on the same event types
	for i := range numOperations {
		wg.Add(2) // One for add, one for remove

		go func(id int) {
			defer wg.Done()

			// Reuse event types
			eventType := dabluveees.EventType(fmt.Sprintf("test.event.%d", id%10))
			handler := func(_ Hub, _ *Client, _ *dabluveees.Event) error {
				return nil
			}
			ehm.Add(eventType, handler)
		}(i)

		go func(id int) {
			defer wg.Done()

			// Same event types
			eventType := dabluveees.EventType(fmt.Sprintf("test.event.%d", id%10))
			ehm.Remove(eventType)
		}(i)
	}

	wg.Wait()

	// Final state should be consistent (no panics, no race conditions)
	// We can't predict final state due to race conditions,
	// but we can verify no crashes
	for i := range 10 {
		eventType := dabluveees.EventType(fmt.Sprintf("test.event.%d", i))

		handler, exists := ehm.Get(eventType)
		if exists {
			assert.NotNil(t, handler)
		} else {
			assert.Nil(t, handler)
		}
	}
}

func TestEventHandlersMap_HandlerTypes(t *testing.T) {
	ehm := NewEventHandlersMap()

	// Test different handler implementations
	tests := []struct {
		name        string
		eventType   dabluveees.EventType
		handler     EventHandler
		expectError bool
		errorMsg    string
	}{
		{
			name:      "success handler",
			eventType: dabluveees.EventTypeSystemLog,
			handler: func(_ Hub, _ *Client, _ *dabluveees.Event) error {
				return nil
			},
			expectError: false,
		},
		{
			name:      "error handler",
			eventType: dabluveees.EventTypeError,
			handler: func(_ Hub, _ *Client, _ *dabluveees.Event) error {
				return errProcessingFailed
			},
			expectError: true,
			errorMsg:    "processing failed",
		},
		{
			name:      "data processing handler",
			eventType: dabluveees.EventTypeShellExec,
			handler: func(_ Hub, _ *Client, event *dabluveees.Event) error {
				// Simulate data processing
				if event.Data == nil {
					return errNoDataProvided
				}

				return nil
			},
			expectError: true,
			errorMsg:    "no data provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ehm.Add(tt.eventType, tt.handler)

			handler, exists := ehm.Get(tt.eventType)
			assert.True(t, exists)
			assert.NotNil(t, handler)

			// Test handler execution
			hub := &mockHubForHandlers{name: "test"}
			testClient := NewClient()
			event := dabluveees.NewEvent(tt.eventType, nil) // No data for testing

			err := handler(hub, testClient, event)
			if tt.expectError {
				assert.Error(t, err)

				if tt.errorMsg != "" {
					assert.Equal(t, tt.errorMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
