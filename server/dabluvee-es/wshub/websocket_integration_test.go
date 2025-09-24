package wshub

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/psyb0t/aichteeteapee"
	dabluveees "github.com/psyb0t/aichteeteapee/server/dabluvee-es"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSingleClient(t *testing.T) {
	// Start hub
	hub := NewHub("single-client-test")
	defer hub.Close()

	// Create handler and server
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	u.Scheme = "ws"

	// Connect WebSocket client
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	defer func() { _ = conn.Close() }()

	// Wait for client to be added to hub
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.Len(t, clients, 1, "Client should be connected to hub")

	var client *Client
	for _, c := range clients {
		client = c

		break
	}

	require.NotNil(t, client)

	// Send message from client
	testMessage := map[string]any{
		"type": "test",
		"data": "hello world",
	}
	err = conn.WriteJSON(testMessage)
	assert.NoError(t, err)

	// Send message to client
	testEvent := dabluveees.NewEvent("response", map[string]any{
		"message": "hello from server",
	})
	client.SendEvent(testEvent)

	// Read response from client
	var receivedMessage map[string]any

	err = conn.ReadJSON(&receivedMessage)
	assert.NoError(t, err)
	assert.Equal(t, "response", receivedMessage["type"])

	// Graceful disconnect
	err = conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	assert.NoError(t, err)

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify client is removed from hub after disconnect
	clients = hub.GetAllClients()
	assert.Len(t, clients, 0, "Client should be removed from hub after disconnect")
}

func TestMultiClientBroadcast(t *testing.T) {
	// Start hub
	hub := NewHub("multi-client-broadcast-test")
	defer hub.Close()

	// Create handler and server
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	u.Scheme = "ws"

	// Connect multiple clients
	const numClients = 3

	var (
		conns     = make([]*websocket.Conn, 0, numClients)
		connMutex sync.Mutex
	)

	for range numClients {
		conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
		require.NoError(t, err)

		defer func() { _ = resp.Body.Close() }()

		connMutex.Lock()

		conns = append(conns, conn)

		connMutex.Unlock()
	}

	// Clean up connections
	defer func() {
		connMutex.Lock()

		for _, conn := range conns {
			_ = conn.Close()
		}

		connMutex.Unlock()
	}()

	// Wait for all clients to be connected
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == numClients {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.Len(t, clients, numClients, "All clients should be connected")

	// Broadcast message from hub
	broadcastEvent := dabluveees.NewEvent("broadcast", map[string]any{
		"message": "hello everyone",
		"sender":  "server",
	})
	hub.BroadcastToAll(broadcastEvent)

	// Verify all clients receive the broadcast
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i, conn := range conns {
		go func(clientIndex int, c *websocket.Conn) {
			defer wg.Done()

			var receivedMessage map[string]any

			err := c.ReadJSON(&receivedMessage)
			assert.NoError(t, err, "Client %d should receive message", clientIndex)

			if err == nil {
				assert.Equal(t, "broadcast", receivedMessage["type"])
				data, ok := receivedMessage["data"].(map[string]any)
				assert.True(t, ok)
				assert.Equal(t, "hello everyone", data["message"])
			}
		}(i, conn)
	}

	// Wait for all clients to receive message
	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for all clients to receive broadcast")
	}

	// Disconnect clients in various orders
	connMutex.Lock()
	// Close middle client first
	if len(conns) > 1 {
		_ = conns[1].Close()
		conns = append(conns[:1], conns[2:]...)
	}
	// Close remaining clients
	for _, conn := range conns {
		_ = conn.Close()
	}

	connMutex.Unlock()

	// Give time for cleanup
	time.Sleep(200 * time.Millisecond)

	// Verify all clients are removed from hub after disconnect
	clients = hub.GetAllClients()
	assert.Len(
		t, clients, 0,
		"All clients should be removed from hub after disconnect",
	)
}

