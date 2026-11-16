package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/ai/clientresponse"
	"github.com/higress-group/wasm-go/pkg/ai/resourceref"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

// This transport stub still exercises the real MGET construction, complete
// Lookup record validation, owner selection and all JSON replacements.
type incidentResourceLookup struct {
	reply resp.Value
	calls int
}

func (r *incidentResourceLookup) Command(command []interface{}, done wrapper.RedisResponseCallback) error {
	r.calls++
	done(r.reply)
	return nil
}

func incidentResourceLookupFixture(tb testing.TB, count int) ([]byte, *managedExecutionState, *incidentResourceLookup) {
	tb.Helper()
	owner := resourceref.Owner{Provider: "owned-provider", Account: "owned-account", Region: "owned-region", Model: "upstream-model", Protocol: clientresponse.Responses}
	var body strings.Builder
	body.WriteString(`{"model":"upstream-model","input":[`)
	records := make([]resp.Value, count)
	for i := range count {
		if i != 0 {
			body.WriteByte(',')
		}
		alias := clientresponse.ResourceAlias{Kind: "call", Public: fmt.Sprintf("call_pf_%032x", i+1), Original: "native-call-" + strconv.Itoa(i)}
		fmt.Fprintf(&body, `{"type":"function_call_output","call_id":%q,"output":%q}`, alias.Public, strings.Repeat("x", 800))
		encoded, err := json.Marshal(resourceref.NewRecord(alias, owner, true))
		require.NoError(tb, err)
		records[i] = resp.StringValue(string(encoded))
	}
	body.WriteString(`]}`)
	raw := []byte(body.String())
	lookup := &incidentResourceLookup{reply: resp.ArrayValue(records)}
	state := &managedExecutionState{publicRequest: raw, identity: balanceidentity.Snapshot{TenantID: "owned-tenant", PrincipalID: "owned-user"}, config: &managedExecutionConfig{
		profile:        managedProtocolProfile{InputProtocol: clientresponse.Responses, UpstreamProtocol: clientresponse.Responses},
		release:        managedRelease{SalesAssetID: "owned-asset", PublicModel: "public-model", Descriptor: managedDescriptor{ProviderID: owner.Provider, ProviderAccountID: owner.Account, RegionEndpointRef: owner.Region, UpstreamModel: owner.Model}},
		referenceRedis: lookup,
	}}
	return raw, state, lookup
}

func TestResponsesIncidentFullHistoryLookupRestoresEveryAlias(t *testing.T) {
	body, state, lookup := incidentResourceLookupFixture(t, 1090)
	var restored []byte
	restoreManagedResourceReferences(nil, state, body, func(value []byte) { restored = value })
	require.Equal(t, 1, lookup.calls)
	items := gjson.GetBytes(restored, "input").Array()
	require.Len(t, items, 1090)
	for i, item := range items {
		require.Equal(t, "native-call-"+strconv.Itoa(i), item.Get("call_id").String())
		require.Equal(t, strings.Repeat("x", 800), item.Get("output").String())
	}
	require.Equal(t, "upstream-model", gjson.GetBytes(restored, "model").String())
}

func TestResponsesIncidentResourceSpansKeepOrderEscapesAndOtherFields(t *testing.T) {
	for _, objectConversation := range []bool{false, true} {
		t.Run(strconv.FormatBool(objectConversation), func(t *testing.T) {
			_, state, lookup := incidentResourceLookupFixture(t, 1)
			owner := managedResourceOwner(state.config)
			aliases := []clientresponse.ResourceAlias{
				{Kind: "response", Public: "resp_pf_11111111111111111111111111111111", Original: "native-response-\"\n中"},
				{Kind: "conversation", Public: "conv_pf_22222222222222222222222222222222", Original: "native-conversation-\\中"},
				{Kind: "item", Public: "item_pf_33333333333333333333333333333333", Original: "native-item-\"中"},
				{Kind: "call", Public: "call_pf_44444444444444444444444444444444", Original: "native-call-\\\n中"},
			}
			records := make([]resp.Value, len(aliases))
			for i, alias := range aliases {
				raw, err := json.Marshal(resourceref.NewRecord(alias, owner, true))
				require.NoError(t, err)
				records[i] = resp.StringValue(string(raw))
			}
			lookup.reply = resp.ArrayValue(records)
			conversation := fmt.Sprintf("%q", aliases[1].Public)
			if objectConversation {
				conversation = `{"id":` + conversation + `}`
			}
			// Collection order differs from byte order, and the public call ID is
			// escaped in JSON. Every replacement must still use original offsets.
			body := []byte(fmt.Sprintf(`{ "input": [{"call_id":"\u0063all_pf_44444444444444444444444444444444","type":"item_reference","id":%q}], "conversation":%s, "previous_response_id":%q, "untouched" : "\u4e2d\\path", "integer":9007199254740993, "model":"upstream-model" }`, aliases[2].Public, conversation, aliases[0].Public))
			state.publicRequest = body
			var restored []byte
			restoreManagedResourceReferences(nil, state, body, func(value []byte) { restored = value })
			require.Equal(t, 1, lookup.calls)
			require.True(t, gjson.ValidBytes(restored))
			require.Equal(t, aliases[0].Original, gjson.GetBytes(restored, "previous_response_id").String())
			conversationPath := "conversation"
			if objectConversation {
				conversationPath += ".id"
			}
			require.Equal(t, aliases[1].Original, gjson.GetBytes(restored, conversationPath).String())
			require.Equal(t, aliases[2].Original, gjson.GetBytes(restored, "input.0.id").String())
			require.Equal(t, aliases[3].Original, gjson.GetBytes(restored, "input.0.call_id").String())
			require.Contains(t, string(restored), `"untouched" : "\u4e2d\\path"`)
			require.Equal(t, "9007199254740993", gjson.GetBytes(restored, "integer").Raw)
		})
	}
}

// #11 follow-up: complete Lookup with legal aliases, rather than only scanning
// ordinary native IDs. Redis network latency is intentionally excluded.
func BenchmarkRestoreManagedResourceLookup(b *testing.B) {
	for _, count := range []int{545, 1090, 2180} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			body, state, _ := incidentResourceLookupFixture(b, count)
			var restored []byte
			b.SetBytes(int64(len(body)))
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				state.publicAliases = nil
				restoreManagedResourceReferences(nil, state, body, func(value []byte) { restored = value })
			}
			b.StopTimer()
			if len(gjson.GetBytes(restored, "input").Array()) != count {
				b.Fatal("complete alias lookup lost history")
			}
		})
	}
}
