package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name              string
		existingRequestID string
		expectNewID       bool
	}{
		{
			name:              "generates new request ID when none exists",
			existingRequestID: "",
			expectNewID:       true,
		},
		{
			name:              "preserves existing request ID",
			existingRequestID: "existing-id-123",
			expectNewID:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := RequestID()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqID := GetRequestID(r)
				assert.NotEmpty(t, reqID)

				if tt.expectNewID {
					assert.NotEqual(t, tt.existingRequestID, reqID)
				} else {
					assert.Equal(t, tt.existingRequestID, reqID)
				}

				// Check response header
				assert.Equal(t, reqID, w.Header().Get(aichteeteapee.HeaderNameXRequestID))
				w.WriteHeader(http.StatusOK)
			})

			req := createTestRequest(http.MethodGet, "/test")
			if tt.existingRequestID != "" {
				req.Header.Set(aichteeteapee.HeaderNameXRequestID, tt.existingRequestID)
			}

			w := httptest.NewRecorder()
			middleware(handler).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}
