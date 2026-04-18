package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/common-go/slogging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChain(t *testing.T) {
	logger, buf := createTestLogger()

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
		Logger(),
	)

	req := createTestRequest(http.MethodGet, "/test")

	ctx := slogging.GetCtxWithLogger(
		req.Context(), logger,
	)

	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	chainedHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "chained response", w.Body.String())
	assert.NotEmpty(t, w.Header().Get(aichteeteapee.HeaderNameXRequestID))

	// Check that logger middleware logged the request
	logOutput := buf.String()
	require.NotEmpty(
		t, logOutput, "should have log output from logger middleware",
	)
}
