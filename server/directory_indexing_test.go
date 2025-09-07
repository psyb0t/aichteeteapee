package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_buildParentURL(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name         string
		relativePath string
		staticConfig StaticRouteConfig
		expected     string
	}{
		{
			name:         "root static path with subdirectory",
			relativePath: "/subdir/",
			staticConfig: StaticRouteConfig{Path: "/"},
			expected:     "/",
		},
		{
			name:         "static path with subdirectory - parent is root",
			relativePath: "/subdir/",
			staticConfig: StaticRouteConfig{Path: "/static"},
			expected:     "/static/",
		},
		{
			name:         "nested subdirectory",
			relativePath: "/subdir/nested/",
			staticConfig: StaticRouteConfig{Path: "/files"},
			expected:     "/files/subdir/",
		},
		{
			name:         "single directory level",
			relativePath: "/docs/",
			staticConfig: StaticRouteConfig{Path: "/api"},
			expected:     "/api/",
		},
		{
			name:         "path without trailing slash",
			relativePath: "/images",
			staticConfig: StaticRouteConfig{Path: "/assets"},
			expected:     "/assets/",
		},
		{
			name:         "deep nested path",
			relativePath: "/a/b/c/d/",
			staticConfig: StaticRouteConfig{Path: "/public"},
			expected:     "/public/a/b/c/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.buildParentURL(tt.relativePath, tt.staticConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServer_generateDirectoryListing(t *testing.T) {
	logger := logrus.New()
	server := &Server{logger: logger}

	// Create a temporary directory with some test files
	tempDir, err := os.MkdirTemp("", "test_dir_listing")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	tests := []struct {
		name               string
		fullPath           string
		staticConfig       StaticRouteConfig
		expectStatus       int
		expectContentType  string
	}{
		{
			name:     "HTML directory listing",
			fullPath: tempDir,
			staticConfig: StaticRouteConfig{
				Path:                  "/static",
				DirectoryIndexingType: DirectoryIndexingTypeHTML,
			},
			expectStatus:      http.StatusOK,
			expectContentType: "text/html",
		},
		{
			name:     "JSON directory listing",
			fullPath: tempDir,
			staticConfig: StaticRouteConfig{
				Path:                  "/api/files",
				DirectoryIndexingType: DirectoryIndexingTypeJSON,
			},
			expectStatus:      http.StatusOK,
			expectContentType: aichteeteapee.ContentTypeJSON,
		},
		{
			name:     "DirectoryIndexingTypeNone should return forbidden",
			fullPath: tempDir,
			staticConfig: StaticRouteConfig{
				Path:                  "/static",
				DirectoryIndexingType: DirectoryIndexingTypeNone,
			},
			expectStatus:      http.StatusForbidden,
			expectContentType: aichteeteapee.ContentTypeJSON,
		},
		{
			name:     "non-existent directory should return error",
			fullPath: "/non/existent/directory",
			staticConfig: StaticRouteConfig{
				Path:                  "/static",
				DirectoryIndexingType: DirectoryIndexingTypeHTML,
			},
			expectStatus:      http.StatusInternalServerError,
			expectContentType: aichteeteapee.ContentTypeJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.staticConfig.Path+"/", nil)
			w := httptest.NewRecorder()

			server.generateDirectoryListing(w, req, tt.fullPath, tt.staticConfig)

			assert.Equal(t, tt.expectStatus, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), tt.expectContentType)
		})
	}
}