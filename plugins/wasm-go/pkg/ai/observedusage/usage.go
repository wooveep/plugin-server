// Package observedusage carries validated delivery metering across request filters.
// The billing bridge consumer is the writer; HTTP fields never create authority.
package observedusage

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
)

// Property is request-local FilterState, shared by filters and never sent on HTTP.
const Property = "modelfusion.delivery.usage.v1"

// RouterDeliveryProperty identifies a call whose final delivery is submitted
// by the upstream stream observer after the response headers have been sent.
const RouterDeliveryProperty = "modelfusion.delivery.router-call.v1"
const maxBytes = 2048 // A bounded identity and four decimal counters, without secrets.

type observation struct {
	Version string `json:"version"`
	CallID  string `json:"call_id"`
	Model   string `json:"model"`
	Source  string `json:"source"`
	Input   string `json:"input"`
	Output  string `json:"output"`
	Cached  string `json:"cached,omitempty"`
}

// Clear invalidates any earlier value before native response processing begins.
func Clear() error { return proxywasm.SetProperty([]string{Property}, []byte("{}")) }

// Publish accepts only the original delivery after its billing bridge is verified.
// It does not change the financial snapshot, estimate counters, or emit a header.
func Publish(delivery executionfacts.Envelope) error {
	u := delivery.Usage
	if u == nil || u.Status == "missing" || u.Status == "not_applicable" {
		return Clear()
	}
	if delivery.Kind != "call_delivery" || delivery.CallID == "" || delivery.PublicModel == "" {
		return errors.New("usage observation requires its original delivery identity")
	}
	o := observation{Version: "1", CallID: delivery.CallID, Model: delivery.PublicModel, Source: u.Source, Input: u.Quantities["input"], Output: u.Quantities["output"], Cached: u.Quantities["cache_hit"]}
	if executionfacts.IsInputOnly(delivery.OperationKind) && o.Output == "" {
		o.Output = "0"
	}
	// Financial quantities retain their exact original strings. Consumers retain
	// their existing int64 observation range; no overflowing count becomes zero.
	if _, ok := o.tokens(); !ok {
		return errors.New("usage observation is outside the consumer counter contract")
	}
	raw, err := json.Marshal(o)
	if err != nil || len(raw) > maxBytes {
		return errors.New("usage observation exceeds its bounded contract")
	}
	return proxywasm.SetProperty([]string{Property}, raw)
}

// Read observes only typed FilterState installed by the trusted billing filter.
func Read() (tokenusage.TokenUsage, bool) {
	raw, err := proxywasm.GetProperty([]string{Property})
	if err != nil || len(raw) == 0 || len(raw) > maxBytes {
		return tokenusage.TokenUsage{}, false
	}
	exact, err := mfjson.Canonicalize(raw)
	if err != nil {
		return tokenusage.TokenUsage{}, false
	}
	var o observation
	d := json.NewDecoder(bytes.NewReader(exact))
	d.DisallowUnknownFields()
	if d.Decode(&o) != nil {
		return tokenusage.TokenUsage{}, false
	}
	return o.tokens()
}

func (o observation) tokens() (tokenusage.TokenUsage, bool) {
	if o.Version != "1" || o.CallID == "" || len(o.CallID) > 256 || o.Model == "" || len(o.Model) > 256 || (o.Source != "provider" && o.Source != "estimator" && o.Source != "meter" && o.Source != "protocol_zero") {
		return tokenusage.TokenUsage{}, false
	}
	count := func(value string) (int64, bool) {
		n, err := strconv.ParseInt(value, 10, 64)
		return n, err == nil && n >= 0 && strconv.FormatInt(n, 10) == value
	}
	i, ok := count(o.Input)
	if !ok {
		return tokenusage.TokenUsage{}, false
	}
	out, ok := count(o.Output)
	if !ok || i > int64(^uint64(0)>>1)-out {
		return tokenusage.TokenUsage{}, false
	}
	u := tokenusage.TokenUsage{Model: o.Model, InputToken: i, OutputToken: out, TotalToken: i + out, InputTokenDetails: map[string]int64{}, OutputTokenDetails: map[string]int64{}}
	if o.Cached != "" {
		cached, ok := count(o.Cached)
		if !ok || cached > i {
			return tokenusage.TokenUsage{}, false
		}
		u.InputTokenDetails["cached_tokens"] = cached
	}
	return u, true
}
