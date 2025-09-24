package wsunixbridge

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/psyb0t/aichteeteapee"
	dabluveees "github.com/psyb0t/aichteeteapee/server/dabluvee-es"
	commontypes "github.com/psyb0t/common-go/types"
	"github.com/psyb0t/ctxerrors"
	"github.com/sirupsen/logrus"
)

const (
	dirPermissions = 0o750
	bufferSize     = 4096

	// Socket constants.
	writerUnixSockSuffix = "_output"
	readerUnixSockSuffix = "_input"

	// Event type for initialization.
	EventTypeWSUnixBridgeInitialized dabluveees.EventType = "wsunixbridge.init"
)

// InitMessageData represents the data sent in the initialization event.
type InitMessageData struct {
	WriterSocket string `json:"writerSocket"`
	ReaderSocket string `json:"readerSocket"`
}

// UnixSock represents Unix socket resources for a connection.
type UnixSock struct {
	Listener   net.Listener
	Clients    []net.Conn
	ClientsMux sync.RWMutex
	Path       string // Full path to the Unix socket file
}

// Broadcast sends data to all connected clients.
func (us *UnixSock) Broadcast(data []byte, logger *logrus.Entry) {
	us.ClientsMux.RLock()
	clients := make([]net.Conn, len(us.Clients))
	copy(clients, us.Clients)
	us.ClientsMux.RUnlock()

	for _, client := range clients {
		if _, err := client.Write(data); err != nil {
			logger.WithError(err).
				Debug("failed to write to UnixSock client")
		}
	}

	logger.WithField(aichteeteapee.FieldBytes, len(data)).
		Debug("broadcast data to UnixSock clients")
}

// Connection represents a WebSocket connection with Unix socket streams.
type Connection struct {
	ID             uuid.UUID
	Conn           *websocket.Conn
	WriterUnixSock UnixSock // Unix socket for writing data
	ReaderUnixSock UnixSock // Unix socket for reading data
}

// ConnectionHandler is called when a new connection is established.
type ConnectionHandler func(connection *Connection) error

// Global map to track active connections.
//
//nolint:gochecknoglobals
var connectionSockets = commontypes.NewMapWithMutex[uuid.UUID, *Connection]()

// NewUpgradeHandler creates a new WebSocket Unix socket upgrade handler.
func NewUpgradeHandler(
	socketsDir string,
	connHandler ConnectionHandler,
) http.HandlerFunc {
	config := dabluveees.NewUpgradeHandlerConfig()

	upgrader := websocket.Upgrader{
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		HandshakeTimeout:  config.HandshakeTimeout,
		CheckOrigin:       config.CheckOrigin,
		Subprotocols:      config.Subprotocols,
		EnableCompression: config.EnableCompression,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		handleConnection(w, r, socketsDir, connHandler, upgrader)
	}
}

func handleConnection(
	w http.ResponseWriter,
	r *http.Request,
	socketsDir string,
	connHandler ConnectionHandler,
	upgrader websocket.Upgrader,
) {
	connID := uuid.New()
	logger := logrus.WithFields(logrus.Fields{
		aichteeteapee.FieldRemoteAddr:   r.RemoteAddr,
		aichteeteapee.FieldOrigin:       r.Header.Get(aichteeteapee.HeaderNameOrigin),
		aichteeteapee.FieldConnectionID: connID,
	})

	logger.Debug("unixsock websocket upgrade request received")

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WithError(err).Error("websocket upgrade failed")

		return // upgrader already wrote HTTP error response
	}

	logger.Info("unixsock websocket connection established")

	err = setupConnection(
		r.Context(),
		wsConn,
		socketsDir,
		connID,
		connHandler,
		logger,
	)
	if err != nil {
		logger.WithError(err).Error("connection setup failed")

		if err := wsConn.Close(); err != nil {
			logger.WithError(err).Debug("error closing websocket connection")
		}
	}
}

