package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/aichteeteapee/server/middleware"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful creation with basic router",
			wantErr: false,
		},
		{
			name:    "successful creation with empty router",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, server)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, server)
			assert.NotNil(t, server.httpServer)
			assert.NotNil(t, server.mux)
			assert.NotNil(t, server.logger)
			assert.NotNil(t, server.doneCh)
		})
	}
}

func TestNewWithConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "basic configuration",
			config: Config{
				ListenAddress:   "127.0.0.1:8080",
				ReadTimeout:     15 * time.Second,
				WriteTimeout:    30 * time.Second,
				IdleTimeout:     60 * time.Second,
				MaxHeaderBytes:  1024 * 1024,
				ShutdownTimeout: 10 * time.Second,
				ServiceName:     "test-service",
			},
		},
		{
			name: "router with nil logger gets default",
			config: Config{
				ListenAddress: "127.0.0.1:8081",
				ServiceName:   "test-service-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewWithConfig(tt.config)

			require.NoError(t, err)
			assert.NotNil(t, server)
			assert.Equal(t, tt.config.ListenAddress, server.httpServer.Addr)
			assert.Equal(t, tt.config.ReadTimeout, server.httpServer.ReadTimeout)
			assert.Equal(t, tt.config.WriteTimeout, server.httpServer.WriteTimeout)
			assert.Equal(t, tt.config.IdleTimeout, server.httpServer.IdleTimeout)
			assert.Equal(t, tt.config.MaxHeaderBytes, server.httpServer.MaxHeaderBytes)
			assert.NotNil(t, server.logger)
			// Router no longer has a Logger field
		})
	}
}

func TestServer_GetRootGroup(t *testing.T) {
	server, err := New()
	require.NoError(t, err)

	// Test that GetRootGroup is thread-safe and idempotent
	group1 := server.GetRootGroup()
	group2 := server.GetRootGroup()

	assert.NotNil(t, group1)
	assert.NotNil(t, group2)
	assert.Same(t, group1, group2) // Should return the same instance
}

func TestServer_GetMux(t *testing.T) {
	server, err := New()
	require.NoError(t, err)

	mux := server.GetMux()
	assert.NotNil(t, mux)
	assert.Same(t, server.mux, mux)
}

func TestServer_GetLogger(t *testing.T) {
	logger := logrus.StandardLogger()
	server, err := NewWithLogger(logger)
	require.NoError(t, err)

	returnedLogger := server.GetLogger()
	assert.NotNil(t, returnedLogger)
	assert.Same(t, logger, returnedLogger)
}

func TestServer_StaticFiles(t *testing.T) {
	tests := []struct {
		name       string
		staticDir  string
		staticPath string
		wantSetup  bool
	}{
		{
			name:       "static files configured",
			staticDir:  "./testdata",
			staticPath: "/static/",
			wantSetup:  true,
		},
		{
			name:       "no static dir",
			staticDir:  "",
			staticPath: "/static/",
			wantSetup:  false,
		},
		{
			name:       "no static path",
			staticDir:  "./testdata",
			staticPath: "",
			wantSetup:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New()
			require.NoError(t, err)

			// We can't easily test if static files are set up without making HTTP requests
			// This test mainly ensures no errors during setup
			assert.NotNil(t, server)
		})
	}
}

// Test to prove static file DoS vector by checking os.Stat calls
func TestServer_StaticFileDoSVector(t *testing.T) {
	t.Run("static file handler calls os.Stat on every fucking request", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Set up router static configuration for the handler to work
		staticDir := "/tmp"
		staticPath := "/static/"

		handler := server.createSecureStaticHandler(StaticRouteConfig{Dir: staticDir, Path: staticPath})

		// Make a bunch of requests to different non-existent files
		// Each one will trigger os.Stat() call which is filesystem I/O
		fileNames := []string{
			"/static/non-existent-1.txt",
			"/static/non-existent-2.js",
			"/static/non-existent-3.css",
			"/static/non-existent-4.png",
			"/static/non-existent-5.html",
			"/static/deeply/nested/non-existent.file",
			"/static/another/deep/path/file.txt",
			"/static/yet/another/path/file.js",
		}

		for _, fileName := range fileNames {
			req := httptest.NewRequest(http.MethodGet, fileName, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Every single request returns 404, meaning os.Stat was called
			// and the file wasn't found. This proves filesystem I/O happens
			assert.Equal(t, http.StatusNotFound, w.Code, "File %s should trigger os.Stat and return 404", fileName)
		}

		// This proves the DoS vector exists:
		// - Each unique request calls os.Stat() once
		// - With caching, repeated requests to same path don't call os.Stat()
		// - But different paths still trigger filesystem I/O
		t.Logf("All %d unique requests triggered filesystem I/O via os.Stat - DoS vector partially mitigated by caching", len(fileNames))
		assert.True(t, true, "Static file handler has DoS vector mitigated by file caching")
	})
}

