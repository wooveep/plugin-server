package main

import (
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

func TestHealthTransportUsesRegisteredServiceAndOriginalStartupOnly(t *testing.T) {
	type healthHostConfig struct{}
	var client *upstreamhealth.GatewayClient
	var result upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]
	endpoint := healthGatewayEndpoint{ServiceName: "console.dns", ServicePort: 8080, Host: "console.internal", AuthToken: "controlled-token", TimeoutMS: 1000}
	owner := "original-pod/original-boot"
	status := uint16(429)
	request := upstreamhealth.OutcomeRequest{EventID: "original-response", ObservationPhase: "upstream_response",
		Selection: upstreamhealth.AttemptSelection{RuntimeConfigID: "runtime-1", ConfigRevision: "9007199254740993", TargetID: "target-1", CredentialRef: "upstream-key", CredentialGeneration: "1",
			OperationKind: "text_generate", CallID: "call-1", AttemptID: "attempt-1", SelectionDigest: "sha256:" + strings.Repeat("a", 64)},
		AffectedScopeIDs: []string{"scope:model"}, ObservedScopeRevisions: map[string]upstreamhealth.Revision{"scope:model": "4"},
		OutcomeEvidenceRef: "original-history/attempt-1", OccurredAt: "2026-10-29T00:00:01Z", HTTPStatus: &status, Effect: upstreamhealth.HealthEffect{Kind: "rate_limited"}}
	vm := wrapper.NewCommonVmCtx("health-transport",
		wrapper.ParseConfig(func(gjson.Result, *healthHostConfig) error {
			var err error
			client, err = newHealthGatewayClient(endpoint, owner)
			return err
		}),
		wrapper.ProcessRequestHeaders(func(wrapper.HttpContext, healthHostConfig) types.Action {
			if err := client.ReportOutcome(request, func(value upstreamhealth.GatewayResult[upstreamhealth.OutcomeReceipt]) { result = value }); err != nil {
				t.Error(err)
			}
			return types.HeaderStopAllIterationAndWatermark
		}),
	)
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm).WithPluginConfiguration([]byte(`{}`)))
	t.Cleanup(reset)
	host.RegisterForeignFunction("get_log_level", func([]byte) []byte { return []byte{0, 0, 0, 0} })
	if host.StartVM() != types.OnVMStartStatusOK || host.StartPlugin() != types.OnPluginStartStatusOK {
		t.Fatal("health host did not start")
	}
	id := host.InitializeHttpContext()
	host.CallOnRequestHeaders(id, [][2]string{{":method", "POST"}, {":path", "/model/alias"}, {":authority", "gateway.example"},
		{"authorization", "Bearer client-secret"}, {"x-modelfusion-gateway-instance", "forged-startup"}}, false)
	calls := host.GetCalloutAttributesFromContext(id)
	if len(calls) != 1 || calls[0].Upstream != "outbound|8080||console.dns" {
		t.Fatalf("unexpected health transport: %#v", calls)
	}
	values := map[string]string{}
	for _, header := range calls[0].Headers {
		values[header[0]] = header[1]
	}
	if values[":path"] != upstreamhealth.OutcomePath || values[":authority"] != endpoint.Host || values["authorization"] != "Bearer controlled-token" || values["x-modelfusion-gateway-instance"] != owner {
		t.Fatalf("health transport used client identity: %#v", values)
	}
	body := []byte(`{"event_id":"original-response","revision":"9007199254740993","decision":"applied","scopes":[],"received_at":"2026-10-29T00:00:02Z"}`)
	host.CallOnHttpCallResponse(calls[0].CalloutID, [][2]string{{":status", "200"}}, nil, body)
	if result.State != upstreamhealth.GatewayAcknowledged || result.Value.Revision != "9007199254740993" {
		t.Fatalf("health acknowledgement: %#v", result)
	}
	if host.GetCurrentHttpStreamAction(id) == types.ActionContinue || host.GetSentLocalResponse(id) != nil || len(host.GetRedisCalloutAttributesFromContext(id)) != 0 {
		t.Fatal("health callback resumed customer traffic, wrote Redis or elected a response")
	}
}

func TestHealthTransportRejectsUnregisteredEndpointAndStartup(t *testing.T) {
	endpoint := healthGatewayEndpoint{ServiceName: "console.dns", ServicePort: 8080, Host: "console.internal", AuthToken: "controlled-token", TimeoutMS: 1000}
	for _, change := range []func(*healthGatewayEndpoint){func(e *healthGatewayEndpoint) { e.ServiceName = "client.example/path" }, func(e *healthGatewayEndpoint) { e.ServicePort = 0 },
		func(e *healthGatewayEndpoint) { e.Host = "client.example/path" }, func(e *healthGatewayEndpoint) { e.AuthToken = "token\r\ninjected" }} {
		bad := endpoint
		change(&bad)
		if _, err := newHealthGatewayClient(bad, "original-pod/original-boot"); err == nil {
			t.Fatal("invalid registered endpoint was accepted")
		}
	}
	if _, err := newHealthGatewayClient(endpoint, ""); err == nil {
		t.Fatal("missing real startup was accepted")
	}
}
