package mediaexecution

import (
	"bytes"
	"encoding/json"
	"math/big"

	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
)

const (
	estimationDisabled             = "disabled"
	estimationWanTotalMinusInputV1 = "wan_total_minus_input_v1"
	// This version identifies the exact observed-field algorithm, not a price.
	WanDurationEstimatorVersion = "wan-total-minus-input.v1"
)

// UsageEstimationPolicy is the resolved original Provider/Binding policy. Only
// the authenticated archive supplies it; request parameters never select it.
// A missing legacy field is disabled. Explicit disabled overrides inheritance
// on Console before the immutable archive is returned.
type UsageEstimationPolicy struct {
	Mode string `json:"mode"`
}

func (p *UsageEstimationPolicy) UnmarshalJSON(raw []byte) error {
	var value struct {
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return ErrUnsupported
	}
	if value.Mode != estimationDisabled && value.Mode != estimationWanTotalMinusInputV1 {
		return ErrUnsupported
	}
	p.Mode = value.Mode
	return nil
}

func (p UsageEstimationPolicy) resolved() UsageEstimationPolicy {
	if p.Mode == "" {
		p.Mode = estimationDisabled
	}
	return p
}

func (p UsageEstimationPolicy) validate(profile profileDefinition) error {
	p = p.resolved()
	if p.Mode == estimationDisabled {
		return nil
	}
	if p.Mode != estimationWanTotalMinusInputV1 ||
		(profile.name != Wan30Text && profile.name != Wan30FirstFrame) {
		return ErrUnsupported
	}
	return nil
}

// estimateWanOutput uses only two explicitly present physical provider meters.
// The official Wan usage.duration is total input plus output video duration;
// neither an absent input nor a requested duration is a measured zero. Both
// original seconds must be exactly representable as native milliseconds.
func (r Request) estimateWanOutput(usageRaw json.RawMessage, projection *OutputProjection) {
	if r.estimation.Mode != estimationWanTotalMinusInputV1 || projection.Usage != nil ||
		len(projection.Outputs) != 1 || !projection.Outputs[0].Successful {
		return
	}
	usage, err := object(usageRaw)
	if err != nil {
		return
	}
	// A malformed, null, or conflicting direct meter must not be repaired by
	// an estimator. Only an actually absent direct output meter qualifies.
	if _, present := usage["output_video_duration"]; present {
		return
	}
	total, totalOK := milliseconds(usage["duration"])
	input, inputOK := milliseconds(usage["input_video_duration"])
	if !totalOK || !inputOK {
		return
	}
	totalValue, _ := new(big.Int).SetString(total, 10)
	inputValue, _ := new(big.Int).SetString(input, 10)
	duration := new(big.Int).Sub(totalValue, inputValue)
	if duration.Sign() < 0 || duration.BitLen() > 128 {
		return
	}
	quantity := duration.String()
	projection.Outputs[0].Metrics["video_output_duration"] = MeasuredQuantity{Unit: "millisecond", Quantity: quantity}
	projection.Usage = &executionfacts.Usage{Status: "estimated", Unit: "millisecond", Source: "estimator",
		EstimatorVersion: WanDurationEstimatorVersion, Quantities: map[string]string{"video_output_duration": quantity}}
}

// EstimatedWouldDowngrade keeps a trusted retained generation from regressing.
// The independent actual GET audit is still retained by the caller. It must
// not submit a financial correction based on a weaker inferred quantity.
func EstimatedWouldDowngrade(previous, next *executionfacts.Usage) bool {
	return previous != nil && next != nil && next.Status == "estimated" &&
		(previous.Status == "reported" || previous.Status == "known_zero")
}
