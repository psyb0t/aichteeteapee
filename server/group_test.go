package server

import (
	"net/http"
	"testing"

	"github.com/psyb0t/aichteeteapee/server/middleware"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCleanPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "root path",
			input:    "/",
			expected: "",
		},
		{
			name:     "simple path",
			input:    "/api",
			expected: "/api",
		},
		{
			name:     "path with trailing slash",
			input:    "/api/",
			expected: "/api",
		},
		{
			name:     "path without leading slash",
			input:    "api",
			expected: "api",
		},
		{
			name:     "complex path",
			input:    "/api/v1/",
			expected: "/api/v1",
		},
		{
			name:     "path with double slashes",
			input:    "//api//v1//",
			expected: "/api/v1",
		},
		{
			name:     "path with dot",
			input:    "/api/./v1",
			expected: "/api/v1",
		},
		{
			name:     "just dot",
			input:    ".",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanPrefix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinPaths(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		sub      string
		expected string
	}{
		{
			name:     "empty sub path",
			base:     "/api",
			sub:      "",
			expected: "/api",
		},
		{
			name:     "root sub path",
			base:     "/api",
			sub:      "/",
			expected: "/api",
		},
		{
			name:     "simple join",
			base:     "/api",
			sub:      "/v1",
			expected: "/api/v1",
		},
		{
			name:     "sub without leading slash",
			base:     "/api",
			sub:      "v1",
			expected: "/api/v1",
		},
		{
			name:     "empty base",
			base:     "",
			sub:      "/users",
			expected: "/users",
		},
		{
			name:     "both empty results in empty",
			base:     "",
			sub:      "",
			expected: "",
		},
		{
			name:     "complex paths",
			base:     "/api/v1",
			sub:      "/users/{id}",
			expected: "/api/v1/users/{id}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinPaths(tt.base, tt.sub)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildRoute(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		expected string
	}{
		{
			name:     "GET with path",
			method:   http.MethodGet,
			path:     "/users",
			expected: "GET /users",
		},
		{
			name:     "POST with path",
			method:   http.MethodPost,
			path:     "/users",
			expected: "POST /users",
		},
		{
			name:     "method with spaces",
			method:   " GET ",
			path:     "/users",
			expected: "GET /users",
		},
		{
			name:     "lowercase method",
			method:   "get",
			path:     "/users",
			expected: "GET /users",
		},
		{
			name:     "empty method",
			method:   "",
			path:     "/users",
			expected: "/users",
		},
		{
			name:     "empty path defaults to root",
			method:   http.MethodGet,
			path:     "",
			expected: "GET /",
		},
		{
			name:     "both empty",
			method:   "",
			path:     "",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRoute(tt.method, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewGroup(t *testing.T) {
	mux := http.NewServeMux()
	logger := logrus.StandardLogger()

	group := NewGroup(mux, "/api", logger)

	assert.NotNil(t, group)
	assert.Equal(t, "/api", group.prefix)
	assert.Same(t, mux, group.mux)
	assert.Same(t, logger, group.logger)
	assert.Empty(t, group.middlewares)
}

func TestGroup_Group(t *testing.T) {
	mux := http.NewServeMux()
	logger := logrus.StandardLogger()

	parentGroup := NewGroup(mux, "/api", logger)
	subGroup := parentGroup.Group("/v1")

	assert.NotNil(t, subGroup)
	assert.Equal(t, "/api/v1", subGroup.prefix)
	assert.Same(t, mux, subGroup.mux)
	assert.Same(t, logger, subGroup.logger)
}

func TestGroup_HTTPMethods(t *testing.T) {
	mux := http.NewServeMux()
	logger := logrus.StandardLogger()
	group := NewGroup(mux, "/api", logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name   string
		method func(string, http.HandlerFunc, ...middleware.Middleware)
		path   string
	}{
		{
			name:   "GET method",
			method: group.GET,
			path:   "/users",
		},
		{
			name:   "POST method",
			method: group.POST,
			path:   "/users",
		},
		{
			name:   "PUT method",
			method: group.PUT,
			path:   "/users/{id}",
		},
		{
			name:   "PATCH method",
			method: group.PATCH,
			path:   "/users/{id}",
		},
		{
			name:   "DELETE method",
			method: group.DELETE,
			path:   "/users/{id}",
		},
		{
			name:   "OPTIONS method",
			method: group.OPTIONS,
			path:   "/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			assert.NotPanics(t, func() {
				tt.method(tt.path, handler)
			})
		})
	}
}
