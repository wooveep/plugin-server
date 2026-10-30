package upstreamhealth

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

func TestNativeEmbeddingRecoveryCandidateRetainsNewOperationAndOneFinalClaim(t *testing.T) {
	var fields map[string]interface{}
	_ = json.Unmarshal(gatewaySelection(t), &fields)
	fields["operation_kind"] = executionfacts.NativeEmbeddingOperation
	delete(fields, "selection_digest")
	raw, _ := json.Marshal(fields)
	digest, err := executionfacts.EmbeddingSelectionDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	fields["selection_digest"] = digest
	raw, _ = json.Marshal(fields)
	candidate, err := NewRecoveryCandidate(raw, "runtime-1", "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	final, err := candidate.FinalSelection(gatewayClaimID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executionfacts.CanonicalEmbeddingSelection(final); err != nil {
		t.Fatal(err)
	}
	if _, err := executionfacts.CanonicalSelection(final); err == nil {
		t.Fatal("new final selection passed the original v1 codec")
	}
	var frozen map[string]interface{}
	_ = json.Unmarshal(final, &frozen)
	if frozen["operation_kind"] != executionfacts.NativeEmbeddingOperation || frozen["parent_attempt_id"] != nil || frozen["task_id"] != nil || len(frozen["recovery_claim_refs"].([]interface{})) != 1 {
		t.Fatal("candidate changed original operation or fabricated another identity")
	}
}

func TestNativeEmbeddingConsumeRequiresExactCommittedV2Selection(t *testing.T) {
	var fields map[string]interface{}
	_ = json.Unmarshal(gatewaySelection(t), &fields)
	fields["operation_kind"] = executionfacts.NativeEmbeddingOperation
	delete(fields, "selection_digest")
	raw, _ := json.Marshal(fields)
	digest, err := executionfacts.EmbeddingSelectionDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	fields["selection_digest"] = digest
	raw, _ = json.Marshal(fields)
	candidate, err := NewRecoveryCandidate(raw, "runtime-1", "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := ClaimRequest{ClaimRequestID: "embedding-original-command", OwnerInstance: gatewayOwner, ExpectedEpochs: map[string]Revision{"scope:model": "3"}, Candidate: candidate, CandidateDigest: candidate.Digest()}
	allocation := gatewayAllocation(request)
	final, err := candidate.FinalSelection(allocation.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	recorder := &gatewayRecorder{}
	client := gatewayClientFixture(t, recorder)
	var result GatewayResult[RecoveryClaim]
	if err := client.Consume(allocation, final, func(value GatewayResult[RecoveryClaim]) { result = value }); err != nil {
		t.Fatal(err)
	}
	if len(recorder.calls) != 1 || result.State == GatewayAcknowledged {
		t.Fatal("allocated or dispatch acknowledgement fabricated a Consume commit")
	}
	allocation.Status, allocation.ConsumedSelection = "consumed", final
	recorder.calls[0].callback(gatewayReply(t, allocation))
	if result.State != GatewayAcknowledged || result.Value == nil {
		t.Fatal("exact committed native:v2 Consume rejected", result.Reason)
	}
}
