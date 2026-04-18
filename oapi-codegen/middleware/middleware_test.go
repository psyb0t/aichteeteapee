package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func minimalSpec() *openapi3.T {
	return &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "test",
			Version: "1.0",
		},
		Paths: openapi3.NewPaths(
			openapi3.WithPath(
				"/test",
				&openapi3.PathItem{
					Get: &openapi3.Operation{
						OperationID: "getTest",
						Responses: openapi3.NewResponses(
							openapi3.WithStatus(
								http.StatusOK,
								&openapi3.ResponseRef{
									Value: openapi3.NewResponse().
										WithDescription("ok"),
								},
							),
						),
					},
				},
			),
		),
	}
}

func TestOapiValidatorMiddleware(t *testing.T) {
	spec := minimalSpec()
	require.NoError(t, spec.Validate(
		context.Background(),
	))

	mw := OapiValidatorMiddleware(spec)
	require.NotNil(t, mw)

	e := echo.New()
	e.Use(mw)
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "valid path",
			path:       "/test",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid path",
			path:       "/nonexistent",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet, tt.path, nil,
			)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			assert.Equal(
				t, tt.wantStatus, rec.Code,
			)
		})
	}
}
