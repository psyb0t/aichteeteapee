package aichteeteapee

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultCORSAllowMethods(t *testing.T) {
	result := GetDefaultCORSAllowMethods()

	// Check that all expected HTTP methods are included
	expectedMethods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}

	for _, method := range expectedMethods {
		assert.Contains(
			t, result, method,
			"Expected method %s to be in allowed methods", method,
		)
	}

	// Check that it's a comma-separated string
	methods := strings.Split(result, ", ")
	assert.Equal(
		t, len(expectedMethods), len(methods),
		"Number of methods should match expected count",
	)

	// Verify exact format
	expected := strings.Join(expectedMethods, ", ")
	assert.Equal(t, expected, result)
}

func TestGetDefaultCORSAllowHeaders(t *testing.T) {
	result := GetDefaultCORSAllowHeaders()

	// Check that all expected headers are included
	expectedHeaders := []string{
		HeaderNameAuthorization,
		HeaderNameContentType,
		HeaderNameXRequestID,
	}

	for _, header := range expectedHeaders {
		assert.Contains(
			t, result, header,
			"Expected header %s to be in allowed headers", header,
		)
	}

	// Check that it's a comma-separated string
	headers := strings.Split(result, ", ")
	assert.Equal(
		t, len(expectedHeaders), len(headers),
		"Number of headers should match expected count",
	)

	// Verify exact format
	expected := strings.Join(expectedHeaders, ", ")
	assert.Equal(t, expected, result)
}

func TestGetDefaultWebSocketCheckOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{
			name:   "matching origin allowed",
			origin: "https://myapp.local:8080",
			host:   "myapp.local:8080",
			want:   true,
		},
		{
			name:   "empty origin allowed (same-origin request)",
			origin: "",
			host:   "myapp.local:8080",
			want:   true,
		},
		{
			name:   "mismatched origin rejected",
			origin: "https://evil.com",
			host:   "myapp.local:8080",
			want:   false,
		},
		{
			name:   "localhost mismatch rejected",
			origin: "http://localhost:3000",
			host:   "myapp.local:8080",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)

			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			result := GetDefaultWebSocketCheckOrigin(req)
			assert.Equal(
				t, tt.want, result,
				"origin=%q host=%q", tt.origin, tt.host,
			)
		})
	}
}

func TestDevMode(t *testing.T) {
	t.Run("FuckSecurity enables dev mode", func(t *testing.T) {
		FuckSecurity()

		defer UnfuckSecurity()

		assert.True(t, IsDevMode())
		assert.True(t, GetDefaultCORSAllowAllOrigins())

		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Host = "myapp.local:8080"
		req.Header.Set("Origin", "https://evil.com")
		assert.True(t, GetDefaultWebSocketCheckOrigin(req))
	})

	t.Run("UnfuckSecurity restores secure defaults", func(t *testing.T) {
		FuckSecurity()
		UnfuckSecurity()

		assert.False(t, IsDevMode())
		assert.False(t, GetDefaultCORSAllowAllOrigins())

		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Host = "myapp.local:8080"
		req.Header.Set("Origin", "https://evil.com")
		assert.False(t, GetDefaultWebSocketCheckOrigin(req))
	})

	t.Run("permissive check origin always allows", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		req.Header.Set("Origin", "https://evil.com")
		assert.True(t, GetPermissiveWebSocketCheckOrigin(req))
	})
}
