package websocket

import (
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestNewUpgradeHandlerConfig(t *testing.T) {
	config := NewUpgradeHandlerConfig()

	// Test defaults from http/defaults.go
	assert.Equal(
		t, aichteeteapee.DefaultWebSocketHandlerReadBufferSize,
		config.ReadBufferSize,
	)
	assert.Equal(
		t, aichteeteapee.DefaultWebSocketHandlerWriteBufferSize,
		config.WriteBufferSize,
	)
	assert.Equal(
		t, aichteeteapee.DefaultWebSocketHandlerHandshakeTimeout,
		config.HandshakeTimeout,
	)
	assert.Equal(
		t, aichteeteapee.DefaultWebSocketHandlerEnableCompression,
		config.EnableCompression,
	)
	assert.NotNil(t, config.CheckOrigin)
	assert.Empty(t, config.Subprotocols)
}
