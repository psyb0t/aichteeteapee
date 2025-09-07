package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a test server for testing handlers
func createTestServer() *Server {
	srv, err := New()
	if err != nil {
		panic("Failed to create test server: " + err.Error())
	}
	return srv
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       any
		expectJSON bool
	}{
		{
			name:       "simple object",
			statusCode: http.StatusOK,
			data: map[string]string{
				"message": "success",
				"status":  "ok",
			},
			expectJSON: true,
		},
		{
			name:       "array data",
			statusCode: http.StatusOK,
			data:       []string{"item1", "item2", "item3"},
			expectJSON: true,
		},
		{
			name:       "error response",
			statusCode: http.StatusBadRequest,
			data:       aichteeteapee.ErrorResponseValidationFailed,
			expectJSON: true,
		},
		{
			name:       "nil data",
			statusCode: http.StatusNoContent,
			data:       nil,
			expectJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			aichteeteapee.WriteJSON(w, tt.statusCode, tt.data)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))

			if tt.expectJSON {
				var result any
				err := json.Unmarshal(w.Body.Bytes(), &result)
				assert.NoError(t, err)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
	}{
		{
			name:      "health check without request ID",
			requestID: "",
		},
		{
			name:      "health check with request ID",
			requestID: "health-req-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)

			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), aichteeteapee.ContextKeyRequestID, tt.requestID)
				req = req.WithContext(ctx)
			}

			srv := createTestServer()
			w := httptest.NewRecorder()
			srv.HealthHandler(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "ok", response["status"])
			assert.NotNil(t, response["timestamp"])

			// Request ID is no longer in JSON response - it's only in HTTP header
			assert.NotContains(t, response, "requestId")

			// Verify timestamp format
			timestampStr, ok := response["timestamp"].(string)
			require.True(t, ok)
			_, err = time.Parse(time.RFC3339, timestampStr)
			assert.NoError(t, err)
		})
	}
}

func TestEchoHandler(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		query        string
		body         string
		headers      map[string]string
		requestID    string
		user         string
		expectUser   bool
		expectStatus int
	}{
		{
			name:   "GET request with query params",
			method: http.MethodGet,
			path:   "/echo",
			query:  "?param1=value1&param2=value2",
			body:   "",
			headers: map[string]string{
				"User-Agent": "test-client",
			},
			requestID: "echo-req-1",
		},
		{
			name:   "POST request with JSON body",
			method: http.MethodPost,
			path:   "/echo",
			body:   `{"name": "test", "value": 123}`,
			headers: map[string]string{
				"Content-Type": aichteeteapee.ContentTypeJSON,
			},
			requestID: "echo-req-2",
		},
		{
			name:       "request with authenticated user",
			method:     http.MethodGet,
			path:       "/echo",
			requestID:  "echo-req-3",
			user:       "testuser",
			expectUser: true,
		},
		{
			name:   "request with invalid JSON body",
			method: http.MethodPost,
			path:   "/echo",
			body:   `{"invalid": json}`,
			headers: map[string]string{
				"Content-Type": aichteeteapee.ContentTypeJSON,
			},
		},
		{
			name:   "request with non-JSON content type should return error",
			method: http.MethodPost,
			path:   "/echo",
			body:   "plain text body",
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			expectStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path+tt.query, bodyReader)

			// Set headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Set context values
			ctx := req.Context()
			if tt.requestID != "" {
				ctx = context.WithValue(ctx, aichteeteapee.ContextKeyRequestID, tt.requestID)
			}
			if tt.user != "" {
				ctx = context.WithValue(ctx, aichteeteapee.ContextKeyUser, tt.user)
			}
			req = req.WithContext(ctx)

			srv := createTestServer()
			w := httptest.NewRecorder()
			srv.EchoHandler(w, req)

			expectedStatus := http.StatusOK
			if tt.expectStatus != 0 {
				expectedStatus = tt.expectStatus
			}
			assert.Equal(t, expectedStatus, w.Code)
			assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))

			// For error responses, don't check the echo response structure
			if expectedStatus != http.StatusOK {
				return
			}

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.method, response["method"])
			assert.Equal(t, tt.path, response["path"])

			// Check query parameters
			if tt.query != "" {
				query, ok := response["query"].(map[string]any)
				assert.True(t, ok)
				assert.NotEmpty(t, query)
			}

			// Check headers
			headers, ok := response["headers"].(map[string]any)
			assert.True(t, ok)
			for key, expectedValue := range tt.headers {
				headerValues, exists := headers[key].([]any)
				assert.True(t, exists)
				assert.Contains(t, headerValues, expectedValue)
			}

			// Request ID is no longer in JSON response - it's only in HTTP header
			assert.NotContains(t, response, "requestId")

			// Check user
			if tt.expectUser {
				assert.Equal(t, tt.user, response["user"])
			}

			// Check body parsing
			if tt.body != "" && json.Valid([]byte(tt.body)) {
				assert.NotNil(t, response["body"])
			}
		})
	}
}