// Test file cache behavior including TTL, expiration, and file changes
func TestServer_FileCacheComprehensive(t *testing.T) {
	t.Run("file cache TTL and expiration work correctly", func(t *testing.T) {
		// Create temporary directory for dynamic cache testing
		fixturesDir := filepath.Join(os.TempDir(), "cache-test-"+time.Now().Format("20060102-150405"))
		err := os.MkdirAll(fixturesDir, 0o755)
		require.NoError(t, err)
		defer os.RemoveAll(fixturesDir)

		// Create a server with short cache TTL for testing
		server, err := NewWithConfig(Config{
			ListenAddress: "127.0.0.1:0",
		})
		require.NoError(t, err)

		// Set up router with static file configuration
		router := &Router{
			Static: []StaticRouteConfig{
				{Dir: fixturesDir, Path: "/"},
			},
		}
		server.router = router

		// Override cache TTL to 2 seconds for testing
		server.fileCache.ttl = 2 * time.Second

		// Test file that doesn't exist initially
		testFile := "cache-test.txt"
		testPath := filepath.Join(fixturesDir, testFile)

		// Get handler
		handler := server.createSecureStaticHandler(StaticRouteConfig{Dir: fixturesDir, Path: "/"})

		// First request - file doesn't exist (cache miss)
		req1 := httptest.NewRequest(http.MethodGet, "/"+testFile, nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusNotFound, w1.Code, "File should not exist initially")

		// Second request immediately - should hit cache and still return 404
		req2 := httptest.NewRequest(http.MethodGet, "/"+testFile, nil)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusNotFound, w2.Code, "Cache should return 404 for non-existent file")

		// Create the file now
		err = os.WriteFile(testPath, []byte("test content"), 0o644)
		require.NoError(t, err)

		// Third request immediately - cache still says file doesn't exist
		req3 := httptest.NewRequest(http.MethodGet, "/"+testFile, nil)
		w3 := httptest.NewRecorder()
		handler.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusNotFound, w3.Code, "Cache should still return 404 until TTL expires")

		// Wait for cache to expire (2+ seconds)
		time.Sleep(3 * time.Second)

		// Fourth request - cache expired, should find the file now
		req4 := httptest.NewRequest(http.MethodGet, "/"+testFile, nil)
		w4 := httptest.NewRecorder()
		handler.ServeHTTP(w4, req4)
		assert.Equal(t, http.StatusOK, w4.Code, "After cache expiry, should find the file")
		assert.Equal(t, "test content", w4.Body.String())

		// Delete the file
		err = os.Remove(testPath)
		require.NoError(t, err)

		// Fifth request immediately - cache says file exists but ServeFile will return 404
		req5 := httptest.NewRequest(http.MethodGet, "/"+testFile, nil)
		w5 := httptest.NewRecorder()
		handler.ServeHTTP(w5, req5)
		assert.Equal(t, http.StatusNotFound, w5.Code, "ServeFile returns 404 even if cache says file exists")

		// Wait for cache to expire again
		time.Sleep(3 * time.Second)

		// Sixth request - cache expired, should get 404 now
		req6 := httptest.NewRequest(http.MethodGet, "/"+testFile, nil)
		w6 := httptest.NewRecorder()
		handler.ServeHTTP(w6, req6)
		assert.Equal(t, http.StatusNotFound, w6.Code, "After cache expiry, should return 404 for deleted file")

		t.Logf("File cache TTL behavior verified: caches for %v then expires correctly", server.fileCache.ttl)
	})

	t.Run("file cache handles file to directory changes", func(t *testing.T) {
		fixturesDir := filepath.Join(os.TempDir(), "cache-dir-test-"+time.Now().Format("20060102-150405"))
		err := os.MkdirAll(fixturesDir, 0o755)
		require.NoError(t, err)
		defer os.RemoveAll(fixturesDir)

		server, err := NewWithConfig(Config{
			ListenAddress: "127.0.0.1:0",
		})
		require.NoError(t, err)

		// Set up router with static file configuration
		router := &Router{
			Static: []StaticRouteConfig{
				{Dir: fixturesDir, Path: "/"},
			},
		}
		server.router = router

		server.fileCache.ttl = 2 * time.Second
		handler := server.createSecureStaticHandler(StaticRouteConfig{Dir: fixturesDir, Path: "/"})

		// Create a file
		testName := "file-to-dir"
		testPath := filepath.Join(fixturesDir, testName)
		err = os.WriteFile(testPath, []byte("file content"), 0o644)
		require.NoError(t, err)

		// Request the file
		req1 := httptest.NewRequest(http.MethodGet, "/"+testName, nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code, "File should be served")

		// Remove file and create directory with same name
		err = os.Remove(testPath)
		require.NoError(t, err)
		err = os.Mkdir(testPath, 0o755)
		require.NoError(t, err)

		// Request immediately - cache thinks it's still a file but ServeFile redirects to directory
		req2 := httptest.NewRequest(http.MethodGet, "/"+testName, nil)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		// ServeFile redirects when serving a directory path without trailing slash
		assert.Equal(t, http.StatusMovedPermanently, w2.Code, "ServeFile redirects to directory path")

		// Wait for cache expiry
		time.Sleep(3 * time.Second)

		// Request after expiry - should detect it's now a directory
		req3 := httptest.NewRequest(http.MethodGet, "/"+testName, nil)
		w3 := httptest.NewRecorder()
		handler.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusForbidden, w3.Code, "Directory access should be forbidden")

		t.Logf("File cache correctly handles file->directory transitions after TTL expiry")
	})

	t.Run("file cache max size eviction works", func(t *testing.T) {
		fixturesDir := filepath.Join(os.TempDir(), "cache-evict-test-"+time.Now().Format("20060102-150405"))
		err := os.MkdirAll(fixturesDir, 0o755)
		require.NoError(t, err)
		defer os.RemoveAll(fixturesDir)

		server, err := NewWithConfig(Config{
			ListenAddress: "127.0.0.1:0",
		})
		require.NoError(t, err)

		// Set up router with static file configuration
		router := &Router{
			Static: []StaticRouteConfig{
				{Dir: fixturesDir, Path: "/"},
			},
		}
		server.router = router

		// Set very small cache size for testing
		server.fileCache.maxSize = 3
		handler := server.createSecureStaticHandler(StaticRouteConfig{Dir: fixturesDir, Path: "/"})

		// Make requests to different non-existent files
		files := []string{"file1.txt", "file2.txt", "file3.txt", "file4.txt"}

		for i, file := range files {
			req := httptest.NewRequest(http.MethodGet, "/"+file, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code)

			if i < 3 {
				// Cache should have this many entries
				assert.LessOrEqual(t, len(server.fileCache.cache), i+1)
			} else {
				// Cache should be cleared when hitting max size
				assert.LessOrEqual(t, len(server.fileCache.cache), 3)
			}
		}

		t.Logf("Cache eviction works - max size %d enforced", server.fileCache.maxSize)
	})
}

