package wrapper

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClusterClientPostSensitivePropagatesSynchronousDispatchFailure(t *testing.T) {
	dispatchError := errors.New("sensitive dispatch unavailable")
	secretBody := []byte("sensitive-request-body-marker")
	originalDispatcher := dispatchSensitiveHttpCall
	t.Cleanup(func() {
		dispatchSensitiveHttpCall = originalDispatcher
	})

	dispatchCalls := 0
	dispatchSensitiveHttpCall = func(
		cluster string,
		headers [][2]string,
		body []byte,
		trailers [][2]string,
		timeoutMillisecond uint32,
		callback func(numHeaders, bodySize, numTrailers int),
	) (uint32, error) {
		dispatchCalls++
		require.Equal(t, "sensitive-oauth-cluster", cluster)
		require.Equal(t, secretBody, body)
		require.Nil(t, trailers)
		require.Equal(t, uint32(1234), timeoutMillisecond)

		require.Equal(t, 1, countHeader(headers, ":method"))
		require.Equal(t, 1, countHeader(headers, ":path"))
		require.Equal(t, 1, countHeader(headers, ":authority"))
		require.Equal(t, http.MethodPost, headerValue(headers, ":method"))
		require.Equal(
			t,
			"/oauth/token?assertion=a%2Bb&dup=1&dup=2",
			headerValue(headers, ":path"),
		)
		require.Equal(t, "oauth.example:9443", headerValue(headers, ":authority"))
		require.Equal(t, "application/x-www-form-urlencoded", headerValue(headers, "content-type"))
		require.Equal(t, "preserve", headerValue(headers, "x-operation-marker"))
		return 0, dispatchError
	}

	client := NewClusterClient(TargetCluster{
		Host:    "configured.example",
		Cluster: "sensitive-oauth-cluster",
	})
	callbackCalled := false
	err := client.PostSensitive(
		"https://oauth.example:9443/oauth/token?assertion=a%2Bb&dup=1&dup=2",
		[][2]string{
			{":method", http.MethodGet},
			{":path", "/attacker-path"},
			{":authority", "attacker.example"},
			{"content-type", "application/x-www-form-urlencoded"},
			{"x-operation-marker", "preserve"},
		},
		secretBody,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			callbackCalled = true
		},
		1234,
		5678,
	)

	require.ErrorIs(t, err, dispatchError)
	require.Equal(t, 1, dispatchCalls)
	require.False(t, callbackCalled)
	require.NotContains(t, err.Error(), string(secretBody))
}

func countHeader(headers [][2]string, name string) int {
	count := 0
	for _, header := range headers {
		if header[0] == name {
			count++
		}
	}
	return count
}

func headerValue(headers [][2]string, name string) string {
	for _, header := range headers {
		if header[0] == name {
			return header[1]
		}
	}
	return ""
}