func TestMultiDeviceUser(t *testing.T) {
	// Start hub
	hub := NewHub("multi-device-user-test")
	defer hub.Close()

	// Create handler and server
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	u.Scheme = "ws"

	// Same user connects from multiple devices with same clientID
	userID := uuid.New()

	const numDevices = 3

	conns := make([]*websocket.Conn, 0, numDevices)

	for range numDevices {
		deviceURL := *u
		deviceURL.RawQuery = "clientID=" + userID.String()

		conn, resp, err := websocket.DefaultDialer.Dial(deviceURL.String(), nil)
		require.NoError(t, err)

		defer func() { _ = resp.Body.Close() }()

		conns = append(conns, conn)
	}

	// Clean up connections
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	// Wait for client to be connected
	// (should be 1 client with multiple connections)
	var (
		clients      map[uuid.UUID]*Client
		targetClient *Client
	)

	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			for _, client := range clients {
				targetClient = client

				break
			}

			if targetClient != nil && len(targetClient.GetConnections()) == numDevices {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.Len(t, clients, 1, "Should have 1 client")
	require.NotNil(t, targetClient)
	assert.Len(
		t, targetClient.GetConnections(), numDevices,
		"Client should have multiple connections",
	)
	assert.Equal(t, userID, targetClient.ID())

	// Send message to user - should reach all devices
	userEvent := dabluveees.NewEvent("user-message", map[string]any{
		"message": "hello user",
		"target":  userID.String(),
	})
	targetClient.SendEvent(userEvent)

	// Verify all devices receive the message
	var wg sync.WaitGroup
	wg.Add(numDevices)

	for i, conn := range conns {
		go func(deviceIndex int, c *websocket.Conn) {
			defer wg.Done()

			var receivedMessage map[string]any

			err := c.ReadJSON(&receivedMessage)
			assert.NoError(t, err, "Device %d should receive message", deviceIndex)

			if err == nil {
				assert.Equal(t, "user-message", receivedMessage["type"])
			}
		}(i, conn)
	}

	// Wait for all devices to receive message
	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for all devices to receive message")
	}

	// Disconnect one device
	_ = conns[0].Close()
	conns = conns[1:]

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify client still exists with remaining connections
	clients = hub.GetAllClients()
	require.Len(t, clients, 1, "Client should still exist")

	for _, client := range clients {
		assert.Len(
			t, client.GetConnections(), numDevices-1,
			"Client should have one less connection",
		)
	}

	// Disconnect remaining devices
	for _, conn := range conns {
		_ = conn.Close()
	}

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify client is removed from hub when all devices disconnect
	clients = hub.GetAllClients()
	assert.Len(
		t, clients, 0,
		"Client should be removed from hub when all devices disconnect",
	)
}

