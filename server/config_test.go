package server

import (
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		expected  Config
		expectErr bool
	}{
		{
			name:    "default values only",
			envVars: map[string]string{},
			expected: Config{
				ListenAddress:       aichteeteapee.DefaultHTTPServerListenAddress,
				ReadTimeout:         aichteeteapee.DefaultHTTPServerReadTimeout,
				ReadHeaderTimeout:   aichteeteapee.DefaultHTTPServerReadHeaderTimeout,
				WriteTimeout:        aichteeteapee.DefaultHTTPServerWriteTimeout,
				IdleTimeout:         aichteeteapee.DefaultHTTPServerIdleTimeout,
				MaxHeaderBytes:      aichteeteapee.DefaultHTTPServerMaxHeaderBytes,
				ShutdownTimeout:     aichteeteapee.DefaultHTTPServerShutdownTimeout,
				ServiceName:         aichteeteapee.DefaultHTTPServerServiceName,
				FileUploadMaxMemory: aichteeteapee.DefaultFileUploadMaxMemory,
				TLSEnabled:          aichteeteapee.DefaultHTTPServerTLSEnabled,
				TLSListenAddress:    aichteeteapee.DefaultHTTPServerTLSListenAddress,
				TLSCertFile:         aichteeteapee.DefaultHTTPServerTLSCertFile,
				TLSKeyFile:          aichteeteapee.DefaultHTTPServerTLSKeyFile,
			},
			expectErr: false,
		},
		{
			name: "custom values from environment",
			envVars: map[string]string{
				aichteeteapee.EnvVarNameHTTPServerListenAddress:     ":9090",
				aichteeteapee.EnvVarNameHTTPServerReadTimeout:       "45s",
				aichteeteapee.EnvVarNameHTTPServerReadHeaderTimeout: "15s",
				aichteeteapee.EnvVarNameHTTPServerWriteTimeout:      "45s",
				aichteeteapee.EnvVarNameHTTPServerIdleTimeout:       "180s",
				aichteeteapee.EnvVarNameHTTPServerMaxHeaderBytes:    "2048000",
				aichteeteapee.EnvVarNameHTTPServerShutdownTimeout:   "45s",
				aichteeteapee.EnvVarNameHTTPServerServiceName:       "my-custom-service",
			},
			expected: Config{
				ListenAddress:       ":9090",
				ReadTimeout:         45 * time.Second,
				ReadHeaderTimeout:   15 * time.Second,
				WriteTimeout:        45 * time.Second,
				IdleTimeout:         180 * time.Second,
				MaxHeaderBytes:      2048000,
				ShutdownTimeout:     45 * time.Second,
				ServiceName:         "my-custom-service",
				FileUploadMaxMemory: aichteeteapee.DefaultFileUploadMaxMemory,
				TLSEnabled:          aichteeteapee.DefaultHTTPServerTLSEnabled,
				TLSListenAddress:    aichteeteapee.DefaultHTTPServerTLSListenAddress,
				TLSCertFile:         aichteeteapee.DefaultHTTPServerTLSCertFile,
				TLSKeyFile:          aichteeteapee.DefaultHTTPServerTLSKeyFile,
			},
			expectErr: false,
		},
		{
			name: "partial override with defaults",
			envVars: map[string]string{
				aichteeteapee.EnvVarNameHTTPServerListenAddress: ":3000",
				aichteeteapee.EnvVarNameHTTPServerServiceName:   "test-service",
			},
			expected: Config{
				ListenAddress:       ":3000",
				ReadTimeout:         aichteeteapee.DefaultHTTPServerReadTimeout,
				ReadHeaderTimeout:   aichteeteapee.DefaultHTTPServerReadHeaderTimeout,
				WriteTimeout:        aichteeteapee.DefaultHTTPServerWriteTimeout,
				IdleTimeout:         aichteeteapee.DefaultHTTPServerIdleTimeout,
				MaxHeaderBytes:      aichteeteapee.DefaultHTTPServerMaxHeaderBytes,
				ShutdownTimeout:     aichteeteapee.DefaultHTTPServerShutdownTimeout,
				ServiceName:         "test-service",
				FileUploadMaxMemory: aichteeteapee.DefaultFileUploadMaxMemory,
				TLSEnabled:          aichteeteapee.DefaultHTTPServerTLSEnabled,
				TLSListenAddress:    aichteeteapee.DefaultHTTPServerTLSListenAddress,
				TLSCertFile:         aichteeteapee.DefaultHTTPServerTLSCertFile,
				TLSKeyFile:          aichteeteapee.DefaultHTTPServerTLSKeyFile,
			},
			expectErr: false,
		},
		{
			name: "invalid timeout format",
			envVars: map[string]string{
				aichteeteapee.EnvVarNameHTTPServerReadTimeout: "invalid-duration",
			},
			expected:  Config{},
			expectErr: true,
		},
		{
			name: "invalid max header bytes",
			envVars: map[string]string{
				aichteeteapee.EnvVarNameHTTPServerMaxHeaderBytes: "not-a-number",
			},
			expected:  Config{},
			expectErr: true,
		},
		{
			name: "zero timeout values",
			envVars: map[string]string{
				aichteeteapee.EnvVarNameHTTPServerReadTimeout:  "0s",
				aichteeteapee.EnvVarNameHTTPServerWriteTimeout: "0s",
			},
			expected: Config{
				ListenAddress:       aichteeteapee.DefaultHTTPServerListenAddress,
				ReadTimeout:         0,
				ReadHeaderTimeout:   aichteeteapee.DefaultHTTPServerReadHeaderTimeout,
				WriteTimeout:        0,
				IdleTimeout:         aichteeteapee.DefaultHTTPServerIdleTimeout,
				MaxHeaderBytes:      aichteeteapee.DefaultHTTPServerMaxHeaderBytes,
				ShutdownTimeout:     aichteeteapee.DefaultHTTPServerShutdownTimeout,
				ServiceName:         aichteeteapee.DefaultHTTPServerServiceName,
				FileUploadMaxMemory: aichteeteapee.DefaultFileUploadMaxMemory,
				TLSEnabled:          aichteeteapee.DefaultHTTPServerTLSEnabled,
				TLSListenAddress:    aichteeteapee.DefaultHTTPServerTLSListenAddress,
				TLSCertFile:         aichteeteapee.DefaultHTTPServerTLSCertFile,
				TLSKeyFile:          aichteeteapee.DefaultHTTPServerTLSKeyFile,
			},
			expectErr: false,
		},
		{
			name: "extreme values",
			envVars: map[string]string{
				aichteeteapee.EnvVarNameHTTPServerReadTimeout:    "1h",
				aichteeteapee.EnvVarNameHTTPServerMaxHeaderBytes: "10485760", // 10MB
			},
			expected: Config{
				ListenAddress:       aichteeteapee.DefaultHTTPServerListenAddress,
				ReadTimeout:         1 * time.Hour,
				ReadHeaderTimeout:   aichteeteapee.DefaultHTTPServerReadHeaderTimeout,
				WriteTimeout:        aichteeteapee.DefaultHTTPServerWriteTimeout,
				IdleTimeout:         aichteeteapee.DefaultHTTPServerIdleTimeout,
				MaxHeaderBytes:      10485760,
				ShutdownTimeout:     aichteeteapee.DefaultHTTPServerShutdownTimeout,
				ServiceName:         aichteeteapee.DefaultHTTPServerServiceName,
				FileUploadMaxMemory: aichteeteapee.DefaultFileUploadMaxMemory,
				TLSEnabled:          aichteeteapee.DefaultHTTPServerTLSEnabled,
				TLSListenAddress:    aichteeteapee.DefaultHTTPServerTLSListenAddress,
				TLSCertFile:         aichteeteapee.DefaultHTTPServerTLSCertFile,
				TLSKeyFile:          aichteeteapee.DefaultHTTPServerTLSKeyFile,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper state between tests
			defer viper.Reset()

			// Set environment variables for the test using t.Setenv
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			// Parse configuration
			actual, err := parseConfig()

			if tt.expectErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestConfig_EdgeCases(t *testing.T) {
	t.Run("empty service name uses empty value", func(t *testing.T) {
		defer viper.Reset()

		t.Setenv(aichteeteapee.EnvVarNameHTTPServerServiceName, "")

		config, err := parseConfig()
		require.NoError(t, err)

		// Empty string environment variable should result in empty string value
		assert.Equal(t, "", config.ServiceName)
	})

	t.Run("whitespace in service name", func(t *testing.T) {
		defer viper.Reset()

		t.Setenv(aichteeteapee.EnvVarNameHTTPServerServiceName, "  my service  ")

		config, err := parseConfig()
		require.NoError(t, err)

		// Whitespace should be preserved
		assert.Equal(t, "  my service  ", config.ServiceName)
	})

	t.Run("negative timeout values", func(t *testing.T) {
		defer viper.Reset()

		t.Setenv(aichteeteapee.EnvVarNameHTTPServerReadTimeout, "-5s")

		config, err := parseConfig()
		require.NoError(t, err)

		// Negative duration should be parsed correctly
		assert.Equal(t, -5*time.Second, config.ReadTimeout)
	})

	t.Run("negative max header bytes", func(t *testing.T) {
		defer viper.Reset()

		t.Setenv(aichteeteapee.EnvVarNameHTTPServerMaxHeaderBytes, "-1")

		config, err := parseConfig()
		require.NoError(t, err)

		// Negative value should be parsed correctly
		assert.Equal(t, -1, config.MaxHeaderBytes)
	})
}
