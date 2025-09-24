package wshub

import (
	"net/http"
	"testing"
	"time"

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
	assert.Empty(t, config.ClientOptions)
}

func TestUpgradeHandlerConfigOptions(t *testing.T) {
	tests := []struct {
		name         string
		option       UpgradeHandlerOption
		validateFunc func(*testing.T, UpgradeHandlerConfig, UpgradeHandlerConfig)
	}{
		{
			name:   "WithUpgradeHandlerBufferSizes",
			option: WithUpgradeHandlerBufferSizes(2048, 4096),
			validateFunc: func(t *testing.T, original, modified UpgradeHandlerConfig) {
				t.Helper()
				assert.Equal(t, 2048, modified.ReadBufferSize)
				assert.Equal(t, 4096, modified.WriteBufferSize)
				assert.NotEqual(t, original.ReadBufferSize, modified.ReadBufferSize)
				assert.NotEqual(t, original.WriteBufferSize, modified.WriteBufferSize)
			},
		},
		{
			name:   "WithUpgradeHandlerHandshakeTimeout",
			option: WithUpgradeHandlerHandshakeTimeout(60 * time.Second),
			validateFunc: func(t *testing.T, original, modified UpgradeHandlerConfig) {
				t.Helper()
				assert.Equal(t, 60*time.Second, modified.HandshakeTimeout)
				assert.NotEqual(t, original.HandshakeTimeout, modified.HandshakeTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewUpgradeHandlerConfig()
			modified := NewUpgradeHandlerConfig()
			tt.option(&modified)
			tt.validateFunc(t, original, modified)
		})
	}
}

func TestWithUpgradeHandlerCompression(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		expected bool
	}{
		{
			name:     "enable compression",
			value:    true,
			expected: true,
		},
		{
			name:     "disable compression",
			value:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewUpgradeHandlerConfig()
			originalCompression := config.EnableCompression

			opt := WithUpgradeHandlerCompression(tt.value)
			opt(&config)

			assert.Equal(t, tt.expected, config.EnableCompression)

			if tt.value != originalCompression {
				assert.NotEqual(t, originalCompression, config.EnableCompression)
			}
		})
	}
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

	// Test that default CheckOrigin allows all origins
	req := &http.Request{
		Header: make(http.Header),
	}
	req.Header.Set("Origin", "https://evil.example.com")
	assert.True(t, config.CheckOrigin(req))

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

func TestWithUpgradeHandlerClientOptions(t *testing.T) {
	config := NewUpgradeHandlerConfig()
	assert.Empty(t, config.ClientOptions)

	// Create mock client options
	opt1 := WithSendBufferSize(512)
	opt2 := WithReadTimeout(30 * time.Second)

	handlerOpt := WithUpgradeHandlerClientOptions(opt1, opt2)
	handlerOpt(&config)

	assert.Len(t, config.ClientOptions, 2)

	// Add more options
	opt3 := WithWriteTimeout(15 * time.Second)
	handlerOpt2 := WithUpgradeHandlerClientOptions(opt3)
	handlerOpt2(&config)

	assert.Len(t, config.ClientOptions, 3)
}

func TestUpgradeHandlerConfig_ProductionReadyDefaults(t *testing.T) {
	config := NewUpgradeHandlerConfig()

	// Test that defaults are reasonable for production
	assert.Greater(t, config.ReadBufferSize, 0)
	assert.Greater(t, config.WriteBufferSize, 0)
	assert.Greater(t, config.HandshakeTimeout, time.Duration(0))

	// Test that handshake timeout is reasonable (not too short, not too long)
	assert.GreaterOrEqual(t, config.HandshakeTimeout, 10*time.Second)
	assert.LessOrEqual(t, config.HandshakeTimeout, 60*time.Second)

	// Test that compression is disabled by default (safer)
	assert.False(t, config.EnableCompression)

	// Test that CheckOrigin function is set
	assert.NotNil(t, config.CheckOrigin)
}

func TestUpgradeHandlerConfig_SecurityDefaults(t *testing.T) {
	config := NewUpgradeHandlerConfig()

	// Test default CheckOrigin behavior - should allow all
	// (dev-friendly, but needs configuration for production)
	req := &http.Request{
		Header: make(http.Header),
	}

	// Test various origins
	testOrigins := []string{
		"https://example.com",
		"http://localhost:3000",
		"https://evil.com",
		"null",
		"",
	}

	for _, origin := range testOrigins {
		req.Header.Set("Origin", origin)
		// Default allows all - this is by design for development ease
		// Production deployments should configure a proper CheckOrigin function
		assert.True(
			t, config.CheckOrigin(req),
			"Default CheckOrigin should allow origin: %s", origin,
		)
	}
}

func TestUpgradeHandlerOptions_Chaining(t *testing.T) {
	config := NewUpgradeHandlerConfig()

	// Test that we can chain multiple options
	opts := []UpgradeHandlerOption{
		WithUpgradeHandlerBufferSizes(2048, 4096),
		WithUpgradeHandlerHandshakeTimeout(30 * time.Second),
		WithUpgradeHandlerCompression(true),
		WithUpgradeHandlerSubprotocols("chat", "echo"),
		WithUpgradeHandlerClientOptions(WithSendBufferSize(256)),
	}

	// Apply all options
	for _, opt := range opts {
		opt(&config)
	}

	assert.Equal(t, 2048, config.ReadBufferSize)
	assert.Equal(t, 4096, config.WriteBufferSize)
	assert.Equal(t, 30*time.Second, config.HandshakeTimeout)
	assert.True(t, config.EnableCompression)
	assert.Equal(t, []string{"chat", "echo"}, config.Subprotocols)
	assert.Len(t, config.ClientOptions, 1)
}

func TestHandlerConfig_DefaultsMatchConstants(t *testing.T) {
	config := NewUpgradeHandlerConfig()

	// Verify that our config defaults exactly match the constants in defaults.go
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
}