func TestWriteJSON_ErrorHandling(t *testing.T) {
	// Test with data that causes JSON encoding to fail
	w := httptest.NewRecorder()

	// Create data with circular reference that will cause JSON encoding to fail
	circular := make(map[string]any)
	circular["self"] = circular

	// This should not panic and should handle the error gracefully
	assert.NotPanics(t, func() {
		aichteeteapee.WriteJSON(w, http.StatusOK, circular)
	})

	// Should have written the status code before the encoding error
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, aichteeteapee.ContentTypeJSON, w.Header().Get(aichteeteapee.HeaderNameContentType))
}

func TestFileUploadHandler(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		fixtureFile      string
		formFieldName    string
		uploadedFilename string
		expectStatus     int
		expectError      string
		createUploadDir  bool
		uploadDirPath    string
	}{
		{
			name:             "successful file upload",
			method:           http.MethodPost,
			fixtureFile:      "test-file.txt",
			formFieldName:    "file",
			uploadedFilename: "test-file.txt",
			expectStatus:     http.StatusOK,
			createUploadDir:  true,
			uploadDirPath:    "test-uploads",
		},
		{
			name:             "successful empty file upload",
			method:           http.MethodPost,
			fixtureFile:      "empty-file.txt",
			formFieldName:    "file",
			uploadedFilename: "empty-file.txt",
			expectStatus:     http.StatusOK,
			createUploadDir:  true,
			uploadDirPath:    "test-uploads-empty",
		},
		{
			name:             "successful binary file upload",
			method:           http.MethodPost,
			fixtureFile:      "binary-file.dat",
			formFieldName:    "file",
			uploadedFilename: "binary-file.dat",
			expectStatus:     http.StatusOK,
			createUploadDir:  true,
			uploadDirPath:    "test-uploads-binary",
		},
		{
			name:             "successful large file upload",
			method:           http.MethodPost,
			fixtureFile:      "large-file.txt",
			formFieldName:    "file",
			uploadedFilename: "large-file.txt",
			expectStatus:     http.StatusOK,
			createUploadDir:  true,
			uploadDirPath:    "test-uploads-large",
		},
		{
			name:          "method not allowed",
			method:        http.MethodGet,
			expectStatus:  http.StatusMethodNotAllowed,
			expectError:   aichteeteapee.ErrorCodeMethodNotAllowed,
			uploadDirPath: "test-uploads",
		},
		{
			name:          "no file provided",
			method:        http.MethodPost,
			formFieldName: "notfile", // Wrong field name
			expectStatus:  http.StatusBadRequest,
			expectError:   aichteeteapee.ErrorCodeNoFileProvided,
			uploadDirPath: "test-uploads",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temp directory for uploads
			tempDir := filepath.Join(os.TempDir(), tt.uploadDirPath+"-"+time.Now().Format("20060102-150405"))
			if tt.createUploadDir {
				defer os.RemoveAll(tempDir)
			}

			srv := createTestServer()
			handler := srv.FileUploadHandler(tempDir)

			var req *http.Request
			if tt.method == http.MethodPost && tt.fixtureFile != "" {
				// Create multipart form request with file from fixtures
				body, contentType := createMultipartFormWithFile(t, tt.fixtureFile, tt.formFieldName, tt.uploadedFilename)
				req = httptest.NewRequest(tt.method, "/upload", body)
				req.Header.Set("Content-Type", contentType)
			} else if tt.method == http.MethodPost {
				// Create multipart form request without file or wrong field name
				body, contentType := createMultipartFormEmpty(t, tt.formFieldName)
				req = httptest.NewRequest(tt.method, "/upload", body)
				req.Header.Set("Content-Type", contentType)
			} else {
				req = httptest.NewRequest(tt.method, "/upload", nil)
			}

			w := httptest.NewRecorder()
			handler(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError != "" {
				if errorMsg, ok := response["error"].(string); ok {
					assert.Contains(t, errorMsg, tt.expectError)
				} else if code, ok := response["code"].(string); ok {
					assert.Equal(t, tt.expectError, code)
				} else {
					t.Errorf("Expected error message but got response: %+v", response)
				}
			} else {
				// Successful upload - check response structure
				assert.Equal(t, "success", response["status"])
				assert.Equal(t, tt.uploadedFilename, response["original_filename"])
				assert.NotNil(t, response["saved_filename"])
				assert.NotNil(t, response["size"])
				assert.NotNil(t, response["path"])

				// Verify saved filename contains UUID prefix
				savedFilename, ok := response["saved_filename"].(string)
				require.True(t, ok)
				assert.Contains(t, savedFilename, tt.uploadedFilename, "saved filename should contain original filename")
				assert.NotEqual(t, tt.uploadedFilename, savedFilename, "saved filename should be different from original due to UUID prefix")

				// Verify file was actually saved
				filePath, ok := response["path"].(string)
				require.True(t, ok)
				assert.FileExists(t, filePath)

				// Verify file content matches fixture
				uploadedContent, err := os.ReadFile(filePath)
				require.NoError(t, err)

				fixtureContent, err := os.ReadFile(filepath.Join(".fixtures", tt.fixtureFile))
				require.NoError(t, err)

				assert.Equal(t, fixtureContent, uploadedContent, "Uploaded file content should match fixture")

				// Verify file size in response
				expectedSize := len(fixtureContent)
				actualSize := int(response["size"].(float64))
				assert.Equal(t, expectedSize, actualSize)
			}
		})
	}
}

