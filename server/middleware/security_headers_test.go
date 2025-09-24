package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	middleware := SecurityHeaders()

	handler := createTestHandler()

	req := createTestRequest(http.MethodGet, "/test")
	w := httptest.NewRecorder()

	middleware(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check security headers

	expectedHeaders := map[string]string{
		aichteeteapee.HeaderNameXContentTypeOptions:     aichteeteapee.DefaultSecurityXContentTypeOptionsNoSniff, //nolint:lll
		aichteeteapee.HeaderNameXFrameOptions:           aichteeteapee.DefaultSecurityXFrameOptionsDeny,          //nolint:lll
		aichteeteapee.HeaderNameXXSSProtection:          aichteeteapee.DefaultSecurityXXSSProtectionBlock,        //nolint:lll
		aichteeteapee.HeaderNameStrictTransportSecurity: aichteeteapee.DefaultSecurityStrictTransportSecurity,    //nolint:lll
		aichteeteapee.HeaderNameReferrerPolicy:          aichteeteapee.DefaultSecurityReferrerPolicyStrictOrigin, //nolint:lll
	}

	assertSecurityHeaders(t, w, expectedHeaders)
}

func TestSecurityHeadersMiddleware_CustomOptions(t *testing.T) {
	t.Run("with custom values", func(t *testing.T) {
		middleware := SecurityHeaders(
			WithXFrameOptions("SAMEORIGIN"),
			WithContentSecurityPolicy("default-src 'self'"),
			DisableHSTS(),
		)

		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		assert.Equal(
			t, "SAMEORIGIN", w.Header().Get(aichteeteapee.HeaderNameXFrameOptions),
		)
		assert.Equal(

			t, "default-src 'self'",
			w.Header().Get(aichteeteapee.HeaderNameContentSecurityPolicy),
		)

		assert.Equal(
			t, "", w.Header().Get(aichteeteapee.HeaderNameStrictTransportSecurity),
		) // Disabled
		// Still enabled

		assert.NotEmpty(
			t, w.Header().Get(aichteeteapee.HeaderNameXContentTypeOptions),
		)
	})

	t.Run("all options disabled", func(t *testing.T) {
		middleware := SecurityHeaders(
			WithXContentTypeOptions("nosniff"),
			WithXXSSProtection("1; mode=block"),
			WithStrictTransportSecurity("max-age=31536000; includeSubDomains"),
			WithReferrerPolicy("strict-origin-when-cross-origin"),
			DisableXContentTypeOptions(),
			DisableXFrameOptions(),
			DisableXXSSProtection(),
			DisableReferrerPolicy(),
			DisableCSP(),
		)

		handler := createTestHandler()

		req := createTestRequest(http.MethodGet, "/test")
		w := httptest.NewRecorder()

		middleware(handler).ServeHTTP(w, req)

		// All disabled headers should be empty

		assert.Equal(
			t, "", w.Header().Get(aichteeteapee.HeaderNameXContentTypeOptions),
		)
		assert.Equal(t, "", w.Header().Get(aichteeteapee.HeaderNameXFrameOptions))
		assert.Equal(t, "", w.Header().Get(aichteeteapee.HeaderNameXXSSProtection))
		assert.Equal(t, "", w.Header().Get(aichteeteapee.HeaderNameReferrerPolicy))

		assert.Equal(
			t, "", w.Header().Get(aichteeteapee.HeaderNameContentSecurityPolicy),
		)

		// HSTS should still be set (not disabled)

		assert.NotEmpty(
			t, w.Header().Get(aichteeteapee.HeaderNameStrictTransportSecurity),
		)
	})
}
