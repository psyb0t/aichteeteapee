package proxy

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/common-go/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwardRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       []byte
		wantStatus int
		wantBody   string
	}{
		{
			name:       "GET request",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "POST with body",
			method:     http.MethodPost,
			body:       []byte("payload"),
			wantStatus: http.StatusOK,
			wantBody:   "payload",
		},
		{
			name:       "HEAD request",
			method:     http.MethodHead,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(
				http.HandlerFunc(
					func(
						w http.ResponseWriter,
						r *http.Request,
					) {
						b, _ := io.ReadAll(r.Body)
						if len(b) > 0 {
							_, _ = w.Write(b)

							return
						}

						_, _ = io.WriteString(w, "ok")
					},
				),
			)
			defer upstream.Close()

			cfg := ForwardConfig{
				HTTPClient: upstream.Client(),
			}

			payload := &RequestPayload{
				Method: tt.method,
				URL:    upstream.URL + "/test",
				Body:   tt.body,
			}

			result, err := ForwardRequest(
				context.Background(), cfg, payload,
			)
			require.NoError(t, err)
			assert.Equal(
				t, tt.wantStatus,
				result.StatusCode,
			)

			if tt.wantBody != "" {
				assert.Equal(
					t, tt.wantBody,
					string(result.Body),
				)
			}
		})
	}
}

func TestForwardRequest_HopByHopStripped(
	t *testing.T,
) {
	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				for h := range hopByHopHeaders {
					if r.Header.Get(h) != "" {
						w.WriteHeader(
							http.StatusBadRequest,
						)

						return
					}
				}

				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer upstream.Close()

	payload := &RequestPayload{
		Method: http.MethodGet,
		URL:    upstream.URL,
		Headers: map[string][]string{
			aichteeteapee.HeaderNameConnection: {
				"keep-alive",
			},
			aichteeteapee.HeaderNameUpgrade: {
				"websocket",
			},
			"X-Custom": {"keep"},
		},
	}

	result, err := ForwardRequest(
		context.Background(),
		ForwardConfig{HTTPClient: upstream.Client()},
		payload,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
}

func TestForwardRequest_ProxyHeaders(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				xff := r.Header.Get(
					aichteeteapee.HeaderNameXForwardedFor,
				)
				xri := r.Header.Get(
					aichteeteapee.HeaderNameXRealIP,
				)
				xfp := r.Header.Get(
					aichteeteapee.HeaderNameXForwardedProto,
				)

				if xff != "1.2.3.4" ||
					xri != "1.2.3.4" ||
					xfp != "https" {
					w.WriteHeader(
						http.StatusBadRequest,
					)

					return
				}

				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer upstream.Close()

	payload := &RequestPayload{
		Method:   http.MethodGet,
		URL:      upstream.URL,
		ClientIP: "1.2.3.4",
		Proto:    aichteeteapee.SchemeHTTPS,
	}

	result, err := ForwardRequest(
		context.Background(),
		ForwardConfig{HTTPClient: upstream.Client()},
		payload,
	)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
}

func TestForwardRequest_UpstreamError(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				w.WriteHeader(
					http.StatusInternalServerError,
				)
				_, _ = io.WriteString(w, "boom")
			},
		),
	)
	defer upstream.Close()

	result, err := ForwardRequest(
		context.Background(),
		ForwardConfig{HTTPClient: upstream.Client()},
		&RequestPayload{
			Method: http.MethodGet,
			URL:    upstream.URL,
		},
	)
	require.NoError(t, err)
	assert.Equal(
		t, http.StatusInternalServerError,
		result.StatusCode,
	)
	assert.Equal(t, "boom", string(result.Body))
}

func TestForwardRequest_ConnectionRefused(
	t *testing.T,
) {
	dead := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	deadURL := dead.URL
	dead.Close()

	_, err := ForwardRequest(
		context.Background(),
		ForwardConfig{HTTPClient: http.DefaultClient},
		&RequestPayload{
			Method: http.MethodGet,
			URL:    deadURL,
		},
	)
	require.Error(t, err)
}

func TestForwardRequest_WithCache(t *testing.T) {
	callCount := 0

	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				callCount++
				_, _ = io.WriteString(w, "cached")
			},
		),
	)
	defer upstream.Close()

	c := cache.NewMemory(100)

	defer func() { _ = c.Close() }()

	cfg := ForwardConfig{
		HTTPClient: upstream.Client(),
		Cache:      c,
		CacheTTL:   time.Minute,
	}

	payload := &RequestPayload{
		Method: http.MethodGet,
		URL:    upstream.URL + "/data",
	}

	r1, err := ForwardRequest(
		context.Background(), cfg, payload,
	)
	require.NoError(t, err)
	assert.Equal(t, "cached", string(r1.Body))
	assert.Equal(t, 1, callCount)

	// Second call — should come from cache.
	r2, err := ForwardRequest(
		context.Background(), cfg, payload,
	)
	require.NoError(t, err)
	assert.Equal(t, "cached", string(r2.Body))
	assert.Equal(t, 1, callCount)
}

