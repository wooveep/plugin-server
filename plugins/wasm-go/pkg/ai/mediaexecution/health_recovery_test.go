package mediaexecution_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/ai/upstreamhealth"
	"github.com/tidwall/resp"
)

// Exercise the actual media selection producer through recovery and the original
// Redis permission receiver. Relabeling a text fixture misses this boundary.
func TestMediaRecoveryPreservesOriginalCreateAndTaskQuery(t *testing.T) {
	for _, profile := range []string{mediaexecution.QwenImage20ProSync, mediaexecution.QwenImage30NativeSync, mediaexecution.QwenImage30Compatible, mediaexecution.Profile, mediaexecution.Wan30Text, mediaexecution.Wan30FirstFrame} {
		protocol, err := mediaexecution.LookupProfile(profile)
		if err != nil {
			t.Fatal(err)
		}
		modes := []string{"create"}
		if protocol.Asynchronous {
			modes = append(modes, "query")
		}
		for _, mode := range modes {
			t.Run(profile+"/"+mode, func(t *testing.T) {
				f := profileFixture(t, profile)
				var task *string
				queryAttempt := ""
				if mode == "query" {
					originalTask := "task-original"
					task, queryAttempt = &originalTask, "original-query-after-429"
					history := f.sealed(f.selection(t, false), originalTask)
					history["attempt"].(map[string]any)["procurement"].(map[string]any)["upstream_model_name"] = f.descriptor["upstream_model"]
					f.archive["dispatch"], f.archive["history"] = f.dispatch("consumed"), history
				}
				executor, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
				if err != nil {
					t.Fatal(err)
				}
				selection, err := executor.Selection(f.live, queryAttempt, nil)
				if err != nil {
					t.Fatal(err)
				}
				candidate, err := upstreamhealth.NewRecoveryCandidate(selection, executor.RuntimeConfigID(), upstreamhealth.Revision(executor.ConfigRevision()), task)
				if err != nil {
					t.Fatal("original media selection cannot enter recovery:", err)
				}
				final, err := candidate.FinalSelection("20000000-0000-0000-0000-000000000001")
				if err != nil {
					t.Fatal(err)
				}
				var before, after map[string]json.RawMessage
				if json.Unmarshal(selection, &before) != nil || json.Unmarshal(final, &after) != nil {
					t.Fatal("selection is not an object")
				}
				delete(before, "selection_digest")
				delete(after, "selection_digest")
				if string(after["recovery_claim_refs"]) != `["20000000-0000-0000-0000-000000000001"]` {
					t.Fatal("recovery did not retain its sole claim")
				}
				delete(after, "recovery_claim_refs")
				if !bytes.Equal(raw(t, before), raw(t, after)) {
					t.Fatal("recovery changed original media semantics")
				}
				if _, err := executionfacts.CanonicalSelection(final); err == nil {
					t.Fatal("media entered the frozen text-only registration")
				}
				reply := resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("123-0")})
				if mode == "query" {
					request, err := executor.PrepareRedisQuery(reply, final, f.now.Add(time.Second), f.now)
					if err != nil || !request.IsQuery() || request.Path() != "/api/v1/tasks/task-original" || request.AttemptID() != queryAttempt {
						t.Fatal("recovery changed or rejected the original query:", err)
					}
					if _, err := executor.PrepareRedisQuery(reply, final, f.now.Add(time.Hour), f.now); err == nil {
						t.Fatal("recovery replay authorized another query")
					}
				} else {
					request, err := executor.PrepareRedisCreate(reply, final, f.live, f.now)
					if err != nil || request.IsQuery() || request.CreateAttemptID() != "attempt-create" {
						t.Fatal("recovery changed or rejected the original create:", err)
					}
				}
			})
		}
	}
}

func TestMediaRecoveryRejectsMixedCreateAndQueryIdentity(t *testing.T) {
	f := profileFixture(t, mediaexecution.Profile)
	executor, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name, query, field, value string
	}{
		{"missing operation", "", "operation_id", ""},
		{"missing dispatch", "", "dispatch_id", ""},
		{"missing invocation", "", "invocation_id", ""},
		{"creation retry", "", "mechanism", "native_retry"},
		{"query with dispatch", "original-query", "dispatch_id", "original-dispatch"},
		{"query as creation", "original-query", "mechanism", "router"},
		{"other sender", "", "recording_owner", "router_observer"},
		{"undeclared field", "", "extra_selection", "extra"},
	} {
		t.Run(check.name, func(t *testing.T) {
			selection, err := executor.Selection(f.live, check.query, nil)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(selection, &fields); err != nil {
				t.Fatal(err)
			}
			delete(fields, "selection_digest")
			if check.value == "" {
				delete(fields, check.field)
			} else {
				fields[check.field] = raw(t, check.value)
			}
			if _, err := upstreamhealth.NewRecoveryCandidate(raw(t, fields), executor.RuntimeConfigID(), upstreamhealth.Revision(executor.ConfigRevision()), nil); err == nil {
				t.Fatal("recovery accepted a selection outside the original media contract")
			}
		})
	}
}
