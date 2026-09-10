package klhayyent

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPayload struct {
	Value string `json:"value"`
}

type testClientCall func(
	context.Context,
	*testPayload,
) (*Response, error)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	return fn(request)
}

func TestClientConvenienceMethods(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		assert.Equal(t, "/api/resource", request.URL.Path)
		assert.Equal(t, "request-value", request.Header.Get("X-Test"))

		if methodHasRequestBody(request.Method) {
			body, err := io.ReadAll(request.Body)
			assert.NoError(t, err)
			assert.JSONEq(t, `{"value":"input"}`, string(body))
		}

		writer.Header().Set(
			aichteeteapee.HeaderNameContentType,
			aichteeteapee.ContentTypeJSON,
		)
		writer.WriteHeader(http.StatusCreated)
		_, err := writer.Write([]byte(`{"value":"output"}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := New(
		server.URL+"/api",
		WithDefaultHeader("X-Test", "default-value"),
	)
	require.NoError(t, err)

	testCases := []struct {
		name string
		call testClientCall
	}{
		{
			name: "get",
			call: func(
				ctx context.Context,
				output *testPayload,
			) (*Response, error) {
				return client.Get(
					ctx,
					"/resource",
					output,
					WithHeader("X-Test", "request-value"),
				)
			},
		},
		{
			name: "delete",
			call: func(
				ctx context.Context,
				output *testPayload,
			) (*Response, error) {
				return client.Delete(
					ctx,
					"/resource",
					output,
					WithHeader("X-Test", "request-value"),
				)
			},
		},
		{
			name: "post",
			call: func(
				ctx context.Context,
				output *testPayload,
			) (*Response, error) {
				return client.Post(
					ctx,
					"/resource",
					testPayload{Value: "input"},
					output,
					WithHeader("X-Test", "request-value"),
				)
			},
		},
		{
			name: "put",
			call: func(
				ctx context.Context,
				output *testPayload,
			) (*Response, error) {
				return client.Put(
					ctx,
					"/resource",
					testPayload{Value: "input"},
					output,
					WithHeader("X-Test", "request-value"),
				)
			},
		},
		{
			name: "patch",
			call: func(
				ctx context.Context,
				output *testPayload,
			) (*Response, error) {
				return client.Patch(
					ctx,
					"/resource",
					testPayload{Value: "input"},
					output,
					WithHeader("X-Test", "request-value"),
				)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var output testPayload

			response, err := tc.call(context.Background(), &output)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, response.StatusCode)
			assert.Equal(t, testPayload{Value: "output"}, output)
		})
	}
}

func TestClientRequestBodiesAndQuery(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		body        any
		contentType string
		wantBody    string
	}{
		{
			name:        "bytes",
			body:        []byte("raw"),
			contentType: aichteeteapee.ContentTypeOctetStream,
			wantBody:    "raw",
		},
		{
			name:        "reader",
			body:        bytes.NewBufferString("reader"),
			contentType: "application/custom",
			wantBody:    "reader",
		},
		{
			name:        "string",
			body:        "text",
			contentType: aichteeteapee.ContentTypeTextPlain,
			wantBody:    "text",
		},
		{
			name:        "form",
			body:        url.Values{"key": {"one", "two"}},
			contentType: aichteeteapee.ContentTypeApplicationFormURLEncoded,
			wantBody:    "key=one&key=two",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				body, err := io.ReadAll(request.Body)
				assert.NoError(t, err)
				assert.Equal(t, tc.wantBody, string(body))
				assert.Equal(
					t,
					tc.contentType,
					request.Header.Get(aichteeteapee.HeaderNameContentType),
				)
				assert.Equal(
					t,
					[]string{"one", "two"},
					request.URL.Query()["q"],
				)
				writer.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			client, err := New(server.URL)
			require.NoError(t, err)

			options := []RequestOption{
				WithQueryValues(url.Values{"q": {"one", "two"}}),
			}
			if tc.name == "reader" {
				options = append(options, WithHeader(
					aichteeteapee.HeaderNameContentType,
					tc.contentType,
				))
			}

			response, err := client.Post(
				context.Background(),
				"/body",
				tc.body,
				nil,
				options...,
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, response.StatusCode)
		})
	}
}

func TestClientCookieJar(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		switch request.URL.Path {
		case "/set":
			http.SetCookie(writer, &http.Cookie{
				Name:     "session",
				Value:    "secret-value",
				Path:     "/allowed",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
		case "/allowed/check":
			cookie, err := request.Cookie("session")
			assert.NoError(t, err)
			assert.Equal(t, "secret-value", cookie.Value)
		case "/blocked":
			_, err := request.Cookie("session")
			assert.ErrorIs(t, err, http.ErrNoCookie)
		}

		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)
	require.NotNil(t, client.CookieJar())

	_, err = client.Get(context.Background(), "/set", nil)
	require.NoError(t, err)
	_, err = client.Get(context.Background(), "/allowed/check", nil)
	require.NoError(t, err)
	_, err = client.Get(context.Background(), "/blocked", nil)
	require.NoError(t, err)
}

func TestClientCookieJarRejectsSecureAndExpiredCookiesOverHTTP(t *testing.T) {
	t.Parallel()

	var receivedCookie atomic.Bool

	transport := roundTripperFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		receivedCookie.Store(len(request.Cookies()) > 0)

		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       http.NoBody,
			Request:    request,
		}, nil
	})

	client, err := New(
		"http://example.test",
		WithTransport(transport),
	)
	require.NoError(t, err)

	serverURL, err := url.Parse("http://example.test")
	require.NoError(t, err)

	client.CookieJar().SetCookies(serverURL, []*http.Cookie{
		{
			Name:   "secure-session",
			Value:  "secret-value",
			Path:   "/",
			Secure: true,
		},
		{
			Name:    "expired-session",
			Value:   "secret-value",
			Path:    "/",
			Expires: time.Unix(1, 0),
		},
	})

	_, err = client.Get(context.Background(), "/", nil)
	require.NoError(t, err)
	assert.False(t, receivedCookie.Load())
}

func TestClientRejectsCrossOriginRedirect(t *testing.T) {
	t.Parallel()

	var redirectedRequests atomic.Int32

	targetServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		redirectedRequests.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(targetServer.Close)

	sourceServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		http.Redirect(
			writer,
			request,
			targetServer.URL,
			http.StatusTemporaryRedirect,
		)
	}))
	t.Cleanup(sourceServer.Close)

	client, err := New(sourceServer.URL)
	require.NoError(t, err)

	response, err := client.Get(context.Background(), "/redirect", nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrInvalidRequestTarget)
	assert.Zero(t, redirectedRequests.Load())
}

func TestClientRejectsRedirectCallbackURLMutation(t *testing.T) {
	t.Parallel()

	var targetRequests atomic.Int32

	targetServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		targetRequests.Add(1)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(targetServer.Close)

	sourceServer := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		http.Redirect(writer, request, "/safe", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(sourceServer.Close)

	targetURL, err := url.Parse(targetServer.URL)
	require.NoError(t, err)

	client, err := New(
		sourceServer.URL,
		WithCheckRedirect(func(
			request *http.Request,
			_ []*http.Request,
		) error {
			request.URL = targetURL

			return nil
		}),
	)
	require.NoError(t, err)

	response, err := client.Get(context.Background(), "/redirect", nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrInvalidRequestTarget)
	assert.Zero(t, targetRequests.Load())
}

func TestClientHTTPError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		body         string
		wantCode     string
		wantMessage  string
		wantSentinel error
	}{
		{
			name: "envelope",
			body: `{
				"code":"MISSING_THING",
				"message":"gone",
				"details":{"id":"x"}
			}`,
			wantCode:     "MISSING_THING",
			wantMessage:  "gone",
			wantSentinel: aichteeteapee.ErrNotFound,
		},
		{
			name:         "fallback",
			body:         "not json",
			wantCode:     aichteeteapee.ErrorCodeNotFound,
			wantMessage:  http.StatusText(http.StatusNotFound),
			wantSentinel: aichteeteapee.ErrNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				_ *http.Request,
			) {
				writer.WriteHeader(http.StatusNotFound)
				_, err := writer.Write([]byte(tc.body))
				assert.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			client, err := New(server.URL)
			require.NoError(t, err)

			response, err := client.Get(context.Background(), "/missing", nil)
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, response.StatusCode)
			assert.ErrorIs(t, err, tc.wantSentinel)

			var httpError *HTTPError
			require.ErrorAs(t, err, &httpError)
			assert.Equal(t, tc.wantCode, httpError.ErrorResponse.Code)
			assert.Equal(t, tc.wantMessage, httpError.ErrorResponse.Message)
		})
	}
}

func TestClientResponseBodyLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, err := writer.Write([]byte("12345"))
		assert.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL, WithMaxResponseBodySize(4))
	require.NoError(t, err)

	response, err := client.Get(context.Background(), "/large", nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrResponseBodyTooLarge)
}

func TestClientTransportErrorDoesNotExposeURL(t *testing.T) {
	t.Parallel()

	const secretQuery = "private-token-value"

	transport := roundTripperFunc(func(
		request *http.Request,
	) (*http.Response, error) {
		return nil, &url.Error{
			Op:  request.Method,
			URL: request.URL.String(),
			Err: context.DeadlineExceeded,
		}
	})
	client, err := New(
		"https://example.com",
		WithTransport(transport),
	)
	require.NoError(t, err)

	response, err := client.Get(
		context.Background(),
		"/resource?token="+secretQuery,
		nil,
	)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotContains(t, err.Error(), secretQuery)
}

func TestClientUnknownHTTPStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	response, err := client.Get(context.Background(), "/", nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusTeapot, response.StatusCode)
	assert.ErrorIs(t, err, aichteeteapee.ErrUnexpectedResponseStatus)
	assert.NotErrorIs(t, err, aichteeteapee.ErrInternalServer)
}

func TestStatusErrorMappings(t *testing.T) {
	t.Parallel()

	unexpectedStatusError := aichteeteapee.ErrUnexpectedResponseStatus
	testCases := map[int]error{
		http.StatusBadRequest:          aichteeteapee.ErrBadRequest,
		http.StatusUnauthorized:        aichteeteapee.ErrUnauthorized,
		http.StatusForbidden:           aichteeteapee.ErrForbidden,
		http.StatusNotFound:            aichteeteapee.ErrNotFound,
		http.StatusMethodNotAllowed:    aichteeteapee.ErrMethodNotAllowed,
		http.StatusConflict:            aichteeteapee.ErrConflict,
		http.StatusGone:                aichteeteapee.ErrGone,
		http.StatusUnprocessableEntity: aichteeteapee.ErrUnprocessableEntity,
		http.StatusTooManyRequests:     aichteeteapee.ErrTooManyRequests,
		http.StatusInternalServerError: aichteeteapee.ErrInternalServer,
		http.StatusBadGateway:          aichteeteapee.ErrBadGateway,
		http.StatusServiceUnavailable:  aichteeteapee.ErrServiceUnavailable,
		http.StatusGatewayTimeout:      aichteeteapee.ErrGatewayTimeout,
		http.StatusTeapot:              unexpectedStatusError,
	}

	for status, expected := range testCases {
		assert.ErrorIs(t, statusError(status), expected)
	}
}

func TestClientConcurrentUse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		_, err := writer.Write([]byte(request.URL.Query().Get("value")))
		assert.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	const requestCount = 20

	var waitGroup sync.WaitGroup
	waitGroup.Add(requestCount)

	for index := range requestCount {
		go func() {
			defer waitGroup.Done()

			var body []byte

			_, requestErr := client.Get(
				context.Background(),
				"/concurrent",
				&body,
				WithQuery("value", string(rune('a'+index))),
			)
			assert.NoError(t, requestErr)
			assert.NotEmpty(t, body)
		}()
	}

	waitGroup.Wait()
}

func TestClientHeadAndOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set("X-Method", request.Method)

		if request.Method == http.MethodOptions {
			_, err := writer.Write([]byte("options"))
			assert.NoError(t, err)
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	headResponse, err := client.Head(context.Background(), "/resource")
	require.NoError(t, err)
	assert.Equal(t, http.MethodHead, headResponse.Header.Get("X-Method"))

	var optionsBody []byte

	optionsResponse, err := client.Options(
		context.Background(),
		"/resource",
		&optionsBody,
	)
	require.NoError(t, err)
	assert.Equal(t, http.MethodOptions, optionsResponse.Header.Get("X-Method"))
	assert.Equal(t, []byte("options"), optionsBody)
}

func TestClientResponseDecodingFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, err := writer.Write([]byte("not-json"))
		assert.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	var decoded testPayload

	response, err := client.Get(context.Background(), "/invalid", &decoded)
	require.Error(t, err)
	assert.NotNil(t, response)
}

func TestClientInvalidRequestInputs(t *testing.T) {
	t.Parallel()

	client, err := New("https://example.com")
	require.NoError(t, err)

	//nolint:staticcheck // A nil context is the boundary under test.
	response, err := client.Do(nil, http.MethodGet, "/", nil, nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrInvalidOption)

	response, err = client.Do(context.Background(), "", "/", nil, nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrInvalidOption)

	response, err = client.Post(
		context.Background(),
		"/",
		make(chan int),
		nil,
	)
	require.Error(t, err)
	assert.Nil(t, response)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	response, err = client.Get(ctx, "/", nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestNilClient(t *testing.T) {
	t.Parallel()

	var client *Client

	assert.Nil(t, client.CookieJar())

	response, err := client.Get(context.Background(), "/", nil)
	require.Error(t, err)
	assert.Nil(t, response)
	assert.ErrorIs(t, err, ErrInvalidOption)
}

func methodHasRequestBody(method string) bool {
	return method == http.MethodPost || method == http.MethodPut ||
		method == http.MethodPatch
}
