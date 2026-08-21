package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestBearerAuth(t *testing.T) {
	const token = "s3cr3t-api-key" //gitleaks:allow

	testCases := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{"valid token", "Bearer " + token, http.StatusOK},
		{"case-insensitive scheme", "bearer " + token, http.StatusOK},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
		{"missing scheme", token, http.StatusUnauthorized},
		{"basic scheme", "Basic dXNlcjpwYXNz", http.StatusUnauthorized},
		{"scheme only, no token", "Bearer ", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mw := BearerAuth(WithBearerAuthTokens(token))
			handler := createTestHandler()

			req := createTestRequest(http.MethodGet, "/test")
			if tc.authHeader != "" {
				req.Header.Set(
					aichteeteapee.HeaderNameAuthorization, tc.authHeader,
				)
			}

			w := httptest.NewRecorder()
			mw(handler).ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusUnauthorized {
				assert.Contains(
					t, w.Body.String(),
					aichteeteapee.ErrorCodeUnauthorized,
				)
			}
		})
	}
}

func TestBearerAuth_Options(t *testing.T) {
	t.Run("multiple tokens, empty ignored", func(t *testing.T) {
		mw := BearerAuth(WithBearerAuthTokens("tok-a", "", "tok-b"))
		handler := createTestHandler()

		for _, tok := range []string{"tok-a", "tok-b"} {
			req := createTestRequest(http.MethodGet, "/test")
			req.Header.Set(
				aichteeteapee.HeaderNameAuthorization, "Bearer "+tok,
			)

			w := httptest.NewRecorder()
			mw(handler).ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		// An empty bearer token must never authenticate, even though "" was
		// passed to WithBearerAuthTokens.
		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set(aichteeteapee.HeaderNameAuthorization, "Bearer ")

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("custom validator replaces token set", func(t *testing.T) {
		mw := BearerAuth(
			WithBearerAuthValidator(func(token string) bool {
				return token == "let-me-in"
			}),
		)
		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set(
			aichteeteapee.HeaderNameAuthorization, "Bearer let-me-in",
		)

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		req = createTestRequest(http.MethodGet, "/test")
		req.Header.Set(
			aichteeteapee.HeaderNameAuthorization, "Bearer wrong",
		)

		w = httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("skip paths bypass auth", func(t *testing.T) {
		mw := BearerAuth(
			WithBearerAuthTokens("tok"),
			WithBearerAuthSkipPaths("/health", "/metrics"),
		)
		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/health")
		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		req = createTestRequest(http.MethodGet, "/api")
		w = httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("no tokens and no validator rejects everything", func(t *testing.T) {
		mw := BearerAuth()
		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set(
			aichteeteapee.HeaderNameAuthorization, "Bearer anything",
		)

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("non-constant-time still authenticates", func(t *testing.T) {
		mw := BearerAuth(
			WithBearerAuthTokens("tok"),
			WithBearerAuthConstantTimeComparison(false),
		)
		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		req.Header.Set(aichteeteapee.HeaderNameAuthorization, "Bearer tok")

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		req = createTestRequest(http.MethodGet, "/test")
		req.Header.Set(aichteeteapee.HeaderNameAuthorization, "Bearer nope")

		w = httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("custom unauthorized message in json body", func(t *testing.T) {
		mw := BearerAuth(
			WithBearerAuthTokens("tok"),
			WithBearerAuthUnauthorizedMessage("nope, get a token"),
		)
		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "nope, get a token")
		assert.Contains(
			t, w.Body.String(), aichteeteapee.ErrorCodeUnauthorized,
		)
	})
}
