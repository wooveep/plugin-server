package wrapper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommonHttpCtxEnableRequestBodyIsExplicitOptIn(t *testing.T) {
	ctx := &CommonHttpCtx[struct{}]{}
	require.False(t, ctx.needRequestBody, "the conservative default must remain unchanged")

	ctx.EnableRequestBody()
	require.True(t, ctx.needRequestBody, "explicit opt-in must make body callbacks eligible")

	ctx.DontReadRequestBody()
	require.False(t, ctx.needRequestBody, "the existing opt-out must remain authoritative")
}
