package klhayyent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestOptions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		assert.Equal(t, "Bearer secret-token",
			request.Header.Get(aichteeteapee.HeaderNameAuthorization))
		assert.Equal(
			t,
			[]string{"one", "two"},
			request.Header.Values("X-Multi"),
		)
		assert.Equal(t, []string{"first", "second"}, request.URL.Query()["key"])
		assert.Equal(t, "request-id",
			request.Header.Get(aichteeteapee.HeaderNameXRequestID))
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	ctx := context.WithValue(
		context.Background(),
		aichteeteapee.ContextKeyRequestID,
		"request-id",
	)
	_, err = client.Get(
		ctx,
		"/request?key=first",
		nil,
		WithBearerToken("secret-token"),
		WithHeaders(http.Header{"X-Multi": {"one", "two"}}),
		WithQueryValues(url.Values{"key": {"second"}}),
	)
	require.NoError(t, err)
}

func TestInvalidRequestTargets(t *testing.T) {
	t.Parallel()

	client, err := New("https://example.com/api")
	require.NoError(t, err)

	testCases := []string{
		"https://attacker.example/path",
		"//attacker.example/path",
		"../secret",
		"%2e%2e/secret",
		"%252e%252e/secret",
		"path#fragment",
		"path\\segment",
		"path\x00other",
		"%",
	}

	for _, target := range testCases {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			response, requestErr := client.Get(
				context.Background(),
				target,
				nil,
			)
			assert.Nil(t, response)
			require.Error(t, requestErr)
			assert.ErrorIs(t, requestErr, ErrInvalidRequestTarget)
		})
	}
}

func TestInvalidBaseURLs(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"",
		"relative/path",
		"ftp://example.com",
		"https://user:password@example.com",
		"https://example.com?secret=value",
		"https://example.com#fragment",
		"https://",
		"%",
	}

	for _, baseURL := range testCases {
		t.Run(baseURL, func(t *testing.T) {
			t.Parallel()

			client, err := New(baseURL)
			assert.Nil(t, client)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidBaseURL)
		})
	}
}

func TestBasicAuthRequestOption(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		username, password, ok := request.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "user", username)
		assert.Equal(t, "password", password)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL)
	require.NoError(t, err)

	_, err = client.Get(
		context.Background(),
		"/",
		nil,
		WithBasicAuth("user", "password"),
	)
	require.NoError(t, err)
}
