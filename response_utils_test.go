package aichteeteapee

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       any
		expectJSON string
	}{
		{
			name:       "simple string response",
			statusCode: http.StatusOK,
			data:       "hello world",
			expectJSON: "\"hello world\"\n",
		},
		{
			name:       "struct response",
			statusCode: http.StatusCreated,
			data:       map[string]any{"message": "success", "id": 123},
			expectJSON: "{\n  \"id\": 123,\n  \"message\": \"success\"\n}\n",
		},
		{
			name:       "array response",
			statusCode: http.StatusAccepted,
			data:       []string{"item1", "item2", "item3"},
			expectJSON: "[\n  \"item1\",\n  \"item2\",\n  \"item3\"\n]\n",
		},
		{
			name:       "null response",
			statusCode: http.StatusNoContent,
			data:       nil,
			expectJSON: "null\n",
		},
		{
			name:       "error status code",
			statusCode: http.StatusInternalServerError,
			data:       map[string]string{"error": "internal server error"},
			expectJSON: "{\n  \"error\": \"internal server error\"\n}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteJSON(w, tt.statusCode, tt.data)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, ContentTypeJSON, w.Header().Get(HeaderNameContentType))
			assert.Equal(t, tt.expectJSON, w.Body.String())
		})
	}
}

func TestWriteJSON_EncodingError(t *testing.T) {
	w := httptest.NewRecorder()

	// Create a circular reference that will cause JSON encoding to fail
	circular := make(map[string]any)
	circular["self"] = circular

	// This should not panic, but will log an error
	WriteJSON(w, http.StatusOK, circular)

	// Should still set headers and status code
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, ContentTypeJSON, w.Header().Get(HeaderNameContentType))
	// Body might be empty or partial due to encoding error
}
