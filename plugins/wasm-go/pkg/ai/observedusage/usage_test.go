package observedusage

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

func TestDeliveryMeterSurvivesNullPublicUsageAndKeepsExactIntegers(t *testing.T) {
	_, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	defer reset()
	if _, ok := Read(); ok {
		t.Fatal("HTTP response alone created trusted metering")
	}
	e := executionfacts.Envelope{Kind: "call_delivery", CallID: "original-call", PublicModel: "sales-model", Usage: &executionfacts.Usage{Status: "reported", Source: "provider", Unit: "token", Quantities: map[string]string{"input": "9007199254740993", "output": "2", "cache_hit": "1", "cache_miss": "9007199254740992"}}}
	if err := Publish(e); err != nil {
		t.Fatal(err)
	}
	u, ok := Read()
	if !ok || u.Model != "sales-model" || u.InputToken != 9007199254740993 || u.TotalToken != 9007199254740995 || e.Usage.Quantities["input"] != "9007199254740993" {
		t.Fatal("original delivery metering changed", u)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(); ok {
		t.Fatal("next native request reused a previous measurement")
	}
}

func TestMissingAndInvalidMetersNeverBecomeKnownZero(t *testing.T) {
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption())
	defer reset()
	for _, raw := range []string{`{"version":"1","call_id":"x","model":"s","source":"provider","input":"-1","output":"2"}`, `{"version":"1","call_id":"x","model":"s","source":"provider","input":"01","output":"2"}`, `{"version":"1","call_id":"x","model":"s","source":"provider","input":"9223372036854775807","output":"1"}`, `{"version":"1","version":"1"}`} {
		_ = host.SetProperty([]string{Property}, []byte(raw))
		if _, ok := Read(); ok {
			t.Fatal("invalid metering became an observation")
		}
	}
	if err := Publish(executionfacts.Envelope{Usage: executionfacts.MissingTokenUsage("not_observed")}); err != nil {
		t.Fatal(err)
	}
	if _, ok := Read(); ok {
		t.Fatal("missing usage became zero")
	}
}