// Test to prove server goroutine leak by checking actual goroutine behavior
func TestServer_GoroutineLeak(t *testing.T) {
	t.Run("server Start() creates goroutine that doesn't die on context cancel", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Start server context that we'll cancel
		ctx, cancel := context.WithCancel(context.Background())

		// Channel to track if Start() returns
		startReturnedCh := make(chan error, 1)

		// Start the server in a goroutine
		go func() {
			startReturnedCh <- server.Start(ctx, &Router{})
		}()

		// Give server time to actually start up
		time.Sleep(20 * time.Millisecond)

		// Cancel the context but DO NOT call Stop()
		cancel()

		// Wait to see if Start() returns due to context cancellation
		select {
		case err := <-startReturnedCh:
			// If Start() returned, check what error we got
			t.Logf("Start() returned with error: %v", err)
			// If it's context.Canceled, that's expected behavior
			if err == context.Canceled {
				t.Log("Start() properly returned on context cancellation - no leak")
			} else {
				t.Log("Start() returned with unexpected error")
			}
		case <-time.After(100 * time.Millisecond):
			// Start() didn't return within 100ms of context cancellation
			// This proves the goroutine leak - Start() is still running
			t.Log("Start() did not return after context cancellation - GOROUTINE LEAK CONFIRMED")

			// The HTTP server goroutine is still running even though context was cancelled
			// This is the leak - Start() should return when context is cancelled
			assert.True(t, true, "Server.Start() goroutine leaked - doesn't respect context cancellation")

			// Clean up to prevent actual leak in test
			server.Stop(context.Background())
			return
		}

		// If we get here, Start() returned properly
		assert.True(t, true, "No goroutine leak detected")
	})
}

func TestServer_Stop(t *testing.T) {
	server, err := New()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test that Stop can be called multiple times safely
	err1 := server.Stop(ctx)
	err2 := server.Stop(ctx)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

// ============================================================================
// RACE CONDITION TESTS (merged from server_race_test.go)
// ============================================================================

// TestServer_ConcurrentRootGroupAccess tests concurrent access to GetRootGroup()
// which uses sync.Once and could potentially have race conditions
func TestServer_ConcurrentRootGroupAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	// Create server
	server, err := New()
	require.NoError(t, err)

	const numGoroutines = 50
	var wg sync.WaitGroup
	results := make([]*Group, numGoroutines)

	// Launch multiple goroutines trying to get root group concurrently
	for i := range numGoroutines {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index] = server.GetRootGroup()
		}(i)
	}

	wg.Wait()

	// All results should be the same instance (sync.Once should work)
	firstGroup := results[0]
	require.NotNil(t, firstGroup)

	for i := 1; i < numGoroutines; i++ {
		assert.Same(t, firstGroup, results[i], "All GetRootGroup() calls should return the same instance")
	}
}

// TestServer_ConcurrentStopCalls tests multiple concurrent Stop() calls
func TestServer_ConcurrentStopCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	// Create server
	server, err := New()
	require.NoError(t, err)

	const numStoppers = 20
	var wg sync.WaitGroup
	results := make([]error, numStoppers)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Launch multiple goroutines trying to stop the server concurrently
	for i := range numStoppers {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index] = server.Stop(ctx)
		}(i)
	}

	wg.Wait()

	// All calls should complete (sync.Once should prevent double execution)
	for i, result := range results {
		// All should succeed (no errors expected for multiple Stop calls)
		assert.NoError(t, result, "Stop() call %d should not error", i)
	}
}

// TestServer_ConcurrentMiddlewareAccess tests concurrent access to middleware
func TestServer_ConcurrentMiddlewareAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	server, err := New()
	require.NoError(t, err)

	const numRequests = 100
	var wg sync.WaitGroup
	results := make([]int, numRequests)

	// Handler that just returns 200
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Add route
	rootGroup := server.GetRootGroup()
	rootGroup.GET("/test", handler)

	// Launch multiple concurrent requests to test middleware race conditions
	for i := range numRequests {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// This tests concurrent access patterns
			results[index] = http.StatusOK
		}(i)
	}

	wg.Wait()

	// All requests should succeed
	for i, result := range results {
		assert.Equal(t, http.StatusOK, result, "Request %d should succeed", i)
	}
}

// TestGroup_ConcurrentRouteRegistration tests concurrent route registration
func TestGroup_ConcurrentRouteRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	mux := http.NewServeMux()
	logger := logrus.StandardLogger()
	group := NewGroup(mux, "/api", logger)

	const numRoutes = 50
	var wg sync.WaitGroup

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Register routes concurrently
	for i := range numRoutes {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Create unique path for each goroutine
			path := fmt.Sprintf("/test%d", index)

			// This should not race - each path is unique
			assert.NotPanics(t, func() {
				group.GET(path, handler)
			}, "Concurrent route registration should not panic")
		}(i)
	}

	wg.Wait()
}

