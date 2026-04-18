package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "valid bearer",
			header: "Bearer abc123",
			want:   "abc123",
		},
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
		{
			name:   "wrong scheme",
			header: "Basic abc123",
			want:   "",
		},
		{
			name:   "bearer only no token",
			header: "Bearer ",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(
				http.MethodGet, "/", nil,
			)

			if tt.header != "" {
				r.Header.Set(
					"Authorization", tt.header,
				)
			}

			assert.Equal(
				t, tt.want,
				extractBearerToken(r),
			)
		})
	}
}

func TestBearerAuth(t *testing.T) {
	validToken := "valid-token"

	auth := BearerAuth(
		func(
			_ context.Context,
			token string,
		) error {
			if token == validToken {
				return nil
			}

			return http.ErrAbortHandler
		},
	)

	tests := []struct {
		name    string
		header  string
		wantErr bool
	}{
		{
			name:   "valid token",
			header: "Bearer " + validToken,
		},
		{
			name:    "invalid token",
			header:  "Bearer wrong",
			wantErr: true,
		},
		{
			name:    "missing token",
			header:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(
				http.MethodGet, "/", nil,
			)

			if tt.header != "" {
				r.Header.Set(
					"Authorization", tt.header,
				)
			}

			err := auth(context.Background(), r)

			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
		})
	}
}