func TestFileUploadHandler_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		setupFunc       func(t *testing.T) (*httptest.ResponseRecorder, *http.Request, string)
		expectStatus    int
		expectError     string
		cleanupRequired bool
	}{
		{
			name: "invalid multipart form",
			setupFunc: func(t *testing.T) (*httptest.ResponseRecorder, *http.Request, string) {
				tempDir := filepath.Join(os.TempDir(), "test-invalid-form-"+time.Now().Format("20060102-150405"))
				req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("invalid-form-data"))
				req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
				return httptest.NewRecorder(), req, tempDir
			},
			expectStatus:    http.StatusBadRequest,
			expectError:     aichteeteapee.ErrorCodeInvalidMultipartForm,
			cleanupRequired: true,
		},
		{
			name: "directory creation permissions test",
			setupFunc: func(t *testing.T) (*httptest.ResponseRecorder, *http.Request, string) {
				// Use a non-existent directory that should be created automatically
				tempDir := filepath.Join(os.TempDir(), "nested", "upload", "dir-"+time.Now().Format("20060102-150405"))
				body, contentType := createMultipartFormWithFile(t, "test-file.txt", "file", "test.txt")
				req := httptest.NewRequest(http.MethodPost, "/upload", body)
				req.Header.Set("Content-Type", contentType)
				return httptest.NewRecorder(), req, tempDir
			},
			expectStatus:    http.StatusOK,
			cleanupRequired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, req, tempDir := tt.setupFunc(t)

			if tt.cleanupRequired {
				defer func() {
					// Clean up the temp directory and its parents
					if strings.Contains(tempDir, "nested") {
						os.RemoveAll(filepath.Dir(filepath.Dir(tempDir)))
					} else {
						os.RemoveAll(tempDir)
					}
				}()
			}

			srv := createTestServer()
			handler := srv.FileUploadHandler(tempDir)
			handler(w, req)

			assert.Equal(t, tt.expectStatus, w.Code)

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectError != "" {
				if errorMsg, ok := response["error"].(string); ok {
					assert.Contains(t, errorMsg, tt.expectError)
				} else if code, ok := response["code"].(string); ok {
					assert.Equal(t, tt.expectError, code)
				} else {
					t.Errorf("Expected error message but got response: %+v", response)
				}
			} else {
				assert.Equal(t, "success", response["status"])
			}
		})
	}
}