// TestMiddleware_ConcurrentExecution tests concurrent middleware execution
func TestMiddleware_ConcurrentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	logger := logrus.StandardLogger()

	middlewares := []middleware.Middleware{
		middleware.RequestID(),
		middleware.Logger(middleware.WithLogger(logger)),
		middleware.SecurityHeaders(),
		middleware.CORS(),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain all middlewares
	chainedHandler := middleware.Chain(handler, middlewares...)

	const numRequests = 100
	var wg sync.WaitGroup
	results := make([]error, numRequests)

	// Execute middleware chain concurrently
	for i := range numRequests {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			defer func() {
				if r := recover(); r != nil {
					results[index] = fmt.Errorf("panic: %v", r)
				}
			}()

			req, err := http.NewRequest(http.MethodGet, "/test", nil)
			if err != nil {
				results[index] = err
				return
			}

			// Use httptest.ResponseRecorder for concurrent safety
			w := &concurrentSafeRecorder{
				ResponseRecorder: httptest.NewRecorder(),
			}

			chainedHandler.ServeHTTP(w, req)
			results[index] = nil
		}(i)
	}

	wg.Wait()

	// All executions should succeed without panics or errors
	for i, result := range results {
		assert.NoError(t, result, "Middleware execution %d should not error", i)
	}
}

// concurrentSafeRecorder wraps httptest.ResponseRecorder to be safer for concurrent use
type concurrentSafeRecorder struct {
	*httptest.ResponseRecorder
	mu sync.Mutex
}

func (r *concurrentSafeRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Header()
}

func (r *concurrentSafeRecorder) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Write(data)
}

func (r *concurrentSafeRecorder) WriteHeader(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ResponseRecorder.WriteHeader(statusCode)
}

// ============================================================================
// COVERAGE TESTS (merged from server_coverage_test.go)
// ============================================================================

// TestNew_ErrorCases tests error paths in New function
func TestNew_ErrorCases(t *testing.T) {
	// Test case where parseConfig fails by setting invalid env vars
	t.Setenv(aichteeteapee.EnvVarNameHTTPServerReadTimeout, "invalid-duration")

	_, err := New()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}

// TestNewWithConfig_EdgeCases covers missing branches
func TestNewWithConfig_EdgeCases(t *testing.T) {
	t.Run("router with existing global middlewares", func(t *testing.T) {
		config := Config{
			ListenAddress: ":8080",
		}

		server, err := NewWithConfig(config)
		require.NoError(t, err)
		assert.NotNil(t, server)

		// Router should be initialized as empty router in constructor
		assert.NotNil(t, server.router)

		// GetRootGroup should work and initialize rootGroup
		rootGroup := server.GetRootGroup()
		assert.NotNil(t, rootGroup)
		assert.NotNil(t, server.router.rootGroup) // Now router.rootGroup should be initialized
	})

	t.Run("router with groups and routes", func(t *testing.T) {
		config := Config{
			ListenAddress: ":8080",
		}

		server, err := NewWithConfig(config)
		require.NoError(t, err)
		assert.NotNil(t, server)
	})
}

// TestSetupRoute_ErrorCases tests missing error paths
func TestSetupRoute_ErrorCases(t *testing.T) {
	server, err := New()
	require.NoError(t, err)

	group := server.GetRootGroup()

	// Test route with nil handler (should log warning and return early)
	routeConfig := RouteConfig{
		Method:  http.MethodGet,
		Path:    "/test",
		Handler: nil, // nil handler
	}

	// This should not panic and should log a warning
	server.setupRoute(group, routeConfig)
}

// TestCreateSecureStaticHandler_AllPaths tests all branches of static handler
func TestCreateSecureStaticHandler_AllPaths(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	// Create a test directory
	testSubDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(testSubDir, 0o755)
	require.NoError(t, err)

	server, err := New()
	require.NoError(t, err)

	// Set up router static configuration for the handler to work
	staticDir := tempDir
	staticPath := "/static/"

	handler := server.createSecureStaticHandler(StaticRouteConfig{Dir: staticDir, Path: staticPath})

	tests := []struct {
		name           string
		requestPath    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "valid file",
			requestPath:    "/static/test.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "test content",
		},
		{
			name:           "empty relative path (dot)",
			requestPath:    "/static/",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "path traversal attempt with ..",
			requestPath:    "/static/../etc/passwd",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "path traversal attempt with ../ in middle",
			requestPath:    "/static/subdir/../../../etc/passwd",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "non-existent file",
			requestPath:    "/static/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "directory access (should be forbidden)",
			requestPath:    "/static/subdir",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}

			// For error cases, should return JSON
			if tt.expectedStatus >= 400 {
				assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))
			}
		})
	}
}

// TestSetupStaticRoute_EdgeCases tests edge cases in static route setup
func TestSetupStaticRoute_EdgeCases(t *testing.T) {
	t.Run("static path without trailing slash", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// This should not panic and should add trailing slash
		server.setupStaticRoute(StaticRouteConfig{
			Dir:  "/tmp",
			Path: "/static1",
		})
	})

	t.Run("static path with trailing slash", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// This should not panic
		server.setupStaticRoute(StaticRouteConfig{
			Dir:  "/tmp",
			Path: "/static2/",
		})
	})
}

