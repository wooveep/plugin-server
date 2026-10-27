package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/tidwall/resp"
)

const testScope = "00000000-0000-0000-0000-000000000005"
const testGeneration = "00000000-0000-0000-0000-000000000006"
const testAccount = "00000000-0000-0000-0000-000000000002"
const testAPIKey = "00000000-0000-0000-0000-000000000003"

var testIdentity = balanceidentity.Snapshot{AccountID: testAccount, APIKeyID: testAPIKey, CredentialGeneration: "9007199254740993"}
var testNamespace = balanceNamespace{ScopeID: testScope, Generation: testGeneration}

func testAccountKey() string { return testNamespace.keys(testIdentity)[1].(string) }

// This uses an explicitly supplied owned Redis, not the simulated Wasm host.
// Every case compares serialized records before/after the real Lua read.
func TestNativeRedisVersionedAdmissionIsExactAtomicAndReadOnly(t *testing.T) {
	rawURL := os.Getenv("WG_BALANCE_TEST_REDIS_URL")
	if rawURL == "" {
		t.Skip("WG_BALANCE_TEST_REDIS_URL is not set; actual Redis admission not run")
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "redis" {
		t.Fatal("owned Redis URL must use redis scheme")
	}
	connection, err := net.DialTimeout("tcp", u.Host, 3*time.Second)
	if err != nil {
		t.Fatal("owned Redis unavailable")
	}
	defer connection.Close()
	reader := resp.NewReader(connection)
	command := func(args ...string) resp.Value {
		t.Helper()
		if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var wire bytes.Buffer
		fmt.Fprintf(&wire, "*%d\r\n", len(args))
		for _, arg := range args {
			fmt.Fprintf(&wire, "$%d\r\n%s\r\n", len(arg), arg)
		}
		if _, err := connection.Write(wire.Bytes()); err != nil {
			t.Fatal("owned Redis write failed")
		}
		value, _, err := reader.ReadValue()
		if err != nil {
			t.Fatal("owned Redis response failed")
		}
		return value
	}
	if u.User != nil {
		password, _ := u.User.Password()
		args := []string{"AUTH", password}
		if u.User.Username() != "" {
			args = []string{"AUTH", u.User.Username(), password}
		}
		if err := command(args...).Error(); err != nil {
			t.Fatal("owned Redis authentication failed")
		}
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	namespace := balanceNamespace{ScopeID: fmt.Sprintf("%x-%x-%x-%x-%x", random[:4], random[4:6], random[6:8], random[8:10], random[10:]), Generation: testGeneration}
	keys := namespace.keys(testIdentity)
	control, account, mapping := keys[0].(string), keys[1].(string), keys[2].(string)
	defer command("DEL", control, account, mapping)
	setup := func() {
		t.Helper()
		command("DEL", control, account, mapping)
		for _, args := range [][]string{
			{"HSET", control, "scope_id", namespace.ScopeID, "cache_generation", namespace.Generation, "projection_schema", "balance.v1", "projection_state", "ready"},
			{"HSET", account, "account_id", testAccount, "cache_generation", namespace.Generation, "projection_schema", "balance.v1", "projection_state", "ready", "account_version", "9007199254740993", "amount", "9007199254740993", "currency", "CNY", "scale", "10", "mapping_revision", "9007199254740995", "admission_state_revision", "9007199254740997", "credential_generation", testIdentity.CredentialGeneration},
			{"HSET", mapping, "account_id", testAccount, "cache_generation", namespace.Generation, "projection_schema", "balance.v1", "projection_state", "ready", "mapping_revision", "9007199254740995", "admission_state_revision", "9007199254740997", "credential_generation", testIdentity.CredentialGeneration, "enabled", "1"},
		} {
			if err := command(args...).Error(); err != nil {
				t.Fatal("owned Redis fixture setup failed")
			}
		}
	}
	cases := []struct {
		name, key, field, value, reason string
		delete, dependencyError         bool
	}{
		{name: "positive beyond double precision", reason: "ok"},
		{name: "maximum signed money", key: account, field: "amount", value: "9223372036854775807", reason: "ok"},
		{name: "zero money", key: account, field: "amount", value: "0", reason: "insufficient_balance"},
		{name: "minimum signed debt", key: account, field: "amount", value: "-9223372036854775808", reason: "insufficient_balance"},
		{name: "plus sign", key: account, field: "amount", value: "+1", reason: "invalid_amount"},
		{name: "leading zero", key: account, field: "amount", value: "01", reason: "invalid_amount"},
		{name: "negative zero", key: account, field: "amount", value: "-0", reason: "invalid_amount"},
		{name: "float", key: account, field: "amount", value: "1.0", reason: "invalid_amount"},
		{name: "overflow", key: account, field: "amount", value: "9223372036854775808", reason: "invalid_amount"},
		{name: "wrong unit", key: account, field: "scale", value: "6", reason: "invalid_amount"},
		{name: "wrong currency", key: account, field: "currency", value: "USD", reason: "invalid_amount"},
		{name: "missing amount", key: account, field: "amount", delete: true, reason: "invalid_amount"},
		{name: "bad money version", key: account, field: "account_version", value: "01", reason: "projection_unready"},
		{name: "missing control", key: control, delete: true, reason: "projection_unready"},
		{name: "building control", key: control, field: "projection_state", value: "building", reason: "projection_unready"},
		{name: "old control generation", key: control, field: "cache_generation", value: testAPIKey, reason: "wrong_generation"},
		{name: "wrong scope", key: control, field: "scope_id", value: testAPIKey, reason: "account_mismatch"},
		{name: "missing account", key: account, delete: true, reason: "balance_missing"},
		{name: "wrong account", key: account, field: "account_id", value: testAPIKey, reason: "account_mismatch"},
		{name: "old account generation", key: account, field: "cache_generation", value: testAPIKey, reason: "wrong_generation"},
		{name: "blocked account", key: account, field: "projection_state", value: "blocked", reason: "projection_unready"},
		{name: "missing key mapping", key: mapping, delete: true, reason: "authorization_stale"},
		{name: "key disabled", key: mapping, field: "enabled", value: "0", reason: "authorization_stale"},
		{name: "unready key mapping", key: mapping, field: "projection_state", value: "building", reason: "projection_unready"},
		{name: "stale mapping version", key: mapping, field: "mapping_revision", value: "9007199254740994", reason: "authorization_stale"},
		{name: "stale admission version", key: mapping, field: "admission_state_revision", value: "9007199254740996", reason: "authorization_stale"},
		{name: "old key generation", key: mapping, field: "credential_generation", value: "9007199254740992", reason: "authorization_stale"},
		{name: "control schema", key: control, field: "projection_schema", value: "balance.v0", reason: "projection_schema_mismatch"},
		{name: "account schema", key: account, field: "projection_schema", value: "balance.v0", reason: "projection_schema_mismatch"},
		{name: "mapping schema", key: mapping, field: "projection_schema", value: "balance.v0", reason: "projection_schema_mismatch"},
		{name: "old integer record", key: account, value: "100", dependencyError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setup()
			if tc.key != "" {
				switch {
				case tc.dependencyError:
					command("DEL", tc.key)
					command("SET", tc.key, tc.value)
				case tc.delete && tc.field == "":
					command("DEL", tc.key)
				case tc.delete:
					command("HDEL", tc.key, tc.field)
				default:
					command("HSET", tc.key, tc.field, tc.value)
				}
			}
			dump := func() []string {
				return []string{command("DUMP", control).String(), command("DUMP", account).String(), command("DUMP", mapping).String()}
			}
			before := dump()
			response := command("EVAL", balanceAdmissionScript, "3", control, account, mapping, namespace.ScopeID, namespace.Generation, testAccount, testIdentity.CredentialGeneration)
			result, err := decodeBalanceAdmission(response, namespace)
			if tc.dependencyError {
				if err == nil {
					t.Fatal("accepted an old bare integer record")
				}
			} else if err != nil || result.ReasonCode != tc.reason || (result.Decision == "allow") != (tc.reason == "ok") {
				t.Fatalf("unexpected admission: %+v, %v", result, err)
			}
			if tc.reason == "ok" || tc.reason == "insufficient_balance" {
				if result.ObservedAccountVersion != "9007199254740993" || result.ObservedGeneration != namespace.Generation {
					t.Fatal("lost exact observed identity/version")
				}
			}
			if !reflect.DeepEqual(before, dump()) {
				t.Fatal("read-only admission mutated controlled Redis records")
			}
		})
	}
}
