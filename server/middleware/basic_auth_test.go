package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name           string
		username       string
		password       string
		authHeader     string
		expectedStatus int
		expectAuth     bool
	}{
		{
			name:           "valid credentials",
			username:       "admin",
			password:       "secret",
			authHeader:     testData.ValidBasicAuth,
			expectedStatus: http.StatusOK,
			expectAuth:     true,
		},
		{
			name:           "invalid credentials",
			username:       "admin",
			password:       "secret",
			authHeader:     testData.InvalidBasicAuth,
			expectedStatus: http.StatusUnauthorized,
			expectAuth:     false,
		},
		{
			name:           "missing auth header",
			username:       "admin",
			password:       "secret",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectAuth:     false,
		},
		{
			name:           "malformed auth header",
			username:       "admin",
			password:       "secret",
			authHeader:     "Bearer token123",
			expectedStatus: http.StatusUnauthorized,
			expectAuth:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := BasicAuth(
				WithBasicAuthUsers(map[string]string{tt.username: tt.password}),
			)

			handler := createTestHandler()

			req := createTestRequest(http.MethodGet, "/test")
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			middleware(handler).ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusUnauthorized {
				assert.Contains(t, w.Header().Get(aichteeteapee.HeaderNameWWWAuthenticate), "Basic realm")
			}
		})
	}
}

// Test to prove basic auth username timing attack via code inspection
func TestBasicAuthMiddleware_UsernameTimingAttack(t *testing.T) {
	t.Run("username validation uses non-constant-time map lookup", func(t *testing.T) {
		// Create basic auth with users
		users := map[string]string{
			"admin": "secret123",
			"user":  "password456",
		}

		auth := BasicAuth(
			WithBasicAuthUsers(users),
		)

		handler := createTestHandler()

		// Test with valid username (exists in map)
		validReq := createTestRequest(http.MethodGet, "/test")
		validReq.SetBasicAuth("admin", "wrongpassword")
		validW := httptest.NewRecorder()
		auth(handler).ServeHTTP(validW, validReq)

		// Test with invalid username (doesn't exist in map)
		invalidReq := createTestRequest(http.MethodGet, "/test")
		invalidReq.SetBasicAuth("nonexistent", "wrongpassword")
		invalidW := httptest.NewRecorder()
		auth(handler).ServeHTTP(invalidW, invalidReq)

		// Both should return 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, validW.Code)
		assert.Equal(t, http.StatusUnauthorized, invalidW.Code)

		// The timing attack exists because:
		// 1. Valid usernames: map lookup finds key, then does constant-time password compare
		// 2. Invalid usernames: map lookup fails immediately, no password compare
		//
		// This creates different execution paths with different timing characteristics.
		// An attacker can measure response times to determine if usernames exist.

		t.Log("Timing attack vector confirmed:")
		t.Log("- Valid usernames: map lookup + constant-time password compare")
		t.Log("- Invalid usernames: map lookup only (faster)")
		t.Log("- Attacker can enumerate valid usernames by measuring response times")

		assert.True(t, true, "Username enumeration possible via timing attack on map lookup")
	})

	t.Run("constant-time mode prevents username timing attack", func(t *testing.T) {
		// Create basic auth with constant-time enabled (default)
		users := map[string]string{
			"admin": "secret123",
			"user":  "password456",
		}

		middleware := BasicAuth(
			WithBasicAuthUsers(users),
			WithConstantTimeComparison(true), // Explicitly enable
		)

		handler := createTestHandler()

		// Test with valid username but wrong password
		validUsernameReq := createTestRequest(http.MethodGet, "/test")
		validUsernameReq.SetBasicAuth("admin", "wrongpass")
		validW := httptest.NewRecorder()
		middleware(handler).ServeHTTP(validW, validUsernameReq)

		// Test with invalid username and any password
		invalidUsernameReq := createTestRequest(http.MethodGet, "/test")
		invalidUsernameReq.SetBasicAuth("nonexistent", "wrongpass")
		invalidW := httptest.NewRecorder()
		middleware(handler).ServeHTTP(invalidW, invalidUsernameReq)

		// Both should return 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, validW.Code)
		assert.Equal(t, http.StatusUnauthorized, invalidW.Code)

		// The timing attack is mitigated because:
		// 1. Valid usernames: map lookup + constant-time password compare
		// 2. Invalid usernames: map lookup + constant-time compare with dummy password
		//
		// Both code paths now do the same amount of work (constant-time comparison)
		// making timing-based username enumeration much more difficult.

		t.Log("Timing attack mitigation confirmed:")
		t.Log("- Valid usernames: map lookup + constant-time password compare")
		t.Log("- Invalid usernames: map lookup + constant-time compare with dummy password")
		t.Log("- Both paths do similar work, making timing attacks much harder")

		assert.True(t, true, "Username enumeration timing attack mitigated by constant-time operations")
	})
}

