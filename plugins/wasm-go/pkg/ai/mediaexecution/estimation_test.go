package mediaexecution

import (
	"encoding/json"
	"testing"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

func estimationRequest(t *testing.T, profile string, enabled bool) Request {
	t.Helper()
	protocol, err := registeredProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	policy := UsageEstimationPolicy{Mode: estimationDisabled}
	if enabled {
		policy.Mode = estimationWanTotalMinusInputV1
	}
	return Request{profile: protocol, estimation: policy, query: true, taskID: "original-task", operationID: "original-operation",
		createAttemptID: "original-create", attemptID: "actual-query", createClassifier: "original-classifier",
		outputContract: protocol.outputIdentityContract(), body: []byte(`{"duration":30,"n":99}`)}
}

func estimationResponse(usage string) Response {
	status := uint16(200)
	return Response{HTTPStatus: &status, HeadersComplete: true, BodyComplete: true, BodyObserved: true,
		SendState: "observed_sent", ObservedAt: "2026-10-29T00:00:00Z", Body: []byte(`{"request_id":"actual-provider-request","output":{"task_id":"original-task","task_status":"SUCCEEDED","video_url":"https://result.example/original-video"},"usage":` + usage + `}`)}
}

func TestWanFrozenEstimatorUpgradesToLowerReportedWithoutChangingPhysicalOutput(t *testing.T) {
	request := estimationRequest(t, Wan30Text, true)
	response := estimationResponse(`{"duration":12.25,"input_video_duration":3.5,"SR":720,"fps":30}`)
	observation, err := request.Observe(response)
	if err != nil {
		t.Fatal(err)
	}
	before, err := request.Project(response, observation, nil)
	if err != nil || before.Usage == nil {
		t.Fatalf("actual complete response did not estimate: %v", err)
	}
	if before.Usage.Status != "estimated" || before.Usage.Source != "estimator" || before.Usage.EstimatorVersion != WanDurationEstimatorVersion ||
		before.Usage.Quantities["video_output_duration"] != "8750" || before.Outputs[0].Metrics["video_output_duration"].Quantity != "8750" ||
		before.Outputs[0].Specification["resolution"] != "720p" {
		t.Fatal("lost original observed duration, exact version, or actual specification")
	}
	// A later trustworthy meter may be lower. The original stable output and
	// manifest remain, while the caller uses its existing Correct/FACT port.
	response = estimationResponse(`{"duration":11,"input_video_duration":3.5,"output_video_duration":7.5,"SR":720,"fps":30}`)
	observation, err = request.Observe(response)
	if err != nil {
		t.Fatal(err)
	}
	after, err := request.Project(response, observation, before.Manifest)
	if err != nil || after.Usage == nil || after.Usage.Status != "reported" || after.Usage.EstimatorVersion != "" ||
		after.Usage.Quantities["video_output_duration"] != "7500" || after.Outputs[0].OutputID != before.Outputs[0].OutputID {
		t.Fatalf("reported correction was replaced, clamped, or reassigned: %v", err)
	}
	if !EstimatedWouldDowngrade(after.Usage, before.Usage) || EstimatedWouldDowngrade(before.Usage, after.Usage) {
		t.Fatal("usage grades allow inferred usage to overwrite trustworthy measurements")
	}
}

func TestWanEstimatorRequiresRegisteredPolicyAndBothActualExactMeters(t *testing.T) {
	for _, test := range []struct {
		name, usage string
		enabled     bool
	}{
		{"disabled", `{"duration":12,"input_video_duration":2}`, false},
		{"absent input is not zero", `{"duration":12}`, true},
		{"absent total", `{"input_video_duration":0}`, true},
		{"requests cannot supply meters", `{}`, true},
		{"negative inferred output", `{"duration":2,"input_video_duration":3}`, true},
		{"nonintegral physical millisecond", `{"duration":12.0001,"input_video_duration":2}`, true},
		{"overflow", `{"duration":1e100,"input_video_duration":0}`, true},
		{"malformed direct meter is not absence", `{"duration":12,"input_video_duration":2,"output_video_duration":null}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := estimationRequest(t, Wan30FirstFrame, test.enabled)
			response := estimationResponse(test.usage)
			observation, err := request.Observe(response)
			if err != nil {
				t.Fatal(err)
			}
			projection, err := request.Project(response, observation, nil)
			if err != nil || projection.Usage != nil || len(projection.Outputs[0].Metrics) != 0 {
				t.Fatalf("missing or invalid physical quantity became metered: %v", err)
			}
		})
	}
}

func TestMediaEstimatorPolicyIsClosedAndNeverRelabelsAccurateImageCounts(t *testing.T) {
	for _, raw := range []string{`{"mode":"requested_duration"}`, `{"mode":"wan_total_minus_input_v1","duration":30}`, `null`} {
		var policy UsageEstimationPolicy
		if json.Unmarshal([]byte(raw), &policy) == nil {
			t.Fatal("accepted unregistered estimator policy")
		}
	}
	profile, _ := registeredProfile(Profile)
	if (UsageEstimationPolicy{Mode: estimationWanTotalMinusInputV1}).validate(profile) == nil {
		t.Fatal("registered Wan estimator on image profile")
	}
	request := estimationRequest(t, Profile, false)
	response := estimationResponse(`{}`)
	response.Body = []byte(`{"output":{"task_id":"original-task","task_status":"SUCCEEDED","choices":[{"message":{"content":[{"image":"https://result.example/image","type":"image"}]}}]}}`)
	observation, err := request.Observe(response)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := request.Project(response, observation, nil)
	if err != nil || projection.Usage == nil || projection.Usage.Status != "reported" || projection.Usage.Source != "meter" ||
		projection.Usage.Quantities["image_output_count"] != "1" {
		t.Fatalf("accurate physical image count was relabeled: %v", err)
	}
	if EstimatedWouldDowngrade(&executionfacts.Usage{Status: "missing"}, &executionfacts.Usage{Status: "estimated"}) {
		t.Fatal("blocked an actual missing to estimated upgrade")
	}
}
