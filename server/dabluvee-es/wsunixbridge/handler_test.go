package wsunixbridge

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUpgradeHandler(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewUpgradeHandler(tmpDir, nil)
	assert.NotNil(t, handler)
}

func TestNewUpgradeHandlerWithConnectionHandler(t *testing.T) {
	tmpDir := t.TempDir()
	called := false
	connectionHandler := func(conn *Connection) error {
		called = true

		assert.NotNil(t, conn)
		assert.NotEqual(t, uuid.Nil, conn.ID)

		return nil
	}

	handler := NewUpgradeHandler(tmpDir, connectionHandler)
	assert.NotNil(t, handler)
	assert.False(t, called) // Handler not called until WebSocket connection
}

func TestUpgradeHandlerInvalidUpgrade(t *testing.T) {
	tmpDir := t.TempDir()
	handler := NewUpgradeHandler(tmpDir, nil)

	// Create invalid WebSocket request (missing headers)
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Should fail WebSocket upgrade
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveConnectionNonExistent(t *testing.T) {
	// Create a logger for testing
	logger := logrus.WithField("test", "removeConnection")

	// Try to remove a connection that doesn't exist
	nonExistentID := uuid.New()

	// This should not panic
	removeConnection(nonExistentID, logger)

	// Verify the connection map is still empty
	assert.Equal(t, 0, connectionSockets.Len())
}

func TestConnectionStruct(t *testing.T) {
	connID := uuid.New()

	conn := &Connection{
		ID:   connID,
		Conn: nil, // We can't easily mock this
		WriterUnixSock: UnixSock{
			Listener: nil, // We can't easily mock this
			Clients:  []net.Conn{},
		},
		ReaderUnixSock: UnixSock{
			Listener: nil, // We can't easily mock this
			Clients:  []net.Conn{},
		},
	}

	assert.Equal(t, connID, conn.ID)
	assert.Empty(t, conn.WriterUnixSock.Clients)
}

func TestSocketsDirectoryCreation(t *testing.T) {
	// Test that the handler creates necessary directories
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "sockets")

	handler := NewUpgradeHandler(nestedDir, nil)
	assert.NotNil(t, handler)

	// The directory won't be created until a WebSocket connection is made
	// but the handler should be created without error
}

func TestUnixSockBroadcast(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		clientCount int
		expectError bool
	}{
		{
			name:        "broadcast to no clients",
			data:        []byte("test data"),
			clientCount: 0,
			expectError: false,
		},
		{
			name:        "broadcast to single client",
			data:        []byte("hello world"),
			clientCount: 1,
			expectError: false,
		},
		{
			name:        "broadcast to multiple clients",
			data:        []byte("broadcast message"),
			clientCount: 3,
			expectError: false,
		},
		{
			name:        "broadcast empty data",
			data:        nil,
			clientCount: 2,
			expectError: false,
		},
		{
			name:        "broadcast binary data",
			data:        []byte{0x01, 0x02, 0xFF, 0xFE},
			clientCount: 2,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.WithField("test", tt.name)

			// Create mock clients
			var (
				mockClients []net.Conn
				buffers     []*bytes.Buffer
			)

			for i := 0; i < tt.clientCount; i++ {
				buffer := &bytes.Buffer{}
				buffers = append(buffers, buffer)

				// Create mock connection that writes to buffer
				mockConn := &mockNetConn{buffer: buffer}
				mockClients = append(mockClients, mockConn)
			}

			// Create UnixSock with mock clients
			unixSock := &UnixSock{
				Clients: mockClients,
			}

			// Test broadcast
			unixSock.Broadcast(tt.data, logger)

			// Verify all clients received the data
			for i, buffer := range buffers {
				received := buffer.Bytes()
				if tt.data == nil && len(received) == 0 {
					// Both nil and empty slice are acceptable for empty data
					continue
				}

				assert.Equal(
					t, tt.data, received, "client %d should receive correct data", i,
				)
			}
		})
	}
}

