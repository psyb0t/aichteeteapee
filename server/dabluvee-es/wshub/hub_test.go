package wshub

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	dabluveees "github.com/psyb0t/aichteeteapee/server/dabluvee-es"
	"github.com/stretchr/testify/assert"
)

var errTestHub = errors.New("test error")

func TestNewHub(t *testing.T) {
	hub := NewHub("test-hub")

	assert.NotNil(t, hub)
	assert.Equal(t, "test-hub", hub.Name())
	assert.Equal(t, 0, len(hub.GetAllClients()))
}

func TestHub_CloseIdempotent(_ *testing.T) {
	hub := NewHub("test-hub")

	// Close should be safe to call multiple times
	hub.Close()
	hub.Close()
	hub.Close()
}

func TestHub_ClientOperations(t *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()

	hub := NewHub("test-hub")

	// Create clients with forward declaration struct
	client1 := NewClientWithID(client1ID)
	client2 := NewClientWithID(client2ID)

	// Test AddClient
	hub.AddClient(client1)
	hub.AddClient(client2)

	clients := hub.GetAllClients()
	assert.Len(t, clients, 2)
	assert.Equal(t, client1, clients[client1ID])
	assert.Equal(t, client2, clients[client2ID])

	// Test GetClient
	retrieved := hub.GetClient(client1ID)
	assert.Equal(t, client1, retrieved)

	// Test RemoveClient
	hub.RemoveClient(client1ID)
	clients = hub.GetAllClients()
	assert.Len(t, clients, 1)
	assert.Nil(t, hub.GetClient(client1ID))
	assert.NotNil(t, hub.GetClient(client2ID))
}

func TestHub_EventHandlers(t *testing.T) {
	hub := NewHub("test-hub")

	var (
		handlerCalled  bool
		receivedHub    Hub
		receivedClient *Client
		receivedEvent  dabluveees.Event
	)

	handler := func(h Hub, c *Client, e *dabluveees.Event) error {
		handlerCalled = true
		receivedHub = h
		receivedClient = c
		receivedEvent = *e

		return nil
	}

	// Create a test client
	testClient := NewClient()

	// Test RegisterEventHandler
	hub.RegisterEventHandler(dabluveees.EventTypeSystemLog, handler)

	// Test ProcessEvent
	event := dabluveees.NewEvent(dabluveees.EventTypeSystemLog, "test data")
	hub.ProcessEvent(testClient, event)

	assert.True(t, handlerCalled)
	assert.Equal(t, hub, receivedHub)
	assert.Equal(t, testClient, receivedClient)
	assert.Equal(t, event.ID, receivedEvent.ID)
	assert.Equal(t, event.Type, receivedEvent.Type)

	// Test UnregisterEventHandler
	handlerCalled = false

	hub.UnregisterEventHandler(dabluveees.EventTypeSystemLog)
	hub.ProcessEvent(testClient, event)

	assert.False(t, handlerCalled)
}

func TestHub_ProcessEventNonExistent(_ *testing.T) {
	hub := NewHub("test-hub")
	testClient := NewClient()

	// Processing event with no registered handler should not panic
	event := dabluveees.NewEvent(dabluveees.EventTypeError, "test data")
	hub.ProcessEvent(testClient, event)
}

func TestHub_RegisterEventHandlers(t *testing.T) {
	hub := NewHub("test-hub")
	testClient := NewClient()

	var (
		handledEvents []string
		handlerMutex  sync.Mutex
	)

	// Create multiple handlers
	handlers := map[dabluveees.EventType]EventHandler{
		dabluveees.EventTypeSystemLog: func(
			hub Hub, _ *Client, _ *dabluveees.Event,
		) error {
			handlerMutex.Lock()
			handledEvents = append(handledEvents, "system:"+hub.Name())
			handlerMutex.Unlock()

			return nil
		},
		dabluveees.EventTypeError: func(
			hub Hub, _ *Client, _ *dabluveees.Event,
		) error {
			handlerMutex.Lock()
			handledEvents = append(handledEvents, "error:"+hub.Name())
			handlerMutex.Unlock()

			return nil
		},
		dabluveees.EventTypeShellExec: func(
			hub Hub, _ *Client, _ *dabluveees.Event,
		) error {
			handlerMutex.Lock()
			handledEvents = append(handledEvents, "shell:"+hub.Name())
			handlerMutex.Unlock()

			return nil
		},
	}

	// Register multiple handlers at once
	hub.RegisterEventHandlers(handlers)

	// Test each handler
	systemEvent := dabluveees.NewEvent(
		dabluveees.EventTypeSystemLog, "system test",
	)
	errorEvent := dabluveees.NewEvent(dabluveees.EventTypeError, "error test")
	shellEvent := dabluveees.NewEvent(dabluveees.EventTypeShellExec, "shell test")

	hub.ProcessEvent(testClient, systemEvent)
	hub.ProcessEvent(testClient, errorEvent)
	hub.ProcessEvent(testClient, shellEvent)

	// Give time for handlers to process
	time.Sleep(10 * time.Millisecond)

	// Verify all handlers were called
	handlerMutex.Lock()
	assert.Len(t, handledEvents, 3, "All three event handlers should be called")
	assert.Contains(t, handledEvents, "system:"+hub.Name())
	assert.Contains(t, handledEvents, "error:"+hub.Name())
	assert.Contains(t, handledEvents, "shell:"+hub.Name())
	handlerMutex.Unlock()
}

