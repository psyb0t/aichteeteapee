package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestGroup_Handle(t *testing.T) {
	mux := http.NewServeMux()
	logger := logrus.StandardLogger()
	group := NewGroup(mux, "/api", logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	group.Handle(http.MethodGet, "/test", handler)

	// Test the registered route
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())
}

func TestGroup_HandleFunc(t *testing.T) {
	mux := http.NewServeMux()
	logger := logrus.StandardLogger()
	group := NewGroup(mux, "/api", logger)

	handlerFunc := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("handler func response"))
	}

	group.HandleFunc(http.MethodPost, "/test", handlerFunc)

	// Test the registered route
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "handler func response", w.Body.String())
}

func TestGroup_NestedGroups(t *testing.T) {
	mux := http.NewServeMux()
	logger := logrus.StandardLogger()

	// Create nested groups: /api/v1/users
	apiGroup := NewGroup(mux, "/api", logger)
	v1Group := apiGroup.Group("/v1")
	usersGroup := v1Group.Group("/users")

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("nested response"))
	})

	usersGroup.GET("/{id}", handler)

	// Test the nested route
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nested response", w.Body.String())
}