// Helper function to create multipart form with file from fixtures
func createMultipartFormWithFile(
	t *testing.T,
	fixtureFile, fieldName, uploadFilename string,
) (*bytes.Buffer, string) {
	t.Helper()

	// Read fixture file
	fixtureContent, err := os.ReadFile(filepath.Join(".fixtures", fixtureFile))
	require.NoError(t, err, "Failed to read fixture file: %s", fixtureFile)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, uploadFilename)
	require.NoError(t, err)

	_, err = part.Write(fixtureContent)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}

// Helper function to create empty multipart form or with wrong field name
func createMultipartFormEmpty(
	t *testing.T,
	fieldName string,
) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if fieldName != "" && fieldName != "file" {
		// Create a form field with wrong name
		err := writer.WriteField(fieldName, "some-value")
		require.NoError(t, err)
	}

	err := writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}

func TestHandleFileUpload(t *testing.T) {
	tests := []struct {
		name         string
		fixtureFile  string
		uploadDir    string
		expectError  bool
		errorSubstr  string
		setupRequest func(t *testing.T, fixtureFile string) *http.Request
	}{
		{
			name:        "successful file upload",
			fixtureFile: "test-file.txt",
			uploadDir:   "test-handle-upload",
			expectError: false,
			setupRequest: func(t *testing.T, fixtureFile string) *http.Request {
				body, contentType := createMultipartFormWithFile(t, fixtureFile, "file", "uploaded.txt")
				req := httptest.NewRequest(http.MethodPost, "/upload", body)
				req.Header.Set("Content-Type", contentType)
				return req
			},
		},
		{
			name:        "invalid multipart form",
			uploadDir:   "test-handle-upload-invalid",
			expectError: true,
			errorSubstr: "parse multipart form",
			setupRequest: func(t *testing.T, fixtureFile string) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("invalid-form"))
				req.Header.Set("Content-Type", "multipart/form-data; boundary=invalid")
				return req
			},
		},
		{
			name:        "no file field",
			uploadDir:   "test-handle-upload-nofile",
			expectError: true,
			errorSubstr: "get form file",
			setupRequest: func(t *testing.T, fixtureFile string) *http.Request {
				body, contentType := createMultipartFormEmpty(t, "notfile")
				req := httptest.NewRequest(http.MethodPost, "/upload", body)
				req.Header.Set("Content-Type", contentType)
				return req
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := filepath.Join(os.TempDir(), tt.uploadDir+"-"+time.Now().Format("20060102-150405"))
			defer os.RemoveAll(tempDir)

			// Ensure upload directory exists
			err := os.MkdirAll(tempDir, 0o750)
			require.NoError(t, err)

			req := tt.setupRequest(t, tt.fixtureFile)
			w := httptest.NewRecorder()

			srv := createTestServer()
			config := &FileUploadHandlerConfig{}
			err = srv.handleFileUpload(w, req, tempDir, config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorSubstr)
				assert.Equal(t, http.StatusBadRequest, w.Code)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)

				// Verify response structure
				var response map[string]any
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "success", response["status"])
			}
		})
	}
}

func TestSaveUploadedFile(t *testing.T) {
	tests := []struct {
		name        string
		fixtureFile string
		expectError bool
		setupPath   func(t *testing.T) string
	}{
		{
			name:        "successful save",
			fixtureFile: "test-file.txt",
			expectError: false,
			setupPath: func(t *testing.T) string {
				tempDir := filepath.Join(os.TempDir(), "test-save-"+time.Now().Format("20060102-150405"))
				err := os.MkdirAll(tempDir, 0o750)
				require.NoError(t, err)
				t.Cleanup(func() { os.RemoveAll(tempDir) })
				return filepath.Join(tempDir, "saved-file.txt")
			},
		},
		{
			name:        "save binary file",
			fixtureFile: "binary-file.dat",
			expectError: false,
			setupPath: func(t *testing.T) string {
				tempDir := filepath.Join(os.TempDir(), "test-save-binary-"+time.Now().Format("20060102-150405"))
				err := os.MkdirAll(tempDir, 0o750)
				require.NoError(t, err)
				t.Cleanup(func() { os.RemoveAll(tempDir) })
				return filepath.Join(tempDir, "binary-file.dat")
			},
		},
		{
			name:        "invalid directory path",
			fixtureFile: "test-file.txt",
			expectError: true,
			setupPath: func(t *testing.T) string {
				// Return a path in a non-existent directory without creating it
				return filepath.Join("/non-existent-dir", "file.txt")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read fixture content
			fixtureContent, err := os.ReadFile(filepath.Join(".fixtures", tt.fixtureFile))
			require.NoError(t, err)

			filePath := tt.setupPath(t)
			src := bytes.NewReader(fixtureContent)

			srv := createTestServer()
			err = srv.saveUploadedFile(src, filePath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "create file")
			} else {
				assert.NoError(t, err)

				// Verify file was created and content matches
				savedContent, err := os.ReadFile(filePath)
				require.NoError(t, err)
				assert.Equal(t, fixtureContent, savedContent)

				// Verify file info
				fileInfo, err := os.Stat(filePath)
				require.NoError(t, err)
				assert.Equal(t, int64(len(fixtureContent)), fileInfo.Size())
			}
		})
	}
}

