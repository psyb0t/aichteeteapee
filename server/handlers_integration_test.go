package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlers_Integration(t *testing.T) {
	// Test that handlers work together in a real server setup
	srv, err := New()
	require.NoError(t, err)

	mux := srv.GetMux()

	// Register handlers
	mux.HandleFunc("GET /health", srv.HealthHandler)
	mux.HandleFunc("POST /echo", srv.EchoHandler)

	server := httptest.NewServer(mux)
	defer server.Close()

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{
			name:           "health endpoint",
			method:         http.MethodGet,
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "echo endpoint",
			method:         http.MethodPost,
			path:           "/echo",
			body:           `{"test": "data"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not found endpoint",
			method:         http.MethodGet,
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "method not allowed",
			method:         http.MethodPost,
			path:           "/health",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = bytes.NewBufferString(tt.body)
			}

			req, err := http.NewRequest(tt.method, server.URL+tt.path, body)
			require.NoError(t, err)

			if tt.body != "" {
				req.Header.Set("Content-Type", aichteeteapee.ContentTypeJSON)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Only check JSON content-type for our custom handlers, not for default 404/405
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, aichteeteapee.ContentTypeJSON, resp.Header.Get(aichteeteapee.HeaderNameContentType))
			}
		})
	}
}
