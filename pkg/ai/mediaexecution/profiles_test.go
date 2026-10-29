package mediaexecution_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
)

func profileFixture(t *testing.T, profile string) fixture {
	t.Helper()
	f := original(t)
	model, operation, path, asynchronous := mediaexecution.Model, "image_generate", "/api/v1/services/aigc/multimodal-generation/generation", false
	switch profile {
	case mediaexecution.Profile:
		path, asynchronous = mediaexecution.CreatePath, true
	case mediaexecution.QwenImage20ProSync:
		model = "qwen-image-2.0-pro"
	case mediaexecution.QwenImage30Compatible:
		path = "/compatible-mode/v1/images/generations"
		f.body = []byte(`{"model":"public-image","prompt":"a tree","seed":9007199254740993,"n":2}`)
	case mediaexecution.Wan30Text, mediaexecution.Wan30FirstFrame:
		model, operation, path, asynchronous = "wan3.0-video-prime", "video_generate", "/api/v1/services/aigc/video-generation/video-synthesis", true
		f.body = []byte(`{"model":"public-image","input":{"prompt":"a moving tree"},"parameters":{"duration":8,"seed":9007199254740993}}`)
		if profile == mediaexecution.Wan30FirstFrame {
			f.body = []byte(`{"model":"public-image","input":{"prompt":"a moving tree"},"first_frame_url":"https://input.example/original.png?sig=original","parameters":{"duration":8,"seed":9007199254740993}}`)
		}
	}
	f.descriptor["upstream_model"] = model
	if !asynchronous {
		f.binding.QueryClassifierRef, f.binding.QueryCapabilityRef = "", ""
	}
	capability := map[string]any{"send_mode": "single_send", "max_attempts": 1, "max_fallback_hops": 0, "response_classifier_ref": f.binding.CreateClassifierRef}
	caps := map[string]any{operation: capability}
	if asynchronous {
		capability["query_capability_ref"] = f.binding.QueryCapabilityRef
		caps["task_query"] = capability
	}
	f.descriptor["capabilities"] = caps
	sign(t, f.descriptor, "descriptor_digest")
	f.candidate["upstream_model"], f.candidate["operation_kind"] = model, operation
	basis, err := bodybasis.Capture(f.body)
	if err != nil {
		t.Fatal(err)
	}
	f.candidate["original_body_digest"] = basis.Digest()
	f.history["operation_kind"] = operation
	f.archive["profile"], f.archive["request_body"] = profile, string(f.body)
	f.archive["native_execution"].(map[string]any)["upstream_path"] = path
	protocol, err := mediaexecution.LookupProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	f.binding.OutputIdentityContract = protocol.OutputIdentityContract
	f.archive["output_identity_contract"] = protocol.OutputIdentityContract
	f.history["delivery_writer"] = map[string]string{"producer_id": f.gateway.ProducerID, "role": "ai_proxy"}
	return f
}

