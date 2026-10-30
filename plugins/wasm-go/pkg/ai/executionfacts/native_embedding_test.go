package executionfacts

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func embeddingFactFixture() Envelope {
	original := attemptFixture()
	original.SchemaVersion = NativeEmbeddingSchemaVersion
	original.OperationKind = NativeEmbeddingOperation
	original.PrincipalKind, original.BillingMode, original.PrincipalID = "service", "internal_cost", "service/ai-memory/original-user"
	original.CostOwner = &CostOwner{Kind: "tenant", OwnerID: "tenant-1", SourceJobID: "original-job-1"}
	original.AccountID, original.SalesAssetID = "", ""
	original.Usage = &Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "9007199254740993"}}
	return original
}

func TestNativeEmbeddingCodecKeepsInputOnlyOriginalServiceEvidence(t *testing.T) {
	original := embeddingFactFixture()
	if _, err := Freeze(original); err == nil {
		t.Fatal("original v1 codec accepted native:v2")
	}
	attempt, err := NewEmbeddingAttempt(original)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := attempt.Finish(AttemptOutcome{ObservedAt: original.ObservedAt, SendState: "observed_sent", Acceptance: "accepted", ExecutionResult: "succeeded", HTTPStatus: statusFact(200), HeadersComplete: true, BodyComplete: true, ResponseBodyDigest: "sha256:" + strings.Repeat("c", 64), EvidenceRefs: []string{"original-http-response"}, Usage: original.Usage})
	if err != nil {
		t.Fatal(err)
	}
	original.Usage.Quantities["input"] = "1"
	fact := snapshot.Envelope()
	if fact.SchemaVersion != NativeEmbeddingSchemaVersion || fact.Usage.Quantities["input"] != "9007199254740993" || len(fact.Usage.Quantities) != 1 || fact.AccountID != "" || fact.SalesAssetID != "" || fact.CostOwner.SourceJobID != "original-job-1" {
		t.Fatal("native:v2 lost original job/input evidence")
	}
	if _, err := Correct(snapshot, fact); err == nil {
		t.Fatal("v1 correction changed native:v2 schema")
	}
	fact.ObservedAt = "2026-10-27T10:00:02Z"
	corrected, err := CorrectEmbedding(snapshot, fact)
	if err != nil || corrected.Revision() != "2" || corrected.SourceFactID() != snapshot.SourceFactID() || corrected.Envelope().SupersedesEventID != snapshot.EventID() {
		t.Fatal("embedding correction lost its predecessor", err)
	}
}

func TestNativeEmbeddingCodecRejectsSaleMediaAndInventedUsage(t *testing.T) {
	for name, mutate := range map[string]func(*Envelope){
		"customer":  func(e *Envelope) { e.BillingMode = "customer" },
		"wallet":    func(e *Envelope) { e.AccountID = "wallet-1" },
		"sale":      func(e *Envelope) { e.SalesAssetID = "sale-1" },
		"task":      func(e *Envelope) { e.ProviderTaskID = "task-1" },
		"operation": func(e *Envelope) { e.OperationID = "operation-1" },
		"parent":    func(e *Envelope) { e.ParentAttemptID = "parent-1" },
		"retry":     func(e *Envelope) { e.Mechanism = "credential_retry" },
		"output":    func(e *Envelope) { e.Usage.Quantities["output"] = "0" },
		"cache":     func(e *Envelope) { e.Usage.Quantities = map[string]string{"cache_hit": "0"} },
		"estimated": func(e *Envelope) {
			e.Usage.Status = "estimated"
			e.Usage.Source = "estimator"
			e.Usage.EstimatorVersion = "invented"
		},
		"missing_zero": func(e *Envelope) { e.Usage.Status = "missing"; e.Usage.Source = "none"; e.Usage.ReasonCode = "missing" },
	} {
		t.Run(name, func(t *testing.T) {
			original := embeddingFactFixture()
			mutate(&original)
			if _, err := FreezeEmbedding(original); err == nil {
				t.Fatal("invalid native:v2 evidence accepted")
			}
		})
	}
	original := embeddingFactFixture()
	original.Usage = MissingTokenUsage("provider_usage_missing")
	snapshot, err := FreezeEmbedding(original)
	if err != nil || bytes.Contains(snapshot.Bytes(), []byte(`"quantities"`)) {
		t.Fatal("missing input invented a quantity", err)
	}
	original.Usage = &Usage{Status: "reported", Unit: "token", Source: "provider", Quantities: map[string]string{"input": "0"}}
	if _, err := FreezeEmbedding(original); err != nil {
		t.Fatal("actual provider zero rejected", err)
	}
}

