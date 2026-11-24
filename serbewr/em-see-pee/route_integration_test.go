package emseepee_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/aichteeteapee/serbewr"
	emseepee "github.com/psyb0t/aichteeteapee/serbewr/em-see-pee"
	"github.com/psyb0t/aichteeteapee/serbewr/middleware"
	"github.com/psyb0t/ctxerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	routePath       = "/mcp"
	routeAudience   = "https://example.org/apps/image/mcp"
	routeIssuer     = "https://example.authkit.app"
	routeOrigin     = "https://example.org"
	routeSubject    = "user_example"
	routeCredential = "example-credential"
	toolName        = "identity"
	listenerTimeout = 100 * time.Millisecond
)

func TestAuthenticatedStreamableHTTP(t *testing.T) {
	t.Parallel()

	progress := make(chan struct{}, 1)
	server := mcp.NewServer(&mcp.Implementation{Name: "fixture"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: toolName}, identityTool(progress))

	route, err := emseepee.NewRoute(
		routePath,
		func(*http.Request) *mcp.Server { return server },
		emseepee.WithPublicOrigin(routeOrigin),
		emseepee.WithWriteTimeout(3*time.Second),
	)
	require.NoError(t, err)

	auth, err := middleware.JWTBearerAuth(fixtureVerifier, routeAudience)
	require.NoError(t, err)

	endpoint := startRouter(t, route, auth) + routePath
	client := &http.Client{
		Transport: credentialTransport{},
		Timeout:   3 * time.Second,
	}
	t.Cleanup(client.CloseIdleConnections)

	for _, version := range mcp.SupportedProtocolVersions()[:2] {
		exerciseMCP(t, endpoint, client, progress, version)
	}

	checkRouteMethod(t, endpoint, http.MethodGet,
		"", "", http.StatusUnauthorized)
	checkRouteMethod(t, endpoint, http.MethodDelete,
		"", "", http.StatusUnauthorized)
	checkRouteMethod(t, endpoint, http.MethodGet, "https://other.example",
		routeCredential, http.StatusForbidden)
	checkRouteMethod(t, endpoint, http.MethodDelete, routeOrigin,
		routeCredential, http.StatusMethodNotAllowed)
}

func exerciseMCP(
	t *testing.T,
	endpoint string,
	client *http.Client,
	progress chan<- struct{},
	version string,
) {
	t.Helper()

	clientMCP := mcp.NewClient(
		&mcp.Implementation{Name: "fixture"},
		&mcp.ClientOptions{
			ProgressNotificationHandler: func(
				_ context.Context, _ *mcp.ProgressNotificationClientRequest,
			) {
				select {
				case progress <- struct{}{}:
				default:
				}
			},
		},
	)

	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           client,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}

	options := &mcp.ClientSessionOptions{ProtocolVersion: version}

	session, err := clientMCP.Connect(t.Context(), transport, options)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, session.Close()) })

	list, err := session.ListTools(t.Context(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	require.Equal(t, toolName, list.Tools[0].Name)

	call := &mcp.CallToolParams{Name: toolName}
	call.SetProgressToken("identity-progress")

	result, err := session.CallTool(t.Context(), call)
	require.NoError(t, err)
	require.False(t, result.IsError, "%+v", result)
	require.Len(t, result.Content, 1)
	require.Equal(t, &mcp.TextContent{Text: routeSubject}, result.Content[0])
}

func identityTool(progress <-chan struct{}) mcp.ToolHandlerFor[struct{}, any] {
	return func(
		ctx context.Context,
		request *mcp.CallToolRequest,
		_ struct{},
	) (*mcp.CallToolResult, any, error) {
		principal, ok := middleware.BearerPrincipalFromContext(ctx)
		if !ok {
			return nil, nil, ctxerrors.New("verified identity missing")
		}

		params := &mcp.ProgressNotificationParams{
			ProgressToken: request.Params.GetProgressToken(),
			Progress:      1,
		}

		err := request.Session.NotifyProgress(ctx, params)
		if err != nil {
			return nil, nil, ctxerrors.Wrap(err, "send progress")
		}

		// The final result depends on the client receiving an early SSE frame.
		select {
		case <-progress:
		case <-ctx.Done():
			return nil, nil, ctxerrors.Wrap(ctx.Err(), "wait for progress")
		}

		delay := time.NewTimer(2 * listenerTimeout)
		defer delay.Stop()

		select {
		case <-delay.C:
		case <-ctx.Done():
			return nil, nil, ctxerrors.Wrap(ctx.Err(), "wait for stream")
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: principal.Subject}},
		}, nil, nil
	}
}

