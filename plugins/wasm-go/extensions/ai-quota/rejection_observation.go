package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// Only an authenticated key with a released route produces a business rejection.
// Unknown credentials remain ordinary gateway security/access events.
func observeAdmissionRejection(ctx wrapper.HttpContext, config QuotaConfig, identity balanceidentity.Snapshot, reason string) {
	admission, ok := config.ExecutionAdmission.(*executionAdmissionConfig)
	if !ok || admission == nil || identity.TenantID == "" || identity.PrincipalID == "" || ctx.GetContext("model-rejection-recorded") == true {
		return
	}
	ctx.SetContext("model-rejection-recorded", true)
	body, err := json.Marshal(struct {
		ID        string `json:"rejection_id"`
		RequestID string `json:"request_id"`
		APIKeyID  string `json:"api_key_id"`
		RouteID   string `json:"route_id"`
		Reason    string `json:"reason_code"`
	}{uuid.New().String(), wrapper.PreserveOriginalRequestID(ctx), identity.APIKeyID, admission.RouteID, reason})
	if err != nil {
		return
	}
	_, err = proxywasm.DispatchHttpCall(admission.cluster, [][2]string{{":method", "POST"}, {":path", "/internal/model-observations/rejection"}, {":authority", admission.Console.Host}, {"content-type", "application/json"}, {"authorization", "Bearer " + admission.Console.AuthToken}}, body, nil, admission.Console.TimeoutMS, func(int, int, int) {})
	if err != nil {
		log.Warn("model rejection observation dispatch failed")
	}
}