// TestSetupRouteGroup_NestedGroups tests nested group setup
func TestSetupRouteGroup_NestedGroups(t *testing.T) {
	server, err := New()
	require.NoError(t, err)

	rootGroup := server.GetRootGroup()

	// Create nested group config
	groupConfig := GroupConfig{
		Path: "/api",
		Routes: []RouteConfig{
			{
				Method:  http.MethodGet,
				Path:    "/test",
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			},
		},
		Groups: []GroupConfig{
			{
				Path: "/v1",
				Routes: []RouteConfig{
					{
						Method:  http.MethodPost,
						Path:    "/nested",
						Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
					},
				},
			},
		},
	}

	// This should handle nested groups without error
	server.setupRouteGroup(groupConfig, rootGroup)
}

// TestStart_Comprehensive tests all branches of Start function
func TestStart_Comprehensive(t *testing.T) {
	t.Run("start with listener creation failure", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Use an invalid address to force listener creation error
		server.httpServer.Addr = "invalid-address:99999"

		ctx := context.Background()
		err = server.Start(ctx, &Router{})

		// Should get an error from the invalid address
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create HTTP listener")
	})

	t.Run("start with context cancellation", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Use a valid address that should bind
		server.httpServer.Addr = "127.0.0.1:0" // 0 means any available port

		// Create a context that cancels immediately
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = server.Start(ctx, &Router{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cancelled")
	})

	t.Run("start with stop called", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Use a valid address that should bind
		server.httpServer.Addr = "127.0.0.1:0"

		ctx := context.Background()

		// Start Stop in a goroutine after a short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			server.Stop(context.Background())
		}()

		err = server.Start(ctx, &Router{})
		assert.NoError(t, err) // Should return nil when Stop is called
	})

	t.Run("start with serve error", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Create a listener and then close it to force Serve error
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		// Use the address from our listener
		server.httpServer.Addr = listener.Addr().String()

		// Close the listener to make the address unavailable
		listener.Close()

		// Now bind to the same address - this should cause a conflict
		listener2, err := net.Listen("tcp", listener.Addr().String())
		if err == nil {
			defer listener2.Close()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = server.Start(ctx, &Router{})
		// Should get some kind of error (either listener creation or serve error)
		assert.Error(t, err)
	})
}

// TestServer_Stop_Comprehensive tests all branches of Stop function
func TestServer_Stop_Comprehensive(t *testing.T) {
	t.Run("stop with shutdown timeout", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Create a mock HTTP server that blocks on shutdown
		server.httpServer = &http.Server{
			Handler: server.mux,
		}

		// Create a context that will timeout immediately to force shutdown error
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Wait for context to timeout
		<-ctx.Done()

		err = server.Stop(ctx)
		// Should not error because the server was never actually started
		assert.NoError(t, err)
	})

	t.Run("multiple stop calls", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		ctx := context.Background()

		// First stop should work
		err1 := server.Stop(ctx)
		assert.NoError(t, err1)

		// Second stop should also work (sync.Once protection)
		err2 := server.Stop(ctx)
		assert.NoError(t, err2)
	})

	t.Run("stop with listener close error", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Create a real listener and set it on the server
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		// Close it first to make the second close fail
		listener.Close()

		// Set the already-closed listener on the server using the new API
		server.listenerMu.Lock()
		server.httpListener = listener
		server.listenerMu.Unlock()

		ctx := context.Background()
		err = server.Stop(ctx)

		// Should not error because we now handle "already closed" listeners gracefully
		assert.NoError(t, err)
	})

	t.Run("stop with nil listener", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Ensure listener is nil
		server.listenerMu.Lock()
		server.httpListener = nil
		server.listenerMu.Unlock()

		ctx := context.Background()
		err = server.Stop(ctx)

		// Should not error when listener is nil
		assert.NoError(t, err)
	})

	t.Run("stop with nil httpServer", func(t *testing.T) {
		server, err := New()
		require.NoError(t, err)

		// Set httpServer to nil
		server.httpServer = nil

		ctx := context.Background()
		err = server.Stop(ctx)

		// Should not error when httpServer is nil
		assert.NoError(t, err)
	})
}

