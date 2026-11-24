package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://example.authkit.app"
	testAudience = "https://example.org/v1/apps/images/mcp"
	testMetadata = "https://example.org/.well-known/oauth-protected-resource"
	testSubject  = "user_example"
	testKeyID    = "example-key"
)

func TestJWTBearerAuthWithJWKS(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var fetches atomic.Int64

	server := httptest.NewTLSServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			fetches.Add(1)
			aichteeteapee.WriteJSON(
				w, http.StatusOK, testKeySet(key, testKeyID),
			)
		},
	))
	t.Cleanup(server.Close)
	t.Cleanup(func() { assert.EqualValues(t, 1, fetches.Load()) })

	verifier, err := NewJWKSVerifier(
		testIssuer, server.URL, WithJWKSTransport(server.Client().Transport),
	)
	require.NoError(t, err)

	auth, err := JWTBearerAuth(
		verifier.Verify, testAudience,
		WithJWTBearerResourceMetadata(testMetadata),
	)
	require.NoError(t, err)

	handler := auth(checkPrincipal(t))
	runJWTClaimsCases(t, handler, key, otherKey)

	token := signJWT(t, key, testKeyID, testClaims())
	oversized := strings.Repeat("x", aichteeteapee.DefaultJWTBearerMaxBytes+1)

	for _, headers := range [][]string{
		nil,
		{"Bearer invalid"},
		{"Bearer " + oversized},
		{"Bearer " + token, "Bearer " + token},
	} {
		assertJWTRejected(t, sendBearer(handler, headers...))
	}
}

func checkPrincipal(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := BearerPrincipalFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, BearerPrincipal{
			Issuer:         testIssuer,
			Subject:        testSubject,
			Audience:       testAudience,
			Scope:          "openid offline_access",
			ClientID:       "client_example",
			OrganizationID: "org_example",
			SessionID:      "consent_example",
		}, principal)
		w.WriteHeader(http.StatusNoContent)
	}
}

func runJWTClaimsCases(
	t *testing.T,
	handler http.Handler,
	key, otherKey *rsa.PrivateKey,
) {
	t.Helper()

	valid := testClaims()
	cases := []struct {
		name   string
		edit   func(*connectClaims)
		key    *rsa.PrivateKey
		status int
	}{
		{name: "valid", key: key, status: http.StatusNoContent},
		{
			name: "wrong audience", key: key,
			edit: func(c *connectClaims) {
				c.Audience = jwt.Audience{"other-app"}
			},
		},
		{
			name: "many audiences", key: key,
			edit: func(c *connectClaims) {
				c.Audience = jwt.Audience{testAudience, "other-app"}
			},
		},
		{
			name: "wrong issuer", key: key,
			edit: func(c *connectClaims) { c.Issuer = "https://other.example" },
		},
		{
			name: "future", key: key,
			edit: func(c *connectClaims) {
				c.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Minute))
			},
		},
		{
			name: "expired", key: key,
			edit: func(c *connectClaims) {
				c.Expiry = jwt.NewNumericDate(time.Now().Add(-time.Minute))
			},
		},
		{
			name: "missing expiry", key: key,
			edit: func(c *connectClaims) { c.Expiry = nil },
		},
		{
			name: "missing subject", key: key,
			edit: func(c *connectClaims) { c.Subject = "" },
		},
		{name: "wrong signing key", key: otherKey},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			claims := valid
			if tt.edit != nil {
				tt.edit(&claims)
			}

			token := signJWT(t, tt.key, testKeyID, claims)
			bearer := aichteeteapee.AuthSchemeBearer + token

			response := sendBearer(handler, bearer)
			if tt.status == http.StatusNoContent {
				require.Equal(t, tt.status, response.Code)

				return
			}

			assertJWTRejected(t, response)
		})
	}
}

