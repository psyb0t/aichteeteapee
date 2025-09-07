package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecovery(t *testing.T) {
	logger, hook := createTestLogger()
	middleware := Recovery(
		WithRecoveryLogger(logger),
	)

	handler := createPanicHandler("test panic")

	req := createTestRequestWithHeaders(http.MethodGet, "/test", map[string]string{
		"X-Real-IP": "10.0.0.1",
	})

	// Add request ID to context
	req = createTestRequestWithContext(http.MethodGet, "/test", map[any]any{
		aichteeteapee.ContextKeyRequestID: "panic-req-456",
	})
	req.Header.Set("X-Real-IP", "10.0.0.1")

	w := httptest.NewRecorder()

	// Should not panic
	assert.NotPanics(t, func() {
		middleware(handler).ServeHTTP(w, req)
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))

	// Check that panic was logged
	entries := hook.AllEntries()
	require.Len(t, entries, 1)

	entry := entries[0]
	expectedFields := map[string]any{
		"error":     "test panic",
		"method":    http.MethodGet,
		"path":      "/test",
		"ip":        "10.0.0.1",
		"requestId": "panic-req-456",
	}

	assert.Equal(t, logrus.ErrorLevel, entry.Level)
	assert.Contains(t, entry.Message, "Panic recovered")
	assertLogEntry(t, entry, expectedFields)
}

func TestRecoveryMiddleware_EdgeCases(t *testing.T) {
	t.Run("panic with different types", func(t *testing.T) {
		logger, hook := createTestLogger()
		middleware := Recovery(WithRecoveryLogger(logger))

		handler := createPanicHandler(42) // Non-string panic

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			middleware(handler).ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		entries := hook.AllEntries()
		assert.NotEmpty(t, entries)
	})

	t.Run("panic with struct type", func(t *testing.T) {
		logger, hook := createTestLogger()
		middleware := Recovery(WithRecoveryLogger(logger))

		type CustomError struct {
			Message string
		}

		handler := createPanicHandler(CustomError{Message: "custom error"})

		req := createTestRequestWithHeaders(http.MethodPost, "/panic", map[string]string{
			"X-Forwarded-For": "192.168.1.100",
		})
		req = createTestRequestWithContext(http.MethodPost, "/panic", map[any]any{
			aichteeteapee.ContextKeyRequestID: "panic-test-789",
		})
		req.Header.Set("X-Forwarded-For", "192.168.1.100")

		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			middleware(handler).ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))

		entries := hook.AllEntries()
		require.Len(t, entries, 1)

		entry := entries[0]
		expectedFields := map[string]any{
			"method":    http.MethodPost,
			"path":      "/panic",
			"ip":        "192.168.1.100",
			"requestId": "panic-test-789",
		}

		assert.Equal(t, logrus.ErrorLevel, entry.Level)
		assertLogEntry(t, entry, expectedFields)
	})
}

func TestRecoveryMiddleware_CustomHandler(t *testing.T) {
	var recoveredValue any
	customHandler := func(recovered any, w http.ResponseWriter, r *http.Request) {
		recoveredValue = recovered
		w.WriteHeader(http.StatusTeapot) // Custom status
		w.Write([]byte("Custom recovery"))
	}

	middleware := Recovery(
		WithCustomRecoveryHandler(customHandler),
	)

	handler := createPanicHandler("custom panic")

	req := createTestRequest(http.MethodGet, "/test")
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "Custom recovery", w.Body.String())
	assert.Equal(t, "custom panic", recoveredValue)
}

// Test to prove recovery middleware can fail during recovery
func TestRecoveryMiddleware_CanFailDuringRecovery(t *testing.T) {
	t.Run("recovery fails when response encoding fails", func(t *testing.T) {
		// Create a custom response that will fail JSON encoding
		cyclicMap := make(map[string]any)
		cyclicMap["self"] = cyclicMap // This creates a cycle that json.Marshal can't handle

		recovery := Recovery(
			WithRecoveryResponse(cyclicMap), // This will fail to encode
			WithRecoveryContentType(aichteeteapee.ContentTypeJSON),
		)

		panicHandler := createPanicHandler("test panic")

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		// This should panic during JSON encoding of the response
		recovery(panicHandler).ServeHTTP(w, req)

		// The response should use fallback when JSON encoding fails
		// This proves recovery handles encoding failures gracefully
		assert.Equal(t, http.StatusInternalServerError, w.Code, "Recovery middleware should write status even when JSON encoding fails")
		assert.Contains(t, w.Body.String(), "Internal server error", "Recovery should provide fallback response when JSON encoding fails")
	})
}

func TestRecoveryMiddleware_AllOptions(t *testing.T) {
	logger, hook := createTestLogger()

	middleware := Recovery(
		WithRecoveryLogger(logger),
		WithRecoveryLogLevel(logrus.FatalLevel),
		WithRecoveryLogMessage("Panic occurred"),
		WithRecoveryStatusCode(http.StatusBadGateway),
		WithRecoveryResponse(map[string]string{"error": "server_panic"}),
		WithRecoveryContentType("application/json"),
		WithIncludeStack(true), // Note: not implemented yet
		WithRecoveryExtraFields(map[string]any{
			"alert": "critical",
			"team":  "backend",
		}),
	)

	handler := createPanicHandler("test panic")

	req := createTestRequest(http.MethodGet, "/test")
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "server_panic")

	entries := hook.AllEntries()
	require.Len(t, entries, 1)

	entry := entries[0]
	expectedFields := map[string]any{
		"alert": "critical",
		"team":  "backend",
	}

	assert.Equal(t, logrus.FatalLevel, entry.Level)
	assert.Equal(t, "Panic occurred", entry.Message)
	assertLogEntry(t, entry, expectedFields)
}
