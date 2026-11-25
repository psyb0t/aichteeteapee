package emseepee_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/psyb0t/aichteeteapee"
	emseepee "github.com/psyb0t/aichteeteapee/serbewr/em-see-pee"
	"github.com/psyb0t/ctxerrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policyPath          = "/mcp"
	policyLegacyVersion = "2025-06-18"
	policyBlockTool     = "block"
	policySlowTool      = "slow"
	policySlowText      = "slow-done"
	policyWait          = 3 * time.Second
)

func passThrough(next http.Handler) http.Handler { return next }

func policyRoute(
	t *testing.T, opts ...emseepee.RouteOption,
) func(*http.Request) *httptest.ResponseRecorder {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "policy"}, nil)
	route, err := emseepee.NewRoute(policyPath,
		func(*http.Request) *mcp.Server { return server }, opts...)
	require.NoError(t, err)

	return func(request *http.Request) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		route.Handler(response, request)

		return response
	}
}

func policyRequest(method, host string, origins ...string) *http.Request {
	request := httptest.NewRequest(method, "http://"+host+policyPath, nil)
	request.Host = host

	for _, origin := range origins {
		request.Header.Add(aichteeteapee.HeaderNameOrigin, origin)
	}

	return request
}

func requireEnvelope(
	t *testing.T, response *httptest.ResponseRecorder, status int, code string,
) {
	t.Helper()

	require.Equal(t, status, response.Code)

	var envelope aichteeteapee.ErrorResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	assert.Equal(t, code, envelope.Code)
}

func TestAllowedHosts(t *testing.T) {
	t.Parallel()

	serve := policyRoute(t,
		emseepee.WithAllowedHosts("LocalHost", "[::1]", "mcp.example.org"))

	for _, host := range []string{
		"localhost", "localhost:8080", "[::1]:9", "MCP.example.org:443",
	} {
		response := serve(policyRequest(http.MethodGet, host))
		assert.Equal(t, http.StatusMethodNotAllowed, response.Code, host)
	}

	for _, host := range []string{
		"evil.example", "localhost:0", "localhost:99999", "127.0.0.1",
		"mcp.example.org.evil.example",
	} {
		requireEnvelope(t, serve(policyRequest(http.MethodGet, host)),
			http.StatusMisdirectedRequest,
			aichteeteapee.ErrorCodeMisdirectedRequest)
	}
}

func TestAllowedOrigins(t *testing.T) {
	t.Parallel()

	serve := policyRoute(t,
		emseepee.WithAllowedOrigins("http://localhost:8080"),
		emseepee.WithPublicOrigin(routeOrigin),
		emseepee.WithAllowedOrigins("http://[::1]:8080"),
	)

	for _, origins := range [][]string{
		nil,
		{"http://localhost:8080"},
		{"HTTP://LOCALHOST:8080"},
		{"http://[::1]:8080"},
		{routeOrigin},
	} {
		response := serve(
			policyRequest(http.MethodGet, "localhost", origins...),
		)
		assert.Equal(t, http.StatusMethodNotAllowed, response.Code, origins)
	}

	for _, origins := range [][]string{
		{"http://localhost:8081"},
		{"https://localhost:8080"},
		{"http://localhost"},
		{"https://evil.example"},
		{"null"},
		{"http://localhost:8080", "http://localhost:8080"},
	} {
		requireEnvelope(t,
			serve(policyRequest(http.MethodGet, "localhost", origins...)),
			http.StatusForbidden, aichteeteapee.ErrorCodeForbidden)
	}
}

func TestAllowedOriginsCrossOriginProtection(t *testing.T) {
	t.Parallel()

	serve := policyRoute(t, emseepee.WithAllowedOrigins(routeOrigin))

	// A browser request without Origin but marked cross-site is refused.
	request := policyRequest(http.MethodPost, "localhost")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	requireEnvelope(t, serve(request),
		http.StatusForbidden, aichteeteapee.ErrorCodeForbidden)

	// A trusted origin passes cross-origin protection and reaches the SDK.
	request = policyRequest(http.MethodPost, "localhost", routeOrigin)
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	assert.NotEqual(t, http.StatusForbidden, serve(request).Code)

	// Without WithAllowedOrigins, v1.17.0 behavior is kept.
	legacy := policyRoute(t, emseepee.WithPublicOrigin(routeOrigin))
	request = policyRequest(http.MethodGet, "localhost")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	assert.Equal(t, http.StatusMethodNotAllowed, legacy(request).Code)

	requireEnvelope(t,
		legacy(policyRequest(http.MethodGet, "localhost",
			strings.ToUpper(routeOrigin))),
		http.StatusForbidden, aichteeteapee.ErrorCodeForbidden)
}

func TestPolicyConfiguration(t *testing.T) {
	t.Parallel()

	getServer := func(*http.Request) *mcp.Server { return nil }

	for name, opts := range map[string][]emseepee.RouteOption{
		"empty hosts":   {emseepee.WithAllowedHosts()},
		"host port":     {emseepee.WithAllowedHosts("localhost:8080")},
		"host zone":     {emseepee.WithAllowedHosts("[fe80::1%eth0]")},
		"host label":    {emseepee.WithAllowedHosts("bad_host")},
		"origin path":   {emseepee.WithAllowedOrigins(routeOrigin + "/")},
		"origin scheme": {emseepee.WithAllowedOrigins("ftp://example.org")},
		"origin empty":  {emseepee.WithAllowedOrigins("")},
		"deadline conflict": {
			emseepee.WithoutWriteDeadline(),
			emseepee.WithWriteTimeout(time.Second),
		},
	} {
		_, err := emseepee.NewRoute(policyPath, getServer, opts...)
		require.ErrorIs(t, err, aichteeteapee.ErrBadRequest, name)
	}

	// No origins allows only non-browser clients and the public origin.
	serve := policyRoute(t, emseepee.WithAllowedOrigins())
	assert.Equal(t, http.StatusMethodNotAllowed,
		serve(policyRequest(http.MethodGet, "localhost")).Code)
	requireEnvelope(t,
		serve(policyRequest(http.MethodGet, "localhost", "http://localhost")),
		http.StatusForbidden, aichteeteapee.ErrorCodeForbidden)

	// A recorder has no deadline support; WithoutWriteDeadline still serves.
	serve = policyRoute(t, emseepee.WithoutWriteDeadline())
	assert.Equal(t, http.StatusMethodNotAllowed,
		serve(policyRequest(http.MethodGet, "localhost")).Code)
}

