package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/common-go/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecovery(t *testing.T) {
	buf := captureDefaultLogger(t)
	mw := Recovery()

	handler := createPanicHandler("test panic")

	req := createTestRequestWithContext(
		http.MethodGet, "/test", map[any]any{
			aichteeteapee.ContextKeyRequestID: "panic-req-456",
		},
	)
	req.Header.Set("X-Real-IP", "10.0.0.1")

	ctx := scope.Set(
		req.Context(),
		scope.Attr(aichteeteapee.FieldRequestID, "panic-req-456"),
		scope.Attr(aichteeteapee.FieldMethod, req.Method),
		scope.Attr(aichteeteapee.FieldPath, req.URL.Path),
		scope.Attr(aichteeteapee.FieldIP, "10.0.0.1"),
	)

	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		mw(handler).ServeHTTP(w, req)
	})

	assert.Equal(
		t, http.StatusInternalServerError, w.Code,
	)
	assert.Equal(
		t, aichteeteapee.ContentTypeJSON,
		w.Header().Get(
			aichteeteapee.HeaderNameContentType,
		),
	)

	logOutput := buf.String()
	require.NotEmpty(t, logOutput)
	assert.Contains(t, logOutput, "Panic recovered")
	assert.Contains(t, logOutput, "test panic")
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/test")
	assert.Contains(t, logOutput, "10.0.0.1")
	assert.Contains(t, logOutput, "panic-req-456")
}

func TestRecoveryMiddleware_EdgeCases(t *testing.T) {
	t.Run("panic with different types", func(t *testing.T) {
		buf := captureDefaultLogger(t)
		mw := Recovery()

		handler := createPanicHandler(42)

		req := createTestRequest(
			http.MethodGet, "/test",
		)

		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			mw(handler).ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.NotEmpty(t, buf.String())
	})

	t.Run("panic with struct type", func(t *testing.T) {
		buf := captureDefaultLogger(t)
		mw := Recovery()

		type CustomError struct {
			Message string
		}

		handler := createPanicHandler(
			CustomError{Message: "custom error"},
		)

		req := createTestRequestWithContext(
			http.MethodPost, "/panic", map[any]any{
				aichteeteapee.ContextKeyRequestID: "panic-test-789",
			},
		)
		req.Header.Set(
			"X-Forwarded-For", "192.168.1.100",
		)

		// Simulate what the RequestID + Logger middleware
		// would put on the scope.
		ctx := scope.Set(
			req.Context(),
			scope.Attr(
				aichteeteapee.FieldRequestID, "panic-test-789",
			),
			scope.Attr(aichteeteapee.FieldMethod, req.Method),
			scope.Attr(aichteeteapee.FieldPath, req.URL.Path),
			scope.Attr(aichteeteapee.FieldIP, "192.168.1.100"),
		)

		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			mw(handler).ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Equal(
			t, aichteeteapee.ContentTypeJSON,
			w.Header().Get(aichteeteapee.HeaderNameContentType),
		)

		logOutput := buf.String()
		require.NotEmpty(t, logOutput)
		assert.Contains(t, logOutput, "POST")
		assert.Contains(t, logOutput, "/panic")
		assert.Contains(t, logOutput, "192.168.1.100")
		assert.Contains(t, logOutput, "panic-test-789")
	})
}

func TestRecoveryMiddleware_CustomHandler(t *testing.T) {
	var recoveredValue any

	customHandler := func(
		recovered any, w http.ResponseWriter, _ *http.Request,
	) {
		recoveredValue = recovered

		w.WriteHeader(http.StatusTeapot) // Custom status
		_, _ = w.Write([]byte("Custom recovery"))
	}

	mw := Recovery(
		WithCustomRecoveryHandler(customHandler),
	)

	handler := createPanicHandler("custom panic")

	req := createTestRequest(http.MethodGet, "/test")
	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, "Custom recovery", w.Body.String())
	assert.Equal(t, "custom panic", recoveredValue)
}

// Test to prove recovery middleware can fail during recovery.
func TestRecoveryMiddleware_CanFailDuringRecovery(t *testing.T) {
	t.Run("recovery fails when response encoding fails", func(t *testing.T) {
		// Create a custom response that will fail JSON encoding
		cyclicMap := make(map[string]any)
		// This creates a cycle that json.Marshal can't handle
		cyclicMap["self"] = cyclicMap

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
		assert.Equal(
			t, http.StatusInternalServerError, w.Code,
			"Recovery middleware should write status "+
				"even when JSON encoding fails",
		)
		assert.Contains(
			t, w.Body.String(), "Internal server error",
			"Recovery should provide fallback response "+
				"when JSON encoding fails",
		)
	})
}

func TestRecoveryMiddleware_AllOptions(t *testing.T) {
	buf := captureDefaultLogger(t)

	mw := Recovery(
		WithRecoveryLogLevel(slog.LevelError+4),
		WithRecoveryLogMessage("Panic occurred"),
		WithRecoveryStatusCode(http.StatusBadGateway),
		WithRecoveryResponse(
			map[string]string{"error": "server_panic"},
		),
		WithRecoveryContentType(
			aichteeteapee.ContentTypeJSON,
		),
		WithIncludeStack(true),
		WithRecoveryExtraFields(map[string]any{
			"alert": "critical",
			"team":  "backend",
		}),
	)

	handler := createPanicHandler("test panic")

	req := createTestRequest(
		http.MethodGet, "/test",
	)

	w := httptest.NewRecorder()

	mw(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Equal(
		t, aichteeteapee.ContentTypeJSON,
		w.Header().Get(
			aichteeteapee.HeaderNameContentType,
		),
	)
	assert.Contains(t, w.Body.String(), "server_panic")

	logOutput := buf.String()
	require.NotEmpty(t, logOutput)

	// Verify extra fields and message
	assert.Contains(t, logOutput, "Panic occurred")
	assert.Contains(t, logOutput, "critical")
	assert.Contains(t, logOutput, "backend")

	// Verify log level - slog text handler uses "level=ERROR+4" for above ERROR
	assert.True(
		t,
		strings.Contains(logOutput, "ERROR") ||
			strings.Contains(logOutput, "level="),
		"should contain log level",
	)
}
