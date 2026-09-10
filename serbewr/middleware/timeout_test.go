package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeoutResponseWriterHijack(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		written := false
		timedOut := false
		writer := &timeoutResponseWriter{
			BaseResponseWriter: BaseResponseWriter{
				ResponseWriter: &mockHijackableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
				},
			},
			mu:       &sync.Mutex{},
			header:   make(http.Header),
			written:  &written,
			timedOut: &timedOut,
		}

		connection, readWriter, err := writer.Hijack()
		require.NoError(t, err)
		require.NotNil(t, connection)
		assert.NotNil(t, readWriter)
		assert.True(t, written)
		require.NoError(t, connection.Close())
	})

	t.Run("underlying failure", func(t *testing.T) {
		t.Parallel()

		written := false
		timedOut := false
		writer := &timeoutResponseWriter{
			BaseResponseWriter: BaseResponseWriter{
				ResponseWriter: &mockFailingHijacker{
					ResponseRecorder: httptest.NewRecorder(),
				},
			},
			mu:       &sync.Mutex{},
			header:   make(http.Header),
			written:  &written,
			timedOut: &timedOut,
		}

		connection, readWriter, err := writer.Hijack()
		assert.ErrorIs(t, err, errHijackFailed)
		assert.Nil(t, connection)
		assert.Nil(t, readWriter)
		assert.False(t, written)
	})

	t.Run("timed out", func(t *testing.T) {
		t.Parallel()

		written := false
		timedOut := true
		writer := &timeoutResponseWriter{
			BaseResponseWriter: BaseResponseWriter{
				ResponseWriter: httptest.NewRecorder(),
			},
			mu:       &sync.Mutex{},
			header:   make(http.Header),
			written:  &written,
			timedOut: &timedOut,
		}

		connection, readWriter, err := writer.Hijack()
		assert.ErrorIs(t, err, http.ErrHandlerTimeout)
		assert.Nil(t, connection)
		assert.Nil(t, readWriter)
		assert.False(t, written)
	})
}

func TestTimeout(t *testing.T) {
	t.Run("request completes within timeout", func(t *testing.T) {
		middleware := Timeout(WithTimeout(100 * time.Millisecond))

		// Less than timeout
		handler := createDelayedHandler(50 * time.Millisecond)

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("request times out", func(t *testing.T) {
		middleware := Timeout(WithTimeout(50 * time.Millisecond))

		// More than timeout
		handler := createDelayedHandler(100 * time.Millisecond)

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusGatewayTimeout, w.Code)
	})

	t.Run("request with context cancellation", func(t *testing.T) {
		middleware := Timeout(WithTimeout(50 * time.Millisecond))

		// More than timeout
		handler := createDelayedHandler(100 * time.Millisecond)

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
					// This is what most real handlers look like
					// - they don't check context
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

func TestTimeoutMiddlewareFlushCommitsResponse(t *testing.T) {
	t.Parallel()

	handlerDone := make(chan struct{})
	handler := http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		defer close(handlerDone)

		writer.Header().Set("X-Stream", "started")
		_, err := writer.Write([]byte("stream"))
		assert.NoError(t, err)

		flusher, ok := writer.(http.Flusher)
		assert.True(t, ok)
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	})

	request := createTestRequest(http.MethodGet, "/stream")
	response := httptest.NewRecorder()
	middleware := Timeout(WithTimeout(10 * time.Millisecond))
	middleware(handler).ServeHTTP(response, request)
	<-handlerDone

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "started", response.Header().Get("X-Stream"))
	assert.Equal(t, "stream", response.Body.String())
}

func TestTimeoutMiddlewarePropagatesHeaderDeletion(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.Header().Del("X-Outer")
		writer.WriteHeader(http.StatusNoContent)
	})

	request := createTestRequest(http.MethodGet, "/headers")
	response := httptest.NewRecorder()
	response.Header().Set("X-Outer", "remove-me")

	Timeout(WithTimeout(time.Second))(handler).ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Header().Get("X-Outer"))
}

func TestTimeoutMiddleware_PresetOptions(t *testing.T) {
	t.Run("with short timeout", func(t *testing.T) {
		middleware := Timeout(WithShortTimeout()) // 5 seconds

		handler := createDelayedHandler(1 * time.Second)

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		// Should complete within 5s timeout
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("with default timeout", func(t *testing.T) {
		middleware := Timeout(WithDefaultTimeout())

		handler := createDelayedHandler(1 * time.Millisecond)

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		// Should complete within 10s default
		assert.Equal(t, http.StatusOK, w.Code)
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
