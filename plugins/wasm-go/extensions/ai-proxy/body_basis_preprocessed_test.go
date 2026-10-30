package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/test"
)

func TestFirstBodyProxyKeepsOriginalAfterBothProcessorsAndMapper(t *testing.T) {
	for _, tc := range append(test.BodyBasisContracts(), test.LargeHistoryBodyBasisContracts()...) {
		name := tc.ClientProtocol
		if tc.LargeHistory {
			name += "/large-history"
		}
		t.Run(name, func(t *testing.T) {
			fixture := managedConfigFixture()
			settings := fixture["managed_execution"].(managedSettings)
			settings.InputProtocol, settings.UpstreamProtocol = tc.ClientProtocol, tc.ClientProtocol
			fixture["managed_execution"] = settings
			h := newManagedHost(t, fixture)
			original := []byte(tc.Original)
			mapped := []byte(strings.Replace(tc.AfterMemory, `"model":"public-model"`, `"model":"upstream-model"`, 1))
			h.start(t, original, mapped)
			calls := h.host.GetCalloutAttributesFromContext(h.id)
			if len(calls) != 1 || string(calls[0].Body) != string(mapped) {
				t.Fatal("native protocol must send the exact body handed off by mapper once")
			}
			for _, header := range calls[0].Headers {
				if strings.EqualFold(header[0], bodybasis.DigestHeader) || strings.EqualFold(header[0], bodybasis.PublicModelHeader) {
					t.Fatal("private original-body metadata leaked to provider I/O")
				}
			}
			records := h.records(t)
			if len(records) != 1 || records[0].messageType != "execution_registration" {
				t.Fatal("the actual single send needs its independent original registration")
			}
			var registration executionfacts.Registration
			if err := json.Unmarshal(records[0].raw, &registration); err != nil {
				t.Fatal(err)
			}
			var selection map[string]string
			if err := json.Unmarshal(registration.Selection, &selection); err != nil {
				t.Fatal(err)
			}
			basis, err := bodybasis.Capture(original)
			if err != nil {
				t.Fatal(err)
			}
			if selection["original_body_digest"] != basis.Digest() {
				t.Fatal("registration replaced the original client body/model with preprocessed context")
			}
			state := h.ctx.GetContext(managedExecutionContextKey).(*managedExecutionState)
			if state.original.OutboundBodyDigest != managedBodyDigest(mapped) || state.original.UpstreamModel != "upstream-model" {
				t.Fatal("actual outgoing body digest must cover the final mapped native bytes")
			}
			if selection["original_body_digest"] == state.original.OutboundBodyDigest {
				t.Fatal("original and actual outgoing body identities were conflated")
			}
		})
	}
}
