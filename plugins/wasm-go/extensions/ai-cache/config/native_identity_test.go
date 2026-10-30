package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const nativeIdentityConfigFields = `"release_id":"opaque-release","target_id":"original-target","route_id":"11111111-1111-4111-8111-111111111111","policy_id":"22222222-2222-4222-8222-222222222222"`

func nativeIdentityConfigJSON(version string) string {
	return fmt.Sprintf(`{"native_identity":{%s,"policy_version":%s},"cache_policy_version":%q,"redis_stream":{"enabled":true,"service_name":"redis.static","service_port":6379,"timeout":80}}`, nativeIdentityConfigFields, version, version)
}

func TestNativeCacheIdentityConfigurationIsExact(t *testing.T) {
	for _, version := range []string{"1", "9007199254740993", "9223372036854775807"} {
		t.Run(version, func(t *testing.T) {
			var cfg PluginConfig
			cfg.FromJson(gjson.Parse(nativeIdentityConfigJSON(version)), noopLogger{})
			require.NoError(t, cfg.Validate())
			require.NotNil(t, cfg.NativeIdentity)
			require.Equal(t, version, fmt.Sprint(cfg.NativeIdentity.PolicyVersion), "configuration must not round through float64")
		})
	}
	for _, version := range []string{`"1"`, "1.0", "1e0", "0", "-1", "9223372036854775808", "null"} {
		t.Run(version, func(t *testing.T) {
			var cfg PluginConfig
			cfg.FromJson(gjson.Parse(nativeIdentityConfigJSON(version)), noopLogger{})
			require.ErrorContains(t, cfg.Validate(), "native_identity")
		})
	}
	for _, tc := range []struct{ name, field, raw string }{
		{"null declaration", "native_identity", `null`},
		{"incomplete declaration", "native_identity", `{"release_id":"opaque-release"}`},
		{"configured caller", "native_identity.caller", `{}`},
		{"unknown authority", "native_identity.actor", `"admin"`},
		{"wrong string type", "native_identity.release_id", `4`},
		{"empty original target", "native_identity.target_id", `""`},
		{"oversized original target", "native_identity.target_id", fmt.Sprintf("%q", strings.Repeat("a", 257))},
		{"nil route", "native_identity.route_id", `"00000000-0000-0000-0000-000000000000"`},
		{"noncanonical policy UUID", "native_identity.policy_id", `"AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"`},
		{"different policy version", "cache_policy_version", `"2"`},
		{"noncanonical policy version", "cache_policy_version", `"01"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := sjson.SetRaw(nativeIdentityConfigJSON("1"), tc.field, tc.raw)
			require.NoError(t, err)
			var cfg PluginConfig
			cfg.FromJson(gjson.Parse(raw), noopLogger{})
			require.ErrorContains(t, cfg.Validate(), "native_identity")
		})
	}
}

func TestNativeCacheIdentityOverrideReplacesTheCompleteStaticTuple(t *testing.T) {
	var global, route PluginConfig
	global.FromJson(gjson.Parse(nativeIdentityConfigJSON("1")), noopLogger{})
	require.NoError(t, global.Validate())
	route.FromJsonWithGlobal(gjson.Parse(`{"route_policy":{"enable_replay":false}}`), global, noopLogger{})
	require.NoError(t, route.Validate())
	require.Equal(t, global.NativeIdentity, route.NativeIdentity)
	route.FromJsonWithGlobal(gjson.Parse(`{"native_identity":{"policy_version":2},"cache_policy_version":"2"}`), global, noopLogger{})
	require.ErrorContains(t, route.Validate(), "native_identity", "a partial route tuple must not inherit a different release or policy")
	require.Equal(t, int64(1), global.NativeIdentity.PolicyVersion, "override parsing must not mutate the original global tuple")
	complete := nativeIdentityConfigJSON("2")
	complete, err := sjson.Set(complete, "native_identity.release_id", "another-original-release")
	require.NoError(t, err)
	route.FromJsonWithGlobal(gjson.Parse(complete), global, noopLogger{})
	require.NoError(t, route.Validate())
	require.Equal(t, "another-original-release", route.NativeIdentity.ReleaseID)
	require.Equal(t, int64(2), route.NativeIdentity.PolicyVersion)
	require.Equal(t, int64(1), global.NativeIdentity.PolicyVersion)
}
