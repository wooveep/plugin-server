package main

import "testing"

func TestManagedGenericGatewayErrorsDoNotClaimProviderSend(t *testing.T) {
	for _, test := range []struct {
		name, status, contentType, body string
		provider                        bool
	}{
		{"local-connect-failure", "503", "text/plain", "upstream connect error or disconnect/reset before headers. reset reason: remote connection failure", false},
		{"ambiguous-timeout", "504", "text/plain", "upstream request timeout", false},
		{"provider-error-without-usage", "503", "application/json", `{"error":{"message":"provider temporarily unavailable"}}`, true},
		{"provider-error-with-reported-usage", "503", "application/json", `{"error":{"message":"provider temporarily unavailable"},"usage":{"prompt_tokens":7,"completion_tokens":2}}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newManagedHost(t)
			h.start(t, []byte(managedOriginalBody), []byte(managedMappedBody))
			h.response(t, [][2]string{{":status", test.status}, {"content-type", test.contentType}}, []byte(test.body))
			observations := 0
			for _, record := range h.records(t) {
				if record.messageType != "observation" {
					continue
				}
				observations++
				outcome := decodeManagedFact(t, record.raw)
				if test.provider {
					if outcome.SendState != "observed_sent" || outcome.HTTPStatus == nil || *outcome.HTTPStatus != 503 || outcome.ExecutionResult != "failed" || outcome.TransportError != "" {
						t.Fatal("recognized provider error lost its actual response", outcome)
					}
				} else {
					if outcome.SendState != "handed_off" || outcome.Acceptance != "unknown" || outcome.ExecutionResult != "unknown" || outcome.HTTPStatus != nil || outcome.TransportError != "upstream_response_unverified" || outcome.Usage.Status != "estimated" || outcome.Usage.Source != "estimator" {
						t.Fatal("generic gateway response lost its honest transport state or dispatch-based input charge", outcome)
					}
					if got := h.host.GetSentLocalResponse(h.id); got == nil || got.StatusCode != 502 {
						t.Fatal("unknown transport did not return the explicit local gateway error")
					}
				}
			}
			if observations != 1 {
				t.Fatal("one dispatch must retain one original attempt")
			}
		})
	}
}