func fixtureVerifier(
	_ context.Context,
	token, audience string,
) (middleware.BearerPrincipal, error) {
	if token != routeCredential {
		return middleware.BearerPrincipal{}, aichteeteapee.ErrUnauthorized
	}

	return middleware.BearerPrincipal{
		Issuer: routeIssuer, Subject: routeSubject, Audience: audience,
	}, nil
}

type credentialTransport struct{}

func (credentialTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	request := r.Clone(r.Context())
	request.Header.Set(aichteeteapee.HeaderNameAuthorization,
		aichteeteapee.AuthSchemeBearer+routeCredential)

	response, err := http.DefaultTransport.RoundTrip(request)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "send MCP test request")
	}

	return response, nil
}

func startRouter(
	t *testing.T, route serbewr.RouteConfig, auth middleware.Middleware,
) string {
	t.Helper()

	server, err := serbewr.NewWithConfig(serbewr.Config{
		ListenAddress:   "127.0.0.1:0",
		WriteTimeout:    listenerTimeout,
		ShutdownTimeout: time.Second,
	})
	require.NoError(t, err)

	router := &serbewr.Router{
		GlobalMiddlewares: []middleware.Middleware{
			middleware.RequestID(), middleware.Logger(), middleware.Recovery(),
		},
		Groups: []serbewr.GroupConfig{{
			Path:        "/",
			Middlewares: []middleware.Middleware{auth},
			Routes:      []serbewr.RouteConfig{route},
		}},
	}
	done := make(chan error, 1)

	ctx, cancel := context.WithCancel(t.Context())

	go func() { done <- server.Start(ctx, router) }()

	t.Cleanup(func() {
		cancel()

		select {
		case err := <-done:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})

	require.Eventually(t, func() bool {
		return server.GetHTTPListenerAddr() != nil
	}, time.Second, time.Millisecond)

	return "http://" + server.GetHTTPListenerAddr().String()
}

func checkRouteMethod(
	t *testing.T,
	endpoint, method, origin, token string,
	status int,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	require.NoError(t, err)

	if origin != "" {
		request.Header.Set(aichteeteapee.HeaderNameOrigin, origin)
	}

	if token != "" {
		request.Header.Set(aichteeteapee.HeaderNameAuthorization,
			aichteeteapee.AuthSchemeBearer+token)
	}

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)

	defer func() { assert.NoError(t, response.Body.Close()) }()

	assert.Equal(t, status, response.StatusCode)
}

func TestRouteConfiguration(t *testing.T) {
	t.Parallel()

	getServer := func(*http.Request) *mcp.Server { return nil }
	_, err := emseepee.NewRoute("", getServer)
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	_, err = emseepee.NewRoute(routePath, getServer,
		emseepee.WithPublicOrigin(routeOrigin+"/path"))
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	_, err = emseepee.NewRoute(routePath, getServer,
		emseepee.WithWriteTimeout(-time.Second))
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	route, err := emseepee.NewRoute(routePath, getServer,
		emseepee.WithSDKOptions(mcp.StreamableHTTPOptions{JSONResponse: true}))
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, routeOrigin+routePath, nil)
	request.Header.Set(aichteeteapee.HeaderNameOrigin, routeOrigin)

	response := httptest.NewRecorder()
	route.Handler(response, request)
	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
}