func prepareProfile(t *testing.T, profile string) (mediaexecution.Request, fixture, *mediaexecution.Executor) {
	t.Helper()
	f := profileFixture(t, profile)
	e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	selection := f.selection(t, false)
	reply := f.commit(t, selection)
	reply["history"].(map[string]any)["attempt"].(map[string]any)["procurement"].(map[string]any)["upstream_model_name"] = f.descriptor["upstream_model"]
	r, err := e.PrepareCreate(raw(t, reply), raw(t, selection), f.live, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return r, f, e
}

func TestSixRegisteredProtocolsPrepareExactSingleSendBodies(t *testing.T) {
	for _, profile := range []string{mediaexecution.QwenImage20ProSync, mediaexecution.QwenImage30NativeSync, mediaexecution.QwenImage30Compatible, mediaexecution.Profile, mediaexecution.Wan30Text, mediaexecution.Wan30FirstFrame} {
		t.Run(profile, func(t *testing.T) {
			r, f, _ := prepareProfile(t, profile)
			if r.Profile() != profile || !bytes.Contains(r.Body(), []byte("9007199254740993")) || bytes.Equal(r.Body(), f.body) || r.OutboundBodyDigest() == f.candidate["original_body_digest"] {
				t.Fatal("lost frozen profile, original precision, or separate body basis")
			}
			var body map[string]json.RawMessage
			if json.Unmarshal(r.Body(), &body) != nil {
				t.Fatal("outbound body is not native JSON")
			}
			isAsync := profile == mediaexecution.Profile || profile == mediaexecution.Wan30Text || profile == mediaexecution.Wan30FirstFrame
			var asyncHeader bool
			for _, header := range r.Headers() {
				asyncHeader = asyncHeader || header[0] == "X-DashScope-Async" && header[1] == "enable"
			}
			if asyncHeader != isAsync {
				t.Fatal("sync and async HTTP protocols were mixed")
			}
			if strings.HasPrefix(profile, "qwen_") && profile != mediaexecution.QwenImage30Compatible && !bytes.Contains(body["input"], []byte(`"messages":[{"role":"user","content":[{"text":"a tree"}]}]`)) {
				t.Fatal("native Qwen wire did not use documented user message")
			}
			if profile == mediaexecution.Wan30FirstFrame && (body["first_frame_url"] != nil || !bytes.Contains(body["input"], []byte(`"media":[{"type":"first_frame","url":"https://input.example/original.png?sig=original"}]`))) {
				t.Fatal("Wan first frame did not map to its independent native media input")
			}
			if !isAsync {
				if _, err := r.NewAttempt(f.gateway.ProducerID); err != nil {
					t.Fatalf("original actual attempt: %v", err)
				}
				if _, err := ePrepareQueryReplay(f, t); err == nil {
					t.Fatal("synchronous profile acquired an async task capability")
				}
			}
		})
	}
}

func ePrepareQueryReplay(f fixture, t *testing.T) (mediaexecution.Request, error) {
	e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		return mediaexecution.Request{}, err
	}
	return e.PrepareQuery(raw(t, f.query(t, f.selection(t, false))), f.now)
}

func TestDocumentedSynchronousImageResponseKeepsMissingOutputIdentity(t *testing.T) {
	status := uint16(200)
	for _, profile := range []string{mediaexecution.QwenImage20ProSync, mediaexecution.QwenImage30NativeSync, mediaexecution.QwenImage30Compatible} {
		t.Run(profile, func(t *testing.T) {
			r, f, _ := prepareProfile(t, profile)
			body := []byte(`{"request_id":"native-request","output":{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":[{"image":"https://output.example/image.png"}]}}]},"usage":{"image_count":1,"output_width":2048,"output_height":2048}}`)
			if profile == mediaexecution.QwenImage30Compatible {
				body = []byte(`{"created":1780074155,"data":[{"url":"https://output.example/image.png"}],"usage":{"image_count":1,"output_image_count":1,"input_image_count":0,"output_width":2048,"output_height":2048}}`)
			}
			o, err := r.Observe(mediaexecution.Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true, Body: body,
				Headers: [][2]string{{"x-request-id", "actual-compatible-request"}}, SendState: "observed_sent", ObservedAt: f.now.Format(time.RFC3339)})
			if err != nil || !o.Terminal || o.GenerationResult != "succeeded" || len(o.Outputs) != 1 || !o.Outputs[0].Successful || !o.MeasurementsPending || o.GenerationUsage.Unit != "image" || o.GenerationUsage.Status != "reported" {
				t.Fatalf("actual native image response classification: %+v %v", o, err)
			}
			if profile == mediaexecution.QwenImage30Compatible && o.ProviderRequestID != "actual-compatible-request" {
				t.Fatal("compatible request identity was invented from a native body field")
			}
			if o.GenerationUsage.Quantities["input_tokens"] != "" || o.GenerationUsage.Quantities["output_tokens"] != "" {
				t.Fatal("media quantity became tokens")
			}
		})
	}
}

