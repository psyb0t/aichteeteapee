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
		want   bool
	}{
		{
			name:   "any origin should be allowed",
			origin: "https://example.com",
			want:   true,
		},
		{
			name:   "localhost should be allowed",
			origin: "http://localhost:3000",
			want:   true,
		},
		{
			name:   "malicious origin should still be allowed (unsafe default)",
			origin: "https://malicious.com",
			want:   true,
		},
		{
			name:   "empty origin should be allowed",
			origin: "",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			result := GetDefaultWebSocketCheckOrigin(req)
			assert.Equal(
				t, tt.want, result,
				"GetDefaultWebSocketCheckOrigin should return %v for origin %s",
				tt.want, tt.origin,
			)
		})
	}
}
