package echo

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWithConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				ListenAddress: "127.0.0.1:0",
			},
		},
		{
			name:    "empty listen address",
			cfg:     Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewWithConfig(
				tt.cfg, "/api", nil, nil,
			)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, e)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, e)
			assert.NotNil(t, e.Echo)
			assert.NotNil(t, e.RouterGroup)
		})
	}
}

func TestNewWithConfig_OASRoute(t *testing.T) {
	yaml := []byte("openapi: 3.0.0")

	e, err := NewWithConfig(
		Config{
			ListenAddress: "127.0.0.1:0",
			OASPath:       "/openapi.yaml",
		},
		"/api",
		yaml,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, e)

	routes := e.Echo.Routes()
	found := false

	for _, r := range routes {
		if r.Path == "/api/openapi.yaml" &&
			r.Method == http.MethodGet {
			found = true

			break
		}
	}

	assert.True(t, found, "OAS route not registered")
}

func TestNewWithConfig_SwaggerUI(t *testing.T) {
	yaml := []byte("openapi: 3.0.0")

	e, err := NewWithConfig(
		Config{
			ListenAddress: "127.0.0.1:0",
			OASPath:       "/openapi.yaml",
			SwaggerUIPath: "/docs/*",
		},
		"/api",
		yaml,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, e)

	routes := e.Echo.Routes()
	found := false

	for _, r := range routes {
		if r.Path == "/api/docs/*" &&
			r.Method == http.MethodGet {
			found = true

			break
		}
	}

	assert.True(t, found, "Swagger UI route not registered")
}

func TestEcho_StartAndShutdown(t *testing.T) {
	e, err := NewWithConfig(
		Config{ListenAddress: "127.0.0.1:0"},
		"/", nil, nil,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		500*time.Millisecond,
	)
	defer cancel()

	err = e.Start(ctx)
	assert.NoError(t, err)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "valid",
			cfg: Config{
				ListenAddress: aichteeteapee.DefaultEchoListenAddress,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()

			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
		})
	}
}
