// Package auth protects Streamable HTTP MCP routes with OAuth bearer tokens.
package auth

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/aichteeteapee/serbewr"
	"github.com/psyb0t/aichteeteapee/serbewr/middleware"
	"github.com/psyb0t/ctxerrors"
)

const metadataPrefix = "/.well-known/oauth-protected-resource"

type config struct {
	requiredScopes []string
}

// Option configures OAuth protection for an MCP endpoint.
type Option func(*config)

// WithRequiredScopes requires every listed scope on the endpoint and advertises
// those scopes in discovery and bearer challenges.
func WithRequiredScopes(scopes ...string) Option {
	scopes = slices.Clone(scopes)

	return func(config *config) {
		config.requiredScopes = scopes
	}
}

// Protection holds the authorization middleware and public discovery route for
// a single MCP resource.
type Protection struct {
	Middleware    middleware.Middleware
	MetadataRoute serbewr.RouteConfig
}

// New binds tokens to an exact HTTPS resource URL and trusted issuer. Mount
// Middleware on the MCP endpoint only and MetadataRoute at the router root.
// The verifier checks signatures or introspection results and token expiry.
func New(
	resourceURL, issuer string,
	verify middleware.BearerTokenVerifier,
	opts ...Option,
) (Protection, error) {
	resource, err := parseResource(resourceURL, issuer)
	if err != nil {
		return Protection{}, ctxerrors.Wrap(
			err, "configure MCP OAuth URLs",
		)
	}

	settings := config{}
	for _, opt := range opts {
		opt(&settings)
	}

	for _, scope := range settings.requiredScopes {
		if !validScope(scope) {
			return Protection{}, ctxerrors.Wrap(
				aichteeteapee.ErrBadRequest, "invalid MCP OAuth scope",
			)
		}
	}

	return settings.protection(resource, issuer, verify)
}

func validScope(scope string) bool {
	// RFC 6749 scope-token permits visible ASCII except quote and backslash.
	return scope != "" && !strings.ContainsFunc(scope, func(r rune) bool {
		return r <= ' ' || r > '~' || r == '"' || r == '\\'
	})
}

func parseResource(resourceURL, issuer string) (*url.URL, error) {
	resource, resourceErr := url.Parse(resourceURL)
	issuerURL, issuerErr := url.Parse(issuer)

	if resourceErr != nil || issuerErr != nil ||
		!validHTTPS(resource, resourceURL) || !validHTTPS(issuerURL, issuer) {
		return nil, ctxerrors.Wrap(
			aichteeteapee.ErrBadRequest, "MCP OAuth requires HTTPS URLs",
		)
	}

	resourcePath := resource.Path
	if resourcePath == "" || path.Clean(resourcePath) != resourcePath ||
		resource.EscapedPath() != resourcePath ||
		strings.HasPrefix(resourcePath, metadataPrefix) {
		return nil, ctxerrors.Wrap(
			aichteeteapee.ErrBadRequest,
			"MCP OAuth requires a literal endpoint path",
		)
	}

	return resource, nil
}

func validHTTPS(value *url.URL, raw string) bool {
	return value.Scheme == "https" && value.Hostname() != "" &&
		value.User == nil && value.RawQuery == "" && !value.ForceQuery &&
		!strings.Contains(raw, "#")
}

func (config config) protection(
	resource *url.URL,
	issuer string,
	verify middleware.BearerTokenVerifier,
) (Protection, error) {
	authenticate, err := middleware.NewBearerAuthenticator(
		verify, resource.String(),
	)
	if err != nil {
		return Protection{}, ctxerrors.Wrap(err, "configure MCP token verifier")
	}

	metadataPath := metadataPrefix + strings.TrimSuffix(resource.Path, "/")
	metadataURL := url.URL{
		Scheme: resource.Scheme, Host: resource.Host, Path: metadataPath,
	}

	guard, err := middleware.RequireAuth(
		config.checkAccess(authenticate, issuer, resource.Path),
		middleware.WithAuthChallenge(config.challenge(metadataURL.String())),
	)
	if err != nil {
		return Protection{}, ctxerrors.Wrap(err, "configure MCP authorization")
	}

	metadata := oauthex.ProtectedResourceMetadata{
		Resource: resource.String(), AuthorizationServers: []string{issuer},
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        config.requiredScopes,
	}

	return Protection{
		Middleware: guard,
		MetadataRoute: serbewr.RouteConfig{
			Method: http.MethodGet,
			Path:   metadataPath,
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				aichteeteapee.WriteJSON(w, http.StatusOK, metadata)
			},
		},
	}, nil
}

func (config config) checkAccess(
	authenticate middleware.Authenticator, issuer, resourcePath string,
) middleware.Authenticator {
	return func(r *http.Request) (middleware.Principal, error) {
		if r.URL.Path != resourcePath {
			return middleware.Principal{}, aichteeteapee.ErrUnauthorized
		}

		principal, err := authenticate(r)
		if err != nil {
			return middleware.Principal{}, ctxerrors.Wrap(
				err, "authenticate MCP request",
			)
		}

		if principal.Issuer != issuer {
			return middleware.Principal{}, aichteeteapee.ErrUnauthorized
		}

		granted := strings.Split(principal.Scope, " ")
		for _, required := range config.requiredScopes {
			if !slices.Contains(granted, required) {
				return middleware.Principal{}, aichteeteapee.ErrForbidden
			}
		}

		return principal, nil
	}
}

func (config config) challenge(metadataURL string) func(error) string {
	challenge := aichteeteapee.AuthSchemeBearer +
		"resource_metadata=" + strconv.Quote(metadataURL)

	if len(config.requiredScopes) > 0 {
		scope := strings.Join(config.requiredScopes, " ")
		challenge += ", scope=" + strconv.Quote(scope)
	}

	return func(err error) string {
		if errors.Is(err, aichteeteapee.ErrForbidden) {
			return challenge + `, error="insufficient_scope"`
		}

		return challenge
	}
}