func createUnixSockets(
	ctx context.Context,
	socketsDir string,
	connID uuid.UUID,
	conn *Connection,
	logger *logrus.Entry,
) error {
	basePath := filepath.Join(socketsDir, connID.String())
	outputPath := basePath + writerUnixSockSuffix
	inputPath := basePath + readerUnixSockSuffix

	// Remove any existing sockets
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		logger.WithError(err).WithField(aichteeteapee.FieldPath, outputPath).
			Debug("error removing existing WriterUnixSock socket")
	}

	if err := os.Remove(inputPath); err != nil && !os.IsNotExist(err) {
		logger.WithError(err).WithField(aichteeteapee.FieldPath, inputPath).
			Debug("error removing existing ReaderUnixSock socket")
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

	conn.ReaderUnixSock.Listener = readerUnixSockListener
	conn.ReaderUnixSock.Path = inputPath

	return nil
}

func setupConnection( //nolint:funlen
	ctx context.Context,
	wsConn *websocket.Conn,
	socketsDir string,
	connID uuid.UUID,
	connHandler ConnectionHandler,
	logger *logrus.Entry,
) error {
	if err := os.MkdirAll(socketsDir, dirPermissions); err != nil {
		return ctxerrors.Wrap(err, "failed to create sockets directory")
	}

	conn := &Connection{
		ID:   connID,
		Conn: wsConn,
	}

	if err := createUnixSockets(
		ctx, socketsDir, connID, conn, logger,
	); err != nil {
		return err
	}

	logger.Info("created connection Unix sockets",
		"outputPath", conn.WriterUnixSock.Path,
		"inputPath", conn.ReaderUnixSock.Path,
	)

	// Send initialization event to client with socket paths
	initData := InitMessageData{
		WriterSocket: conn.WriterUnixSock.Path,
		ReaderSocket: conn.ReaderUnixSock.Path,
	}
	initEvent := dabluveees.NewEvent(EventTypeWSUnixBridgeInitialized, initData)

	if err := wsConn.WriteJSON(initEvent); err != nil {
		logger.WithError(err).Error("failed to send initialization event")

		return ctxerrors.Wrap(err, "failed to send initialization event")
	}

	logger.Info("sent wsunixbridge initialization event to client")

	// Store connection
	connectionSockets.Set(connID, conn)

	// Handle connection cleanup
	defer func() {
		removeConnection(connID, logger)
	}()

	// Start socket servers
	serverCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go acceptWriterUnixSockClients(serverCtx, conn, logger)
	go acceptReaderUnixSockClients(serverCtx, conn, logger)

	// Call user handler
	if connHandler != nil {
		go func() {
			if err := connHandler(conn); err != nil {
				logger.WithError(err).Error("connection handler error")
			}
		}()
	}

	// Handle WebSocket messages
	handleWebSocketMessages(wsConn, conn, logger)

	return nil
}

func acceptWriterUnixSockClients(
	ctx context.Context,
	conn *Connection,
	logger *logrus.Entry,
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

				logger.WithError(err).Debug("WriterUnixSock socket accept error")

				continue
			}

			conn.WriterUnixSock.ClientsMux.Lock()
			conn.WriterUnixSock.Clients = append(conn.WriterUnixSock.Clients, client)
			conn.WriterUnixSock.ClientsMux.Unlock()

			logger.Debug("new WriterUnixSock client connected")

			// Handle client disconnection
			go func(c net.Conn) {
				defer func() {
					// Close connection and log any error
					if err := c.Close(); err != nil {
						logger.WithError(err).
							Debug("error closing WriterUnixSock client connection")
					}

					// Remove client from list
					conn.WriterUnixSock.ClientsMux.Lock()

					for i, connClient := range conn.WriterUnixSock.Clients {
						if connClient == c {
							//nolint:lll
							conn.WriterUnixSock.Clients = append(conn.WriterUnixSock.Clients[:i], conn.WriterUnixSock.Clients[i+1:]...)

							break
						}
					}

					conn.WriterUnixSock.ClientsMux.Unlock()

					logger.Debug("WriterUnixSock client disconnected")
				}()

				// Keep connection alive until context is cancelled
				<-ctx.Done()
			}(client)
		}
	}
}

