package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

// Common test utilities and shared test data

// createTestHandler creates a simple test handler.
func createTestHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})
}

// createPanicHandler creates a handler that panics.
func createPanicHandler(panicValue any) http.HandlerFunc {
	return http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic(panicValue)
	})
}

// createDelayedHandler creates a handler with a specified delay.
func createDelayedHandler(delay time.Duration) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			w.WriteHeader(http.StatusGatewayTimeout)

			return
		case <-time.After(delay):
			w.WriteHeader(http.StatusOK)
		}
	})
}

// createTestRequest creates a basic test request.
func createTestRequest(method, path string) *http.Request {
	return httptest.NewRequest(method, path, nil)
}

// createTestRequestWithContext creates a test request with context values.
func createTestRequestWithContext(
	method, path string, contextValues map[any]any,
) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := req.Context()

	for key, value := range contextValues {
		ctx = context.WithValue(ctx, key, value)
	}

	return req.WithContext(ctx)
}

// createTestRequestWithHeaders creates a test request with headers.
func createTestRequestWithHeaders(
	method, path string, headers map[string]string,
) *http.Request {
	req := httptest.NewRequest(method, path, nil)

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req
}

// createTestLogger creates a test logger with hook for capturing logs.
func createTestLogger() (*logrus.Logger, *test.Hook) {
	return test.NewNullLogger()
}

// assertSecurityHeaders checks that security headers are set correctly.
func assertSecurityHeaders(
	t *testing.T, w *httptest.ResponseRecorder, expectedHeaders map[string]string,
) {
	t.Helper()

	for header, expectedValue := range expectedHeaders {
		assert.Equal(
			t, expectedValue, w.Header().Get(header),
			"Header %s should match", header,
		)
	}
}

// assertCORSHeaders checks that CORS headers are set correctly.
func assertCORSHeaders(
	t *testing.T, w *httptest.ResponseRecorder, expectedOrigin string,
	hasMethods, hasHeaders bool,
) {
	t.Helper()

	if expectedOrigin != "" {
		assert.Equal(t, expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
	}

	if hasMethods {
		assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
	}

	if hasHeaders {
		assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
	}
}

// assertLogEntry checks that a log entry contains expected fields.
func assertLogEntry(
	t *testing.T, entry *logrus.Entry, expectedFields map[string]any,
) {
	t.Helper()

	for field, expectedValue := range expectedFields {
		assert.Equal(
			t, expectedValue, entry.Data[field], "Log field %s should match", field,
		)
	}
}

// getTestData returns common test data.
func getTestData() struct {
	ValidBasicAuth   string
	InvalidBasicAuth string
	TestUserAgent    string
	TestOrigin       string
} {
	return struct {
		ValidBasicAuth   string
		InvalidBasicAuth string
		TestUserAgent    string
		TestOrigin       string
	}{
		ValidBasicAuth:   "Basic YWRtaW46c2VjcmV0", // admin:secret in base64
		InvalidBasicAuth: "Basic dXNlcjp3cm9uZw==", // user:wrong in base64
		TestUserAgent:    "Test-Agent/1.0",
		TestOrigin:       "https://example.com",
	}
}