func TestWanOutputDurationUsesOnlyExactPhysicalOutputEvidence(t *testing.T) {
	status := uint16(200)
	r, f, _ := prepareProfile(t, mediaexecution.Wan30Text)
	for _, check := range []struct{ usage, status, milliseconds string }{
		{`"output_video_duration":5.125,"input_video_duration":2.25,"duration":7.375,"video_count":1`, "reported", "5125"},
		{`"duration":8,"input_video_duration":0`, "missing", ""},
		{`"output_video_duration":5.0001`, "missing", ""},
	} {
		body := []byte(`{"request_id":"actual-create","output":{"task_id":"actual-task","task_status":"SUCCEEDED","video_url":"https://output.example/video.mp4"},"usage":{` + check.usage + `}}`)
		o, err := r.Observe(mediaexecution.Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true, Body: body, SendState: "observed_sent", ObservedAt: f.now.Format(time.RFC3339)})
		if err != nil || o.GenerationUsage.Status != check.status || o.GenerationUsage.Unit != "millisecond" || o.GenerationUsage.Quantities["output_video_duration_ms"] != check.milliseconds || !o.MeasurementsPending {
			t.Fatalf("duration inferred, rounded, or substituted: %+v %v", o, err)
		}
	}
}

func TestMediaSelectionUsesActualInvocationAndIndependentQueryNamespace(t *testing.T) {
	f := original(t)
	e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "media-query:original-command-digest"} {
		selection, err := e.Selection(f.live, query, []string{"original-server-claim"})
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(selection, &fields) != nil {
			t.Fatal("selection is not JSON")
		}
		var digest string
		_ = json.Unmarshal(fields["selection_digest"], &digest)
		delete(fields, "selection_digest")
		unsigned, _ := json.Marshal(fields)
		expected, err := mfjson.Digest(unsigned)
		if err != nil || digest != expected || !bytes.Contains(selection, []byte(f.gateway.BootID)) || !bytes.Contains(selection, []byte(f.live.ID)) {
			t.Fatal("full original frame was not signed")
		}
		if query != "" && (fields["dispatch_id"] != nil || !bytes.Equal(fields["operation_kind"], []byte(`"task_query"`)) || !bytes.Equal(fields["original_body_digest"], raw(t, bodybasis.DigestBytes(nil)))) {
			t.Fatal("query reused the create grant or request body")
		}
	}
	if _, err := e.Selection(f.live, "attempt-create", nil); err == nil {
		t.Fatal("query reused creation attempt")
	}
}

func TestFrozenOutputManifestSurvivesURLRotationAndRejectsChangedSlots(t *testing.T) {
	for _, profile := range []string{mediaexecution.QwenImage20ProSync, mediaexecution.QwenImage30NativeSync, mediaexecution.QwenImage30Compatible, mediaexecution.Profile, mediaexecution.Wan30Text, mediaexecution.Wan30FirstFrame} {
		t.Run(profile, func(t *testing.T) {
			r, f, _ := prepareProfile(t, profile)
			body := `{"request_id":"actual","output":{"task_id":"original-task","task_status":"SUCCEEDED","choices":[{"message":{"content":[{"image":"https://output.example/a.png?signature=first","type":"image"}]}}]},"usage":{"image_count":1,"output_width":1024,"output_height":1024}}`
			if profile == mediaexecution.QwenImage30Compatible {
				body = `{"data":[{"url":"https://output.example/a.png?signature=first"}],"usage":{"image_count":1}}`
			}
			video := profile == mediaexecution.Wan30Text || profile == mediaexecution.Wan30FirstFrame
			if video {
				body = `{"request_id":"actual","output":{"task_id":"original-task","task_status":"SUCCEEDED","video_url":"https://output.example/a.mp4?signature=first"},"usage":{"output_video_duration":5.125,"fps":30,"SR":720,"ratio":"16:9"}}`
			}
			status := uint16(200)
			response := mediaexecution.Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true,
				Body: []byte(body), SendState: "observed_sent", ObservedAt: f.now.Format(time.RFC3339)}
			o, err := r.Observe(response)
			if err != nil {
				t.Fatal(err)
			}
			projection, err := r.Project(response, o, nil)
			if err != nil || projection.Manifest == nil || len(projection.Outputs) != 1 || len(projection.References) != 1 || projection.Usage == nil {
				t.Fatalf("missing authoritative output projection: %v", err)
			}
			if video {
				if projection.Outputs[0].Metrics["video_output_duration"].Quantity != "5125" || projection.Usage.Quantities["video_output_duration"] != "5125" {
					t.Fatal("video cumulative and per-output exact milliseconds differ")
				}
			} else if projection.Outputs[0].Metrics["image_output_count"].Quantity != "1" || projection.Usage.Quantities["image_output_count"] != "1" {
				t.Fatal("actual image slot count was replaced by requested n")
			}
			response.Body = bytes.Replace(response.Body, []byte("signature=first"), []byte("signature=rotated"), 1)
			o, err = r.Observe(response)
			if err != nil {
				t.Fatal(err)
			}
			rotated, err := r.Project(response, o, projection.Manifest)
			if err != nil || rotated.Outputs[0].OutputID != projection.Outputs[0].OutputID || rotated.References[0].URL == projection.References[0].URL {
				t.Fatal("URL rotation changed original output or failed to update the reference")
			}
			if !video {
				response.Body = bytes.Replace(response.Body, []byte(`"image_count":1`), []byte(`"image_count":2`), 1)
				o, _ = r.Observe(response)
				if _, err := r.Project(response, o, projection.Manifest); err == nil {
					t.Fatal("provider count contradicted the frozen physical slots without review")
				}
			}
		})
	}
}

