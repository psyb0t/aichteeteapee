package wshub

import (
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestNewClientConfig(t *testing.T) {
	config := NewClientConfig()

	// Test that all values match defaults from http/defaults.go
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientSendBufferSize,
		config.SendBufferSize,
	)
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientReadBufferSize,
		config.ReadBufferSize,
	)
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientWriteBufferSize,
		config.WriteBufferSize,
	)
	assert.Equal(
		t,
		int64(aichteeteapee.DefaultWebSocketClientReadLimit),
		config.ReadLimit,
	)
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientReadTimeout,
		config.ReadTimeout,
	)
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientWriteTimeout,
		config.WriteTimeout,
	)
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientPingInterval,
		config.PingInterval,
	)
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientPongTimeout,
		config.PongTimeout,
	)

	// Test that defaults are sensible for production
	assert.Greater(t, config.SendBufferSize, 0)
	assert.Greater(t, config.ReadBufferSize, 0)
	assert.Greater(t, config.WriteBufferSize, 0)
	assert.Greater(t, config.ReadLimit, int64(0))
	assert.Greater(t, config.ReadTimeout, time.Duration(0))
	assert.Greater(t, config.WriteTimeout, time.Duration(0))
	assert.Greater(t, config.PingInterval, time.Duration(0))
	assert.Greater(t, config.PongTimeout, time.Duration(0))
}

func TestClientConfigOptions(t *testing.T) {
	tests := []struct {
		name         string
		option       ClientOption
		validateFunc func(*testing.T, ClientConfig, ClientConfig)
	}{
		{
			name:   "WithSendBufferSize",
			option: WithSendBufferSize(512),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, 512, modified.SendBufferSize)
				assert.NotEqual(t, original.SendBufferSize, modified.SendBufferSize)
			},
		},
		{
			name:   "WithReadBufferSize",
			option: WithReadBufferSize(2048),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, 2048, modified.ReadBufferSize)
				assert.NotEqual(t, original.ReadBufferSize, modified.ReadBufferSize)
			},
		},
		{
			name:   "WithWriteBufferSize",
			option: WithWriteBufferSize(4096),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, 4096, modified.WriteBufferSize)
				assert.NotEqual(t, original.WriteBufferSize, modified.WriteBufferSize)
			},
		},
		{
			name:   "WithReadLimit",
			option: WithReadLimit(1024),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, int64(1024), modified.ReadLimit)
				assert.NotEqual(t, original.ReadLimit, modified.ReadLimit)
			},
		},
		{
			name:   "WithReadTimeout",
			option: WithReadTimeout(30 * time.Second),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, 30*time.Second, modified.ReadTimeout)
				assert.NotEqual(t, original.ReadTimeout, modified.ReadTimeout)
			},
		},
		{
			name:   "WithWriteTimeout",
			option: WithWriteTimeout(5 * time.Second),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, 5*time.Second, modified.WriteTimeout)
				assert.NotEqual(t, original.WriteTimeout, modified.WriteTimeout)
			},
		},
		{
			name:   "WithPingInterval",
			option: WithPingInterval(30 * time.Second),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, 30*time.Second, modified.PingInterval)
				assert.NotEqual(t, original.PingInterval, modified.PingInterval)
			},
		},
		{
			name:   "WithPongTimeout",
			option: WithPongTimeout(30 * time.Second),
			validateFunc: func(t *testing.T, original, modified ClientConfig) {
				t.Helper()
				assert.Equal(t, 30*time.Second, modified.PongTimeout)
				assert.NotEqual(t, original.PongTimeout, modified.PongTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := NewClientConfig()
			modified := NewClientConfig()
			tt.option(&modified)
			tt.validateFunc(t, original, modified)
		})
	}
}

func TestClientOptions_Chaining(t *testing.T) {
	config := NewClientConfig()

	// Apply multiple options
	options := []ClientOption{
		WithSendBufferSize(512),
		WithReadTimeout(45 * time.Second),
		WithWriteTimeout(15 * time.Second),
		WithPingInterval(25 * time.Second),
	}

	for _, option := range options {
		option(&config)
	}

	// Verify all options were applied
	assert.Equal(t, 512, config.SendBufferSize)
	assert.Equal(t, 45*time.Second, config.ReadTimeout)
	assert.Equal(t, 15*time.Second, config.WriteTimeout)
	assert.Equal(t, 25*time.Second, config.PingInterval)

	// Verify other fields remain at defaults
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientReadBufferSize,
		config.ReadBufferSize,
	)
	assert.Equal(
		t,
		aichteeteapee.DefaultWebSocketClientWriteBufferSize,
		config.WriteBufferSize,
	)
}

func TestClientConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		option      ClientOption
		expectValid bool
	}{
		{
			name:        "positive send buffer size",
			option:      WithSendBufferSize(100),
			expectValid: true,
		},
		{
			name:        "zero send buffer size",
			option:      WithSendBufferSize(0),
			expectValid: false,
		},
		{
			name:        "negative send buffer size",
			option:      WithSendBufferSize(-1),
			expectValid: false,
		},
		{
			name:        "reasonable timeout",
			option:      WithReadTimeout(30 * time.Second),
			expectValid: true,
		},
		{
			name:        "very short timeout",
			option:      WithReadTimeout(1 * time.Millisecond),
			expectValid: false,
		},
		{
			name:        "zero timeout",
			option:      WithReadTimeout(0),
			expectValid: false,
		},
		{
			name:        "negative timeout",
			option:      WithReadTimeout(-1 * time.Second),
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewClientConfig()
			tt.option(&config)

			// Basic validation rules for production-readiness
			valid := config.SendBufferSize > 0 &&
				config.ReadBufferSize > 0 &&
				config.WriteBufferSize > 0 &&
				config.ReadLimit > 0 &&
				config.ReadTimeout > time.Second &&
				config.WriteTimeout > 0 &&
				config.PingInterval > 0 &&
				config.PongTimeout > 0

			if tt.expectValid {
				assert.True(t, valid, "Configuration should be valid")
			} else {
				assert.False(t, valid, "Configuration should be invalid")
			}
		})
	}
}
