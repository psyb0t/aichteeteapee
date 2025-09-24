package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/aichteeteapee/server/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_Integration(t *testing.T) {
	// Integration test using httptest
	server, err := New()
	require.NoError(t, err)

	// Register handlers after server creation
	rootGroup := server.GetRootGroup()
	rootGroup.HandleFunc(http.MethodGet, "/health", server.HealthHandler)
	rootGroup.HandleFunc(http.MethodPost, "/echo", server.EchoHandler)

	// Create test server
	testServer := httptest.NewServer(server.GetMux())
	defer testServer.Close()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "health endpoint",
			method:         http.MethodGet,
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "echo endpoint",
			method:         http.MethodPost,
			path:           "/echo",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "not found",
			method:         http.MethodGet,
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				context.Background(), tt.method, testServer.URL+tt.path, nil,
			)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestStaticFileServing_Integration(t *testing.T) {
	// Test the actual HTTP server behavior using real server.Start()
	// with full middleware stack
	router := &Router{
		Static: []StaticRouteConfig{
			{Dir: "./.fixtures/static", Path: "/static"},
		},
		GlobalMiddlewares: []middleware.Middleware{
			middleware.Recovery(),
			middleware.RequestID(),
			middleware.Logger(),
			middleware.SecurityHeaders(),
			middleware.Timeout(),
			middleware.CORS(),
		},
	}

	server, err := NewWithConfig(Config{
		ListenAddress: "127.0.0.1:0", // Use random available port
	})
	require.NoError(t, err)

	// Start the actual server in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverStarted := make(chan string, 1)
	serverError := make(chan error, 1)

	go func() {
		if err := server.Start(ctx, router); err != nil {
			if !errors.Is(err, context.Canceled) {
				serverError <- err
			}
		}
	}()

	// Wait for server to start and get the actual address
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if addr := server.GetListenerAddr(); addr != nil {
					serverStarted <- "http://" + addr.String()

					return
				}

				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	var baseURL string
	select {
	case baseURL = <-serverStarted:
		// Server started successfully
	case err := <-serverError:
		t.Fatalf("Server failed to start: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("Server failed to start within 5 seconds")
	}

	// Ensure server stops when test completes
	defer func() {
		cancel()

		_ = server.Stop(context.Background())
	}()

	tests := []struct {
		name         string
		path         string
		expectError  bool
		expectedBody string
	}{
		{
			name: "index directory serves HTML",
			path: "/static/index/",
			expectedBody: `<!DOCTYPE html>
<html>
<head>
    <title>Index Page</title>
</head>
<body>
    <h1>Welcome to Index Directory</h1>
    <p>This is the index.html file served for directory access.</p>
</body>
</html>`,
		},
		{
			name: "direct file access works",
			path: "/static/index/data.txt",
			expectedBody: `This is a data file in the index directory.
It contains some text content for testing.
Line 3 of data.`,
		},
		{
			name:        "noindex directory returns error",
			path:        "/static/noindex/",
			expectError: true,
		},
		{
			name: "direct file in noindex works",
			path: "/static/noindex/readme.txt",
			expectedBody: `This directory does NOT have an index.html file.
Accessing this directory should return a 403 Forbidden error.
This prevents directory listing for security.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(
				context.Background(), http.MethodGet, baseURL+tt.path, nil,
			)
			require.NoError(t, err)

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)

			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			if tt.expectError {
				assert.Equal(t, http.StatusForbidden, resp.StatusCode)
				assert.Equal(
					t, aichteeteapee.ContentTypeJSON,
					resp.Header.Get(aichteeteapee.HeaderNameContentType),
				)

				var errorResponse aichteeteapee.ErrorResponse

				err := json.Unmarshal(body, &errorResponse)
				require.NoError(t, err)
				assert.Equal(
					t, aichteeteapee.ErrorCodeDirectoryListingNotSupported,
					errorResponse.Code,
				)
			} else {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Equal(t, tt.expectedBody, string(body))
			}
		})
	}
}

func TestStaticPathTraversalSecurity_Integration(t *testing.T) {
	server, err := NewWithConfig(Config{
		ListenAddress: "127.0.0.1:0",
	})
	require.NoError(t, err)

	// Set up router with static file configuration
	router := &Router{
		Static: []StaticRouteConfig{
			{Dir: ".fixtures/static", Path: "/static"},
		},
	}

	// Manually set router and setup routes (since we're not calling Start)
	server.router = router
	if len(router.GlobalMiddlewares) > 0 {
		server.setupGlobalMiddlewares()
	}

	server.setupRoutes()

	// Test path traversal attempts that should be caught by our validation
	maliciousPaths := []struct {
		path           string
		expectedStatus int
		expectedError  string
	}{
		{
			path:           "/static/../../../etc/passwd",
			expectedStatus: http.StatusMovedPermanently, // HTTP router redirects first
		},
		{
			path: "/static/..%2F..%2F..%2Fetc%2Fpasswd",
			// URL encoding doesn't bypass our protection
			expectedStatus: http.StatusNotFound,
			expectedError:  aichteeteapee.ErrorCodeFileNotFound,
		},
		{
			path:           "/static/../server_test.go",
			expectedStatus: http.StatusMovedPermanently, // HTTP router redirects first
		},
		{
			path:           "/static/index/../../../http/defaults.go",
			expectedStatus: http.StatusMovedPermanently, // HTTP router redirects first
		},
	}

	for _, tt := range maliciousPaths {
		t.Run("path traversal: "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			server.GetMux().ServeHTTP(w, req)

			assert.Equal(
				t, tt.expectedStatus, w.Code,
				"Status code mismatch for %s", tt.path,
			)

			if tt.expectedError != "" {
				assert.Equal(
					t, aichteeteapee.ContentTypeJSON,
					w.Header().Get(aichteeteapee.HeaderNameContentType),
				)

				var errorResponse aichteeteapee.ErrorResponse

				err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedError, errorResponse.Code)
			}
		})
	}

	// Test paths that would bypass HTTP router redirects
	// but should be caught by our validation
	dangerousPaths := []string{
		"/static/index/../../server_test.go",
		// Try to access files outside static dir
		"/static/noindex/../../defaults.go", // Another attempt
	}

	for _, dangerousPath := range dangerousPaths {
		t.Run("clean path traversal: "+dangerousPath, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, dangerousPath, nil)
			w := httptest.NewRecorder()

			server.GetMux().ServeHTTP(w, req)

			// These should be caught by our validateAndBuildPath function
			// and return 403 Forbidden with path traversal error
			if w.Code == http.StatusForbidden {
				assert.Equal(
					t, aichteeteapee.ContentTypeJSON,
					w.Header().Get(aichteeteapee.HeaderNameContentType),
				)

				var errorResponse aichteeteapee.ErrorResponse

				err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
				require.NoError(t, err)
				assert.Equal(
					t, aichteeteapee.ErrorCodePathTraversalDenied,
					errorResponse.Code,
				)
			} else {
				// HTTP router may redirect before our handler sees it, which is also secure
				assert.True(t,
					w.Code == http.StatusNotFound ||
						w.Code == http.StatusForbidden ||
						w.Code == http.StatusMovedPermanently,
					"Expected 403, 404, or 301, got %d for path %s", w.Code, dangerousPath)
			}
		})
	}
}
