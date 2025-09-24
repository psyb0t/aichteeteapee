package websocket

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithUpgradeHandlerBufferSizes(t *testing.T) {
	config := NewUpgradeHandlerConfig()
	originalRead := config.ReadBufferSize
	originalWrite := config.WriteBufferSize

	opt := WithUpgradeHandlerBufferSizes(2048, 4096)
	opt(&config)

	assert.Equal(t, 2048, config.ReadBufferSize)
	assert.Equal(t, 4096, config.WriteBufferSize)
	assert.NotEqual(t, originalRead, config.ReadBufferSize)
	assert.NotEqual(t, originalWrite, config.WriteBufferSize)
}

func TestWithUpgradeHandlerHandshakeTimeout(t *testing.T) {
	config := NewUpgradeHandlerConfig()
	originalTimeout := config.HandshakeTimeout

	opt := WithUpgradeHandlerHandshakeTimeout(60 * time.Second)
	opt(&config)

	assert.Equal(t, 60*time.Second, config.HandshakeTimeout)
	assert.NotEqual(t, originalTimeout, config.HandshakeTimeout)
}

func TestWithUpgradeHandlerCompression(t *testing.T) {
	config := NewUpgradeHandlerConfig()

	opt := WithUpgradeHandlerCompression(true)
	opt(&config)

	assert.True(t, config.EnableCompression)
}

func TestWithUpgradeHandlerSubprotocols(t *testing.T) {
	config := NewUpgradeHandlerConfig()
	assert.Empty(t, config.Subprotocols)

	protocols := []string{"chat", "echo", "json"}
	opt := WithUpgradeHandlerSubprotocols(protocols...)
	opt(&config)

	assert.Equal(t, protocols, config.Subprotocols)
	assert.Len(t, config.Subprotocols, 3)
}

func TestWithUpgradeHandlerCheckOrigin(t *testing.T) {
	config := NewUpgradeHandlerConfig()

	// Test custom CheckOrigin function
	customCheckOrigin := func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		return origin == "https://trusted.example.com"
	}

	opt := WithUpgradeHandlerCheckOrigin(customCheckOrigin)
	opt(&config)

	// Test with trusted origin
	trustedReq := &http.Request{
		Header: make(http.Header),
	}
	trustedReq.Header.Set("Origin", "https://trusted.example.com")
	assert.True(t, config.CheckOrigin(trustedReq))

	// Test with untrusted origin
	untrustedReq := &http.Request{
		Header: make(http.Header),
	}
	untrustedReq.Header.Set("Origin", "https://evil.example.com")
	assert.False(t, config.CheckOrigin(untrustedReq))
}
