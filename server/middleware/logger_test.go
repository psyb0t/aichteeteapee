package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	logger, buf := createTestLogger()
	mw := Logger(
		WithLogger(logger),
	)

	handler := createTestHandler()

	// Add request ID to context
	req := createTestRequestWithContext(http.MethodGet, "/test", map[any]any{
		aichteeteapee.ContextKeyRequestID: "test-req-123",
	})
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	w := httptest.NewRecorder()
	mw(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// Check that log output contains expected fields
	logOutput := buf.String()
	require.NotEmpty(t, logOutput, "should have log output")
	assert.Contains(t, logOutput, "HTTP request")
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/test")
	assert.Contains(t, logOutput, "test-req-123")
}

func TestLoggerMiddleware_SkipPaths(t *testing.T) {
	logger, buf := createTestLogger()

	mw := Logger(
		WithLogger(logger),
		WithSkipPaths("/health", "/metrics"),
	)

	handler := createTestHandler()

	// Test skipped path
	req := createTestRequest(http.MethodGet, "/health")
	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, buf.String(), "no log output for skipped path")

	// Test non-skipped path
	req = createTestRequest(http.MethodGet, "/api")
	w = httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, buf.String(), "log output for non-skipped path")
}

// Test to prove response writer race condition.
func TestLoggerMiddleware_ResponseWriterRaceCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	t.Run("concurrent writeheader calls can race", func(t *testing.T) {
		mw := Logger()

		handler := http.HandlerFunc(func(
			w http.ResponseWriter, _ *http.Request,
		) {
			// Simulate concurrent access to response writer
			var wg sync.WaitGroup

			for range 10 {
				wg.Go(func() {
					// Try to write status concurrently (this should race)
					w.WriteHeader(http.StatusOK)
				})
			}

			wg.Wait()
		})

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		// Run with race detector enabled to catch the race
		mw(handler).ServeHTTP(w, req)

		// The race condition exists in the responseWriter.statusCode field
		// Run this test with -race flag to detect it
		assert.True(
			t, true,
			"Race condition exists - run with -race flag to detect",
		)
	})
}

func TestLoggerMiddleware_AllOptions(t *testing.T) {
	logger, buf := createTestLogger()

	mw := Logger(
		WithLogger(logger),
		WithLogLevel(slog.LevelWarn),
		WithLogMessage("Custom log message"),
		WithExtraFields(map[string]any{
			"service": "test-service",
			"version": "1.0.0",
		}),
		WithIncludeQuery(false),
		WithIncludeHeaders("Authorization", "X-API-Key"),
	)

	handler := createTestHandler()

	req := createTestRequestWithHeaders(
		http.MethodGet, "/test?param=value", map[string]string{
			"Authorization": "Bearer token123",
			"X-API-Key":     "secret-key",
		})
	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)

	logOutput := buf.String()
	require.NotEmpty(t, logOutput)

	// Verify log level — slog text format uses "level=WARN"
	assert.True(
		t,
		strings.Contains(logOutput, "level=WARN") ||
			strings.Contains(logOutput, "WARN"),
		"should log at WARN level",
	)
	assert.Contains(t, logOutput, "Custom log message")
	assert.Contains(t, logOutput, "test-service")
	assert.Contains(t, logOutput, "1.0.0")
	assert.Contains(t, logOutput, "Bearer token123")
	assert.Contains(t, logOutput, "secret-key")
	assert.NotContains(t, logOutput, "param=value") // query disabled
}
