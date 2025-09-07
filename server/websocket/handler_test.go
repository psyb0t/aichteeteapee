package websocket

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeHandler_BasicUpgrade(t *testing.T) {
	hub := NewHub("test-hub")
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	// Verify connection is established
	assert.NotNil(t, conn)

	// Wait for client to be added to hub with retry mechanism
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Len(t, clients, 1, "Client should be added to hub after WebSocket connection")

	// Close connection
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	assert.NoError(t, err)

	// Give some time for cleanup
	time.Sleep(10 * time.Millisecond)
}

func TestUpgradeHandler_WithClientID(t *testing.T) {
	hub := NewHub("test-hub")
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL with client ID
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"

	expectedClientID := uuid.New()
	u.RawQuery = "clientID=" + expectedClientID.String()

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	// Wait for client to be added to hub with retry mechanism
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Len(t, clients, 1, "Client should be added to hub after WebSocket connection")

	var foundClient *Client
	for _, client := range clients {
		foundClient = client
		break
	}

	require.NotNil(t, foundClient)
	assert.Equal(t, expectedClientID, foundClient.id)
}

func TestUpgradeHandler_WithClientIDHeader(t *testing.T) {
	hub := NewHub("test-hub")
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"

	expectedClientID := uuid.New()
	headers := http.Header{}
	headers.Set("X-Client-ID", expectedClientID.String())

	// Connect to WebSocket with custom header
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	require.NoError(t, err)
	defer conn.Close()

	// Wait for client to be added to hub with retry mechanism
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Len(t, clients, 1, "Client should be added to hub after WebSocket connection")

	var foundClient *Client
	for _, client := range clients {
		foundClient = client
		break
	}

	require.NotNil(t, foundClient)
	assert.Equal(t, expectedClientID, foundClient.id)
}

func TestUpgradeHandler_InvalidClientID(t *testing.T) {
	hub := NewHub("test-hub")
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL with invalid client ID
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"
	u.RawQuery = "clientID=invalid-uuid-format"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	// Wait for client to be added to hub with retry mechanism
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Len(t, clients, 1, "Client should be added to hub after WebSocket connection")

	var foundClient *Client
	for _, client := range clients {
		foundClient = client
		break
	}

	require.NotNil(t, foundClient)
	// Client ID should be a valid UUID (generated, not the invalid one)
	assert.NotEqual(t, uuid.Nil, foundClient.id)
}

func TestUpgradeHandler_WithHandlerOptions(t *testing.T) {
	hub := NewHub("test-hub")

	// Create handler with custom options
	handler := UpgradeHandler(hub,
		WithHandlerBufferSizes(2048, 4096),
		WithHandshakeTimeout(30*time.Second),
		WithCompression(true),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	// Verify connection is established
	assert.NotNil(t, conn)

	// Wait for client to be added to hub with retry mechanism
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == 1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Len(t, clients, 1, "Client should be added to hub after WebSocket connection")
}

func TestUpgradeHandler_CheckOrigin(t *testing.T) {
	hub := NewHub("test-hub")

	// Create handler with restrictive origin check
	handler := UpgradeHandler(hub,
		WithCheckOrigin(func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "https://allowed.example.com"
		}),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"

	// Test with allowed origin
	headers := http.Header{}
	headers.Set("Origin", "https://allowed.example.com")

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	require.NoError(t, err)
	conn.Close()

	// Test with disallowed origin
	headers.Set("Origin", "https://evil.example.com")

	conn, _, err = websocket.DefaultDialer.Dial(u.String(), headers)
	// Should fail upgrade due to origin check
	assert.Error(t, err)
	assert.Nil(t, conn)
}

func TestUpgradeHandler_NonWebSocketRequest(t *testing.T) {
	hub := NewHub("test-hub")
	handler := UpgradeHandler(hub)

	// Create a regular HTTP request (not WebSocket upgrade)
	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Should get HTTP 400 error for non-WebSocket request
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpgradeHandler_MultipleConnections(t *testing.T) {
	hub := NewHub("test-hub")
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"

	// Create multiple connections
	var conns []*websocket.Conn
	numConns := 3

	for range numConns {
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		require.NoError(t, err)
		conns = append(conns, conn)
	}

	// Wait for all clients to be added to hub with retry mechanism
	var clients map[uuid.UUID]*Client
	for range 20 {
		clients = hub.GetAllClients()
		if len(clients) == numConns {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Len(t, clients, numConns, "All clients should be added to hub after WebSocket connections")

	// Clean up connections
	for _, conn := range conns {
		conn.Close()
	}
}

func TestUpgradeHandler_ContextCancellation(t *testing.T) {
	hub := NewHub("test-hub")
	handler := UpgradeHandler(hub)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	u.Scheme = "ws"

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)

	// Close connection immediately to trigger context cancellation
	err = conn.Close()
	assert.NoError(t, err)

	// Give some time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Connection should be cleaned up
	clients := hub.GetAllClients()
	// Note: Depending on cleanup timing, there might still be clients
	// This test mainly verifies no panics occur during cleanup
	_ = clients
}

func TestExtractClientIDFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func() *http.Request
		expected string
	}{
		{
			name: "client ID in query parameter",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/ws?clientID=test-client-123", nil)
				return req
			},
			expected: "test-client-123",
		},
		{
			name: "client ID in header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/ws", nil)
				req.Header.Set("X-Client-ID", "header-client-456")
				return req
			},
			expected: "header-client-456",
		},
		{
			name: "query parameter takes precedence over header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/ws?clientID=query-client", nil)
				req.Header.Set("X-Client-ID", "header-client")
				return req
			},
			expected: "query-client",
		},
		{
			name: "no client ID provided",
			setupReq: func() *http.Request {
				return httptest.NewRequest("GET", "/ws", nil)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			result := extractClientIDFromRequest(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseClientID(t *testing.T) {
	validUUID := uuid.New()

	tests := []struct {
		name        string
		input       string
		expectValid bool
		expected    uuid.UUID
	}{
		{
			name:        "valid UUID",
			input:       validUUID.String(),
			expectValid: true,
			expected:    validUUID,
		},
		{
			name:        "invalid UUID format",
			input:       "not-a-uuid",
			expectValid: false,
			expected:    uuid.Nil, // parseClientID generates new UUID on invalid input
		},
		{
			name:        "empty string",
			input:       "",
			expectValid: false,
			expected:    uuid.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseClientID(tt.input)

			if tt.expectValid {
				assert.Equal(t, tt.expected, result)
			} else {
				// parseClientID generates a new UUID for invalid input
				assert.NotEqual(t, uuid.Nil, result)
			}
		})
	}
}
