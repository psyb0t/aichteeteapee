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
		"X-Content-Type-Options":    aichteeteapee.DefaultSecurityXContentTypeOptionsNoSniff,
		"X-Frame-Options":           aichteeteapee.DefaultSecurityXFrameOptionsDeny,
		"X-XSS-Protection":          aichteeteapee.DefaultSecurityXXSSProtectionBlock,
		"Strict-Transport-Security": aichteeteapee.DefaultSecurityStrictTransportSecurity,
		"Referrer-Policy":           aichteeteapee.DefaultSecurityReferrerPolicyStrictOrigin,
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

		assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
		assert.Equal(t, "", w.Header().Get("Strict-Transport-Security")) // Disabled
		assert.NotEmpty(t, w.Header().Get("X-Content-Type-Options"))     // Still enabled
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
		assert.Equal(t, "", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "", w.Header().Get("X-Frame-Options"))
		assert.Equal(t, "", w.Header().Get("X-XSS-Protection"))
		assert.Equal(t, "", w.Header().Get("Referrer-Policy"))
		assert.Equal(t, "", w.Header().Get("Content-Security-Policy"))

		// HSTS should still be set (not disabled)
		assert.NotEmpty(t, w.Header().Get("Strict-Transport-Security"))
	})
}
