package upstreamhealth

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/resp"
)

// This host test exercises the production wrapper Redis client and native Wasm
// SDK callback. It proves read transport only, not live Redis/Console behavior.
func TestWasmHostReadsOneCompleteHashWithoutWriteOrProviderSend(t *testing.T) {
	type hostConfig struct{}
	var reader *Reader
	var result ReadResult
	vm := wrapper.NewCommonVmCtx("upstream-health-read-only",
		wrapper.ParseConfig(func(value gjson.Result, config *hostConfig) error {
			client := wrapper.NewRedisClusterClient(wrapper.FQDNCluster{FQDN: "redis.static", Port: 6379})
			if err := client.Init("", "", 1000); err != nil {
				return err
			}
			var err error
			reader, err = NewReader(client, fixtureConfig())
			if err != nil {
				return err
			}
			reader.clock = func() time.Time { return fixtureNow }
			return nil
		}),
		wrapper.ProcessRequestHeaders(func(ctx wrapper.HttpContext, config hostConfig) types.Action {
			reader.Lookup(func(current ReadResult) { result = current })
			return types.HeaderStopAllIterationAndWatermark
		}),
	)
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
	t.Cleanup(reset)
	// Match the production host's logging capability before plugin startup. One
	// emulator belongs to this test and releases the SDK global host on cleanup.
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
		t.Fatal("health host did not start")
	}
	id := host.InitializeHttpContext()
	if action := host.CallOnRequestHeaders(id, [][2]string{{":method", "POST"}, {":path", "/customer/alias"}, {":authority", "gateway.example"}}, false); action != types.HeaderStopAllIterationAndWatermark {
		t.Fatalf("headers action: %v", action)
	}
	queries := host.GetRedisCalloutAttributesFromContext(id)
	if len(queries) != 1 || queries[0].Upstream != "outbound|6379||redis.static" {
		t.Fatalf("read transport: %#v", queries)
	}
	command, _, err := resp.NewReader(bytes.NewReader(queries[0].Query)).ReadValue()
	if err != nil || command.Type() != resp.Array {
		t.Fatalf("invalid HMGET transport: %v", err)
	}
	parts := command.Array()
	if len(parts) != 10 || !strings.EqualFold(parts[0].String(), "HMGET") || parts[1].String() != reader.SnapshotKey() {
		t.Fatalf("unexpected command: %s", queries[0].Query)
	}
	for index, field := range HashFields() {
		if parts[index+2].String() != field {
			t.Fatalf("projection field %d changed", index)
		}
	}
	response := redisHash(t, hashFixture(t, fixtureWire()))
	var encoded bytes.Buffer
	if err := resp.NewWriter(&encoded).WriteArray(response.Array()); err != nil {
		t.Fatal(err)
	}
	host.CallOnRedisCallResponse(queries[0].CalloutID, 0, encoded.Bytes())
	if result.Status != ReadReady || result.DecideNewCall(Candidate{Target: fixtureTarget()}, fixtureNow).Kind != Eligible {
		t.Fatalf("native host result: %#v", result)
	}
	if len(host.GetRedisCalloutAttributesFromContext(id)) != 0 || len(host.GetCalloutAttributesFromContext(id)) != 0 || host.GetSentLocalResponse(id) != nil || host.GetCurrentHttpStreamAction(id) == types.ActionContinue {
		t.Fatal("health read wrote state, dispatched/resumed business traffic or elected a response")
	}
}
