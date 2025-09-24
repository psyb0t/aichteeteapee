package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeout(t *testing.T) {
	t.Run("request completes within timeout", func(t *testing.T) {
		middleware := Timeout(WithTimeout(100 * time.Millisecond))

		handler := createDelayedHandler(50 * time.Millisecond) // Less than timeout

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("request times out", func(t *testing.T) {
		middleware := Timeout(WithTimeout(50 * time.Millisecond))

		handler := createDelayedHandler(100 * time.Millisecond) // More than timeout

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	})

	t.Run("request with context cancellation", func(t *testing.T) {
		middleware := Timeout(WithTimeout(50 * time.Millisecond))

		handler := createDelayedHandler(100 * time.Millisecond) // More than timeout

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	})
}

// Test to prove timeout middleware doesn't actually handle timeouts.
func TestTimeoutMiddleware_RealHandlerTimeout(t *testing.T) {
	t.Run(
		"normal handler without context checking gets no response on timeout",
		func(t *testing.T) {
			// Create a channel to synchronize the handler completion
			handlerDone := make(chan bool, 1)

			// Create a NORMAL handler that doesn't check context cancellation
			normalHandler := http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					defer func() { handlerDone <- true }()
					// This is what most real handlers look like - they don't check context
					time.Sleep(100 * time.Millisecond) // Just do some work
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("completed"))
				},
			)

			timeoutMiddleware := Timeout(WithTimeout(50 * time.Millisecond))

			req := createTestRequest(http.MethodGet, "/test")
			w := httptest.NewRecorder()

			// Apply timeout middleware to normal handler
			timeoutMiddleware(normalHandler).ServeHTTP(w, req)

			// Wait for the background handler to finish to avoid race condition
			select {
			case <-handlerDone:
				// Handler finished
			case <-time.After(200 * time.Millisecond):
				// Safety timeout to prevent test from hanging
			}

			// This proves the fix works - client gets proper timeout response
			assert.Equal(
				t, http.StatusGatewayTimeout, w.Code,
				"Timeout middleware should return 504 on timeout",
			)
			assert.Contains(
				t, w.Body.String(), "Gateway timeout",
				"Timeout middleware should return timeout error message",
			)
		})
}

func TestTimeoutMiddleware_PresetOptions(t *testing.T) {
	t.Run("with short timeout", func(t *testing.T) {
		middleware := Timeout(WithShortTimeout()) // 5 seconds

		handler := createDelayedHandler(1 * time.Second)

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // Should complete within 5s timeout
	})

	t.Run("with default timeout", func(t *testing.T) {
		middleware := Timeout(WithDefaultTimeout())

		handler := createDelayedHandler(1 * time.Millisecond)

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // Should complete within 10s default
	})

	t.Run("with long timeout", func(t *testing.T) {
		middleware := Timeout(WithLongTimeout())

		handler := createDelayedHandler(1 * time.Millisecond)

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		// Should complete within 30s long timeout
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
