package executionfixture

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
)

func TestD0SnapshotCopiesPrivateInputs(t *testing.T) {
	source := Snapshot{ID: "snapshot-A", Call: "call-A", Attempt: "attempt-A", Context: 37, Generation: 7, Selection: map[string]string{"binding": "A"}, Body: []byte("original"), Metadata: map[string][]string{"tags": {"A"}}}
	got := CaptureSnapshot(source)
	source.Selection["binding"] = "B"
	source.Body[0] = 'X'
	source.Metadata["tags"][0] = "B"
	if got.Selection["binding"] != "A" || string(got.Body) != "original" || got.Metadata["tags"][0] != "A" {
		t.Fatalf("private send snapshot changed: %+v", got)
	}
}

func decodeSnapshot(t *testing.T, id string, raw json.RawMessage) Snapshot {
	t.Helper()
	var value map[string]any
	if err := decode(raw, &value); err != nil {
		t.Fatal(err)
	}
	selection := map[string]string{}
	for key, v := range mapping(value["selection"]) {
		selection[key] = str(v)
	}
	metadata := map[string][]string{}
	for key, v := range mapping(value["mutable_metadata"]) {
		if list, ok := v.([]any); ok {
			for _, item := range list {
				metadata[key] = append(metadata[key], str(item))
			}
		} else {
			metadata[key] = []string{str(v)}
		}
	}
	return Snapshot{ID: id, Call: str(value["call_label"]), Attempt: str(value["attempt_label"]), Context: number(value["context_number"]), Generation: number(value["context_generation"]), Selection: selection, Body: []byte(str(value["body_utf8"])), Outbound: []byte(str(value["outbound_utf8"])), Metadata: metadata}
}
func TestD0ScheduleCorpus(t *testing.T) {
	doc, err := Load(FixturePath("callback-schedules.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 15 {
		t.Fatal("callback catalog shrank")
	}
	for _, item := range doc.Cases {
		t.Run(item.ID, func(t *testing.T) {
			host, context, observed := newCallbackHost(t)
			adapter := NewHostAdapter(host, context)
			sources := map[string]Snapshot{}
			for _, raw := range values(item.Input["pending"]) {
				pending := mapping(raw)
				id := number(pending["synthetic_callout_id"])
				ref := str(pending["snapshot_ref"])
				source := decodeSnapshot(t, ref, doc.Snapshots[ref])
				sources[ref] = source
				upstream := "fixture-A"
				if id == 202 {
					upstream = "fixture-B"
				}
				if err := adapter.Bind(id, upstream, source); err != nil {
					t.Fatal(err)
				}
			}
			var actual error
			t.Logf("subject_layer=host_emulator captured_CalloutID_bindings=%v", adapter.CapturedIDs())
			parent := mapping(item.Input["parent_stream_info"])
			for _, raw := range values(item.Input["events"]) {
				event := mapping(raw)
				t.Logf("subject_layer=contract_fixture explicit_action=%v", event)
				switch str(event["action"]) {
				case "barrier":
					if !reflect.DeepEqual(adapter.Pending(), []uint32{101, 202}) {
						t.Fatal("both-pending barrier not met")
					}
				case "close_context":
					adapter.Close(number(event["context_number"]))
				case "open_context":
					adapter.Open(number(event["context_number"]), number(event["context_generation"]))
				case "mutate_source":
					ref := str(event["snapshot_ref"])
					source := sources[ref]
					if str(event["path"]) == "/selection/release_label" {
						source.Selection["release_label"] = str(event["value"])
					} else {
						source.Metadata["tags"][0] = str(event["value"])
					}
				case "mutate_source_bytes":
					source := sources[str(event["snapshot_ref"])]
					source.Body[number(event["offset"])] = byte(number(event["byte"]))
				case "overwrite_parent_stream_info":
					parent = event
				case "callback":
					var response struct {
						Headers  [][2]string `json:"headers"`
						Body     *string     `json:"body_utf8"`
						Complete bool        `json:"body_complete"`
						Read     string      `json:"body_read"`
					}
					if err := decode(doc.Responses[str(event["response_ref"])], &response); err != nil {
						t.Fatal(err)
					}
					var body []byte
					if response.Body != nil {
						body = []byte(*response.Body)
					}
					actual = adapter.Release(number(event["synthetic_callout_id"]), str(event["asserted_snapshot_ref"]), Response{Headers: response.Headers, Body: body, Complete: response.Complete, Read: response.Read})
				default:
					t.Fatalf("unknown event %v", event)
				}
				if actual != nil {
					break
				}
			}
			code := ""
			if actual != nil {
				code = actual.Error()
			}
			if code != str(item.Expected["error_code"]) {
				t.Fatalf("code=%q, want=%v", code, item.Expected["error_code"])
			}
			records := adapter.Records()
			if len(records) != int(number(item.Expected["record_count"])) {
				t.Fatalf("records=%d, want=%v", len(records), item.Expected["record_count"])
			}
			if want, exists := item.Expected["pending_remaining"]; exists {
				var ids []uint32
				for _, v := range values(want) {
					ids = append(ids, number(v))
				}
				if !reflect.DeepEqual(ids, adapter.Pending()) {
					t.Fatalf("pending=%v, want=%v", adapter.Pending(), ids)
				}
			}
			if want, exists := item.Expected["callback_order"]; exists {
				if len(records) != len(values(want)) {
					t.Fatal("callback order size")
				}
				for i, v := range values(want) {
					if records[i].SyntheticID != number(v) {
						t.Fatal("wrong callback attribution")
					}
				}
			}
			for _, record := range records {
				key := fmt.Sprint(record.SyntheticID)
				if want := mapping(item.Expected["records_by_callout"]); want != nil && record.Snapshot.ID != str(want[key]) {
					t.Fatal("private snapshot was reattributed")
				}
				if want := mapping(item.Expected["attempts_by_callout"]); want != nil && record.Snapshot.Attempt != str(want[key]) {
					t.Fatal("attempt reconstructed from mutable parent")
				}
				if status, exists := item.Expected["http_status"]; exists {
					if status == nil && record.Status != nil {
						t.Fatal("missing status replaced by synthetic value")
					}
					if status != nil && (record.Status == nil || *record.Status != int(number(status))) {
						t.Fatal("HTTP status changed")
					}
				}
				if complete, exists := item.Expected["body_complete"]; exists && record.Response.Complete != complete {
					t.Fatal("body completeness changed")
				}
				if record.SyntheticID == 101 {
					if want := mapping(item.Expected["snapshot_A_selection"]); want != nil {
						for k, v := range want {
							if record.Snapshot.Selection[k] != str(v) {
								t.Fatal("selection map was not private")
							}
						}
					}
					if item.Expected["snapshot_A_metadata"] != nil && record.Snapshot.Metadata["tags"][0] != "A" {
						t.Fatal("nested metadata was not private")
					}
					if want := str(item.Expected["snapshot_A_body_raw_sha256"]); want != "" && Hash(record.Snapshot.Body) != want {
						t.Fatal("body slice was not private")
					}
				}
			}
			if truth(item.Expected["parent_stream_info_must_not_reconstruct_private_records"]) && str(parent["latest_attempt_label"]) != "attempt-retry-B" {
				t.Fatal("parent mutation did not occur")
			}
			t.Logf("evidence_level=offline_fixture records=%d real_callbacks=%v model_code=%q real_gateway=not_run", len(records), *observed, code)
		})
	}
}

func TestD0CalloutMissingCapabilityIsBlocked(t *testing.T) {
	if err := NewHostAdapter(nil, 0).Bind(101, "fixture-A", Snapshot{}); err == nil || err.Error() != "WG_FIXTURE_HOST_CAPABILITY_BLOCKED" {
		t.Fatalf("missing SDK capability passed: %v", err)
	}
}
func TestD0CalloutEmulatorCapabilityProbe(t *testing.T) {
	host, context, order := newCallbackHost(t)
	attrs := host.GetCalloutAttributesFromContext(context)
	if len(attrs) != 2 {
		t.Fatal("two callouts not observed")
	}
	var id, otherID uint32
	foundA, foundB := false, false
	for _, item := range attrs {
		if item.Upstream == "fixture-A" {
			id = item.CalloutID
			foundA = true
		} else if item.Upstream == "fixture-B" {
			otherID = item.CalloutID
			foundB = true
		}
	}
	if !foundA || !foundB || id == otherID {
		t.Fatal("explicit upstream ID capture failed")
	}
	host.CallOnHttpCallResponse(id, [][2]string{{":status", "200"}}, nil, nil)
	probe := func(label string, id uint32) {
		panicked := false
		before := len(*order)
		func() {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			host.CallOnHttpCallResponse(id, nil, nil, nil)
		}()
		t.Logf("subject_layer=host_emulator %s: unsupported_in_emulator=%v panic=%v fresh_callback=%v", label, len(*order) == before, panicked, len(*order) > before)
	}
	probe("duplicate_callback", id)
	host.CompleteHttpContext(context)
	probe("closed_context_callback", otherID)
}

type callbackVM struct {
	types.DefaultVMContext
	order *[]string
}
type callbackPlugin struct {
	types.DefaultPluginContext
	order *[]string
}
type callbackHTTP struct {
	types.DefaultHttpContext
	order *[]string
}

func (v *callbackVM) NewPluginContext(uint32) types.PluginContext {
	return &callbackPlugin{order: v.order}
}
func (p *callbackPlugin) NewHttpContext(uint32) types.HttpContext {
	return &callbackHTTP{order: p.order}
}
func (h *callbackHTTP) OnHttpRequestHeaders(int, bool) types.Action {
	for _, label := range []string{"A", "B"} {
		label := label
		_, err := proxywasm.DispatchHttpCall("fixture-"+label, [][2]string{{":method", "POST"}, {":path", "/"}, {":authority", "fixture.invalid"}}, []byte(label), nil, 1000, func(int, int, int) { *h.order = append(*h.order, label) })
		if err != nil {
			panic(err)
		}
	}
	return types.ActionPause
}
func newCallbackHost(t *testing.T) (proxytest.HostEmulator, uint32, *[]string) {
	t.Helper()
	order := []string{}
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(&callbackVM{order: &order}))
	t.Cleanup(reset)
	if host.StartPlugin() != types.OnPluginStartStatusOK {
		t.Fatal("host bootstrap failed")
	}
	id := host.InitializeHttpContext()
	host.CallOnRequestHeaders(id, [][2]string{{":method", "POST"}, {":authority", "fixture.invalid"}}, false)
	return host, id, &order
}
func TestD0CalloutExplicitIDForwardAndReverse(t *testing.T) {
	for _, order := range [][]uint32{{101, 202}, {202, 101}} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			host, context, observed := newCallbackHost(t)
			adapter := NewHostAdapter(host, context)
			for _, item := range []struct {
				id    uint32
				label string
			}{{101, "A"}, {202, "B"}} {
				if err := adapter.Bind(item.id, "fixture-"+item.label, Snapshot{ID: "snapshot-" + item.label, Context: 37, Generation: 7}); err != nil {
					t.Fatal(err)
				}
			}
			if len(adapter.Pending()) != 2 {
				t.Fatal("both-pending barrier was not reached")
			}
			for _, id := range order {
				if err := adapter.Release(id, "", Response{Headers: [][2]string{{":status", "200"}}, Complete: true, Read: "ok"}); err != nil {
					t.Fatal(err)
				}
			}
			want := []string{"A", "B"}
			if order[0] == 202 {
				want = []string{"B", "A"}
			}
			if !reflect.DeepEqual(*observed, want) {
				t.Fatalf("real SDK callback order %v, want %v", *observed, want)
			}
			if len(adapter.Records()) != 2 || adapter.Records()[0].SyntheticID != order[0] {
				t.Fatal("callback was attributed to wrong private snapshot")
			}
		})
	}
}
