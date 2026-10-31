package mediaexecution_test

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/tidwall/resp"
)

// Real Redis qualifies the one original send and retains its existing Stream
// evidence. This does not assert provider receipt or completed settlement.
func TestNativeMediaTicketConsumesOnceAndUsesOriginalAccount(t *testing.T) {
	address := os.Getenv("MEDIA_EXECUTION_TEST_REDIS_URL")
	if address == "" {
		t.Skip("MEDIA_EXECUTION_TEST_REDIS_URL is required for actual Redis media admission")
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "redis" {
		t.Fatal("owned Redis URL is invalid")
	}
	connection, err := net.DialTimeout("tcp", u.Host, 3*time.Second)
	if err != nil {
		t.Fatal("owned Redis unavailable")
	}
	t.Cleanup(func() { _ = connection.Close() })
	reader := resp.NewReader(connection)
	command := func(arguments ...string) resp.Value {
		t.Helper()
		if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var buffer bytes.Buffer
		fmt.Fprintf(&buffer, "*%d\r\n", len(arguments))
		for _, argument := range arguments {
			fmt.Fprintf(&buffer, "$%d\r\n%s\r\n", len(argument), argument)
		}
		if _, err := connection.Write(buffer.Bytes()); err != nil {
			t.Fatal("owned Redis write failed")
		}
		value, _, err := reader.ReadValue()
		if err != nil || value.Error() != nil {
			t.Fatalf("owned Redis command failed: %v %v", err, value.Error())
		}
		return value
	}
	if u.User != nil {
		password, _ := u.User.Password()
		arguments := []string{"AUTH", password}
		if u.User.Username() != "" {
			arguments = []string{"AUTH", u.User.Username(), password}
		}
		command(arguments...)
	}
	prefix := "media-test:{" + uuid.NewString() + "}:"
	keys := []string{prefix + "ticket", prefix + "control", prefix + "account", prefix + "key", prefix + "stream"}
	t.Cleanup(func() { command(append([]string{"DEL"}, keys...)...) })
	digest := "sha256:" + strings.Repeat("a", 64)
	event := `{"schema_version":"media-dispatch-observed.v1","operation_id":"original-operation","ticket_digest":"` + digest + `"}`
	seed := func(mode string) {
		t.Helper()
		command(append([]string{"DEL"}, keys...)...)
		command("HSET", keys[0], "state", "prepared", "digest", digest, "expires_at_ms", fmt.Sprint(time.Now().Add(time.Minute).UnixMilli()), "mode", mode,
			"generation", "original-generation", "account_id", "original-wallet", "admission_revision", "9007199254740993", "mapping_revision", "9007199254740995",
			"credential_generation", "9007199254740997", "route_id", "original-route", "key_id", "original-key", "operation_id", "original-operation")
		command("HSET", keys[1], "cache_generation", "original-generation", "projection_schema", "balance.v1", "projection_state", "ready")
		command("HSET", keys[2], "account_id", "original-wallet", "cache_generation", "original-generation", "projection_schema", "balance.v1", "projection_state", "ready",
			"admission_state_revision", "9007199254740993", "mapping_revision", "9007199254740995", "credential_generation", "9007199254740997", "amount", "9007199254740993", "currency", "CNY", "scale", "10")
		command("HSET", keys[3], "account_id", "original-wallet", "cache_generation", "original-generation", "projection_schema", "balance.v1", "projection_state", "ready",
			"admission_state_revision", "9007199254740993", "mapping_revision", "9007199254740995", "credential_generation", "9007199254740997", "enabled", "1", "authorized_routes", `["original-route"]`)
	}
	consume := func() []resp.Value {
		t.Helper()
		arguments := append([]string{"EVAL", mediaexecution.DispatchConsumeScript, "5"}, keys...)
		return command(append(arguments, digest, event)...).Array()
	}
	for _, test := range []struct {
		name, mode, reason string
		change             func()
	}{
		{"one original create", "create", "allow", func() {}},
		{"maintenance denies a new send", "create", "projection_unready", func() { command("HSET", keys[1], "projection_state", "blocked") }},
		{"new generation cannot renew an old handoff", "create", "projection_unready", func() { command("HSET", keys[1], "cache_generation", "new-generation") }},
		{"known negative balance denies", "create", "insufficient_balance", func() { command("HSET", keys[2], "amount", "-1") }},
		{"same user other Key cannot supply a grant", "create", "route_authorization_missing", func() { command("HSET", keys[3], "authorized_routes", `[]`) }},
		{"revoked original Key denies", "create", "authorization_stale", func() { command("HSET", keys[3], "enabled", "0") }},
		{"expired ticket cannot send", "create", "media_ticket_expired", func() { command("HSET", keys[0], "expires_at_ms", "1") }},
		{"lost ticket is never reconstructed", "create", "media_ticket_missing", func() { command("DEL", keys[0]) }},
		{"original query is independent of new-call debt", "query", "allow", func() {
			command("HSET", keys[2], "amount", "-1")
			command("HSET", keys[1], "projection_state", "blocked")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			seed(test.mode)
			test.change()
			result := consume()
			if len(result) != 2 {
				t.Fatalf("unexpected media admission response: %#v", result)
			}
			if test.reason == "allow" {
				if result[0].String() != "allow" || command("XLEN", keys[4]).Integer() != 1 {
					t.Fatal("original send lacks its durable Stream record")
				}
				replay := consume()
				if replay[0].String() != "deny" || replay[1].String() != "media_ticket_consumed" || command("XLEN", keys[4]).Integer() != 1 {
					t.Fatal("duplicate notice produced a second send right or event")
				}
			} else {
				expectedEvents := 0
				if test.reason == "insufficient_balance" || test.reason == "route_authorization_missing" || test.reason == "authorization_stale" {
					expectedEvents = 1
				}
				if result[0].String() != "deny" || result[1].String() != test.reason || command("XLEN", keys[4]).Integer() != expectedEvents {
					t.Fatalf("expected %s and %d retained decision events; got %#v", test.reason, expectedEvents, result)
				}
			}
		})
	}
}