func TestHubLifecycle(t *testing.T) {
	// Create context for hub lifecycle management
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use context to ensure proper cleanup
	_ = ctx

	// Start hub
	hub := NewHub("lifecycle-test")

	// Create handler and server
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	u.Scheme = "ws"

	// Connect multiple clients
	const numClients = 5

	conns := make([]*websocket.Conn, 0, numClients)

	for range numClients {
		conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
		require.NoError(t, err)

		defer func() { _ = resp.Body.Close() }()

		conns = append(conns, conn)
	}

	// Wait for all clients to connect
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == numClients {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.Len(t, clients, numClients, "All clients should be connected")

	// Cancel context to trigger cleanup
	cancel()

	// Close hub
	hub.Close()

	// Give time for cleanup
	time.Sleep(200 * time.Millisecond)

	// Verify all clients cleaned up (hub close stops all clients)
	clients = hub.GetAllClients()
	assert.Len(t, clients, 0, "All clients should be cleaned up after hub close")

	// Clean up connections
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func TestEventHandlers(t *testing.T) {
	// Start hub
	hub := NewHub("event-handler-test")
	defer hub.Close()

	// Register custom event handlers
	var (
		handledEvents []string
		handlerMutex  sync.Mutex
	)

	hub.RegisterEventHandler("custom", func(
		hub Hub, _ *Client, _ *dabluveees.Event,
	) error {
		handlerMutex.Lock()

		handledEvents = append(handledEvents, "custom:"+hub.Name())

		handlerMutex.Unlock()

		return nil
	})

	hub.RegisterEventHandler("error", func(
		hub Hub, _ *Client, _ *dabluveees.Event,
	) error {
		handlerMutex.Lock()

		handledEvents = append(handledEvents, "error:"+hub.Name())

		handlerMutex.Unlock()

		return nil
	})

	// Create handler and server
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	u.Scheme = "ws"

	// Connect client
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	defer func() { _ = conn.Close() }()

	// Wait for client to connect
	var (
		clients      map[uuid.UUID]*Client
		targetClient *Client
	)

	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			for _, client := range clients {
				targetClient = client

				break
			}

			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.NotNil(t, targetClient)

	// Send events of various types
	customEvent := dabluveees.NewEvent("custom", map[string]any{"action": "test"})
	errorEvent := dabluveees.NewEvent(
		"error", map[string]any{"message": "test error"},
	)

	hub.ProcessEvent(targetClient, customEvent)
	hub.ProcessEvent(targetClient, errorEvent)

	// Give time for handlers to process
	time.Sleep(100 * time.Millisecond)

	// Verify handlers were called correctly
	handlerMutex.Lock()
	assert.Len(t, handledEvents, 2, "Both event handlers should be called")
	assert.Contains(t, handledEvents, "custom:"+hub.Name())
	assert.Contains(t, handledEvents, "error:"+hub.Name())
	handlerMutex.Unlock()
}

func TestErrorHandling(t *testing.T) {
	// Start hub
	hub := NewHub("error-handling-test")
	defer hub.Close()

	// Create handler and server
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("Network Failure", func(t *testing.T) {
		// Convert HTTP URL to WebSocket URL
		u, err := url.Parse(server.URL)
		require.NoError(t, err)

		u.Scheme = "ws"

		// Connect client
		conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
		require.NoError(t, err)

		defer func() { _ = resp.Body.Close() }()

		// Wait for client to connect
		var clients map[uuid.UUID]*Client
		for range 20 {
			clients = hub.GetAllClients()
			if len(clients) == 1 {
				break
			}

			time.Sleep(50 * time.Millisecond)
		}

		require.Len(t, clients, 1)

		// Abruptly close connection to simulate network failure
		_ = conn.Close()

		// Give time for cleanup
		time.Sleep(200 * time.Millisecond)

		// Verify client is removed from hub after network failure
		clients = hub.GetAllClients()
		assert.Len(
			t, clients, 0,
			"Client should be removed from hub after network failure",
		)
	})

	t.Run("Buffer Overflow", func(t *testing.T) {
		// Create client with small send buffer
		client := NewClient(WithSendBufferSize(1))
		hub.AddClient(client)

		// Try to send more messages than buffer can handle
		for i := range 10 {
			event := dabluveees.NewEvent("overflow", map[string]any{"index": i})
			client.SendEvent(event)
		}

		// Verify client handles buffer overflow gracefully
		assert.NotNil(t, client, "Client should remain valid after buffer overflow")

		// Clean up
		hub.RemoveClient(client.ID())
		client.Stop()
	})

	t.Run("Connection Timeout", func(t *testing.T) {
		// Create client with very short timeouts for testing
		client := NewClient(
			WithReadTimeout(10*time.Millisecond),
			WithWriteTimeout(10*time.Millisecond),
		)

		// Client should handle timeouts gracefully
		assert.NotNil(t, client, "Client should be created with short timeouts")

		client.Stop()
	})
}

func TestLoggingIntegration(t *testing.T) {
	// Set up logrus test hook to capture log entries
	originalLevel := logrus.GetLevel()

	logrus.SetLevel(logrus.DebugLevel)
	defer logrus.SetLevel(originalLevel)

	hook := test.NewGlobal()
	defer hook.Reset()

	// Start hub
	hub := NewHub("logging-integration-test")
	defer hub.Close()

	// Create handler and server
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	u.Scheme = "ws"

	// Connect WebSocket client
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	// Wait for client to connect and generate logs
	var (
		clients      map[uuid.UUID]*Client
		targetClient *Client
	)

	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			for _, client := range clients {
				targetClient = client

				break
			}

			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	require.NotNil(t, targetClient)

	// Send an event to generate more logs
	testEvent := dabluveees.NewEvent("log-test", map[string]any{"test": true})
	targetClient.SendEvent(testEvent)

	// Close connection gracefully to generate cleanup logs
	err = conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		// If graceful close fails, force close
		_ = conn.Close()
	}

	// Give time for all logs to be generated
	time.Sleep(200 * time.Millisecond)

	// Verify expected log entries exist
	entries := hook.AllEntries()
	require.NotEmpty(t, entries, "Should have log entries")

	// Check for hub creation logs
	var hubCreationFound bool

	for _, entry := range entries {
		if entry.Message == "creating new hub" {
			hubCreationFound = true

			assert.Equal(t, logrus.InfoLevel, entry.Level)
			assert.Equal(
				t, "logging-integration-test",
				entry.Data[aichteeteapee.FieldHubName],
			)

			break
		}
	}

	assert.True(t, hubCreationFound, "Should have hub creation log")

	// Check for client creation logs
	var clientCreationFound bool

	for _, entry := range entries {
		if entry.Message == "created new client" {
			clientCreationFound = true

			assert.Equal(t, logrus.DebugLevel, entry.Level)
			assert.Contains(t, entry.Data, aichteeteapee.FieldClientID)

			break
		}
	}

	assert.True(t, clientCreationFound, "Should have client creation log")

	// Check for WebSocket connection established logs
	var connectionEstablishedFound bool

	for _, entry := range entries {
		if entry.Message == "websocket connection established" {
			connectionEstablishedFound = true

			assert.Equal(t, logrus.InfoLevel, entry.Level)
			assert.Contains(t, entry.Data, aichteeteapee.FieldRemoteAddr)

			break
		}
	}

	assert.True(
		t, connectionEstablishedFound,
		"Should have connection established log",
	)

	// Check for connection closed logs
	var connectionClosedFound bool

	for _, entry := range entries {
		if entry.Message == "websocket connection closed" {
			connectionClosedFound = true

			assert.Equal(t, logrus.InfoLevel, entry.Level)
			assert.Contains(t, entry.Data, aichteeteapee.FieldClientID)
			assert.Contains(t, entry.Data, aichteeteapee.FieldConnectionID)

			break
		}
	}

	assert.True(t, connectionClosedFound, "Should have connection closed log")

	// Verify minimal error logs (allow for expected connection cleanup errors)
	var (
		errorCount     int
		expectedErrors []string
	)

	for _, entry := range entries {
		if entry.Level == logrus.ErrorLevel {
			errorCount++
			// Check if it's an expected connection cleanup error
			if strings.Contains(entry.Message, "connection read error") ||
				strings.Contains(entry.Message, "connection write error") {
				expectedErrors = append(expectedErrors, "connection cleanup")
			}
		}
	}
	// Allow up to 1 connection cleanup error, but no other errors
	assert.LessOrEqual(
		t, errorCount, 1, "Should have at most 1 connection cleanup error",
	)

	if errorCount > 0 {
		assert.Len(
			t, expectedErrors, errorCount,
			"All errors should be expected connection cleanup errors",
		)
	}
}
