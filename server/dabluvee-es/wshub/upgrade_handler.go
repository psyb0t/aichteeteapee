package wshub

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/psyb0t/aichteeteapee"
)

// UpgradeHandler creates an HTTP handler that upgrades connections
// to WebSocket.
func UpgradeHandler( //nolint:funlen
	hub Hub,
	opts ...UpgradeHandlerOption,
) http.HandlerFunc {
	config := NewUpgradeHandlerConfig() // Start with defaults

	// Apply user options to override defaults
	for _, opt := range opts {
		opt(&config)
	}

	// Create WebSocket upgrader with config
	upgrader := websocket.Upgrader{
		ReadBufferSize:    config.ReadBufferSize,
		WriteBufferSize:   config.WriteBufferSize,
		HandshakeTimeout:  config.HandshakeTimeout,
		CheckOrigin:       config.CheckOrigin,
		Subprotocols:      config.Subprotocols,
		EnableCompression: config.EnableCompression,
	}

	slog.Debug(
		"created websocket upgrade handler",
		aichteeteapee.FieldReadBufferSize, config.ReadBufferSize,
		aichteeteapee.FieldWriteBufferSize, config.WriteBufferSize,
		aichteeteapee.FieldHandshakeTimeout, config.HandshakeTimeout,
		aichteeteapee.FieldEnableCompression, config.EnableCompression,
		aichteeteapee.FieldHubName, hub.Name(),
	)

	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug(
			"websocket upgrade request received",
			aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
			aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
			aichteeteapee.FieldUserAgent, r.UserAgent(),
			aichteeteapee.FieldEndpoint, r.URL.Path,
		)

		// Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error(
				"websocket upgrade failed",
				"error", err,
				aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
				aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
			)

			return // upgrader already wrote HTTP error response
		}

		// Handle connection close before any other operations
		conn.SetCloseHandler(func(code int, text string) error {
			slog.Debug(
				"websocket close handler triggered",
				aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
				aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
				aichteeteapee.FieldCloseCode, code,
				aichteeteapee.FieldCloseText, text,
			)

			return nil
		})

		slog.Info(
			"websocket connection established",
			aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
			aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
		)

		// Extract client ID and get or create client
		var client *Client

		clientID := extractClientIDFromRequest(r)
		if clientID != "" {
			parsedClientID := parseClientID(clientID)

			// Use atomic get-or-create to avoid race conditions
			var wasCreated bool

			client, wasCreated = hub.GetOrCreateClient(
				parsedClientID, config.ClientOptions...,
			)

			if !wasCreated {
				slog.Debug(
					"adding connection to existing client",
					aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
					aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
					aichteeteapee.FieldClientID, parsedClientID,
				)
			} else {
				slog.Debug(
					"created new client with specified ID",
					aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
					aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
					aichteeteapee.FieldClientID, parsedClientID,
				)
			}

			// Create and add connection using AddConnection
			connection := NewConnection(conn, client)
			client.AddConnection(connection)
		} else {
			slog.Debug(
				"creating client with generated ID",
				aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
				aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
			)

			client = NewClient(config.ClientOptions...)
			hub.AddClient(client)

			// Create and add connection using AddConnection
			connection := NewConnection(conn, client)
			client.AddConnection(connection)
		}

		slog.Debug(
			"client ready",
			aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
			aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
			aichteeteapee.FieldClientID, client.ID(),
		)

		slog.Debug(
			"client connection handled successfully",
			aichteeteapee.FieldRemoteAddr, r.RemoteAddr,
			aichteeteapee.FieldOrigin, r.Header.Get("Origin"),
			aichteeteapee.FieldClientID, client.ID(),
		)
	}
}

// extractClientIDFromRequest extracts client ID from request
// This can be customized based on your authentication system.
func extractClientIDFromRequest(r *http.Request) string {
	// Try to extract from query parameter first
	if clientID := r.URL.Query().Get("clientID"); clientID != "" {
		return clientID
	}

	// Try to extract from custom header
	clientID := r.Header.Get(aichteeteapee.HeaderNameXClientID)
	if clientID != "" {
		return clientID
	}

	// Could also extract from JWT token, session, cookies, etc.
	// For now, return empty string to generate a new client ID
	return ""
}

// parseClientID converts string client ID to UUID
// Returns zero UUID if parsing fails, which will generate a new UUID.
func parseClientID(clientID string) uuid.UUID {
	if parsedID, err := uuid.Parse(clientID); err == nil {
		return parsedID
	}

	slog.Warn(
		"invalid client ID format, generating new UUID",
		"providedClientID", clientID,
	)

	return uuid.New()
}
