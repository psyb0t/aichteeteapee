package klhayyent

import (
	"math"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientOptions(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	suppliedClient := &http.Client{
		Timeout: time.Second,
	}
	client, err := New(
		"https://example.com/api",
		WithHTTPClient(suppliedClient),
		WithCookieJar(jar),
		WithTimeout(2*time.Second),
		WithMaxResponseBodySize(128),
		WithDependencyName("example_api"),
	)
	require.NoError(t, err)

	assert.Same(t, jar, client.CookieJar())
	assert.Nil(t, suppliedClient.Jar)
	assert.Equal(t, time.Second, suppliedClient.Timeout)
	assert.Equal(t, 2*time.Second, client.httpClient.Timeout)
	assert.Equal(t, int64(128), client.maxResponseBodySize)
	assert.Equal(t, "example_api", client.dependencyName)
}

func TestHTTPClientCookieJarIsPreserved(t *testing.T) {
	t.Parallel()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	suppliedClient := &http.Client{Jar: jar}
	client, err := New(
		"https://example.com",
		WithHTTPClient(suppliedClient),
	)
	require.NoError(t, err)

	assert.Same(t, jar, client.CookieJar())
	assert.Same(t, jar, suppliedClient.Jar)
}

func TestWithDefaultHeadersClonesInput(t *testing.T) {
	t.Parallel()

	headers := http.Header{
		"X-First": {"one", "two"},
		"X-Last":  {"three"},
	}
	client, err := New(
		"https://example.com",
		WithDefaultHeaders(headers),
	)
	require.NoError(t, err)

	headers.Set("X-First", "changed")
	assert.Equal(
		t,
		[]string{"one", "two"},
		client.defaultHeaders.Values("X-First"),
	)
	assert.Equal(t, "three", client.defaultHeaders.Get("X-Last"))
}

func TestInvalidClientOptions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		option Option
	}{
		{name: "nil HTTP client", option: WithHTTPClient(nil)},
		{name: "nil transport", option: WithTransport(nil)},
		{name: "negative timeout", option: WithTimeout(-time.Second)},
		{name: "nil cookie jar", option: WithCookieJar(nil)},
		{name: "empty header name", option: WithDefaultHeader("", "value")},
		{
			name:   "invalid header name",
			option: WithDefaultHeader("Bad Name", "value"),
		},
		{
			name:   "header injection",
			option: WithDefaultHeader("X-Test", "a\r\nb"),
		},
		{
			name: "invalid headers collection",
			option: WithDefaultHeaders(http.Header{
				"Bad Name": {"value"},
			}),
		},
		{name: "zero body limit", option: WithMaxResponseBodySize(0)},
		{
			name:   "overflowing body limit",
			option: WithMaxResponseBodySize(math.MaxInt64),
		},
		{name: "empty dependency", option: WithDependencyName("")},
		{name: "nil redirect policy", option: WithCheckRedirect(nil)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := New("https://example.com", tc.option)
			assert.Nil(t, client)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidOption)
		})
	}
}

func TestHTTPErrorNilSafety(t *testing.T) {
	t.Parallel()

	var nilError *HTTPError
	assert.Equal(t, "HTTP request failed", nilError.Error())
	assert.NoError(t, nilError.Unwrap())

	emptyError := &HTTPError{}
	assert.Equal(t, "HTTP request failed", emptyError.Error())
	assert.NoError(t, emptyError.Unwrap())

	cause := assert.AnError
	httpError := &HTTPError{cause: cause}
	assert.ErrorIs(t, httpError, cause)
}
