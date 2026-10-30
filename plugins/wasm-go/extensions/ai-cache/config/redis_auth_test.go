package config

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRedisAuthenticationAndDatabaseSurviveRouteOverrides(t *testing.T) {
	global := parseThinConfig(t, []byte(`{
		"cache_policy_version":"auth-v1",
		"materialized_lookup":{"redis":{"enabled":true,"service_name":"redis.static","service_port":6379,"username":"cache-user","password":"synthetic-secret","database":7}},
		"redis_stream":{"enabled":true,"service_name":"redis.static","service_port":6379,"username":"cache-user","password":"synthetic-secret","database":7}
	}`))
	for _, path := range [][]string{{"MaterializedLookup", "Redis"}, {"Event", "RedisStream"}} {
		require.Equal(t, "cache-user", mustStringField(t, global, append(path, "Username")...))
		require.Equal(t, "synthetic-secret", mustStringField(t, global, append(path, "Password")...))
		require.Equal(t, int64(7), mustIntField(t, global, append(path, "Database")...))
	}
	var route PluginConfig
	route.FromJsonWithGlobal(gjson.Parse(`{"materialized_lookup":{"redis":{"database":0,"username":"","password":"route-secret"}},"redis_stream":{"database":0,"username":"","password":"route-secret"}}`), global, noopLogger{})
	for _, path := range [][]string{{"MaterializedLookup", "Redis"}, {"Event", "RedisStream"}} {
		require.Equal(t, "", mustStringField(t, route, append(path, "Username")...))
		require.Equal(t, "route-secret", mustStringField(t, route, append(path, "Password")...))
		require.Equal(t, int64(0), mustIntField(t, route, append(path, "Database")...))
	}
}

func TestSharedRedisRejectsDifferentAuthenticationOrDatabase(t *testing.T) {
	for _, mismatch := range []string{`"password":"different"`, `"username":"different"`, `"database":8`} {
		cfg := parseThinConfig(t, []byte(`{"cache_policy_version":"auth-v1",
			"materialized_lookup":{"redis":{"enabled":true,"service_name":"redis.static","service_port":6379}},
			"redis_stream":{"enabled":true,"service_name":"redis.static","service_port":6379,`+mismatch+`}}`))
		require.ErrorContains(t, cfg.Complete(noopLogger{}), "matching credentials and database")
	}
}