func TestFileUploadIntegration(t *testing.T) {
	// Integration test using the actual server with middleware
	tempDir := filepath.Join(os.TempDir(), "integration-upload-"+time.Now().Format("20060102-150405"))
	defer os.RemoveAll(tempDir)

	// Create a test server
	server, err := New()
	require.NoError(t, err)

	// Add file upload handler
	server.GetRootGroup().POST("/upload", server.FileUploadHandler(tempDir))

	testServer := httptest.NewServer(server.GetMux())
	defer testServer.Close()

	tests := []struct {
		name            string
		fixtureFile     string
		uploadFilename  string
		expectStatus    int
		validateContent bool
	}{
		{
			name:            "integration test - text file",
			fixtureFile:     "test-file.txt",
			uploadFilename:  "integration-test.txt",
			expectStatus:    http.StatusOK,
			validateContent: true,
		},
		{
			name:            "integration test - large file",
			fixtureFile:     "large-file.txt",
			uploadFilename:  "integration-large.txt",
			expectStatus:    http.StatusOK,
			validateContent: true,
		},
		{
			name:            "integration test - binary file",
			fixtureFile:     "binary-file.dat",
			uploadFilename:  "integration-binary.dat",
			expectStatus:    http.StatusOK,
			validateContent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create multipart form
			body, contentType := createMultipartFormWithFile(t, tt.fixtureFile, "file", tt.uploadFilename)

			// Create request to test server
			resp, err := http.Post(testServer.URL+"/upload", contentType, body)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectStatus, resp.StatusCode)

			if tt.expectStatus == http.StatusOK && tt.validateContent {
				// Parse response
				var response map[string]any
				err := json.NewDecoder(resp.Body).Decode(&response)
				require.NoError(t, err)

				assert.Equal(t, "success", response["status"])
				assert.Equal(t, tt.uploadFilename, response["original_filename"])

				// Verify saved filename contains UUID prefix
				savedFilename, ok := response["saved_filename"].(string)
				require.True(t, ok)
				assert.Contains(t, savedFilename, tt.uploadFilename, "saved filename should contain original filename")
				assert.NotEqual(t, tt.uploadFilename, savedFilename, "saved filename should be different from original due to UUID prefix")

				// Verify file exists and content matches
				filePath, ok := response["path"].(string)
				require.True(t, ok)
				assert.FileExists(t, filePath)

				uploadedContent, err := os.ReadFile(filePath)
				require.NoError(t, err)

				fixtureContent, err := os.ReadFile(filepath.Join(".fixtures", tt.fixtureFile))
				require.NoError(t, err)

				assert.Equal(t, fixtureContent, uploadedContent)

				// Note: X-Request-ID header would be present if we were using the full middleware stack
				// For this basic test, we're focusing on the file upload functionality
			}
		})
	}
}

