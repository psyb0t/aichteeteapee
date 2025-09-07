package aichteeteapee

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRequestContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    string
		want        bool
	}{
		{
			name:        "exact match",
			contentType: "application/json",
			expected:    "application/json",
			want:        true,
		},
		{
			name:        "with charset parameter",
			contentType: "application/json; charset=utf-8",
			expected:    "application/json",
			want:        true,
		},
		{
			name:        "with multiple parameters",
			contentType: "application/json; charset=utf-8; boundary=something",
			expected:    "application/json",
			want:        true,
		},
		{
			name:        "case insensitive match",
			contentType: "APPLICATION/JSON",
			expected:    "application/json",
			want:        true,
		},
		{
			name:        "no match",
			contentType: "text/plain",
			expected:    "application/json",
			want:        false,
		},
		{
			name:        "empty content type",
			contentType: "",
			expected:    "application/json",
			want:        false,
		},
		{
			name:        "whitespace handling",
			contentType: " application/json ; charset=utf-8",
			expected:    "application/json",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result := IsRequestContentType(req, tt.expected)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestIsRequestContentTypeJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{
			name:        "valid JSON content type",
			contentType: "application/json",
			want:        true,
		},
		{
			name:        "JSON with charset",
			contentType: "application/json; charset=utf-8",
			want:        true,
		},
		{
			name:        "case insensitive",
			contentType: "APPLICATION/JSON",
			want:        true,
		},
		{
			name:        "not JSON",
			contentType: "text/plain",
			want:        false,
		},
		{
			name:        "empty",
			contentType: "",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result := IsRequestContentTypeJSON(req)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestIsRequestContentTypeXML(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{
			name:        "valid XML content type",
			contentType: "application/xml",
			want:        true,
		},
		{
			name:        "XML with charset",
			contentType: "application/xml; charset=utf-8",
			want:        true,
		},
		{
			name:        "not XML",
			contentType: "application/json",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result := IsRequestContentTypeXML(req)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestIsRequestContentTypeFormData(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{
			name:        "valid form data content type",
			contentType: "application/x-www-form-urlencoded",
			want:        true,
		},
		{
			name:        "form data with charset",
			contentType: "application/x-www-form-urlencoded; charset=utf-8",
			want:        true,
		},
		{
			name:        "not form data",
			contentType: "application/json",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result := IsRequestContentTypeApplicationFormURLEncoded(req)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestIsRequestContentTypeMultipartForm(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{
			name:        "valid multipart form content type",
			contentType: "multipart/form-data; boundary=something",
			want:        true,
		},
		{
			name:        "multipart form case insensitive",
			contentType: "MULTIPART/FORM-DATA; boundary=something",
			want:        true,
		},
		{
			name:        "not multipart",
			contentType: "application/json",
			want:        false,
		},
		{
			name:        "empty",
			contentType: "",
			want:        false,
		},
		{
			name:        "multipart without form-data",
			contentType: "multipart/mixed; boundary=something",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result := IsRequestContentTypeMultipartFormData(req)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetRequestID(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		expected string
	}{
		{
			name: "request ID present in context",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), ContextKeyRequestID, "test-request-id-123")
			},
			expected: "test-request-id-123",
		},
		{
			name: "request ID missing from context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			expected: "",
		},
		{
			name: "request ID wrong type in context",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), ContextKeyRequestID, 12345)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req = req.WithContext(tt.setupCtx())

			result := GetRequestID(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func() *http.Request
		expected   string
	}{
		{
			name: "X-Forwarded-For header present",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
				return req
			},
			expected: "203.0.113.195",
		},
		{
			name: "X-Forwarded-For with whitespace",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "  192.168.1.100  , 10.0.0.1")
				return req
			},
			expected: "192.168.1.100",
		},
		{
			name: "X-Real-IP header present",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "192.168.1.200")
				return req
			},
			expected: "192.168.1.200",
		},
		{
			name: "X-Real-IP with whitespace",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "  172.16.0.1  ")
				return req
			},
			expected: "172.16.0.1",
		},
		{
			name: "RemoteAddr without port",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "127.0.0.1"
				return req
			},
			expected: "127.0.0.1",
		},
		{
			name: "RemoteAddr with port",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.1:8080"
				return req
			},
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			result := GetClientIP(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}
