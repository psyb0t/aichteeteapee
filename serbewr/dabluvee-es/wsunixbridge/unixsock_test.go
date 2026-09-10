package wsunixbridge

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnixSockBroadcastContinuesAfterWriteFailure(t *testing.T) {
	t.Parallel()

	received := &bytes.Buffer{}
	unixSocket := &UnixSock{Clients: []net.Conn{
		&mockNetConn{writeErr: io.ErrClosedPipe},
		&mockNetConn{buffer: received},
	}}

	unixSocket.Broadcast([]byte("message"), testLogger())

	assert.Equal(t, "message", received.String())
}

func TestHandleWriterUnixSockClientRemovesDisconnectedClient(t *testing.T) {
	t.Parallel()

	client := &mockNetConn{readErr: io.EOF}
	connection := &Connection{
		WriterUnixSock: UnixSock{Clients: []net.Conn{client}},
	}

	handleWriterUnixSockClient(
		context.Background(),
		connection,
		client,
		testLogger(),
	)

	assert.Empty(t, connection.WriterUnixSock.Clients)
}

func TestHandleReaderUnixSockClientRemovesDisconnectedClient(t *testing.T) {
	t.Parallel()

	client := &mockNetConn{readErr: io.EOF}
	connection := &Connection{
		ReaderUnixSock: UnixSock{Clients: []net.Conn{client}},
	}

	handleReaderUnixSockClient(
		context.Background(),
		connection,
		client,
		testLogger(),
	)

	assert.Empty(t, connection.ReaderUnixSock.Clients)
}

func TestAcceptUnixSockClientsExitWhenCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	connection := &Connection{}
	acceptWriterUnixSockClients(ctx, connection, testLogger())
	acceptReaderUnixSockClients(ctx, connection, testLogger())
}

func TestAcceptUnixSockClientsHandleConnectionThenCancellation(t *testing.T) {
	t.Parallel()

	t.Run("writer", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		client := &mockNetConn{readErr: io.EOF}
		connection := &Connection{}
		connection.WriterUnixSock.Listener = &singleClientListener{
			client: client,
			cancel: cancel,
		}

		acceptWriterUnixSockClients(ctx, connection, testLogger())
		require.Eventually(t, func() bool {
			connection.WriterUnixSock.ClientsMux.RLock()
			defer connection.WriterUnixSock.ClientsMux.RUnlock()

			return len(connection.WriterUnixSock.Clients) == 0
		}, time.Second, time.Millisecond)
	})

	t.Run("reader", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		client := &mockNetConn{readErr: io.EOF}
		connection := &Connection{}
		connection.ReaderUnixSock.Listener = &singleClientListener{
			client: client,
			cancel: cancel,
		}

		acceptReaderUnixSockClients(ctx, connection, testLogger())
		require.Eventually(t, func() bool {
			connection.ReaderUnixSock.ClientsMux.RLock()
			defer connection.ReaderUnixSock.ClientsMux.RUnlock()

			return len(connection.ReaderUnixSock.Clients) == 0
		}, time.Second, time.Millisecond)
	})
}

func TestCreateUnixSocketsRejectsRegularFileAsDirectory(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "socket-parent")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	connection := &Connection{}
	err = createUnixSockets(
		context.Background(),
		file.Name(),
		uuid.New(),
		connection,
		testLogger(),
	)

	require.Error(t, err)
	assert.Nil(t, connection.WriterUnixSock.Listener)
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
			logger := testLogger()

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
					t, tt.data, received,
					"client %d should receive correct data",
					i,
				)
			}
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
			logger := testLogger()

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
				removeReaderUnixSockClient(
					conn, &mockNetConn{buffer: &bytes.Buffer{}},
				)
			}

			if tt.expectRemove {
				assert.Equal(
					t, initialCount-1,
					len(conn.ReaderUnixSock.Clients),
				)
				// Verify target client was removed
				found := slices.Contains(
					conn.ReaderUnixSock.Clients, targetClient,
				)
				assert.False(t, found, "target client should be removed")
			} else {
				assert.Equal(t, initialCount, len(conn.ReaderUnixSock.Clients))
			}
		})
	}
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

			logger := testLogger()

			// Should not panic
			removeSocketFiles(conn, logger)
		})
	}
}

type singleClientListener struct {
	client net.Conn
	cancel context.CancelFunc
	served bool
}

func (listener *singleClientListener) Accept() (net.Conn, error) {
	if !listener.served {
		listener.served = true

		return listener.client, nil
	}

	listener.cancel()

	return nil, io.EOF
}

func (listener *singleClientListener) Close() error {
	return nil
}

func (listener *singleClientListener) Addr() net.Addr {
	return &net.UnixAddr{Name: "/test.sock", Net: "unix"}
}
