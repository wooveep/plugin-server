package main

import (
	"regexp"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/memorycache"
	"github.com/higress-group/wasm-go/pkg/ai/sessionctx"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryTrustedCacheDigest(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("caller value is removed before assembly and replaced after injection", func(t *testing.T) {
			host := startMemoryTrustedDigestRequest(t, memoryConsoleAssembleRequestBody(), "caller-spoof")
			require.False(t, test.HasHeader(host.GetRequestHeaders(), memorycache.DeprecatedDigestHeader))

			completeMemoryTrustedDigestInjection(t, host, "Console memory context")
			digest := requireMemoryTrustedDigestProperty(t, host)
			require.NotEqual(t, "caller-spoof", digest)
			require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), digest)
			require.False(t, test.HasHeader(host.GetRequestHeaders(), memorycache.DeprecatedDigestHeader), "memory-only routes must not expose the private digest upstream")
		})

		t.Run("successful injection writes a canonical digest", func(t *testing.T) {
			host := startMemoryTrustedDigestRequest(t, memoryConsoleAssembleRequestBody(), "")
			completeMemoryTrustedDigestInjection(t, host, "shared assembled context")
			require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{64}$`), requireMemoryTrustedDigestProperty(t, host))
		})

		t.Run("digest is stable for equal context and changes with context", func(t *testing.T) {
			first := memoryTrustedDigestForContext(t, "stable assembled context")
			second := memoryTrustedDigestForContext(t, "stable assembled context")
			changed := memoryTrustedDigestForContext(t, "changed assembled context")
			require.Equal(t, first, second)
			require.NotEqual(t, first, changed)
		})

		t.Run("digest preserves role and order while normalizing whitespace", func(t *testing.T) {
			memory := sessionctx.OpenAIMessage{Role: " system ", Content: "  memory  "}
			digest, err := buildMemoryCacheDigest(memoryMessageAssemblyInput{
				MemoryMessage: &memory,
				Recent: []sessionctx.OpenAIMessage{
					{Role: "assistant", Content: " earlier answer\n"},
				},
			})
			require.NoError(t, err)
			require.Equal(t, "30df36da28dbd61748a8c80b0cc3150606946e814547b16ae6bd736da17955b7", digest)
		})

		t.Run("multiple current user messages exclude recent context", func(t *testing.T) {
			memory := sessionctx.OpenAIMessage{Role: "system", Content: "memory"}
			recent := []sessionctx.OpenAIMessage{{Role: "assistant", Content: "recent"}}
			withoutRecent, err := buildMemoryCacheDigest(memoryMessageAssemblyInput{MemoryMessage: &memory})
			require.NoError(t, err)
			withMultipleUsers, err := buildMemoryCacheDigest(memoryMessageAssemblyInput{
				Current: []sessionctx.OpenAIMessage{
					{Role: "user", Content: "first"},
					{Role: "user", Content: "second"},
				},
				MemoryMessage: &memory,
				Recent:        recent,
			})
			require.NoError(t, err)
			withSingleUser, err := buildMemoryCacheDigest(memoryMessageAssemblyInput{
				Current:       []sessionctx.OpenAIMessage{{Role: "user", Content: "current"}},
				MemoryMessage: &memory,
				Recent:        recent,
			})
			require.NoError(t, err)
			require.Equal(t, withoutRecent, withMultipleUsers)
			require.NotEqual(t, withoutRecent, withSingleUser)
		})

		t.Run("unsuccessful injection leaves the trusted digest absent", func(t *testing.T) {
			host := startMemoryTrustedDigestRequest(t, memoryConsoleAssembleRequestBody(), "caller-spoof")
			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "skip", nil, nil))
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			require.False(t, test.HasHeader(host.GetRequestHeaders(), memorycache.DeprecatedDigestHeader))
			_, err := host.GetProperty([]string{memorycache.TrustedDigestProperty})
			require.Error(t, err)
		})
	})
}

func startMemoryTrustedDigestRequest(t *testing.T, body []byte, callerDigest string) test.TestHost {
	t.Helper()
	return startMemoryTrustedDigestRequestWithCleanup(t, body, callerDigest, true)
}

func startMemoryTrustedDigestRequestWithCleanup(t *testing.T, body []byte, callerDigest string, cleanup bool) test.TestHost {
	t.Helper()
	headers := append([][2]string(nil), memoryConsoleAssembleHeaders()...)
	headers = append(headers, [2]string{memorycache.DeprecatedDigestHeader, callerDigest})
	var host test.TestHost
	if cleanup {
		host = startMemoryConsoleAssembleRequestWithConfig(t, memoryConsoleAssembleConfig(t, "semantic"), body, headers)
	} else {
		host = startMemoryConsoleAssembleRequestWithConfigNoCleanup(t, memoryConsoleAssembleConfig(t, "semantic"), body, headers)
	}
	require.False(t, test.HasHeader(host.GetRequestHeaders(), memorycache.DeprecatedDigestHeader))
	host.CallOnRedisCall(0, test.CreateRedisRespNull())
	requireMemoryAssembleCall(t, host)
	return host
}

func completeMemoryTrustedDigestInjection(t *testing.T, host test.TestHost, context string) {
	t.Helper()
	host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
		"role":    "system",
		"content": context,
	}, nil))
	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
}

func requireMemoryTrustedDigestProperty(t *testing.T, host test.TestHost) string {
	t.Helper()
	value, err := host.GetProperty([]string{memorycache.TrustedDigestProperty})
	require.NoError(t, err, "successful memory injection must set private trusted cache digest state")
	require.NotEmpty(t, value)
	return string(value)
}

func memoryTrustedDigestForContext(t *testing.T, context string) string {
	t.Helper()
	host := startMemoryTrustedDigestRequestWithCleanup(t, memoryConsoleAssembleRequestBody(), "", false)
	defer host.Reset()
	completeMemoryTrustedDigestInjection(t, host, context)
	return requireMemoryTrustedDigestProperty(t, host)
}
