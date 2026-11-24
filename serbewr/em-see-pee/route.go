// Package emseepee mounts stateless MCP Streamable HTTP on a serbewr router.
package emseepee

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/aichteeteapee/serbewr"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
)

type routeConfig struct {
	sdkOptions   mcp.StreamableHTTPOptions
	publicOrigin string
	writeTimeout time.Duration
}

// RouteOption configures a Streamable HTTP route.
type RouteOption func(*routeConfig)

// WithSDKOptions sets the SDK options. The route always uses stateless mode
// and enables cancellation propagation for protocol versions that support it.
func WithSDKOptions(options mcp.StreamableHTTPOptions) RouteOption {
	return func(config *routeConfig) {
		config.sdkOptions = options
	}
}

// WithPublicOrigin sets the exact browser Origin allowed behind a TLS proxy.
// By default, the request's own scheme and Host determine its origin.
func WithPublicOrigin(origin string) RouteOption {
	return func(config *routeConfig) {
		config.publicOrigin = origin
	}
}

// WithWriteTimeout sets a finite write deadline for this route, including
// streaming responses. A zero value inherits the listener's write deadline.
// Give tool work its own timeout, especially for older MCP clients.
func WithWriteTimeout(timeout time.Duration) RouteOption {
	return func(config *routeConfig) {
		config.writeTimeout = timeout
	}
}

// NewRoute builds one endpoint with SDK-owned HTTP method handling.
// Put it under a Router group to apply group auth on every request.
func NewRoute(
	path string,
	getServer func(*http.Request) *mcp.Server,
	opts ...RouteOption,
) (serbewr.RouteConfig, error) {
	if !strings.HasPrefix(path, "/") || getServer == nil {
		return serbewr.RouteConfig{}, ctxerrors.Wrap(
			aichteeteapee.ErrBadRequest, "MCP requires a route path and server",
		)
	}

	config := routeConfig{}
	for _, opt := range opts {
		opt(&config)
	}

	if err := config.validate(); err != nil {
		return serbewr.RouteConfig{}, ctxerrors.Wrap(err, "configure MCP route")
	}

	config.sdkOptions.Stateless = true
	config.sdkOptions.PropagateRequestCancellation = true
	handler := mcp.NewStreamableHTTPHandler(getServer, &config.sdkOptions)

	return serbewr.RouteConfig{
		Path: path,
		Handler: func(w http.ResponseWriter, r *http.Request) {
			config.serve(w, r, handler)
		},
	}, nil
}

func (c routeConfig) validate() error {
	if c.writeTimeout < 0 || c.sdkOptions.MaxRequestBodyBytes < 0 {
		return ctxerrors.Wrap(
			aichteeteapee.ErrBadRequest, "negative MCP route limit",
		)
	}

	if c.publicOrigin == "" {
		return nil
	}

	origin, err := url.Parse(c.publicOrigin)
	if err != nil || !bareOrigin(origin, c.publicOrigin) {
		return ctxerrors.Wrap(aichteeteapee.ErrBadRequest, "invalid MCP origin")
	}

	return nil
}

func bareOrigin(origin *url.URL, raw string) bool {
	return (origin.Scheme == "https" || origin.Scheme == "http") &&
		origin.Hostname() != "" && raw == origin.Scheme+"://"+origin.Host
}

func (c routeConfig) serve(
	w http.ResponseWriter,
	r *http.Request,
	handler http.Handler,
) {
	if !c.allowsOrigin(r) {
		ctxscope.GetLogger(r.Context()).Warn(
			"MCP origin rejected", aichteeteapee.FieldReason, "invalid_origin",
		)
		aichteeteapee.WriteJSON(
			w, http.StatusForbidden, aichteeteapee.ErrorResponseForbidden,
		)

		return
	}

	if c.writeTimeout == 0 {
		handler.ServeHTTP(w, r)

		return
	}

	deadline := time.Now().Add(c.writeTimeout)

	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(deadline); err != nil {
		ctxscope.GetLogger(r.Context()).Error(
			"set MCP write deadline", "err", err,
		)
		aichteeteapee.WriteJSON(
			w, http.StatusInternalServerError,
			aichteeteapee.ErrorResponseInternalServerError,
		)

		return
	}

	ctx, cancel := context.WithDeadline(r.Context(), deadline)
	defer cancel()

	handler.ServeHTTP(w, r.WithContext(ctx))
}

func (c routeConfig) allowsOrigin(r *http.Request) bool {
	origins := r.Header.Values(aichteeteapee.HeaderNameOrigin)
	if len(origins) == 0 {
		return true
	}

	expected := c.publicOrigin
	if expected == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}

		expected = scheme + "://" + r.Host
	}

	return len(origins) == 1 && origins[0] == expected
}