func TestConnectionSockets(t *testing.T) {
	tests := []struct {
		name       string
		setupConns int
		testConn   bool
	}{
		{
			name:       "empty map operations",
			setupConns: 0,
			testConn:   true,
		},
		{
			name:       "single connection",
			setupConns: 1,
			testConn:   true,
		},
		{
			name:       "multiple connections",
			setupConns: 3,
			testConn:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialLen := connectionSockets.Len()

			// Setup connections
			var connIDs []uuid.UUID
			for i := 0; i < tt.setupConns; i++ {
				connID := uuid.New()
				connIDs = append(connIDs, connID)
				mockConn := &Connection{
					ID:   connID,
					Conn: nil,
					WriterUnixSock: UnixSock{
						Listener: nil,
						Clients:  []net.Conn{},
					},
					ReaderUnixSock: UnixSock{
						Listener: nil,
						Clients:  []net.Conn{},
					},
				}
				connectionSockets.Set(connID, mockConn)
			}

			// Verify length
			assert.Equal(t, initialLen+tt.setupConns, connectionSockets.Len())

			if tt.testConn && tt.setupConns > 0 {
				// Test retrieve
				retrieved, exists := connectionSockets.Get(connIDs[0])
				assert.True(t, exists)
				assert.Equal(t, connIDs[0], retrieved.ID)
			}

			// Clean up
			for _, connID := range connIDs {
				connectionSockets.Delete(connID)
			}

			assert.Equal(t, initialLen, connectionSockets.Len())
		})
	}
}

func TestCreateUnixSockets(t *testing.T) {
	tests := []struct {
		name        string
		socketsDir  string
		expectError bool
		setup       func(string) error
	}{
		{
			name:        "valid directory",
			socketsDir:  "",
			expectError: false,
			setup:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			socketsDir := tmpDir
			if tt.socketsDir != "" {
				socketsDir = filepath.Join(tmpDir, tt.socketsDir)
			}

			if tt.setup != nil {
				require.NoError(t, tt.setup(socketsDir))
			}

			ctx := context.Background()
			connID := uuid.New()
			logger := logrus.WithField("test", tt.name)

			conn := &Connection{
				ID:   connID,
				Conn: nil,
			}

			err := createUnixSockets(ctx, socketsDir, connID, conn, logger)

			if tt.expectError { //nolint:nestif
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, conn.WriterUnixSock.Listener)
				assert.NotNil(t, conn.ReaderUnixSock.Listener)

				// Clean up
				if conn.WriterUnixSock.Listener != nil {
					if err := conn.WriterUnixSock.Listener.Close(); err != nil {
						t.Logf("error closing WriterUnixSock listener: %v", err)
					}
				}

				if conn.ReaderUnixSock.Listener != nil {
					if err := conn.ReaderUnixSock.Listener.Close(); err != nil {
						t.Logf("error closing ReaderUnixSock listener: %v", err)
					}
				}
			}
		})
	}
}

func TestRemoveConnection(t *testing.T) {
	tests := []struct {
		name             string
		connectionExists bool
		hasClients       bool
		clientCount      int
	}{
		{
			name:             "non-existent connection",
			connectionExists: false,
			hasClients:       false,
			clientCount:      0,
		},
		{
			name:             "connection with no clients",
			connectionExists: true,
			hasClients:       false,
			clientCount:      0,
		},
		{
			name:             "connection with multiple clients",
			connectionExists: true,
			hasClients:       true,
			clientCount:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logrus.WithField("test", tt.name)
			connID := uuid.New()

			if tt.connectionExists {
				// Create mock connection
				mockConn := &Connection{
					ID:   connID,
					Conn: nil,
					WriterUnixSock: UnixSock{
						Clients: make([]net.Conn, tt.clientCount),
					},
					ReaderUnixSock: UnixSock{
						Clients: make([]net.Conn, tt.clientCount),
					},
				}

				// Add mock clients
				for i := 0; i < tt.clientCount; i++ {
					mockConn.WriterUnixSock.Clients[i] = &mockNetConn{buffer: &bytes.Buffer{}}
					mockConn.ReaderUnixSock.Clients[i] = &mockNetConn{buffer: &bytes.Buffer{}}
				}

				connectionSockets.Set(connID, mockConn)
			}

			initialLen := connectionSockets.Len()

			// Test remove connection - should not panic
			removeConnection(connID, logger)

			if tt.connectionExists {
				assert.Equal(t, initialLen-1, connectionSockets.Len())
				_, exists := connectionSockets.Get(connID)
				assert.False(t, exists)
			} else {
				assert.Equal(t, initialLen, connectionSockets.Len())
			}
		})
	}
}

// Mock net.Conn for testing.
type mockNetConn struct {
	buffer   *bytes.Buffer
	closed   bool
	readErr  error
	writeErr error
}

