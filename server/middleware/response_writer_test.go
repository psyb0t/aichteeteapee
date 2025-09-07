package middleware

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockHijackableResponseWriter implements http.ResponseWriter and http.Hijacker for testing
type mockHijackableResponseWriter struct {
	*httptest.ResponseRecorder
}

func (m *mockHijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// Create mock connections for testing
	conn1, conn2 := net.Pipe()
	rw := bufio.NewReadWriter(bufio.NewReader(conn1), bufio.NewWriter(conn1))
	// Close conn2 as we only need conn1 for the test
	defer conn2.Close()
	return conn1, rw, nil
}

// mockFailingHijacker implements http.Hijacker but returns an error
type mockFailingHijacker struct {
	*httptest.ResponseRecorder
}

func (m *mockFailingHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errors.New("hijack failed")
}

func TestBaseResponseWriter_Hijack(t *testing.T) {
	tests := []struct {
		name          string
		responseWriter http.ResponseWriter
		expectError   bool
		errorType     error
	}{
		{
			name:          "hijacker not supported",
			responseWriter: httptest.NewRecorder(),
			expectError:   true,
			errorType:     http.ErrNotSupported,
		},
		{
			name:          "hijacker supported",
			responseWriter: &mockHijackableResponseWriter{ResponseRecorder: httptest.NewRecorder()},
			expectError:   false,
		},
		{
			name:          "hijacker fails",
			responseWriter: &mockFailingHijacker{ResponseRecorder: httptest.NewRecorder()},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brw := &BaseResponseWriter{
				ResponseWriter: tt.responseWriter,
			}

			conn, rw, err := brw.Hijack()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, conn)
				assert.Nil(t, rw)
				if tt.errorType != nil {
					assert.Equal(t, tt.errorType, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, conn)
				assert.NotNil(t, rw)
				// Close the connection to clean up
				if conn != nil {
					conn.Close()
				}
			}
		})
	}
}