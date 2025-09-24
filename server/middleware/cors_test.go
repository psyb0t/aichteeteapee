package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		origin         string
		requestHeaders string
		expectCORS     bool
	}{
		{
			name:       "preflight OPTIONS request",
			method:     http.MethodOptions,
			origin:     getTestData().TestOrigin,
			expectCORS: true,
		},
		{
			name:       "regular GET request with origin",
			method:     http.MethodGet,
			origin:     "https://api.example.com",
			expectCORS: true,
		},
		{
			name:       "POST request without origin",
			method:     http.MethodPost,
			origin:     "",
			expectCORS: true, // CORS headers should still be set
		},
		{
			name:           "preflight with custom headers",
			method:         http.MethodOptions,
			origin:         "https://frontend.com",
			requestHeaders: "Content-Type,Authorization",
			expectCORS:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CORS()

			handler := createTestHandler()

			req := createTestRequest(tt.method, "/test")
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			if tt.requestHeaders != "" {
				req.Header.Set("Access-Control-Request-Headers", tt.requestHeaders)
			}

			w := httptest.NewRecorder()
			middleware(handler).ServeHTTP(w, req)

			if tt.expectCORS {
				assertCORSHeaders(t, w, "*", true, true)
				assert.NotEmpty(t, w.Header().Get("Access-Control-Max-Age"))
			}

			if tt.method == http.MethodOptions {
				assert.Equal(t, http.StatusNoContent, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestCORSMiddleware_MissingCoverage(t *testing.T) {
	t.Run("with specific allowed origins", func(t *testing.T) {
		middleware := CORS(
			WithAllowedOrigins("https://example.com", "https://test.com"),
		)

		handler := createTestHandler()

		// Test allowed origin
		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set("Origin", "https://example.com")

		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(
			t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"),
		)
	})

	t.Run("with disallowed origin", func(t *testing.T) {
		middleware := CORS(WithAllowedOrigins("https://example.com"))

		handler := createTestHandler()

		// Test disallowed origin
		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set("Origin", "https://evil.com")

		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		// Should not set origin header for disallowed origin
		assert.Equal(t, "", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("OPTIONS without origin", func(t *testing.T) {
		middleware := CORS()

		handler := createTestHandler()

		req := createTestRequest(http.MethodOptions, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})
}

// Test to prove CORS defaults are insecure.
func TestCORSMiddleware_InsecureDefaults(t *testing.T) {
	t.Run("default CORS allows any origin - security risk", func(t *testing.T) {
		// Create CORS middleware with NO configuration (uses defaults)
		corsMiddleware := CORS()

		handler := createTestHandler()

		// Make request from arbitrary evil origin
		req := createTestRequestWithHeaders(
			http.MethodGet, "/test", map[string]string{
				"Origin": "https://evil-hacker-site.com",
			})

		w := httptest.NewRecorder()
		corsMiddleware(handler).ServeHTTP(w, req)

		// Check if evil origin is allowed
		allowedOrigin := w.Header().Get("Access-Control-Allow-Origin")

		// This proves insecure defaults - any origin is allowed
		assert.Equal(
			t, "*", allowedOrigin,
			"CORS defaults allow any origin - security risk for production",
		)
	})
}

func TestCORSMiddleware_AllOptions(t *testing.T) {
	t.Run("with custom configuration", func(t *testing.T) {
		middleware := CORS(
			WithAllowedOrigins("https://api.example.com"),
			WithAllowCredentials(true),
			WithMaxAge(3600),
		)

		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set("Origin", "https://api.example.com")

		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(
			t, "https://api.example.com",
			w.Header().Get("Access-Control-Allow-Origin"),
		)
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "3600", w.Header().Get("Access-Control-Max-Age"))
	})

	t.Run("all options", func(t *testing.T) {
		middleware := CORS(
			WithAllowedMethods(http.MethodGet, http.MethodPost, http.MethodPut),
			WithAllowedHeaders("Content-Type", "Authorization"),
			WithExposedHeaders("X-Total-Count", "X-Page"),
			WithAllowAllOrigins(),
		)

		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set("Origin", "https://example.com")

		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(
			t, "GET, POST, PUT", w.Header().Get("Access-Control-Allow-Methods"),
		)
		assert.Equal(
			t, "Content-Type, Authorization",
			w.Header().Get("Access-Control-Allow-Headers"),
		)
		assert.Equal(
			t, "X-Total-Count, X-Page",
			w.Header().Get("Access-Control-Expose-Headers"),
		)
	})
}
