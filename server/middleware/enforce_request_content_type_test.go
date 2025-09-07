package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestEnforceRequestContentType(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		contentType      string
		allowedTypes     []string
		expectedStatus   int
		expectedResponse string
	}{
		{
			name:           "GET request bypasses enforcement",
			method:         http.MethodGet,
			contentType:    "",
			allowedTypes:   []string{aichteeteapee.ContentTypeJSON},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "HEAD request bypasses enforcement",
			method:         http.MethodHead,
			contentType:    "",
			allowedTypes:   []string{aichteeteapee.ContentTypeJSON},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "DELETE request bypasses enforcement",
			method:         http.MethodDelete,
			contentType:    "",
			allowedTypes:   []string{aichteeteapee.ContentTypeJSON},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with valid JSON content type",
			method:         http.MethodPost,
			contentType:    aichteeteapee.ContentTypeJSON,
			allowedTypes:   []string{aichteeteapee.ContentTypeJSON},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with JSON content type with charset",
			method:         http.MethodPost,
			contentType:    aichteeteapee.ContentTypeJSON + "; charset=utf-8",
			allowedTypes:   []string{aichteeteapee.ContentTypeJSON},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with multiple allowed types",
			method:         http.MethodPost,
			contentType:    aichteeteapee.ContentTypeXML,
			allowedTypes:   []string{aichteeteapee.ContentTypeJSON, aichteeteapee.ContentTypeXML},
			expectedStatus: http.StatusOK,
		},
		{
			name:             "POST with missing content type",
			method:           http.MethodPost,
			contentType:      "",
			allowedTypes:     []string{aichteeteapee.ContentTypeJSON},
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: aichteeteapee.ErrorCodeMissingContentType,
		},
		{
			name:             "POST with invalid content type",
			method:           http.MethodPost,
			contentType:      aichteeteapee.ContentTypeTextPlain,
			allowedTypes:     []string{aichteeteapee.ContentTypeJSON},
			expectedStatus:   http.StatusUnsupportedMediaType,
			expectedResponse: aichteeteapee.ErrorCodeUnsupportedContentType,
		},
		{
			name:           "Case insensitive content type matching",
			method:         http.MethodPost,
			contentType:    "APPLICATION/JSON",
			allowedTypes:   []string{aichteeteapee.ContentTypeJSON},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := EnforceRequestContentType(tt.allowedTypes...)
			handler := createTestHandler()

			req := createTestRequest(tt.method, "/test")
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			middleware(handler).ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedResponse != "" {
				assert.Contains(t, w.Body.String(), tt.expectedResponse)
			}
		})
	}
}

func TestEnforceRequestContentTypeJSON(t *testing.T) {
	// Test the convenience function for JSON-only enforcement
	jsonOnlyMiddleware := EnforceRequestContentTypeJSON()
	handler := createTestHandler()

	tests := []struct {
		name           string
		method         string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "GET request bypasses enforcement",
			method:         http.MethodGet,
			contentType:    "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with valid JSON content type",
			method:         http.MethodPost,
			contentType:    aichteeteapee.ContentTypeJSON,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with JSON and charset",
			method:         http.MethodPost,
			contentType:    aichteeteapee.ContentTypeJSON + "; charset=utf-8",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST with missing content type",
			method:         http.MethodPost,
			contentType:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST with invalid content type",
			method:         http.MethodPost,
			contentType:    "text/plain",
			expectedStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createTestRequest(tt.method, "/test")
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			jsonOnlyMiddleware(handler).ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
