package wsunixbridge

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