func TestFileUploadHandlerWithPostprocessor(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "postprocessor-test-"+time.Now().Format("20060102-150405"))
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name             string
		postprocessor    func(map[string]any) (map[string]any, error)
		expectedStatus   int
		validateResponse func(t *testing.T, response map[string]any)
	}{
		{
			name: "postprocessor adds custom fields",
			postprocessor: func(response map[string]any) (map[string]any, error) {
				response["custom_field"] = "added by postprocessor"
				response["processed"] = true
				response["timestamp"] = "2023-01-01T00:00:00Z"
				return response, nil
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]any) {
				assert.Equal(t, "added by postprocessor", response["custom_field"])
				assert.Equal(t, true, response["processed"])
				assert.Equal(t, "2023-01-01T00:00:00Z", response["timestamp"])
				assert.Equal(t, "success", response["status"])
				assert.Contains(t, response, "original_filename")
				assert.Contains(t, response, "saved_filename")
				assert.Contains(t, response, "size")
				assert.Contains(t, response, "path")
			},
		},
		{
			name: "postprocessor modifies existing fields",
			postprocessor: func(response map[string]any) (map[string]any, error) {
				if status, ok := response["status"]; ok {
					response["status"] = status.(string) + "_modified"
				}
				if filename, ok := response["original_filename"]; ok {
					response["original_filename"] = "processed_" + filename.(string)
				}
				return response, nil
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]any) {
				assert.Equal(t, "success_modified", response["status"])
				assert.Equal(t, "processed_test.txt", response["original_filename"])
			},
		},
		{
			name: "postprocessor removes fields",
			postprocessor: func(response map[string]any) (map[string]any, error) {
				delete(response, "path")
				delete(response, "size")
				response["simplified"] = true
				return response, nil
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]any) {
				assert.NotContains(t, response, "path")
				assert.NotContains(t, response, "size")
				assert.Equal(t, true, response["simplified"])
				assert.Contains(t, response, "status")
				assert.Contains(t, response, "original_filename")
				assert.Contains(t, response, "saved_filename")
			},
		},
		{
			name: "postprocessor returns error",
			postprocessor: func(response map[string]any) (map[string]any, error) {
				return nil, assert.AnError
			},
			expectedStatus: http.StatusInternalServerError,
			validateResponse: func(t *testing.T, response map[string]any) {
				assert.Equal(t, "INTERNAL_SERVER_ERROR", response["code"])
				assert.Equal(t, "Internal server error", response["message"])
				assert.NotContains(t, response, "status")
				assert.NotContains(t, response, "original_filename")
			},
		},
		{
			name: "postprocessor returns completely different response",
			postprocessor: func(response map[string]any) (map[string]any, error) {
				return map[string]any{
					"message": "completely new response",
					"code":    42,
					"success": true,
				}, nil
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response map[string]any) {
				assert.Equal(t, "completely new response", response["message"])
				assert.Equal(t, float64(42), response["code"]) // JSON numbers are float64
				assert.Equal(t, true, response["success"])
				assert.NotContains(t, response, "status")
				assert.NotContains(t, response, "original_filename")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := createTestServer()

			// Create handler with postprocessor
			handler := srv.FileUploadHandler(tempDir, WithFileUploadHandlerPostprocessor(tt.postprocessor))

			// Create test request
			body, contentType := createMultipartFormWithFile(t, "test-file.txt", "file", "test.txt")
			req, err := http.NewRequest("POST", "/upload", body)
			require.NoError(t, err)
			req.Header.Set("Content-Type", contentType)

			// Execute request
			rr := httptest.NewRecorder()
			handler(rr, req)

			// Validate status
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Parse and validate response
			var response map[string]any
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)

			// Run custom validation
			tt.validateResponse(t, response)
		})
	}
}

func TestFileUploadHandlerMultiplePostprocessors(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "multi-postprocessor-test-"+time.Now().Format("20060102-150405"))
	defer os.RemoveAll(tempDir)

	srv := createTestServer()

	// Test that only the last postprocessor is used (functional options pattern)
	handler := srv.FileUploadHandler(tempDir,
		WithFileUploadHandlerPostprocessor(func(response map[string]any) (map[string]any, error) {
			response["first"] = true
			return response, nil
		}),
		WithFileUploadHandlerPostprocessor(func(response map[string]any) (map[string]any, error) {
			response["second"] = true
			return response, nil
		}),
	)

	// Create test request
	body, contentType := createMultipartFormWithFile(t, "test-file.txt", "file", "test.txt")
	req, err := http.NewRequest("POST", "/upload", body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)

	// Execute request
	rr := httptest.NewRecorder()
	handler(rr, req)

	// Validate response
	assert.Equal(t, http.StatusOK, rr.Code)

	var response map[string]any
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	// Only the last postprocessor should have been applied
	assert.NotContains(t, response, "first")
	assert.Equal(t, true, response["second"])
}