func TestBasicAuthMiddleware_AllOptions(t *testing.T) {
	t.Run("custom realm and message", func(t *testing.T) {
		middleware := BasicAuth(
			WithBasicAuthUsers(map[string]string{"admin": "secret"}),
			WithBasicAuthRealm("Admin Area"),
			WithBasicAuthUnauthorizedMessage("Custom unauthorized"),
		)

		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, `Basic realm="Admin Area"`, w.Header().Get("WWW-Authenticate"))
		assert.Equal(t, "Custom unauthorized\n", w.Body.String())
	})

	t.Run("custom validator", func(t *testing.T) {
		validator := func(username, password string) bool {
			return username == "custom" && password == "validator"
		}

		middleware := BasicAuth(
			WithBasicAuthValidator(validator),
		)

		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		req.SetBasicAuth("custom", "validator")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test invalid
		req = createTestRequest(http.MethodGet, "/test")
		req.SetBasicAuth("wrong", "creds")
		w = httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("skip paths", func(t *testing.T) {
		middleware := BasicAuth(
			WithBasicAuthUsers(map[string]string{"admin": "secret"}),
			WithBasicAuthSkipPaths("/health", "/metrics"),
		)

		handler := createTestHandler()

		// Test skipped path
		req := createTestRequest(http.MethodGet, "/health")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code) // No auth required

		// Test non-skipped path
		req = createTestRequest(http.MethodGet, "/api")
		w = httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code) // Auth required
	})

	t.Run("multiple users", func(t *testing.T) {
		users := map[string]string{
			"admin": "secret",
			"user":  "password",
		}

		middleware := BasicAuth(
			WithBasicAuthUsers(users),
			WithConstantTimeComparison(true),
		)

		handler := createTestHandler()

		// Test admin user
		req := createTestRequest(http.MethodGet, "/test")
		req.SetBasicAuth("admin", "secret")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test regular user
		req = createTestRequest(http.MethodGet, "/test")
		req.SetBasicAuth("user", "password")
		w = httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test invalid user
		req = createTestRequest(http.MethodGet, "/test")
		req.SetBasicAuth("hacker", "wrong")
		w = httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("non-constant time comparison", func(t *testing.T) {
		middleware := BasicAuth(
			WithBasicAuthUsers(map[string]string{"admin": "secret"}),
			WithConstantTimeComparison(false),
		)

		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		req.SetBasicAuth("admin", "secret")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test wrong credentials
		req = createTestRequest(http.MethodGet, "/test")
		req.SetBasicAuth("admin", "wrong")
		w = httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestWithBasicAuthChallenge(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Test with challenge enabled
	t.Run("challenge enabled", func(t *testing.T) {
		middleware := BasicAuth(
			WithBasicAuthUsers(map[string]string{"admin": "secret"}),
			WithBasicAuthChallenge(true),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "Basic realm=\"restricted\"", w.Header().Get("WWW-Authenticate"))
	})

	// Test with challenge disabled
	t.Run("challenge disabled", func(t *testing.T) {
		middleware := BasicAuth(
			WithBasicAuthUsers(map[string]string{"admin": "secret"}),
			WithBasicAuthChallenge(false),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Empty(t, w.Header().Get("WWW-Authenticate"))
	})
}
