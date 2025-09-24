package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChain(t *testing.T) {
	logger, hook := createTestLogger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that request ID was added by middleware
		reqID := GetRequestID(r)
		assert.NotEmpty(t, reqID)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("chained response"))
	})

	// Chain multiple middlewares
	chainedHandler := Chain(handler,
		RequestID(),
		Logger(WithLogger(logger)),
	)

	req := createTestRequest(http.MethodGet, "/test")
	w := httptest.NewRecorder()

	chainedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "chained response", w.Body.String())
	assert.NotEmpty(t, w.Header().Get(aichteeteapee.HeaderNameXRequestID))

	// Check that logger middleware logged the request
	entries := hook.AllEntries()
	require.Len(t, entries, 1)
}