func (m *mockNetConn) Read(b []byte) (int, error) {
	if m.readErr != nil {
		return 0, m.readErr
	}

	n, err := m.buffer.Read(b)
	if err != nil {
		//nolint:wrapcheck
		return n, err
	}

	return n, nil
}

func (m *mockNetConn) Write(b []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}

	n, err := m.buffer.Write(b)
	if err != nil {
		//nolint:wrapcheck
		return n, err
	}

	return n, nil
}

func (m *mockNetConn) Close() error {
	m.closed = true

	return nil
}

func (m *mockNetConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"}
}

func (m *mockNetConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "/tmp/test-remote.sock", Net: "unix"}
}

func (m *mockNetConn) SetDeadline(_ time.Time) error {
	return nil
}

func (m *mockNetConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (m *mockNetConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

func TestRemoveReaderUnixSockClient(t *testing.T) {
	tests := []struct {
		name         string
		clientCount  int
		removeIndex  int
		expectRemove bool
	}{
		{
			name:         "remove from empty list",
			clientCount:  0,
			removeIndex:  0,
			expectRemove: false,
		},
		{
			name:         "remove single client",
			clientCount:  1,
			removeIndex:  0,
			expectRemove: true,
		},
		{
			name:         "remove first of multiple clients",
			clientCount:  3,
			removeIndex:  0,
			expectRemove: true,
		},
		{
			name:         "remove middle client",
			clientCount:  3,
			removeIndex:  1,
			expectRemove: true,
		},
		{
			name:         "remove last client",
			clientCount:  3,
			removeIndex:  2,
			expectRemove: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{
				ReaderUnixSock: UnixSock{
					Clients: make([]net.Conn, tt.clientCount),
				},
			}

			var targetClient net.Conn

			for i := 0; i < tt.clientCount; i++ {
				client := &mockNetConn{buffer: &bytes.Buffer{}}

				conn.ReaderUnixSock.Clients[i] = client
				if i == tt.removeIndex {
					targetClient = client
				}
			}

			initialCount := len(conn.ReaderUnixSock.Clients)

			if targetClient != nil {
				removeReaderUnixSockClient(conn, targetClient)
			} else {
				// Try to remove non-existent client
				removeReaderUnixSockClient(conn, &mockNetConn{buffer: &bytes.Buffer{}})
			}

			if tt.expectRemove {
				assert.Equal(t, initialCount-1, len(conn.ReaderUnixSock.Clients))
				// Verify target client was removed (pointer comparison)
				found := slices.Contains(conn.ReaderUnixSock.Clients, targetClient)
				assert.False(t, found, "target client should be removed")
			} else {
				assert.Equal(t, initialCount, len(conn.ReaderUnixSock.Clients))
			}
		})
	}
}

func TestCloseAllClients(t *testing.T) {
	tests := []struct {
		name              string
		writerClientCount int
		readerClientCount int
	}{
		{
			name:              "no clients",
			writerClientCount: 0,
			readerClientCount: 0,
		},
		{
			name:              "only writer clients",
			writerClientCount: 2,
			readerClientCount: 0,
		},
		{
			name:              "only reader clients",
			writerClientCount: 0,
			readerClientCount: 2,
		},
		{
			name:              "mixed clients",
			writerClientCount: 3,
			readerClientCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{
				WriterUnixSock: UnixSock{
					Clients: make([]net.Conn, tt.writerClientCount),
				},
				ReaderUnixSock: UnixSock{
					Clients: make([]net.Conn, tt.readerClientCount),
				},
			}

			var (
				writerClients []*mockNetConn
				readerClients []*mockNetConn
			)

			// Setup writer clients

			for i := 0; i < tt.writerClientCount; i++ {
				client := &mockNetConn{buffer: &bytes.Buffer{}}
				conn.WriterUnixSock.Clients[i] = client
				writerClients = append(writerClients, client)
			}

			// Setup reader clients
			for i := 0; i < tt.readerClientCount; i++ {
				client := &mockNetConn{buffer: &bytes.Buffer{}}
				conn.ReaderUnixSock.Clients[i] = client
				readerClients = append(readerClients, client)
			}

			logger := logrus.WithField("test", tt.name)
			closeAllClients(conn, logger)

			// Verify all clients were closed
			for _, client := range writerClients {
				assert.True(t, client.closed, "writer client should be closed")
			}

			for _, client := range readerClients {
				assert.True(t, client.closed, "reader client should be closed")
			}
		})
	}
}

