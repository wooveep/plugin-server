package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
)

func TestThinCacheReplayBillingFacts(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host := startThinRedisReplayRequest(t)
		defer host.Reset()

		host.CallOnRedisCall(0, test.CreateRedisRespString(validThinReplayRecord(t, map[string]interface{}{
			"upstream_invoked": false,
			"billing": map[string]interface{}{
				"upstream_invoked": false,
			},
		})))

		localResponse := host.GetLocalResponse()
		require.NotNil(t, localResponse, "valid materialized record should be replayed")

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(localResponse.Data, &body), "local replay response must remain valid OpenAI-compatible JSON")
		thinRequireNoBillingFields(t, body)

		attrs := thinCacheReplayAILogAttributes(t, host)
		require.Equal(t, "hit", attrs["cache_status"])
		require.Equal(t, false, attrs["upstream_invoked"])
	})
}

func thinCacheReplayAILogAttributes(t *testing.T, host test.TestHost) map[string]interface{} {
	t.Helper()
	raw, err := host.GetProperty([]string{wrapper.AILogKey})
	require.NoError(t, err)

	var attrs map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(wrapper.UnmarshalStr(`"`+string(raw)+`"`)), &attrs))
	return attrs
}

func thinRequireNoBillingFields(t *testing.T, value interface{}) {
	t.Helper()
	if path, snippet, ok := thinFindBillingLeak(value, "$"); ok {
		require.Failf(t, "user response body exposed billing facts", "path=%s value=%s", path, snippet)
	}
}

func thinFindBillingLeak(value interface{}, path string) (string, string, bool) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			childPath := path + "." + key
			if key == "upstream_invoked" || key == "billing" {
				return childPath, key, true
			}
			if leakPath, snippet, ok := thinFindBillingLeak(child, childPath); ok {
				return leakPath, snippet, true
			}
		}
	case []interface{}:
		for i, child := range typed {
			if leakPath, snippet, ok := thinFindBillingLeak(child, fmt.Sprintf("%s[%d]", path, i)); ok {
				return leakPath, snippet, true
			}
		}
	case string:
		if strings.Contains(typed, "upstream_invoked") || strings.Contains(typed, "billing") {
			return path, thinBillingSnippet(typed, 240), true
		}
	}
	return "", "", false
}

func thinBillingSnippet(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