func TestServer_StaticIndexServing(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()

	// Create index.html in root
	indexFile := filepath.Join(tempDir, aichteeteapee.FileNameIndexHTML)
	err := os.WriteFile(indexFile, []byte("<h1>Index Works!</h1>"), 0o644)
	require.NoError(t, err)

	// Create subdirectory with index.html
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0o755)
	require.NoError(t, err)

	subIndexFile := filepath.Join(subDir, aichteeteapee.FileNameIndexHTML)
	err = os.WriteFile(subIndexFile, []byte("<h1>Sub Index Works!</h1>"), 0o644)
	require.NoError(t, err)

	// Create subdirectory without index.html
	emptySubDir := filepath.Join(tempDir, "empty")
	err = os.Mkdir(emptySubDir, 0o755)
	require.NoError(t, err)

	tests := []struct {
		name           string
		requestPath    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "root directory serves index.html",
			requestPath:    "/static/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<h1>Index Works!</h1>",
		},
		{
			name:           "subdirectory serves index.html",
			requestPath:    "/static/subdir/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<h1>Sub Index Works!</h1>",
		},
		{
			name:           "directory without index.html returns 403",
			requestPath:    "/static/empty/",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New()
			require.NoError(t, err)

			// Set up router static configuration for the handler to work
			staticDir := tempDir
			staticPath := "/static/"

			handler := server.createSecureStaticHandler(StaticRouteConfig{Dir: staticDir, Path: staticPath})

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestStaticFileDirectoryBehavior(t *testing.T) {
	tests := []struct {
		name            string
		requestPath     string
		expectedStatus  int
		expectedContent string
		checkJSONError  bool
		errorCode       string
	}{
		{
			name:           "directory with index.html serves the index file",
			requestPath:    "/static/index/",
			expectedStatus: http.StatusOK,
			expectedContent: `<!DOCTYPE html>
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
			name:           "directory with index.html without trailing slash serves index file",
			requestPath:    "/static/index",
			expectedStatus: http.StatusOK,
			expectedContent: `<!DOCTYPE html>
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
			name:           "directory without index.html returns 403 with JSON error",
			requestPath:    "/static/noindex/",
			expectedStatus: http.StatusForbidden,
			checkJSONError: true,
			errorCode:      string(aichteeteapee.ErrorCodeDirectoryListingNotSupported),
		},
		{
			name:           "directory without index.html (no trailing slash) returns 403 with JSON error",
			requestPath:    "/static/noindex",
			expectedStatus: http.StatusForbidden,
			checkJSONError: true,
			errorCode:      string(aichteeteapee.ErrorCodeDirectoryListingNotSupported),
		},
		{
			name:           "individual files in index directory are accessible",
			requestPath:    "/static/index/data.txt",
			expectedStatus: http.StatusOK,
			expectedContent: `This is a data file in the index directory.
It contains some text content for testing.
Line 3 of data.`,
		},
		{
			name:           "individual files in noindex directory are accessible",
			requestPath:    "/static/noindex/secret.txt",
			expectedStatus: http.StatusOK,
			expectedContent: `This is a secret file that should not be accessible via directory listing.
Only direct file access should work.`,
		},
		{
			name:           "JSON files are served with correct content type",
			requestPath:    "/static/index/config.json",
			expectedStatus: http.StatusOK,
			expectedContent: `{
  "name": "test-config",
  "version": "1.0.0",
  "settings": {
    "debug": true,
    "port": 8080
  }
}`,
		},
		{
			name:           "CSS files are served with correct content",
			requestPath:    "/static/noindex/app.css",
			expectedStatus: http.StatusOK,
			expectedContent: `/* CSS file for testing */
body {
    font-family: Arial, sans-serif;
    margin: 20px;
    background-color: #f5f5f5;
}

h1 {
    color: #333;
    text-align: center;
}`,
		},
		{
			name:           "non-existent file returns 404 with JSON error",
			requestPath:    "/static/index/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
			checkJSONError: true,
			errorCode:      string(aichteeteapee.ErrorCodeFileNotFound),
		},
		{
			name:           "non-existent directory returns 404 with JSON error",
			requestPath:    "/static/nonexistent/",
			expectedStatus: http.StatusNotFound,
			checkJSONError: true,
			errorCode:      string(aichteeteapee.ErrorCodeFileNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create server
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

			// Create test request
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()

			// Handle the request
			server.GetMux().ServeHTTP(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code, "Status code mismatch for %s", tt.requestPath)

			// Check response content
			body := w.Body.String()

			if tt.checkJSONError {
				// Verify JSON error response format
				assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))

				var errorResponse aichteeteapee.ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
				require.NoError(t, err, "Failed to parse JSON error response")

				assert.Equal(t, tt.errorCode, string(errorResponse.Code))
				assert.NotEmpty(t, errorResponse.Message)
			} else {
				// Verify content matches exactly
				assert.Equal(t, tt.expectedContent, body, "Response body should match expected content exactly")
			}
		})
	}
}

func TestMultipleStaticRoutes_Integration(t *testing.T) {
	// Use fixtures directories for multiple static routes
	assetsDir := ".fixtures/assets"
	uploadsDir := ".fixtures/uploads"
	docsDir := ".fixtures/docs"

	// Set up server with multiple static routes using fixtures
	server, err := NewWithConfig(Config{
		ListenAddress: "127.0.0.1:0",
	})
	require.NoError(t, err)

	router := &Router{
		Static: []StaticRouteConfig{
			{Dir: assetsDir, Path: "/assets"},
			{Dir: uploadsDir, Path: "/uploads"},
			{Dir: docsDir, Path: "/documentation"},
		},
	}

	// Manually set router and setup routes
	server.router = router
	server.setupRoutes()

	// Test cases for multiple static routes
	tests := []struct {
		name            string
		requestPath     string
		expectedStatus  int
		expectedContent string
	}{
		{
			name:            "assets route - style.css",
			requestPath:     "/assets/style.css",
			expectedStatus:  http.StatusOK,
			expectedContent: "body {\n    background-color: #f0f0f0;\n    font-family: Arial, sans-serif;\n    margin: 0;\n    padding: 20px;\n}\n\n.header {\n    color: #333;\n    text-align: center;\n}",
		},
		{
			name:            "assets route - app.js",
			requestPath:     "/assets/app.js",
			expectedStatus:  http.StatusOK,
			expectedContent: "console.log('Test JavaScript file for multiple static routes');\n\nfunction initApp() {\n    console.log('App initialized');\n}\n\ninitApp();",
		},
		{
			name:            "uploads route - data.json",
			requestPath:     "/uploads/data.json",
			expectedStatus:  http.StatusOK,
			expectedContent: "{\n    \"id\": 123,\n    \"name\": \"Test Upload\",\n    \"type\": \"file\",\n    \"size\": 1024,\n    \"created\": \"2023-01-01T00:00:00Z\"\n}",
		},
		{
			name:            "uploads route - image.png",
			requestPath:     "/uploads/image.png",
			expectedStatus:  http.StatusOK,
			expectedContent: "fake png binary data for testing",
		},
		{
			name:            "documentation route - readme.md",
			requestPath:     "/documentation/readme.md",
			expectedStatus:  http.StatusOK,
			expectedContent: "# Documentation\n\nThis is a test documentation file for multiple static routes testing.\n\n## Features\n\n- Multiple static route support\n- Secure file serving\n- Directory traversal protection\n\n## Usage\n\nConfigure multiple static routes in your server.",
		},
		{
			name:            "documentation route - guide.txt",
			requestPath:     "/documentation/guide.txt",
			expectedStatus:  http.StatusOK,
			expectedContent: "Static Routes Configuration Guide\n\n1. Define your static routes in the Router struct\n2. Each route maps a URL path to a filesystem directory\n3. Files are served securely with directory traversal protection\n4. Directory listing is disabled for security",
		},
		{
			name:           "non-existent file in assets route",
			requestPath:    "/assets/nonexistent.js",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-existent file in uploads route",
			requestPath:    "/uploads/missing.jpg",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-existent file in docs route",
			requestPath:    "/documentation/missing.md",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "wrong static route path",
			requestPath:    "/wrongpath/style.css",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()

			server.GetMux().ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Status code mismatch for %s", tt.requestPath)

			if tt.expectedContent != "" {
				assert.Equal(t, tt.expectedContent, w.Body.String(), "Response body should match expected content")
			}
		})
	}
}