func embeddingSelectionFixture(t *testing.T) []byte {
	t.Helper()
	var fields map[string]interface{}
	if err := json.Unmarshal(registrationFixture(t).Selection, &fields); err != nil {
		t.Fatal(err)
	}
	fields["operation_kind"] = NativeEmbeddingOperation
	delete(fields, "selection_digest")
	raw, _ := json.Marshal(fields)
	digest, err := EmbeddingSelectionDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	fields["selection_digest"] = digest
	raw, _ = json.Marshal(fields)
	return raw
}

func TestNativeEmbeddingSelectionAndAccountlessRegistrationAreVersioned(t *testing.T) {
	for _, embedding := range []bool{false, true} {
		t.Run(map[bool]string{false: "text_v1_selection", true: "embedding_v2_selection"}[embedding], func(t *testing.T) {
			base := registrationFixture(t)
			if embedding {
				base.Selection = embeddingSelectionFixture(t)
				if _, err := SelectionDigest(base.Selection); err == nil {
					t.Fatal("v1 selection codec accepted embedding")
				}
			}
			input := InternalRegistration{ProducerID: base.ProducerID, CallID: base.CallID, AttemptID: base.AttemptID, ReleaseID: base.ReleaseID, RouteID: base.RouteID, TargetID: base.TargetID, APIKeyID: base.APIKeyID, CredentialGeneration: base.CredentialGeneration, BusinessOccurredAt: base.BusinessOccurredAt, Selection: base.Selection}
			snapshot, err := FreezeInternalRegistration(input)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(snapshot.Bytes(), &fields) != nil || len(fields) != 12 || fields["account_id"] != nil || string(fields["schema_version"]) != `"execution-registration.v2"` {
				t.Fatal("internal carrier is not exact accountless v2")
			}
			again, err := FreezeInternalRegistration(input)
			if err != nil || !bytes.Equal(snapshot.Bytes(), again.Bytes()) {
				t.Fatal("original registration bytes changed")
			}
			base.AccountID = ""
			if _, err := FreezeRegistration(base); err == nil {
				t.Fatal("original v1 accepted an accountless registration")
			}
			input.SchemaVersion = "execution-registration.v1"
			if _, err := FreezeInternalRegistration(input); err == nil {
				t.Fatal("internal carrier used v1")
			}
		})
	}
	for _, field := range []string{"parent_attempt_id", "operation_id", "dispatch_id"} {
		t.Run(field, func(t *testing.T) {
			var fields map[string]interface{}
			_ = json.Unmarshal(embeddingSelectionFixture(t), &fields)
			fields[field] = "foreign"
			raw, _ := json.Marshal(fields)
			if _, err := EmbeddingSelectionDigest(raw); err == nil {
				t.Fatal("native:v2 accepted a non-embedding identity")
			}
		})
	}
}

func TestNativeEmbeddingDeliveryBridgePreservesBytesAndRejectsV1Decoder(t *testing.T) {
	original := embeddingFactFixture()
	original.Kind = "call_delivery"
	original.AttemptID, original.TargetID, original.BindingID, original.ProviderID, original.ProviderAccountID, original.CredentialRef, original.CredentialRevision, original.SelectionDigest, original.UpstreamModel = "", "", "", "", "", "", "", "", ""
	original.SendTransport, original.RecordingOwner, original.Mechanism, original.SendState, original.Acceptance, original.ExecutionResult = "", "", "", "", "", ""
	original.HeadersComplete, original.BodyComplete, original.HTTPStatus = nil, nil, nil
	original.OutboundBodyDigest = ""
	original.DeliveryState, original.ServedFrom, original.PublicModel = "succeeded", "upstream", "public-model"
	snapshot, err := FreezeEmbedding(original)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := EncodeEmbeddingDeliveryBridge(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEmbeddingDeliveryBridge(bridge)
	if err != nil || !bytes.Equal(decoded.Bytes(), snapshot.Bytes()) {
		t.Fatal("embedding bridge changed bytes", err)
	}
	if _, err := DecodeDeliveryBridge(bridge); err == nil {
		t.Fatal("v1 decoder accepted embedding")
	}
	if _, err := EncodeDeliveryBridge(snapshot); err == nil {
		t.Fatal("v1 encoder accepted embedding")
	}
	if _, err := DecodeEmbeddingDeliveryBridge(bridge + "="); err == nil {
		t.Fatal("noncanonical bridge accepted")
	}
}