func TestCloseListeners(t *testing.T) {
	tests := []struct {
		name              string
		hasWriterListener bool
		hasReaderListener bool
		writerCloseErr    error
		readerCloseErr    error
	}{
		{
			name:              "no listeners",
			hasWriterListener: false,
			hasReaderListener: false,
		},
		{
			name:              "only writer listener",
			hasWriterListener: true,
			hasReaderListener: false,
		},
		{
			name:              "only reader listener",
			hasWriterListener: false,
			hasReaderListener: true,
		},
		{
			name:              "both listeners",
			hasWriterListener: true,
			hasReaderListener: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{}

			if tt.hasWriterListener {
				conn.WriterUnixSock.Listener = &mockListener{closeErr: tt.writerCloseErr}
			}

			if tt.hasReaderListener {
				conn.ReaderUnixSock.Listener = &mockListener{closeErr: tt.readerCloseErr}
			}

			logger := logrus.WithField("test", tt.name)

			// Should not panic
			closeListeners(conn, logger)

			if tt.hasWriterListener {
				writerListener, ok := conn.WriterUnixSock.Listener.(*mockListener)
				require.True(t, ok, "expected WriterUnixSock.Listener to be *mockListener")
				assert.True(t, writerListener.closed)
			}

			if tt.hasReaderListener {
				readerListener, ok := conn.ReaderUnixSock.Listener.(*mockListener)
				require.True(t, ok, "expected ReaderUnixSock.Listener to be *mockListener")
				assert.True(t, readerListener.closed)
			}
		})
	}
}

func TestHandleConnection(t *testing.T) {
	tests := []struct {
		name               string
		socketsDir         string
		expectValidUpgrade bool
	}{
		{
			name:               "invalid upgrade request",
			socketsDir:         "",
			expectValidUpgrade: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			socketsDir := tmpDir
			if tt.socketsDir != "" {
				socketsDir = filepath.Join(tmpDir, tt.socketsDir)
			}

			called := false
			connectionHandler := func(_ *Connection) error {
				called = true

				return nil
			}

			handler := NewUpgradeHandler(socketsDir, connectionHandler)

			// Create invalid WebSocket request (missing required headers)
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			w := httptest.NewRecorder()

			handler(w, req)

			if tt.expectValidUpgrade {
				assert.True(t, called)
			} else {
				assert.False(t, called)
				assert.Equal(t, http.StatusBadRequest, w.Code)
			}
		})
	}
}

// Mock net.Listener for testing.
type mockListener struct {
	closed   bool
	closeErr error
	addr     net.Addr
}

func (m *mockListener) Accept() (net.Conn, error) {
	return nil, io.EOF
}

func (m *mockListener) Close() error {
	m.closed = true

	return m.closeErr
}

func (m *mockListener) Addr() net.Addr {
	if m.addr != nil {
		return m.addr
	}

	return &net.UnixAddr{Name: "/tmp/test.sock", Net: "unix"}
}

func TestRemoveSocketFiles(t *testing.T) {
	tests := []struct {
		name            string
		hasWriterSocket bool
		hasReaderSocket bool
		writerAddr      string
		readerAddr      string
	}{
		{
			name:            "no listeners",
			hasWriterSocket: false,
			hasReaderSocket: false,
		},
		{
			name:            "with unix listeners",
			hasWriterSocket: true,
			hasReaderSocket: true,
			writerAddr:      "/tmp/test_writer.sock",
			readerAddr:      "/tmp/test_reader.sock",
		},
		{
			name:            "only writer listener",
			hasWriterSocket: true,
			hasReaderSocket: false,
			writerAddr:      "/tmp/test_writer_only.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			conn := &Connection{}

			if tt.hasWriterSocket {
				conn.WriterUnixSock.Listener = &mockListener{
					addr: &net.UnixAddr{Name: tt.writerAddr, Net: "unix"},
				}
			}

			if tt.hasReaderSocket {
				conn.ReaderUnixSock.Listener = &mockListener{
					addr: &net.UnixAddr{Name: tt.readerAddr, Net: "unix"},
				}
			}

			logger := logrus.WithField("test", tt.name)

			// Should not panic
			removeSocketFiles(conn, logger)
		})
	}
}
