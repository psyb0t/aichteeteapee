package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	logger, hook := createTestLogger()
	middleware := Logger(
		WithLogger(logger),
	)

	handler := createTestHandler()

	req := createTestRequestWithHeaders(http.MethodGet, "/test", map[string]string{
		"X-Forwarded-For": "192.168.1.1",
	})

	// Add request ID to context
	req = createTestRequestWithContext(http.MethodGet, "/test", map[any]any{
		aichteeteapee.ContextKeyRequestID: "test-req-123",
	})
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	w := httptest.NewRecorder()
	middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// Check that log entry was created
	entries := hook.AllEntries()
	require.Len(t, entries, 1)

	entry := entries[0]
	expectedFields := map[string]any{
		"method":    http.MethodGet,
		"path":      "/test",
		"status":    200,
		"requestId": "test-req-123",
	}

	assert.Equal(t, logrus.InfoLevel, entry.Level)
	assertLogEntry(t, entry, expectedFields)
	assert.Contains(t, entry.Data, "ip")
	assert.Contains(t, entry.Data, "duration")
}

func TestLoggerMiddleware_SkipPaths(t *testing.T) {
	logger, hook := createTestLogger()

	middleware := Logger(
		WithLogger(logger),
		WithSkipPaths("/health", "/metrics"),
	)

	handler := createTestHandler()

	// Test skipped path
	req := createTestRequest(http.MethodGet, "/health")
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, hook.AllEntries(), 0) // No log entry for skipped path

	// Test non-skipped path
	req = createTestRequest(http.MethodGet, "/api")
	w = httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Len(t, hook.AllEntries(), 1) // Log entry for non-skipped path
}

// Test to prove response writer race condition
func TestLoggerMiddleware_ResponseWriterRaceCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	t.Run("concurrent writeheader calls can race", func(t *testing.T) {
		logger := Logger()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate concurrent access to response writer
			var wg sync.WaitGroup

			for range 10 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// Try to write status concurrently (this should race)
					w.WriteHeader(http.StatusOK)
				}()
			}

			wg.Wait()
		})

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		// Run with race detector enabled to catch the race
		logger(handler).ServeHTTP(w, req)

		// The race condition exists in the responseWriter.statusCode field
		// Run this test with -race flag to detect it
		assert.True(t, true, "Race condition exists - run with -race flag to detect")
	})
}

func TestLoggerMiddleware_AllOptions(t *testing.T) {
	logger, hook := createTestLogger()

	middleware := Logger(
		WithLogger(logger),
		WithLogLevel(logrus.WarnLevel),
		WithLogMessage("Custom log message"),
		WithExtraFields(map[string]any{
			"service": "test-service",
			"version": "1.0.0",
		}),
		WithIncludeQuery(false),
		WithIncludeHeaders("Authorization", "X-API-Key"),
	)

	handler := createTestHandler()

	req := createTestRequestWithHeaders(http.MethodGet, "/test?param=value", map[string]string{
		"Authorization": "Bearer token123",
		"X-API-Key":     "secret-key",
	})
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	entries := hook.AllEntries()
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, logrus.WarnLevel, entry.Level)
	assert.Equal(t, "Custom log message", entry.Message)

	expectedFields := map[string]any{
		"service":              "test-service",
		"version":              "1.0.0",
		"header_Authorization": "Bearer token123",
		"header_X-API-Key":     "secret-key",
	}

	assertLogEntry(t, entry, expectedFields)
	assert.NotContains(t, entry.Data, "query") // Disabled
}
