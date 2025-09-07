package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestGetClientIP_AllCases(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
			},
			expectedIP: "192.168.1.1",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "10.0.0.1",
			},
			expectedIP: "10.0.0.1",
		},
		{
			name: "X-Forwarded-For with multiple IPs",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1, 10.0.0.1, 172.16.0.1",
			},
			expectedIP: "192.168.1.1",
		},
		{
			name: "X-Forwarded-For empty after split",
			headers: map[string]string{
				"X-Forwarded-For": "",
			},
			remoteAddr: "203.0.113.195:54321",
			expectedIP: "203.0.113.195",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "203.0.113.195:54321",
			expectedIP: "203.0.113.195",
		},
		{
			name:       "RemoteAddr without port (SplitHostPort fails)",
			headers:    map[string]string{},
			remoteAddr: "203.0.113.195",
			expectedIP: "203.0.113.195",
		},
		{
			name:       "IPv6 address with port",
			headers:    map[string]string{},
			remoteAddr: "[::1]:8080",
			expectedIP: "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}

			result := GetClientIP(req)
			assert.Equal(t, tt.expectedIP, result)
		})
	}
}

func TestGetRequestID_EdgeCases(t *testing.T) {
	t.Run("empty context value", func(t *testing.T) {
		req := createTestRequestWithContext(http.MethodGet, "/test", map[any]any{
			aichteeteapee.ContextKeyRequestID: "",
		})

		result := GetRequestID(req)
		assert.Equal(t, "", result)
	})

	t.Run("non-string context value", func(t *testing.T) {
		req := createTestRequestWithContext(http.MethodGet, "/test", map[any]any{
			aichteeteapee.ContextKeyRequestID: 123,
		})

		result := GetRequestID(req)
		assert.Equal(t, "", result)
	})

	t.Run("valid request ID", func(t *testing.T) {
		req := createTestRequestWithContext(http.MethodGet, "/test", map[any]any{
			aichteeteapee.ContextKeyRequestID: "valid-id-123",
		})

		result := GetRequestID(req)
		assert.Equal(t, "valid-id-123", result)
	})
}
