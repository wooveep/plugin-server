package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVertexExpressAPIKeyUsesRequestContext(t *testing.T) {
	config := ProviderConfig{
		apiTokens: []string{"fallback-a", "fallback-b"},
		failover: &failover{
			ctxApiTokenInUse: "selected-credential",
		},
	}
	ctx := newMapCtx()
	ctx.SetContext(config.failover.ctxApiTokenInUse, "request-scoped-key")

	vertex := &vertexProvider{config: config}
	require.Equal(t, "request-scoped-key", vertex.getExpressAPIKey(ctx))
}
