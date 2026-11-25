package emseepee

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const binderGoroutines = 64

func countingBinder(added *atomic.Int32) *cancellationBinder {
	return &cancellationBinder{
		middleware: func(next mcp.MethodHandler) mcp.MethodHandler {
			added.Add(1)

			return next
		},
	}
}

func TestCancellationBinderAddsOncePerServer(t *testing.T) {
	t.Parallel()

	var added atomic.Int32

	binder := countingBinder(&added)
	first := mcp.NewServer(&mcp.Implementation{Name: "first"}, nil)
	second := mcp.NewServer(&mcp.Implementation{Name: "second"}, nil)

	var group sync.WaitGroup

	start := make(chan struct{})

	for index := range binderGoroutines {
		server := first
		if index%2 == 1 {
			server = second
		}

		group.Go(func() {
			<-start
			binder.bind(server)
		})
	}

	close(start)
	group.Wait()

	binder.bind(nil)
	binder.bind(first)

	assert.Equal(t, int32(2), added.Load())
	runtime.KeepAlive(first)
	runtime.KeepAlive(second)
}

func TestCancellationBinderForgetsCollectedServers(t *testing.T) {
	t.Parallel()

	var added atomic.Int32

	binder := countingBinder(&added)

	func() {
		binder.bind(mcp.NewServer(&mcp.Implementation{Name: "short"}, nil))
	}()

	assert.Eventually(t, func() bool {
		runtime.GC()

		empty := true

		binder.servers.Range(func(_, _ any) bool {
			empty = false

			return false
		})

		return empty
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, int32(1), added.Load())
}

func TestCallerCancellation(t *testing.T) {
	t.Parallel()

	observed := make(chan context.Context, 1)
	handler := callerCancellation(func(
		ctx context.Context, _ string, _ mcp.Request,
	) (mcp.Result, error) {
		observed <- ctx

		return nil, ctx.Err()
	})

	// No caller context: the method context passes through unchanged.
	plain := context.WithoutCancel(t.Context())
	_, err := handler(plain, "tools/call", nil)
	require.NoError(t, err)
	assert.Equal(t, plain, <-observed)

	// An ended caller cancels the method before it runs.
	ended, cancelEnded := context.WithCancel(t.Context())
	cancelEnded()

	_, err = handler(
		context.WithValue(context.Background(), callerContextKey{}, ended),
		"tools/call", nil,
	)
	require.ErrorIs(t, err, context.Canceled)
	<-observed

	// A live caller that ends later cancels the running method.
	caller, cancelCaller := context.WithCancel(t.Context())
	blocked := callerCancellation(func(
		ctx context.Context, _ string, _ mcp.Request,
	) (mcp.Result, error) {
		cancelCaller()
		<-ctx.Done()

		return nil, ctx.Err()
	})

	_, err = blocked(
		context.WithValue(context.Background(), callerContextKey{}, caller),
		"tools/call", nil,
	)
	require.ErrorIs(t, err, context.Canceled)
}

func TestPolicyNormalization(t *testing.T) {
	t.Parallel()

	hosts := map[string]struct{}{"localhost": {}, "::1": {}, "10.0.0.1": {}}

	for authority, allowed := range map[string]bool{
		"localhost":       true,
		"LOCALHOST:8080":  true,
		"[::1]:9":         true,
		"[::1]":           true,
		"10.0.0.1:65535":  true,
		"localhost:0":     false,
		"localhost:65536": false,
		"localhost:x":     false,
		"localhost:":      false,
		"example.com":     false,
		"":                false,
		"[fe80::1%eth0]":  false,
		"bad_host":        false,
	} {
		assert.Equal(t, allowed, hostAllowed(hosts, authority), authority)
	}

	for origin, expected := range map[string]string{
		"http://localhost:8080":    "http://localhost:8080",
		"HTTP://LocalHost:8080":    "http://localhost:8080",
		"https://[::1]":            "https://[::1]",
		"https://[::1]:443":        "https://[::1]:443",
		"https://example.org":      "https://example.org",
		"https://example.org/":     "",
		"https://example.org?a=b":  "",
		"https://user@example.org": "",
		"ftp://example.org":        "",
		"https://example.org:0":    "",
		"https://-bad.example":     "",
		"https://[0:0::1]":         "",
		"null":                     "",
	} {
		normalized, valid := normalizeOrigin(origin)
		assert.Equal(t, expected != "", valid, origin)
		assert.Equal(t, expected, normalized, origin)
	}

	assert.True(t, loopbackHost("localhost"))
	assert.True(t, loopbackHost("127.0.0.2"))
	assert.False(t, loopbackHost("example.com"))
	assert.False(t, loopbackHost("10.0.0.1"))
}