func TestPartialOutputsKeepFailedSlotAndUnknownVideoDuration(t *testing.T) {
	r, f, _ := prepareProfile(t, mediaexecution.QwenImage30NativeSync)
	status := uint16(200)
	response := mediaexecution.Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true,
		Body:      []byte(`{"output":{"choices":[{"message":{"content":[{"image":"https://output.example/one.png"},{"code":"Filtered"}]}}]},"usage":{"image_count":1}}`),
		SendState: "observed_sent", ObservedAt: f.now.Format(time.RFC3339)}
	o, _ := r.Observe(response)
	p, err := r.Project(response, o, nil)
	if err != nil || len(p.Outputs) != 2 || p.Outputs[1].Successful || len(p.Outputs[1].Metrics) != 0 || len(p.References) != 1 || len(p.Manifest.Positions) != 2 || o.DeliveryState != "partial" {
		t.Fatal("failed slot disappeared or received a positive minimum")
	}
	response.Body = []byte(`{"output":{"choices":[{"message":{"content":[{"image":"https://output.example/one.png"},{"text":"unregistered output shape"}]}}]}}`)
	o, _ = r.Observe(response)
	if _, err := r.Project(response, o, p.Manifest); err == nil {
		t.Fatal("text/empty content was silently skipped and positions reassigned")
	}
	video, vf, _ := prepareProfile(t, mediaexecution.Wan30Text)
	response.Body = []byte(`{"output":{"task_id":"original-task","task_status":"SUCCEEDED","video_url":"https://output.example/one.mp4"},"usage":{"duration":8}}`)
	response.ObservedAt = vf.now.Format(time.RFC3339)
	o, _ = video.Observe(response)
	p, err = video.Project(response, o, nil)
	if err != nil || len(p.References) != 1 || len(p.Outputs[0].Metrics) != 0 || p.Usage != nil {
		t.Fatal("missing physical duration hid a real URL or became a requested duration/zero")
	}
}

