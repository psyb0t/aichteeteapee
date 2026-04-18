package wsunixbridge

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/ctxerrors"
)

// UnixSock represents Unix socket resources for a connection.
type UnixSock struct {
	Listener   net.Listener
	Clients    []net.Conn
	ClientsMux sync.RWMutex
	Path       string // Full path to the Unix socket file
}

// Broadcast sends data to all connected clients.
func (us *UnixSock) Broadcast(data []byte, logger *slog.Logger) {
	logger.Debug("collecting clients to broadcast")

	us.ClientsMux.RLock()
	clients := make([]net.Conn, len(us.Clients))
	copy(clients, us.Clients)
	us.ClientsMux.RUnlock()

	logger.Debug(
		"broadcasting to clients",
		"count", len(clients),
	)

	for _, client := range clients {
		if _, err := client.Write(data); err != nil {
			logger.Debug(
				"failed to write to UnixSock client",
				"error", err,
			)
		}
	}

	logger.Debug("broadcast complete")
}

//nolint:funlen // Socket setup with permission hardening requires length
func createUnixSockets(
	ctx context.Context,
	socketsDir string,
	connID uuid.UUID,
	conn *Connection,
	logger *slog.Logger,
) error {
	basePath := filepath.Join(socketsDir, connID.String())
	outputPath := basePath + writerUnixSockSuffix
	inputPath := basePath + readerUnixSockSuffix

	// Remove any existing sockets
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		logger.Debug(
			"error removing existing WriterUnixSock socket",
			"error", err,
			aichteeteapee.FieldPath, outputPath,
		)
	}

	if err := os.Remove(inputPath); err != nil && !os.IsNotExist(err) {
		logger.Debug(
			"error removing existing ReaderUnixSock socket",
			"error", err,
			aichteeteapee.FieldPath, inputPath,
		)
	}

	lc := &net.ListenConfig{}

	// Create output socket (external tools read WebSocket data from here)
	writerUnixSockListener, err := lc.Listen(
		ctx,
		aichteeteapee.NetworkTypeUnix,
		outputPath,
	)
	if err != nil {
		return ctxerrors.Wrap(err, "failed to create WriterUnixSock socket")
	}

	if err := os.Chmod(outputPath, socketPermissions); err != nil {
		logger.Warn(
			"failed to chmod WriterUnixSock socket file",
			"error", err,
			aichteeteapee.FieldPath, outputPath,
		)
	}

	conn.WriterUnixSock.Listener = writerUnixSockListener
	conn.WriterUnixSock.Path = outputPath

	// Create input socket (external tools write data here to send to WebSocket)
	readerUnixSockListener, err := lc.Listen(
		ctx,
		aichteeteapee.NetworkTypeUnix,
		inputPath,
	)
	if err != nil {
		return ctxerrors.Wrap(err, "failed to create ReaderUnixSock socket")
	}

	if err := os.Chmod(inputPath, socketPermissions); err != nil {
		logger.Warn(
			"failed to chmod ReaderUnixSock socket file",
			"error", err,
			aichteeteapee.FieldPath, inputPath,
		)
	}

	conn.ReaderUnixSock.Listener = readerUnixSockListener
	conn.ReaderUnixSock.Path = inputPath

	return nil
}

func acceptWriterUnixSockClients(
	ctx context.Context,
	conn *Connection,
	logger *slog.Logger,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			client, err := conn.WriterUnixSock.Listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				logger.Debug(
					"WriterUnixSock socket accept error",
					"error", err,
				)

				continue
			}

			conn.WriterUnixSock.ClientsMux.Lock()
			conn.WriterUnixSock.Clients = append(
				conn.WriterUnixSock.Clients, client,
			)
			conn.WriterUnixSock.ClientsMux.Unlock()

			logger.Debug("new WriterUnixSock client connected")

			// Handle client disconnection
			go handleWriterUnixSockClient(ctx, conn, client, logger)
		}
	}
}

