package wshub

import (
	"net/http"
	"time"

	"github.com/psyb0t/aichteeteapee/server/websocket"
)

// UpgradeHandlerConfig extends the shared websocket config
// with hub-specific options.
type UpgradeHandlerConfig struct {
	websocket.UpgradeHandlerConfig
	ClientOptions []ClientOption
}

// NewUpgradeHandlerConfig creates config with defaults.
func NewUpgradeHandlerConfig() UpgradeHandlerConfig {
	return UpgradeHandlerConfig{
		UpgradeHandlerConfig: websocket.NewUpgradeHandlerConfig(),
		ClientOptions:        []ClientOption{},
	}
}

type UpgradeHandlerOption func(*UpgradeHandlerConfig)

// WithUpgradeHandlerClientOptions adds default client options
// that will be applied to all new clients.
func WithUpgradeHandlerClientOptions(
	opts ...ClientOption,
) UpgradeHandlerOption {
	return func(c *UpgradeHandlerConfig) {
		c.ClientOptions = append(c.ClientOptions, opts...)
	}
}

// Wrapper functions for shared websocket options.
func WithUpgradeHandlerBufferSizes(read, write int) UpgradeHandlerOption {
	return func(c *UpgradeHandlerConfig) {
		websocket.WithUpgradeHandlerBufferSizes(read, write)(&c.UpgradeHandlerConfig)
	}
}

func WithUpgradeHandlerHandshakeTimeout(
	timeout time.Duration,
) UpgradeHandlerOption {
	return func(c *UpgradeHandlerConfig) {
		websocket.WithUpgradeHandlerHandshakeTimeout(timeout)(&c.UpgradeHandlerConfig)
	}
}

func WithUpgradeHandlerCompression(enable bool) UpgradeHandlerOption {
	return func(c *UpgradeHandlerConfig) {
		websocket.WithUpgradeHandlerCompression(enable)(&c.UpgradeHandlerConfig)
	}
}

func WithUpgradeHandlerSubprotocols(protocols ...string) UpgradeHandlerOption {
	return func(c *UpgradeHandlerConfig) {
		websocket.WithUpgradeHandlerSubprotocols(protocols...)(
			&c.UpgradeHandlerConfig,
		)
	}
}

func WithUpgradeHandlerCheckOrigin(
	checkOrigin func(*http.Request) bool,
) UpgradeHandlerOption {
	return func(c *UpgradeHandlerConfig) {
		websocket.WithUpgradeHandlerCheckOrigin(checkOrigin)(&c.UpgradeHandlerConfig)
	}
}
