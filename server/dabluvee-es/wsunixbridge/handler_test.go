package wsunixbridge

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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

func TestSocketsDirectoryCreation(t *testing.T) {
	// Test that the handler creates necessary directories
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "sockets")

	handler := NewUpgradeHandler(nestedDir, nil)
	assert.NotNil(t, handler)

	// The directory won't be created until a WebSocket connection is made
	// but the handler should be created without error
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