func TestAllowedHostsDisableLoopbackProtection(t *testing.T) {
	t.Parallel()

	server := mcp.NewServer(&mcp.Implementation{Name: "policy"}, nil)
	getServer := func(*http.Request) *mcp.Server { return server }

	public, err := emseepee.NewRoute(policyPath, getServer,
		emseepee.WithAllowedHosts("mcp.example.org"))
	require.NoError(t, err)

	loopback, err := emseepee.NewRoute(policyPath, getServer,
		emseepee.WithAllowedHosts("localhost", "127.0.0.1"))
	require.NoError(t, err)

	// The SDK's loopback check refuses a public Host on a loopback listener,
	// unless the allowlist names a public host.
	assert.Equal(t, http.StatusMethodNotAllowed,
		hostStatus(t, startRouter(t, public, passThrough), "mcp.example.org"))

	loopbackEndpoint := startRouter(t, loopback, passThrough)
	assert.Equal(t, http.StatusMethodNotAllowed,
		hostStatus(t, loopbackEndpoint, "localhost"))
	assert.Equal(t, http.StatusMisdirectedRequest,
		hostStatus(t, loopbackEndpoint, "mcp.example.org"))
}

func hostStatus(t *testing.T, endpoint, host string) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), policyWait)
	defer cancel()

	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, endpoint+policyPath, nil,
	)
	require.NoError(t, err)

	request.Host = host

	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)

	defer func() { assert.NoError(t, response.Body.Close()) }()

	return response.StatusCode
}

func toolServer(started chan<- struct{}, stopped chan<- error) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "policy"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: policyBlockTool}, func(
		ctx context.Context, _ *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, any, error) {
		started <- struct{}{}

		<-ctx.Done()

		stopped <- ctx.Err()

		return nil, nil, ctxerrors.Wrap(ctx.Err(), "blocked tool")
	})

	mcp.AddTool(server, &mcp.Tool{Name: policySlowTool}, func(
		ctx context.Context, _ *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, any, error) {
		delay := time.NewTimer(4 * listenerTimeout)
		defer delay.Stop()

		select {
		case <-delay.C:
		case <-ctx.Done():
			return nil, nil, ctxerrors.Wrap(ctx.Err(), "slow tool")
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: policySlowText}},
		}, nil, nil
	})

	return server
}

func postToolCall(
	ctx context.Context, endpoint, tool string,
) (string, error) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
		`"params":{"name":"` + tool + `","arguments":{}}}`

	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint+policyPath, strings.NewReader(body),
	)
	if err != nil {
		return "", ctxerrors.Wrap(err, "build tool call")
	}

	request.Header.Set(aichteeteapee.HeaderNameContentType,
		aichteeteapee.ContentTypeJSON)
	request.Header.Set(aichteeteapee.HeaderNameAccept,
		aichteeteapee.ContentTypeJSON+", "+
			aichteeteapee.ContentTypeTextEventStream)
	request.Header.Set("Mcp-Protocol-Version", policyLegacyVersion)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", ctxerrors.Wrap(err, "send tool call")
	}

	defer func() { _ = response.Body.Close() }()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", ctxerrors.Wrap(err, "read tool call")
	}

	return string(data), nil
}

func TestCallerCancellationForLegacyProtocol(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	stopped := make(chan error, 1)
	server := toolServer(started, stopped)

	route, err := emseepee.NewRoute(policyPath,
		func(*http.Request) *mcp.Server { return server },
		emseepee.WithoutWriteDeadline())
	require.NoError(t, err)

	endpoint := startRouter(t, route, passThrough)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		_, _ = postToolCall(ctx, endpoint, policyBlockTool)
	}()

	select {
	case <-started:
	case <-time.After(policyWait):
		t.Fatal("tool did not start")
	}

	cancel()

	select {
	case err := <-stopped:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(policyWait):
		t.Fatal("tool kept running after the caller left")
	}

	<-done
}

func TestWithoutWriteDeadline(t *testing.T) {
	t.Parallel()

	server := toolServer(make(chan struct{}, 1), make(chan error, 1))
	getServer := func(*http.Request) *mcp.Server { return server }

	open, err := emseepee.NewRoute(policyPath, getServer,
		emseepee.WithoutWriteDeadline())
	require.NoError(t, err)

	limited, err := emseepee.NewRoute(policyPath, getServer)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), policyWait)
	defer cancel()

	body, err := postToolCall(ctx, startRouter(t, open, passThrough),
		policySlowTool)
	require.NoError(t, err)
	assert.Contains(t, body, policySlowText)

	// The listener's write deadline still ends a route without the option.
	body, err = postToolCall(ctx, startRouter(t, limited, passThrough),
		policySlowTool)
	if err == nil {
		assert.NotContains(t, body, policySlowText)
	}
}
