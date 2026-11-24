package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	mcpauth "github.com/psyb0t/aichteeteapee/serbewr/em-see-pee/auth"
	"github.com/psyb0t/aichteeteapee/serbewr/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	resourcePath = "/apps/mailbox/mcp"
	resourceURL  = "https://example.org" + resourcePath
	issuer       = "https://auth.example.org"
	metadataPath = "/.well-known/oauth-protected-resource" + resourcePath
	adminSubject = "admin-user"
	readScope    = "mail:read"
	validToken   = "example-valid"
)

func TestOAuthBoundary(t *testing.T) {
	t.Parallel()

	protection, err := mcpauth.New(resourceURL, issuer, verify,
		mcpauth.WithRequiredScopes(readScope))
	require.NoError(t, err)
	dashboard, err := middleware.RequireAuth(verifyDashboardSession)
	require.NoError(t, err)

	mux := http.NewServeMux()
	metadataRoute := protection.MetadataRoute
	mux.HandleFunc(
		metadataRoute.Method+" "+metadataRoute.Path, metadataRoute.Handler,
	)

	showPrincipal := func(w http.ResponseWriter, r *http.Request) {
		principal, ok := middleware.PrincipalFromContext(r.Context())
		if !ok || principal.Subject != adminSubject {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
	handler := http.HandlerFunc(showPrincipal)
	mux.Handle(resourcePath, protection.Middleware(handler))
	mux.Handle("/other/mcp", protection.Middleware(handler))
	mux.Handle("/dashboard/admin", dashboard(handler))

	metadata := httptest.NewRecorder()
	discovery := httptest.NewRequest(http.MethodGet, metadataPath, nil)
	mux.ServeHTTP(metadata, discovery)
	assert.Equal(t, http.StatusOK, metadata.Code)
	assert.JSONEq(t, `{
		"resource": "https://example.org/apps/mailbox/mcp",
		"authorization_servers": ["https://auth.example.org"],
		"scopes_supported": ["mail:read"],
		"bearer_methods_supported": ["header"]
	}`, metadata.Body.String())

	testCases := []struct {
		name, path, token string
		cookie, duplicate bool
		status            int
	}{
		{
			"valid MCP", resourcePath, validToken,
			false, false, http.StatusNoContent,
		},
		{
			"valid session", "/dashboard/admin", "",
			true, false, http.StatusNoContent,
		},
		{
			"MCP token on dashboard", "/dashboard/admin", validToken,
			false, false, http.StatusUnauthorized,
		},
		{
			"dashboard cookie on MCP", resourcePath, "",
			true, false, http.StatusUnauthorized,
		},
		{
			"wrong path", "/other/mcp", validToken,
			false, false, http.StatusUnauthorized,
		},
		{
			"wrong audience", resourcePath, "example-wrong-resource",
			false, false, http.StatusUnauthorized,
		},
		{
			"wrong issuer", resourcePath, "example-wrong-issuer",
			false, false, http.StatusUnauthorized,
		},
		{
			"missing scope", resourcePath, "example-no-scope",
			false, false, http.StatusForbidden,
		},
		{
			"verifier offline", resourcePath, "example-verifier-offline",
			false, false, http.StatusInternalServerError,
		},
		{
			"invalid token", resourcePath, "example-invalid",
			false, false, http.StatusUnauthorized,
		},
		{
			"malformed token", resourcePath, "a b",
			false, false, http.StatusUnauthorized,
		},
		{
			"duplicate credentials", resourcePath, validToken,
			false, true, http.StatusUnauthorized,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, tc.path, nil)
			if tc.cookie {
				request.AddCookie(&http.Cookie{
					Name: "session", Value: "example-session",
				})
			}

			if tc.token != "" {
				request.Header.Add(
					aichteeteapee.HeaderNameAuthorization,
					aichteeteapee.AuthSchemeBearer+tc.token,
				)
			}

			if tc.duplicate {
				request.Header.Add(
					aichteeteapee.HeaderNameAuthorization, "Bearer duplicate",
				)
			}

			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			assert.Equal(t, tc.status, response.Code)

			if tc.path != resourcePath || tc.status == http.StatusNoContent ||
				tc.status == http.StatusInternalServerError {
				return
			}

			challenge := response.Header().Get(
				aichteeteapee.HeaderNameWWWAuthenticate,
			)
			assert.Contains(t, challenge,
				`resource_metadata="https://example.org`+metadataPath+`"`)
			assert.Contains(t, challenge, `scope="mail:read"`)
			assert.Equal(t, tc.status == http.StatusForbidden,
				strings.Contains(challenge, `error="insufficient_scope"`))
		})
	}
}

func verify(
	_ context.Context, token, audience string,
) (middleware.BearerPrincipal, error) {
	principal := middleware.BearerPrincipal{
		Issuer: issuer, Subject: adminSubject,
		Audience: audience, Scope: readScope,
	}

	switch token {
	case validToken:
	case "example-wrong-resource":
		principal.Audience = "https://example.org/apps/other/mcp"
	case "example-wrong-issuer":
		principal.Issuer = "https://other.example.org"
	case "example-no-scope":
		principal.Scope = "mail:read-extra"
	case "example-verifier-offline":
		return middleware.BearerPrincipal{}, aichteeteapee.ErrAPIError
	default:
		return middleware.BearerPrincipal{}, aichteeteapee.ErrUnauthorized
	}

	return principal, nil
}

func verifyDashboardSession(r *http.Request) (middleware.Principal, error) {
	_, err := r.Cookie("session")
	if err != nil {
		return middleware.Principal{}, aichteeteapee.ErrUnauthorized
	}

	return middleware.Principal{Issuer: issuer, Subject: adminSubject}, nil
}

func TestOAuthConfiguration(t *testing.T) {
	t.Parallel()

	invalidURLs := []string{
		"http://example.org/mcp", "https://example.org",
		resourceURL + "?x=1", resourceURL + "/../other",
	}
	for _, value := range invalidURLs {
		_, err := mcpauth.New(value, issuer, verify)
		assert.ErrorIs(t, err, aichteeteapee.ErrBadRequest, value)
	}

	_, err := mcpauth.New(resourceURL, "http://example.org", verify)
	assert.ErrorIs(t, err, aichteeteapee.ErrBadRequest)
	_, err = mcpauth.New(resourceURL, issuer, nil)
	assert.ErrorIs(t, err, aichteeteapee.ErrBadRequest)
	_, err = mcpauth.New(resourceURL, issuer, verify,
		mcpauth.WithRequiredScopes(`read"invalid`))
	assert.ErrorIs(t, err, aichteeteapee.ErrBadRequest)
}