func TestHub_BroadcastOperations(_ *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()
	client3ID := uuid.New()

	hub := NewHub("test-hub")

	// Add clients
	client1 := NewClientWithID(client1ID)
	client2 := NewClientWithID(client2ID)
	client3 := NewClientWithID(client3ID)

	hub.AddClient(client1)
	hub.AddClient(client2)
	hub.AddClient(client3)

	event := dabluveees.NewEvent(dabluveees.EventTypeSystemLog, "broadcast test")

	// Test BroadcastToAll - should not panic with forward declarations
	hub.BroadcastToAll(event)

	// Test BroadcastToClients - should not panic with forward declarations
	targetClients := []uuid.UUID{client1ID, client3ID, uuid.New()}
	hub.BroadcastToClients(targetClients, event)

	// Test BroadcastToSubscribers - should not panic with forward declarations
	hub.BroadcastToSubscribers(dabluveees.EventTypeSystemLog, event)
}

func TestHub_ConcurrentOperations(t *testing.T) {
	hub := NewHub("test-hub")

	var wg sync.WaitGroup

	numGoroutines := 50

	// Concurrent client additions
	for i := range numGoroutines {
		wg.Add(1)

		go func(_ int) {
			defer wg.Done()

			client := NewClientWithID(uuid.New())
			hub.AddClient(client)
		}(i)
	}

	// Concurrent handler registrations
	for i := range 10 {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			eventType := dabluveees.EventType(fmt.Sprintf("test.event.%d", id))
			handler := func(_ Hub, _ *Client, _ *dabluveees.Event) error {
				return nil
			}
			hub.RegisterEventHandler(eventType, handler)
		}(i)
	}

	// Concurrent reads
	for range 25 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			hub.GetAllClients()
			hub.GetClient(uuid.New()) // Just test with random UUID
		}()
	}

	wg.Wait()

	// Verify final state
	clients := hub.GetAllClients()
	assert.Len(t, clients, numGoroutines)
}

func TestHub_FullLifecycleIntegration(t *testing.T) {
	// Generate UUIDs for this test
	client1ID := uuid.New()
	client2ID := uuid.New()

	hub := NewHub("integration-hub")

	// Add clients
	client1 := NewClientWithID(client1ID)
	client2 := NewClientWithID(client2ID)

	hub.AddClient(client1)
	hub.AddClient(client2)

	// Register event handler
	var (
		processedEvents []dabluveees.Event
		mu              sync.Mutex
	)

	handler := func(h Hub, _ *Client, e *dabluveees.Event) error {
		mu.Lock()
		defer mu.Unlock()

		processedEvents = append(processedEvents, *e)

		// Handler broadcasts to all clients (will use forward declaration methods)
		h.BroadcastToAll(e)

		return nil
	}

	hub.RegisterEventHandler(dabluveees.EventTypeSystemLog, handler)

	// Process some events
	event1 := dabluveees.NewEvent(
		dabluveees.EventTypeSystemLog, "integration test 1",
	)
	event2 := dabluveees.NewEvent(
		dabluveees.EventTypeSystemLog, "integration test 2",
	)

	hub.ProcessEvent(client1, event1)
	hub.ProcessEvent(client1, event2)

	// Allow some processing time
	time.Sleep(10 * time.Millisecond)

	// Close the hub
	hub.Close()

	// Verify handler processed events
	mu.Lock()
	assert.Len(t, processedEvents, 2)
	assert.Equal(t, event1.ID, processedEvents[0].ID)
	assert.Equal(t, event2.ID, processedEvents[1].ID)
	mu.Unlock()

	// Verify clients are properly cleaned up after hub close
	clients := hub.GetAllClients()
	assert.Len(t, clients, 0, "Clients should be cleared from hub after close")
}

func TestHub_Name(t *testing.T) {
	tests := []struct {
		name   string
		hubID  string
		expect string
	}{
		{
			name:   "simple name",
			hubID:  "test-hub",
			expect: "test-hub",
		},
		{
			name:   "empty name",
			hubID:  "",
			expect: "",
		},
		{
			name:   "special characters",
			hubID:  "hub-with-dashes_and_underscores",
			expect: "hub-with-dashes_and_underscores",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub(tt.hubID)
			assert.Equal(t, tt.expect, hub.Name())
		})
	}
}

func TestHub_NameThreadSafety(t *testing.T) {
	hub := NewHub("concurrent-hub")

	var wg sync.WaitGroup

	// Concurrent name reads
	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			name := hub.Name()
			assert.Equal(t, "concurrent-hub", name)
		}()
	}

	wg.Wait()
}

func TestHub_ErrorHandling(_ *testing.T) {
	hub := NewHub("error-hub")
	testClient := NewClient()

	// Handler that returns an error
	errorHandler := func(_ Hub, _ *Client, _ *dabluveees.Event) error {
		return errTestHub
	}

	hub.RegisterEventHandler(dabluveees.EventTypeError, errorHandler)

	// Processing event that causes handler error should not panic
	event := dabluveees.NewEvent(dabluveees.EventTypeError, "error test")
	hub.ProcessEvent(testClient, event)

	// Should still be able to process other events
	successHandler := func(_ Hub, _ *Client, _ *dabluveees.Event) error {
		return nil
	}

	hub.RegisterEventHandler(dabluveees.EventTypeSystemLog, successHandler)
	successEvent := dabluveees.NewEvent(
		dabluveees.EventTypeSystemLog, "success test",
	)
	hub.ProcessEvent(testClient, successEvent)
}