func handleWriterUnixSockClient(
	ctx context.Context,
	conn *Connection,
	client net.Conn,
	logger *slog.Logger,
) {
	defer func() {
		if err := client.Close(); err != nil {
			logger.Debug(
				"error closing WriterUnixSock client connection",
				"error", err,
			)
		}

		removeWriterUnixSockClient(conn, client)
		logger.Debug("WriterUnixSock client disconnected")
	}()

	buffer := make([]byte, 1)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, err := client.Read(buffer)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				logger.Debug(
					"WriterUnixSock client read error",
					"error", err,
				)

				return
			}
		}
	}
}

func acceptReaderUnixSockClients(
	ctx context.Context,
	conn *Connection,
	logger *slog.Logger,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			client, err := conn.ReaderUnixSock.Listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				logger.Debug(
					"ReaderUnixSock socket accept error",
					"error", err,
				)

				continue
			}

			conn.ReaderUnixSock.ClientsMux.Lock()
			conn.ReaderUnixSock.Clients = append(
				conn.ReaderUnixSock.Clients, client,
			)
			conn.ReaderUnixSock.ClientsMux.Unlock()

			logger.Debug("new ReaderUnixSock client connected")

			go handleReaderUnixSockClient(ctx, conn, client, logger)
		}
	}
}

func handleReaderUnixSockClient(
	ctx context.Context,
	conn *Connection,
	client net.Conn,
	logger *slog.Logger,
) {
	defer func() {
		if err := client.Close(); err != nil {
			logger.Debug(
				"error closing ReaderUnixSock client connection",
				"error", err,
			)
		}

		removeReaderUnixSockClient(conn, client)
		logger.Debug("ReaderUnixSock client disconnected")
	}()

	buffer := make([]byte, bufferSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := client.Read(buffer)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				logger.Debug(
					"error reading from ReaderUnixSock socket",
					"error", err,
				)

				return
			}

			if n == 0 {
				continue
			}

			conn.WriteMu.Lock()
			err = conn.Conn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			conn.WriteMu.Unlock()

			if err != nil {
				logger.Debug(
					"error writing to websocket",
					"error", err,
				)

				return
			}
		}
	}
}

func removeReaderUnixSockClient(conn *Connection, client net.Conn) {
	conn.ReaderUnixSock.ClientsMux.Lock()
	defer conn.ReaderUnixSock.ClientsMux.Unlock()

	for i, connClient := range conn.ReaderUnixSock.Clients {
		if connClient == client {
			conn.ReaderUnixSock.Clients = append(
				conn.ReaderUnixSock.Clients[:i],
				conn.ReaderUnixSock.Clients[i+1:]...,
			)

			break
		}
	}
}

func removeWriterUnixSockClient(conn *Connection, client net.Conn) {
	conn.WriterUnixSock.ClientsMux.Lock()
	defer conn.WriterUnixSock.ClientsMux.Unlock()

	for i, connClient := range conn.WriterUnixSock.Clients {
		if connClient == client {
			conn.WriterUnixSock.Clients = append(
				conn.WriterUnixSock.Clients[:i],
				conn.WriterUnixSock.Clients[i+1:]...,
			)

			break
		}
	}
}

func removeSocketFiles(conn *Connection, logger *slog.Logger) {
	// Remove output socket file
	if conn.WriterUnixSock.Listener != nil {
		ul, ok := conn.WriterUnixSock.Listener.(*net.UnixListener)
		if ok {
			addr := ul.Addr().String()
			if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
				logger.Debug(
					"error removing WriterUnixSock socket file",
					"error", err,
					aichteeteapee.FieldPath, addr,
				)
			}
		}
	}

	// Remove input socket file
	if conn.ReaderUnixSock.Listener != nil {
		ul, ok := conn.ReaderUnixSock.Listener.(*net.UnixListener)
		if ok {
			addr := ul.Addr().String()
			if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
				logger.Debug(
					"error removing ReaderUnixSock socket file",
					"error", err,
					aichteeteapee.FieldPath, addr,
				)
			}
		}
	}
}