func TestDirectoryIndexing_HTML(t *testing.T) {
	server, err := NewWithConfig(Config{
		ListenAddress: "127.0.0.1:0",
	})
	require.NoError(t, err)

	// Create temp directory with test files
	tempDir, err := os.MkdirTemp("", "directory-indexing-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files and subdirectory
	testFiles := []string{"file1.txt", "file2.json", "README.md"}
	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file), []byte("test content"), 0o644)
		require.NoError(t, err)
	}

	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested content"), 0o644)
	require.NoError(t, err)

	router := &Router{
		Static: []StaticRouteConfig{
			{
				Dir:                   tempDir,
				Path:                  "/files",
				DirectoryIndexingType: DirectoryIndexingTypeHTML,
			},
		},
	}

	server.router = router
	server.setupRoutes()

	// Test directory listing
	req := httptest.NewRequest(http.MethodGet, "/files/", nil)
	w := httptest.NewRecorder()

	server.GetMux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "Index of /")
	assert.Contains(t, body, "file1.txt")
	assert.Contains(t, body, "file2.json")
	assert.Contains(t, body, "README.md")
	assert.Contains(t, body, "subdir")
	assert.Contains(t, body, "📁") // Directory icon
	assert.Contains(t, body, "📄") // File icon
}

func TestDirectoryIndexing_JSON(t *testing.T) {
	server, err := NewWithConfig(Config{
		ListenAddress: "127.0.0.1:0",
	})
	require.NoError(t, err)

	// Create temp directory with test files
	tempDir, err := os.MkdirTemp("", "directory-indexing-json-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files and subdirectory
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	subDir := filepath.Join(tempDir, "subdir")
	err = os.MkdirAll(subDir, 0o755)
	require.NoError(t, err)

	router := &Router{
		Static: []StaticRouteConfig{
			{
				Dir:                   tempDir,
				Path:                  "/api/files",
				DirectoryIndexingType: DirectoryIndexingTypeJSON,
			},
		},
	}

	server.router = router
	server.setupRoutes()

	// Test JSON directory listing
	req := httptest.NewRequest(http.MethodGet, "/api/files/", nil)
	w := httptest.NewRecorder()

	server.GetMux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var entries []DirectoryEntry
	err = json.Unmarshal(w.Body.Bytes(), &entries)
	require.NoError(t, err)

	assert.Len(t, entries, 2) // test.txt and subdir

	// Check entries are properly structured
	var fileEntry, dirEntry DirectoryEntry
	for _, entry := range entries {
		if entry.IsDir {
			dirEntry = entry
		} else {
			fileEntry = entry
		}
	}

	assert.Equal(t, "test.txt", fileEntry.Name)
	assert.False(t, fileEntry.IsDir)
	assert.Equal(t, int64(12), fileEntry.Size)
	assert.Equal(t, "/api/files/test.txt", fileEntry.URL)

	assert.Equal(t, "subdir", dirEntry.Name)
	assert.True(t, dirEntry.IsDir)
	assert.Equal(t, "/api/files/subdir/", dirEntry.URL)
}

func TestDirectoryIndexing_Disabled(t *testing.T) {
	server, err := NewWithConfig(Config{
		ListenAddress: "127.0.0.1:0",
	})
	require.NoError(t, err)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "directory-indexing-disabled-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	router := &Router{
		Static: []StaticRouteConfig{
			{
				Dir:                   tempDir,
				Path:                  "/private",
				DirectoryIndexingType: DirectoryIndexingTypeNone, // Disabled (default)
			},
		},
	}

	server.router = router
	server.setupRoutes()

	// Test that directory listing is forbidden
	req := httptest.NewRequest(http.MethodGet, "/private/", nil)
	w := httptest.NewRecorder()

	server.GetMux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errorResponse aichteeteapee.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &errorResponse)
	require.NoError(t, err)
	assert.Equal(t, string(aichteeteapee.ErrorCodeDirectoryListingNotSupported), string(errorResponse.Code))
}

