package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/aichteeteapee/serbewr/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name, method string
		principal    middleware.Principal
		err          error
		status       int
	}{
		{
			name:      "identity",
			method:    http.MethodGet,
			principal: middleware.Principal{Subject: "user"},
			status:    http.StatusNoContent,
		},
		{
			name:   "empty identity",
			method: http.MethodPost,
			status: http.StatusUnauthorized,
		},
		{
			name:   "login redirect",
			method: http.MethodGet,
			err:    aichteeteapee.ErrUnauthorized,
			status: http.StatusSeeOther,
		},
		{
			name:   "forbidden",
			method: http.MethodGet,
			err:    aichteeteapee.ErrForbidden,
			status: http.StatusForbidden,
		},
		{
			name:   "backend failure",
			method: http.MethodGet,
			err:    aichteeteapee.ErrAPIError,
			status: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			guard, err := middleware.RequireAuth(
				func(*http.Request) (middleware.Principal, error) {
					return tc.principal, tc.err
				},
				middleware.WithAuthLoginRedirect("/login"),
			)
			require.NoError(t, err)

			called := false
			handler := func(w http.ResponseWriter, r *http.Request) {
				called = true
				principal, ok := middleware.PrincipalFromContext(r.Context())
				assert.True(t, ok)
				assert.Equal(t, tc.principal, principal)
				w.WriteHeader(http.StatusNoContent)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, "/dashboard", nil)
			guard(http.HandlerFunc(handler)).ServeHTTP(response, request)
			assert.Equal(t, tc.status, response.Code)
			assert.Equal(t, tc.status == http.StatusNoContent, called)

			if tc.status == http.StatusSeeOther {
				assert.Equal(t, "/login", response.Header().Get("Location"))
			}
		})
	}
}

func TestRequireAuthConfiguration(t *testing.T) {
	t.Parallel()

	_, err := middleware.RequireAuth(nil)
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	authenticate := func(*http.Request) (middleware.Principal, error) {
		return middleware.Principal{}, aichteeteapee.ErrUnauthorized
	}

	invalidPaths := []string{
		"https://example.org", "//example.org", "/%2fexample.org",
		`/\example.org`,
	}
	for _, path := range invalidPaths {
		_, err := middleware.RequireAuth(
			authenticate, middleware.WithAuthLoginRedirect(path),
		)
		assert.ErrorIs(t, err, aichteeteapee.ErrBadRequest, path)
	}
}