func acceptReaderUnixSockClients(
	ctx context.Context,
	conn *Connection,
	logger *logrus.Entry,
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

				logger.WithError(err).Debug("ReaderUnixSock socket accept error")

				continue
			}

			conn.ReaderUnixSock.ClientsMux.Lock()
			conn.ReaderUnixSock.Clients = append(conn.ReaderUnixSock.Clients, client)
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
	logger *logrus.Entry,
) {
	defer func() {
		if err := client.Close(); err != nil {
			logger.WithError(err).Debug("error closing ReaderUnixSock client connection")
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

				logger.WithError(err).Debug("error reading from ReaderUnixSock socket")

				return
			}

			if n == 0 {
				continue
			}

			err = conn.Conn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			if err != nil {
				logger.WithError(err).Debug("error writing to websocket")

				return
			}

			logger.WithField(aichteeteapee.FieldBytes, n).
				Debug("forwarded data from ReaderUnixSock socket to websocket")
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

func handleWebSocketMessages(
	wsConn *websocket.Conn,
	conn *Connection,
	logger *logrus.Entry,
) {
	for {
		messageType, data, err := wsConn.ReadMessage()
		if err != nil {
			isCloseError := websocket.IsCloseError(
				err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
			)

			if isCloseError {
				logger.Info("websocket connection closed normally")

				return
			}

			logger.WithError(err).Error("websocket read error")

			return
		}

		if messageType != websocket.BinaryMessage &&
			messageType != websocket.TextMessage {
			continue
		}

		// Broadcast to all connected output readers
		conn.WriterUnixSock.Broadcast(data, logger)
	}
}

func removeConnection(connID uuid.UUID, logger *logrus.Entry) {
	conn, exists := connectionSockets.Get(connID)
	if !exists {
		return
	}

	closeAllClients(conn, logger)
	closeListeners(conn, logger)
	removeSocketFiles(conn, logger)

	connectionSockets.Delete(connID)
	logger.Info("removed connection Unix sockets")
}

func closeAllClients(conn *Connection, logger *logrus.Entry) {
	// Close output readers
	conn.WriterUnixSock.ClientsMux.Lock()

	for _, client := range conn.WriterUnixSock.Clients {
		if err := client.Close(); err != nil {
			logger.WithError(err).
				Debug("error closing WriterUnixSock client connection during cleanup")
		}
	}

	conn.WriterUnixSock.ClientsMux.Unlock()

	// Close input writers
	conn.ReaderUnixSock.ClientsMux.Lock()

	for _, client := range conn.ReaderUnixSock.Clients {
		if err := client.Close(); err != nil {
			logger.WithError(err).
				Debug("error closing ReaderUnixSock client connection during cleanup")
		}
	}

	conn.ReaderUnixSock.ClientsMux.Unlock()
}

func closeListeners(conn *Connection, logger *logrus.Entry) {
	// Close output listener
	if conn.WriterUnixSock.Listener != nil {
		if err := conn.WriterUnixSock.Listener.Close(); err != nil {
			logger.WithError(err).Debug("error closing WriterUnixSock listener")
		}
	}

	// Close input listener
	if conn.ReaderUnixSock.Listener != nil {
		if err := conn.ReaderUnixSock.Listener.Close(); err != nil {
			logger.WithError(err).Debug("error closing ReaderUnixSock listener")
		}
	}
}

func removeSocketFiles(conn *Connection, logger *logrus.Entry) {
	// Remove output socket file
	if conn.WriterUnixSock.Listener != nil {
		if ul, ok := conn.WriterUnixSock.Listener.(*net.UnixListener); ok {
			if err := os.Remove(ul.Addr().String()); err != nil && !os.IsNotExist(err) {
				logger.WithError(err).
					WithField(aichteeteapee.FieldPath, ul.Addr().String()).
					Debug("error removing WriterUnixSock socket file")
			}
		}
	}

	// Remove input socket file
	if conn.ReaderUnixSock.Listener != nil {
		if ul, ok := conn.ReaderUnixSock.Listener.(*net.UnixListener); ok {
			if err := os.Remove(ul.Addr().String()); err != nil && !os.IsNotExist(err) {
				logger.WithError(err).
					WithField(aichteeteapee.FieldPath, ul.Addr().String()).
					Debug("error removing ReaderUnixSock socket file")
			}
		}
	}
}