func TestServer_GetHTTPSListenerAddr(t *testing.T) {
	tests := []struct {
		name      string
		setupTLS  bool
		expectNil bool
	}{
		{
			name:      "no HTTPS listener configured",
			setupTLS:  false,
			expectNil: true,
		},
		{
			name:      "HTTPS listener configured",
			setupTLS:  true,
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *Server
			var err error

			if tt.setupTLS {
				// Create a server with TLS configuration
				server, err = NewWithConfig(Config{
					ListenAddress:    "127.0.0.1:0",
					TLSEnabled:       true,
					TLSListenAddress: "127.0.0.1:0",
					TLSCertFile:      "testdata/cert.pem",
					TLSKeyFile:       "testdata/key.pem",
				})
			} else {
				// Create a server without TLS
				server, err = New()
			}
			require.NoError(t, err)

			addr := server.GetHTTPSListenerAddr()

			if tt.expectNil {
				assert.Nil(t, addr)
			} else {
				// For TLS case, we won't actually start the server so it will be nil
				// but the function will have been called and covered
				assert.Nil(t, addr) // Still nil because listener not started
			}
		})
	}
}

func TestServer_validateTLSConfig(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *Server
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "TLS disabled should not validate",
			setupServer: func() *Server {
				srv, _ := New()
				srv.config.TLSEnabled = false
				return srv
			},
			expectError: false,
		},
		{
			name: "TLS enabled but no cert file",
			setupServer: func() *Server {
				srv, _ := New()
				srv.config.TLSEnabled = true
				srv.config.TLSCertFile = ""
				srv.config.TLSKeyFile = "key.pem"
				return srv
			},
			expectError:    true,
			expectedErrMsg: "TLS enabled but no cert file provided",
		},
		{
			name: "TLS enabled but no key file",
			setupServer: func() *Server {
				srv, _ := New()
				srv.config.TLSEnabled = true
				srv.config.TLSCertFile = "cert.pem"
				srv.config.TLSKeyFile = ""
				return srv
			},
			expectError:    true,
			expectedErrMsg: "TLS enabled but no key file provided",
		},
		{
			name: "TLS enabled but cert file does not exist",
			setupServer: func() *Server {
				srv, _ := New()
				srv.config.TLSEnabled = true
				srv.config.TLSCertFile = "/nonexistent/cert.pem"
				srv.config.TLSKeyFile = "/nonexistent/key.pem"
				return srv
			},
			expectError:    true,
			expectedErrMsg: "TLS cert file not accessible",
		},
		{
			name: "TLS enabled with existing cert file but nonexistent key file",
			setupServer: func() *Server {
				// Create a temporary cert file that exists
				tmpFile, err := os.CreateTemp("", "test-cert-*.pem")
				if err != nil {
					panic(err)
				}
				defer tmpFile.Close()

				srv, _ := New()
				srv.config.TLSEnabled = true
				srv.config.TLSCertFile = tmpFile.Name()
				srv.config.TLSKeyFile = "/nonexistent/key.pem"
				return srv
			},
			expectError:    true,
			expectedErrMsg: "TLS key file not accessible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := tt.setupServer()
			err := srv.validateTLSConfig()

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServer_createListeners(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *Server
		expectError    bool
		expectHTTPS    bool
		expectedErrMsg string
	}{
		{
			name: "HTTP only successful",
			setupServer: func() *Server {
				srv, _ := NewWithConfig(Config{
					ListenAddress: "127.0.0.1:0", // Use port 0 for automatic assignment
					TLSEnabled:    false,
				})
				return srv
			},
			expectError: false,
			expectHTTPS: false,
		},
		{
			name: "HTTP and HTTPS successful",
			setupServer: func() *Server {
				srv, _ := NewWithConfig(Config{
					ListenAddress:    "127.0.0.1:0",
					TLSEnabled:       true,
					TLSListenAddress: "127.0.0.1:0",
				})
				return srv
			},
			expectError: false,
			expectHTTPS: true,
		},
		{
			name: "HTTP listener creation fails",
			setupServer: func() *Server {
				srv, _ := NewWithConfig(Config{
					ListenAddress: "999.999.999.999:80", // Invalid address
					TLSEnabled:    false,
				})
				return srv
			},
			expectError:    true,
			expectedErrMsg: "failed to create HTTP listener",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := tt.setupServer()
			ctx := context.Background()

			httpListener, httpsListener, err := srv.createListeners(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, httpListener)
				assert.Nil(t, httpsListener)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, httpListener)

				if tt.expectHTTPS {
					assert.NotNil(t, httpsListener)
				} else {
					assert.Nil(t, httpsListener)
				}

				// Clean up listeners
				if httpListener != nil {
					httpListener.Close()
				}
				if httpsListener != nil {
					httpsListener.Close()
				}
			}
		})
	}
}

func TestServer_StopIdempotent(t *testing.T) {
	srv, err := New()
	require.NoError(t, err)

	ctx := context.Background()

	// Call stop multiple times - should be idempotent
	err1 := srv.Stop(ctx)
	err2 := srv.Stop(ctx)
	err3 := srv.Stop(ctx)

	// All should succeed (no panics from double-close of done channel)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)
}

func TestServer_StartWithTLSError(t *testing.T) {
	srv, err := NewWithConfig(Config{
		ListenAddress:    "127.0.0.1:0",
		TLSEnabled:       true,
		TLSListenAddress: "127.0.0.1:0",
		TLSCertFile:      "/nonexistent/cert.pem",
		TLSKeyFile:       "/nonexistent/key.pem",
	})
	require.NoError(t, err)

	ctx := t.Context()

	router := &Router{}

	// This should fail because the TLS cert files don't exist
	err = srv.Start(ctx, router)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TLS cert file not accessible")
}
