package wrapper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactSensitiveRequestHeaders(t *testing.T) {
	headers := [][2]string{
		{"Authorization", "Bearer <token>"},
		{"x-api-key", "<api-key>"},
		{"X-API-Key", "<api-key-2>"},
		{"content-type", "application/json"},
	}

	redacted := redactSensitiveRequestHeaders(headers)

	require.Equal(t, [][2]string{
		{"Authorization", "[REDACTED]"},
		{"x-api-key", "[REDACTED]"},
		{"X-API-Key", "[REDACTED]"},
		{"content-type", "application/json"},
	}, redacted)
	require.Equal(t, "Bearer <token>", headers[0][1])
	require.Equal(t, "<api-key>", headers[1][1])
}