func TestForwardRequest_CacheDifferentBodies(
	t *testing.T,
) {
	callCount := 0

	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				callCount++

				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer upstream.Close()

	c := cache.NewMemory(100)

	defer func() { _ = c.Close() }()

	cfg := ForwardConfig{
		HTTPClient: upstream.Client(),
		Cache:      c,
		CacheTTL:   time.Minute,
	}

	// Same URL, different bodies = different cache keys.
	_, _ = ForwardRequest(
		context.Background(), cfg,
		&RequestPayload{
			Method: http.MethodPost,
			URL:    upstream.URL,
			Body:   []byte(`{"prompt":"a"}`),
		},
	)
	_, _ = ForwardRequest(
		context.Background(), cfg,
		&RequestPayload{
			Method: http.MethodPost,
			URL:    upstream.URL,
			Body:   []byte(`{"prompt":"b"}`),
		},
	)

	assert.Equal(t, 2, callCount)

	// Same body again = cache hit.
	_, _ = ForwardRequest(
		context.Background(), cfg,
		&RequestPayload{
			Method: http.MethodPost,
			URL:    upstream.URL,
			Body:   []byte(`{"prompt":"a"}`),
		},
	)

	assert.Equal(t, 2, callCount)
}

func TestForwardRequest_CacheSkips5xx(t *testing.T) {
	callCount := 0

	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				callCount++

				w.WriteHeader(
					http.StatusInternalServerError,
				)
			},
		),
	)
	defer upstream.Close()

	c := cache.NewMemory(100)

	defer func() { _ = c.Close() }()

	cfg := ForwardConfig{
		HTTPClient: upstream.Client(),
		Cache:      c,
		CacheTTL:   time.Minute,
	}

	payload := &RequestPayload{
		Method: http.MethodGet,
		URL:    upstream.URL,
	}

	_, _ = ForwardRequest(
		context.Background(), cfg, payload,
	)
	_, _ = ForwardRequest(
		context.Background(), cfg, payload,
	)

	// 5xx should not be cached.
	assert.Equal(t, 2, callCount)
}

func TestForwardRequest_CustomCacheKey(
	t *testing.T,
) {
	upstream := httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				_ *http.Request,
			) {
				_, _ = io.WriteString(w, "ok")
			},
		),
	)
	defer upstream.Close()

	c := cache.NewMemory(100)

	defer func() { _ = c.Close() }()

	cfg := ForwardConfig{
		HTTPClient: upstream.Client(),
		Cache:      c,
		CacheTTL:   time.Minute,
		CacheKeyFn: func(
			_ *RequestPayload,
		) string {
			return "fixed-key"
		},
	}

	_, err := ForwardRequest(
		context.Background(), cfg,
		&RequestPayload{
			Method: http.MethodGet,
			URL:    upstream.URL + "/a",
		},
	)
	require.NoError(t, err)

	// Different URL but same custom key — cache hit.
	r, err := ForwardRequest(
		context.Background(), cfg,
		&RequestPayload{
			Method: http.MethodGet,
			URL:    upstream.URL + "/b",
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(r.Body))
}

func TestRequestScheme(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() *http.Request
		expect string
	}{
		{
			name: "plain HTTP",
			setup: func() *http.Request {
				return httptest.NewRequest(
					http.MethodGet, "/", nil,
				)
			},
			expect: aichteeteapee.SchemeHTTP,
		},
		{
			name: "TLS set",
			setup: func() *http.Request {
				r := httptest.NewRequest(
					http.MethodGet, "/", nil,
				)
				r.TLS = &tls.ConnectionState{}

				return r
			},
			expect: aichteeteapee.SchemeHTTPS,
		},
		{
			name: "X-Forwarded-Proto header",
			setup: func() *http.Request {
				r := httptest.NewRequest(
					http.MethodGet, "/", nil,
				)
				r.Header.Set(
					aichteeteapee.HeaderNameXForwardedProto,
					aichteeteapee.SchemeHTTPS,
				)

				return r
			},
			expect: aichteeteapee.SchemeHTTPS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(
				t, tt.expect,
				RequestScheme(tt.setup()),
			)
		})
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "NOT_FOUND")
}

func TestRequestPayloadHash(t *testing.T) {
	base := &RequestPayload{
		Method: http.MethodGet,
		URL:    "http://example.com/a",
	}

	tests := []struct {
		name    string
		payload *RequestPayload
		differ  bool
	}{
		{
			name:    "same request = same hash",
			payload: base,
		},
		{
			name: "different URL",
			payload: &RequestPayload{
				Method: http.MethodGet,
				URL:    "http://example.com/b",
			},
			differ: true,
		},
		{
			name: "different method",
			payload: &RequestPayload{
				Method: http.MethodPost,
				URL:    "http://example.com/a",
			},
			differ: true,
		},
		{
			name: "with body",
			payload: &RequestPayload{
				Method: http.MethodPost,
				URL:    "http://example.com/a",
				Body:   []byte("data"),
			},
			differ: true,
		},
		{
			name: "different body",
			payload: &RequestPayload{
				Method: http.MethodPost,
				URL:    "http://example.com/a",
				Body:   []byte("other"),
			},
			differ: true,
		},
		{
			name: "with headers",
			payload: &RequestPayload{
				Method: http.MethodGet,
				URL:    "http://example.com/a",
				Headers: map[string][]string{
					"Authorization": {"Bearer x"},
				},
			},
			differ: true,
		},
		{
			name: "different header value",
			payload: &RequestPayload{
				Method: http.MethodGet,
				URL:    "http://example.com/a",
				Headers: map[string][]string{
					"Authorization": {"Bearer y"},
				},
			},
			differ: true,
		},
	}

	baseHash := base.Hash()
	assert.Len(t, baseHash, 64)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.payload.Hash()

			if tt.differ {
				assert.NotEqual(t, baseHash, h)

				return
			}

			assert.Equal(t, baseHash, h)
		})
	}
}
