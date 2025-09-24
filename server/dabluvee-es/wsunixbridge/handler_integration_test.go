package wsunixbridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSocketsPath = "/tmp/ws"

func TestSetupConnectionIntegration(t *testing.T) {
	tests := []struct {
		name          string
		socketsDir    string
		hasHandler    bool
		expectSuccess bool
	}{
		{
			name:          "basic setup without handler",
			socketsDir:    "",
			hasHandler:    false,
			expectSuccess: true,
		},
		{
			name:          "setup with connection handler",
			socketsDir:    "",
			hasHandler:    true,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = t.TempDir() // Not used for sockets due to path length limits

			// Use shorter path for Unix sockets due to path length limits (~108 chars)
			socketsDir := testSocketsPath
			if tt.socketsDir != "" {
				socketsDir = tt.socketsDir
			}

			// Create test WebSocket server
			var (
				handlerCalled     int32 // Use atomic for race-free access
				connectionHandler ConnectionHandler
			)

			if tt.hasHandler {
				connectionHandler = func(conn *Connection) error {
					atomic.StoreInt32(&handlerCalled, 1)

					assert.NotNil(t, conn)
					assert.NotEqual(t, uuid.Nil, conn.ID)

					return nil
				}
			}

			handler := NewUpgradeHandler(socketsDir, connectionHandler)

			server := httptest.NewServer(handler)
			defer server.Close()

			// Convert to WebSocket URL
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

			// Connect WebSocket client
			dialer := websocket.Dialer{
				HandshakeTimeout: 5 * time.Second,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			conn, _, err := dialer.DialContext(ctx, wsURL, nil) //nolint:bodyclose
			if tt.expectSuccess {
				require.NoError(t, err)

				require.NotNil(t, conn)

				defer func() {
					if err := conn.Close(); err != nil {
						t.Logf("error closing connection: %v", err)
					}
				}()

				if tt.hasHandler {
					// Give handler time to execute
					time.Sleep(100 * time.Millisecond)
					assert.Equal(
						t, int32(1), atomic.LoadInt32(&handlerCalled),
						"connection handler should be called",
					)
				}

				// Test basic WebSocket communication
				testMessage := []byte("test message")
				err = conn.WriteMessage(websocket.BinaryMessage, testMessage)
				assert.NoError(t, err)

				// Close connection gracefully

				err = conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				)
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestWebSocketUnixBridgeIntegration(t *testing.T) {
	tests := []struct {
		name        string
		messages    [][]byte
		messageType int
	}{
		{
			name:        "single text message",
			messages:    [][]byte{[]byte("hello world")},
			messageType: websocket.TextMessage,
		},
		{
			name:        "single binary message",
			messages:    [][]byte{[]byte("binary data")},
			messageType: websocket.BinaryMessage,
		},
		{
			name: "multiple messages",
			messages: [][]byte{
				[]byte("message 1"),
				[]byte("message 2"),
				[]byte("message 3"),
			},
			messageType: websocket.TextMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = t.TempDir() // Not used for sockets due to path length limits

			// Use shorter path for Unix sockets due to path length limits (~108 chars)
			socketsDir := testSocketsPath

			// Track connection for cleanup
			var (
				testConnection      *Connection
				testConnectionMutex sync.Mutex
			)

			connectionHandler := func(conn *Connection) error {
				testConnectionMutex.Lock()

				testConnection = conn

				testConnectionMutex.Unlock()

				return nil
			}

			handler := NewUpgradeHandler(socketsDir, connectionHandler)

			server := httptest.NewServer(handler)
			defer server.Close()

			// Convert to WebSocket URL
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

			// Connect WebSocket client
			dialer := websocket.Dialer{
				HandshakeTimeout: 5 * time.Second,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			conn, _, err := dialer.DialContext(ctx, wsURL, nil) //nolint:bodyclose
			require.NoError(t, err)

			require.NotNil(t, conn)

			defer func() {
				if err := conn.Close(); err != nil {
					t.Logf("error closing connection: %v", err)
				}
			}()

			// Give connection time to set up
			time.Sleep(100 * time.Millisecond)

			testConnectionMutex.Lock()
			require.NotNil(t, testConnection, "connection should be tracked")
			testConnectionMutex.Unlock()

			// Send test messages
			for _, message := range tt.messages {
				err = conn.WriteMessage(tt.messageType, message)
				assert.NoError(t, err)
			}

			// Give time for message processing
			time.Sleep(100 * time.Millisecond)

			// Close connection gracefully

			err = conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			assert.NoError(t, err)

			// Wait for connection to close
			time.Sleep(100 * time.Millisecond)

			// Verify connection was cleaned up
			testConnectionMutex.Lock()

			connID := testConnection.ID

			testConnectionMutex.Unlock()

			_, exists := connectionSockets.Get(connID)
			assert.False(t, exists, "connection should be cleaned up after close")
		})
	}
}

func TestUnixSocketPathCreationIntegration(t *testing.T) {
	tests := []struct {
		name       string
		socketsDir string
	}{
		{
			name:       "basic directory",
			socketsDir: "sockets",
		},
		{
			name:       "temp directory",
			socketsDir: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = t.TempDir() // Not used for sockets due to path length limits

			// Use shorter path for Unix sockets due to path length limits (~108 chars)
			socketsDir := testSocketsPath
			if tt.socketsDir != "" {
				socketsDir = tt.socketsDir
			}

			// Track connection details
			var (
				socketPaths      []string
				socketPathsMutex sync.Mutex
			)

			connectionHandler := func(conn *Connection) error {
				// Log socket paths that would be created
				basePath := socketsDir + "/" + conn.ID.String()
				outputPath := basePath + "_output"
				inputPath := basePath + "_input"

				socketPathsMutex.Lock()

				socketPaths = append(socketPaths, outputPath, inputPath)

				socketPathsMutex.Unlock()

				return nil
			}

			handler := NewUpgradeHandler(socketsDir, connectionHandler)

			server := httptest.NewServer(handler)
			defer server.Close()

			// Convert to WebSocket URL
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

			// Connect WebSocket client
			dialer := websocket.Dialer{
				HandshakeTimeout: 5 * time.Second,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			conn, _, err := dialer.DialContext(ctx, wsURL, nil) //nolint:bodyclose
			require.NoError(t, err)

			require.NotNil(t, conn)

			defer func() {
				if err := conn.Close(); err != nil {
					t.Logf("error closing connection: %v", err)
				}
			}()

			// Give connection time to set up
			time.Sleep(100 * time.Millisecond)

			// Close connection

			err = conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			assert.NoError(t, err)

			// Verify handler was called and paths were logged
			socketPathsMutex.Lock()
			assert.NotEmpty(t, socketPaths, "socket paths should be logged")
			socketPathsMutex.Unlock()
		})
	}
}

func TestConnectionCleanupIntegration(t *testing.T) {
	tests := []struct {
		name           string
		closeType      int
		closeMessage   string
		expectedClosed bool
	}{
		{
			name:           "normal closure",
			closeType:      websocket.CloseNormalClosure,
			closeMessage:   "normal close",
			expectedClosed: true,
		},
		{
			name:           "going away closure",
			closeType:      websocket.CloseGoingAway,
			closeMessage:   "going away",
			expectedClosed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = t.TempDir() // Not used for sockets due to path length limits

			// Use shorter path for Unix sockets due to path length limits (~108 chars)
			socketsDir := testSocketsPath

			var (
				testConnection      *Connection
				testConnectionMutex sync.Mutex
			)

			connectionHandler := func(conn *Connection) error {
				testConnectionMutex.Lock()

				testConnection = conn

				testConnectionMutex.Unlock()

				return nil
			}

			handler := NewUpgradeHandler(socketsDir, connectionHandler)

			server := httptest.NewServer(handler)
			defer server.Close()

			// Convert to WebSocket URL
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

			// Connect WebSocket client
			dialer := websocket.Dialer{
				HandshakeTimeout: 5 * time.Second,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			conn, _, err := dialer.DialContext(ctx, wsURL, nil) //nolint:bodyclose
			require.NoError(t, err)
			require.NotNil(t, conn)

			// Give connection time to set up
			time.Sleep(100 * time.Millisecond)

			testConnectionMutex.Lock()
			require.NotNil(t, testConnection)
			connID := testConnection.ID

			testConnectionMutex.Unlock()

			// Verify connection is tracked
			_, exists := connectionSockets.Get(connID)
			assert.True(t, exists, "connection should be tracked before close")

			// Close connection with specific type
			closeMsg := websocket.FormatCloseMessage(tt.closeType, tt.closeMessage)
			err = conn.WriteMessage(websocket.CloseMessage, closeMsg)
			assert.NoError(t, err)

			_ = conn.Close()

			// Give time for cleanup
			time.Sleep(200 * time.Millisecond)

			// Verify connection was cleaned up
			_, exists = connectionSockets.Get(connID)
			if tt.expectedClosed {
				assert.False(t, exists, "connection should be cleaned up after close")
			}
		})
	}
}

func TestInvalidWebSocketUpgradeIntegration(t *testing.T) {
	tests := []struct {
		name          string
		headers       map[string]string
		expectUpgrade bool
	}{
		{
			name:          "missing websocket headers",
			headers:       map[string]string{},
			expectUpgrade: false,
		},
		{
			name: "invalid upgrade header",
			headers: map[string]string{
				"Upgrade":    "http",
				"Connection": "upgrade",
			},
			expectUpgrade: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = t.TempDir() // Not used for sockets due to path length limits

			// Use shorter path for Unix sockets due to path length limits (~108 chars)
			socketsDir := testSocketsPath
			handler := NewUpgradeHandler(socketsDir, nil)

			server := httptest.NewServer(handler)
			defer server.Close()

			// Make regular HTTP request with custom headers
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			// Parse server URL
			u, err := url.Parse(server.URL)
			require.NoError(t, err)

			req, err := http.NewRequestWithContext(
				context.Background(), http.MethodGet, server.URL, nil,
			)
			require.NoError(t, err)

			// Add test headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			resp, err := client.Do(req)
			require.NoError(t, err)

			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("error closing response body: %v", err)
				}
			}()

			if tt.expectUpgrade {
				// Should succeed WebSocket upgrade
				assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
			} else {
				// Should fail WebSocket upgrade
				assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			}

			_ = u // Use the parsed URL to avoid unused variable
		})
	}
}