func TestFileUploadHandlerWithFilenamePrependType(t *testing.T) {
	tests := []struct {
		name         string
		prependType  FilenamePrependType
		uploadedFile string
		validateFunc func(t *testing.T, savedFilename, originalFilename string)
	}{
		{
			name:         "UUID prepend (default)",
			prependType:  FilenamePrependTypeUUID,
			uploadedFile: "test.txt",
			validateFunc: func(t *testing.T, savedFilename, originalFilename string) {
				assert.Contains(t, savedFilename, originalFilename)
				assert.NotEqual(t, savedFilename, originalFilename)
				// Check that it starts with UUID pattern (36 chars + underscore)
				assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}_`, savedFilename)
			},
		},
		{
			name:         "DateTime prepend",
			prependType:  FilenamePrependTypeDateTime,
			uploadedFile: "document.pdf",
			validateFunc: func(t *testing.T, savedFilename, originalFilename string) {
				assert.Contains(t, savedFilename, originalFilename)
				assert.NotEqual(t, savedFilename, originalFilename)
				// Check that it starts with datetime pattern Y_M_D_H_I_S_
				assert.Regexp(t, `^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_`, savedFilename)
			},
		},
		{
			name:         "No prepend",
			prependType:  FilenamePrependTypeNone,
			uploadedFile: "original.txt",
			validateFunc: func(t *testing.T, savedFilename, originalFilename string) {
				assert.Equal(t, savedFilename, originalFilename)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := filepath.Join(os.TempDir(), "prepend-test-"+time.Now().Format("20060102-150405"))
			defer os.RemoveAll(tempDir)

			srv := createTestServer()
			handler := srv.FileUploadHandler(tempDir, WithFilenamePrependType(tt.prependType))

			// Create test request
			body, contentType := createMultipartFormWithFile(t, "test-file.txt", "file", tt.uploadedFile)
			req := httptest.NewRequest(http.MethodPost, "/upload", body)
			req.Header.Set("Content-Type", contentType)

			// Execute request
			w := httptest.NewRecorder()
			handler(w, req)

			// Validate response
			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "success", response["status"])
			assert.Equal(t, tt.uploadedFile, response["original_filename"])

			savedFilename, ok := response["saved_filename"].(string)
			require.True(t, ok)

			// Run the specific validation for this prepend type
			tt.validateFunc(t, savedFilename, tt.uploadedFile)

			// Verify file was actually saved with the expected filename
			filePath, ok := response["path"].(string)
			require.True(t, ok)
			assert.FileExists(t, filePath)
			assert.Contains(t, filePath, savedFilename)
		})
	}
}

func TestGenerateUniqueFilename(t *testing.T) {
	srv := createTestServer()

	tests := []struct {
		name           string
		originalName   string
		prependType    FilenamePrependType
		validateResult func(t *testing.T, result, original string)
	}{
		{
			name:         "UUID prepend",
			originalName: "test.txt",
			prependType:  FilenamePrependTypeUUID,
			validateResult: func(t *testing.T, result, original string) {
				assert.Contains(t, result, original)
				assert.NotEqual(t, result, original)
				assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}_test\.txt$`, result)
			},
		},
		{
			name:         "DateTime prepend",
			originalName: "document.pdf",
			prependType:  FilenamePrependTypeDateTime,
			validateResult: func(t *testing.T, result, original string) {
				assert.Contains(t, result, original)
				assert.NotEqual(t, result, original)
				assert.Regexp(t, `^\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}_document\.pdf$`, result)
			},
		},
		{
			name:         "No prepend",
			originalName: "unchanged.txt",
			prependType:  FilenamePrependTypeNone,
			validateResult: func(t *testing.T, result, original string) {
				assert.Equal(t, result, original)
			},
		},
		{
			name:         "Default case (should be UUID)",
			originalName: "default.txt",
			prependType:  FilenamePrependType(99), // Invalid value, should default to UUID
			validateResult: func(t *testing.T, result, original string) {
				assert.Contains(t, result, original)
				assert.NotEqual(t, result, original)
				assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}_default\.txt$`, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := srv.generateUniqueFilename(tt.originalName, tt.prependType)
			tt.validateResult(t, result, tt.originalName)
		})
	}
}