func TestQueryCorrectionsKeepOriginalGenerationAndDeliverySources(t *testing.T) {
	r, f, executor := prepareProfile(t, mediaexecution.Profile)
	status := uint16(200)
	response := mediaexecution.Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true,
		Body: []byte(`{"request_id":"original-create","output":{"task_id":"task-original","task_status":"PENDING"}}`), SendState: "observed_sent", ObservedAt: f.now.Format(time.RFC3339)}
	o, _ := r.Observe(response)
	attempt, err := r.NewAttempt(f.gateway.ProducerID)
	if err != nil {
		t.Fatal(err)
	}
	first, err := attempt.Finish(o.ActualAttempt)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := r.DeliveryObservation(first, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	q, err := executor.PrepareQuery(raw(t, f.query(t, f.selection(t, false))), f.now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	response.Body = []byte(`{"request_id":"original-query","output":{"task_id":"task-original","task_status":"SUCCEEDED","choices":[{"message":{"content":[{"image":"https://output.example/final.png"}]}}]},"usage":{"image_count":1}}`)
	response.ObservedAt = f.now.Add(time.Minute).Format(time.RFC3339)
	o, _ = q.Observe(response)
	p, err := q.Project(response, o, nil)
	if err != nil {
		t.Fatal(err)
	}
	o.GenerationUsage = p.Usage
	queryAttempt, err := q.NewAttempt(f.gateway.ProducerID)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := queryAttempt.Finish(o.ActualAttempt)
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := q.GenerationObservation(actual, o, &first)
	if err != nil {
		t.Fatal(err)
	}
	finalDelivery, err := q.DeliveryObservation(corrected, o, &delivery)
	if err != nil {
		t.Fatal(err)
	}
	if corrected.SourceFactID() != first.SourceFactID() || corrected.Revision() != "2" || corrected.Envelope().AttemptID != r.AttemptID() ||
		corrected.Envelope().SupersedesEventID != first.EventID() || actual.Envelope().OperationKind != "task_query" || actual.Envelope().Usage.Status != "not_applicable" ||
		finalDelivery.SourceFactID() != delivery.SourceFactID() || finalDelivery.Revision() != "2" || finalDelivery.Envelope().AttemptID != "" || finalDelivery.Envelope().DeliveryState != "succeeded" {
		t.Fatal("query became a second generation/purchase/call or restarted a source at revision one")
	}
	foreign := corrected.Envelope()
	foreign.AccountID = "foreign-wallet"
	foreign.PayloadDigest = ""
	other, err := executionfacts.Freeze(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.GenerationObservation(actual, o, &other); err == nil {
		t.Fatal("query accepted an unrelated predecessor")
	}
}

func TestProviderRequestEvidenceUsesOnlyActualRegisteredProtocolField(t *testing.T) {
	status := uint16(200)
	for _, profile := range []string{mediaexecution.QwenImage30NativeSync, mediaexecution.Profile, mediaexecution.QwenImage30Compatible} {
		t.Run(profile, func(t *testing.T) {
			r, f, _ := prepareProfile(t, profile)
			for _, check := range []struct {
				requestID string
				headers   [][2]string
				expected  string
			}{
				{"actual-native", [][2]string{{"x-request-id", "actual-header"}}, "actual-native"},
				{"", [][2]string{{"x-request-id", "actual-header"}}, ""},
				{"", nil, ""},
			} {
				expected := check.expected
				body := []byte(`{"request_id":"` + check.requestID + `","output":{"task_id":"actual-task","task_status":"SUCCEEDED","choices":[{"message":{"content":[{"image":"https://output.example/result.png"}]}}]},"usage":{"image_count":1}}`)
				if profile == mediaexecution.QwenImage30Compatible {
					body = []byte(`{"request_id":"unregistered-native-or-platform-field","data":[{"url":"https://output.example/result.png"}],"usage":{"image_count":1}}`)
					expected = ""
					if len(check.headers) == 1 {
						expected = "actual-header"
					}
				}
				response := mediaexecution.Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true, Body: body, Headers: check.headers, SendState: "observed_sent", ObservedAt: f.now.Format(time.RFC3339)}
				o, err := r.Observe(response)
				if err != nil {
					t.Fatal(err)
				}
				var refs []string
				for _, ref := range o.ActualAttempt.EvidenceRefs {
					if strings.HasPrefix(ref, "provider-request-id:") {
						refs = append(refs, ref)
					}
				}
				if o.ProviderRequestID != expected || (expected == "" && len(refs) != 0) || (expected != "" && (len(refs) != 1 || refs[0] != "provider-request-id:"+bodybasis.DigestBytes([]byte(expected)))) {
					t.Fatal("supplier request evidence used a client/platform ID or the wrong protocol field")
				}
				if profile == mediaexecution.QwenImage30Compatible && len(check.headers) == 1 {
					response.Headers = append(response.Headers, [2]string{"X-Request-Id", "ambiguous-second-header"})
					o, err = r.Observe(response)
					if err != nil {
						t.Fatal(err)
					}
					if o.ProviderRequestID != "" {
						t.Fatal("duplicate supplier header was selected as authoritative identity")
					}
					for _, ref := range o.ActualAttempt.EvidenceRefs {
						if strings.HasPrefix(ref, "provider-request-id:") {
							t.Fatal("ambiguous header created request evidence")
						}
					}
				}
			}
		})
	}
}