func TestJWKSRotationAndUnavailableIssuer(t *testing.T) {
	t.Parallel()

	oldKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	const newKeyID = "rotated-key"

	var phase atomic.Int32

	server := httptest.NewTLSServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			switch phase.Load() {
			case 0:
				aichteeteapee.WriteJSON(
					w, http.StatusOK, testKeySet(oldKey, testKeyID),
				)
			case 1:
				aichteeteapee.WriteJSON(
					w, http.StatusOK, testKeySet(newKey, newKeyID),
				)
			default:
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		},
	))
	t.Cleanup(server.Close)

	verifier, err := NewJWKSVerifier(
		testIssuer, server.URL, WithJWKSTransport(server.Client().Transport),
	)
	require.NoError(t, err)

	oldToken := signJWT(t, oldKey, testKeyID, testClaims())
	newToken := signJWT(t, newKey, newKeyID, testClaims())

	_, err = verifier.Verify(t.Context(), oldToken, testAudience)
	require.NoError(t, err)

	phase.Store(1)

	_, err = verifier.Verify(t.Context(), newToken, testAudience)
	require.ErrorIs(t, err, aichteeteapee.ErrUnauthorized)

	// Advance the cooldown without a wall-clock sleep.
	verifier.nextFetch = time.Time{}
	_, err = verifier.Verify(t.Context(), newToken, testAudience)
	require.NoError(t, err)

	_, err = verifier.Verify(t.Context(), oldToken, testAudience)
	require.ErrorIs(t, err, aichteeteapee.ErrUnauthorized)

	phase.Store(2)

	verifier.nextFetch = time.Time{}
	verifier.expires = time.Time{}

	_, err = verifier.Verify(t.Context(), newToken, testAudience)
	require.ErrorIs(t, err, aichteeteapee.ErrAPIError)
}

func TestJWTAuthConfiguration(t *testing.T) {
	t.Parallel()

	_, err := NewJWKSVerifier("http://example.org", testIssuer)
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	_, err = NewJWKSVerifier(
		testIssuer, testIssuer, WithJWKSAlgorithms(jose.HS256),
	)
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	_, err = JWTBearerAuth(nil, testAudience)
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	verifier, err := NewJWKSVerifier(testIssuer, testIssuer)
	require.NoError(t, err)

	_, err = JWTBearerAuth(
		verifier.Verify, testAudience,
		WithJWTBearerResourceMetadata("/metadata"),
	)
	require.ErrorIs(t, err, aichteeteapee.ErrBadRequest)

	_, ok := BearerPrincipalFromContext(context.Background())
	assert.False(t, ok)
}

func TestJWKSRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			payload := strings.Repeat(" ", aichteeteapee.DefaultJWKSMaxBytes+1)
			if _, err := w.Write([]byte(payload)); err != nil {
				t.Error(err)
			}
		},
	))
	t.Cleanup(server.Close)

	verifier, err := NewJWKSVerifier(
		testIssuer, server.URL, WithJWKSTransport(server.Client().Transport),
	)
	require.NoError(t, err)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	token := signJWT(t, key, testKeyID, testClaims())

	_, err = verifier.Verify(t.Context(), token, testAudience)
	require.ErrorIs(t, err, aichteeteapee.ErrAPIError)
	require.ErrorContains(t, err, "invalid JWKS response size")
}

func testClaims() connectClaims {
	return connectClaims{
		Claims: jwt.Claims{
			Issuer:   testIssuer,
			Subject:  testSubject,
			Audience: jwt.Audience{testAudience},
			Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
		Scope:          "openid offline_access",
		ClientID:       "client_example",
		OrganizationID: "org_example",
		SessionID:      "consent_example",
	}
}

func testKeySet(key *rsa.PrivateKey, kid string) jose.JSONWebKeySet {
	return jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &key.PublicKey,
		KeyID:     kid,
		Use:       "sig",
		Algorithm: string(jose.RS256),
	}}}
}

func signJWT(
	t *testing.T,
	key *rsa.PrivateKey,
	kid string,
	claims connectClaims,
) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), kid),
	)
	require.NoError(t, err)

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)

	return token
}

func sendBearer(
	handler http.Handler, headers ...string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, testAudience, nil)
	for _, header := range headers {
		request.Header.Add(aichteeteapee.HeaderNameAuthorization, header)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response
}

func assertJWTRejected(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Equal(t, "Bearer resource_metadata=\""+testMetadata+"\"",
		response.Header().Get(aichteeteapee.HeaderNameWWWAuthenticate))

	var envelope aichteeteapee.ErrorResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	assert.Equal(t, aichteeteapee.ErrorResponseUnauthorized, envelope)
}
