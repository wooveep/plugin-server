package executionfixture

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/iface"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/matcher"
	"github.com/tidwall/gjson"
)

type fixtureLogger struct{}

func (*fixtureLogger) Trace(string)                     {}
func (*fixtureLogger) Tracef(string, ...interface{})    {}
func (*fixtureLogger) Debug(string)                     {}
func (*fixtureLogger) Debugf(string, ...interface{})    {}
func (*fixtureLogger) Info(string)                      {}
func (*fixtureLogger) Infof(string, ...interface{})     {}
func (*fixtureLogger) Warn(string)                      {}
func (*fixtureLogger) Warnf(string, ...interface{})     {}
func (*fixtureLogger) Error(string)                     {}
func (*fixtureLogger) Errorf(string, ...interface{})    {}
func (*fixtureLogger) Critical(string)                  {}
func (*fixtureLogger) Criticalf(string, ...interface{}) {}
func (*fixtureLogger) ResetID(string)                   {}

type fixturePluginContext struct {
	iface.PluginContext
	isolate     bool
	fingerprint string
}

func (c *fixturePluginContext) IsRuleLevelConfigIsolation() bool { return c.isolate }
func (c *fixturePluginContext) GetFingerPrint() string           { return c.fingerprint }

type matchConfig struct{ Label string }
type matchingVM struct {
	types.DefaultVMContext
	matcher  *matcher.RuleMatcher[matchConfig]
	captured *string
}
type matchingPlugin struct {
	types.DefaultPluginContext
	vm *matchingVM
}
type matchingHTTP struct {
	types.DefaultHttpContext
	vm     *matchingVM
	cached *matchConfig
}

func (v *matchingVM) NewPluginContext(uint32) types.PluginContext { return &matchingPlugin{vm: v} }
func (p *matchingPlugin) NewHttpContext(uint32) types.HttpContext { return &matchingHTTP{vm: p.vm} }
func (h *matchingHTTP) OnHttpRequestHeaders(int, bool) types.Action {
	config, err := h.vm.matcher.GetMatchConfig()
	if err != nil {
		panic(err)
	}
	h.cached = config
	if config != nil {
		*h.vm.captured = config.Label
	}
	return types.ActionPause
}
func (h *matchingHTTP) OnHttpRequestBody(int, bool) types.Action {
	if h.cached != nil {
		*h.vm.captured = h.cached.Label
	}
	return types.ActionContinue
}
func matcherConfig(input map[string]any) gjson.Result {
	rules := []any{}
	for _, raw := range values(input["rules"]) {
		rule := mapping(raw)
		key := "_match_route_"
		if str(rule["category"]) == "service" {
			key = "_match_service_"
		}
		rules = append(rules, map[string]any{key: []string{str(rule["match"])}, "label": str(rule["config_label"]), "fail": rule["parse_result"] == "error"})
	}
	doc := map[string]any{"_rules_": rules}
	if input["global_config"] != nil {
		doc["label"] = input["global_config"]
	}
	raw, _ := json.Marshal(doc)
	return gjson.ParseBytes(raw)
}
func observeMatcher(t *testing.T, item Case, doc Document) (string, string) {
	t.Helper()
	log.SetPluginLog(&fixtureLogger{})
	var subject matcher.RuleMatcher[matchConfig]
	ctx := &fixturePluginContext{isolate: truth(item.Input["rule_level_isolation"]), fingerprint: "d0-wg/" + item.ID}
	parse := func(value gjson.Result, result *matchConfig) error {
		if value.Get("fail").Bool() {
			return fmt.Errorf("synthetic parse rejection")
		}
		result.Label = value.Get("label").String()
		return nil
	}
	if previous := mapping(item.Input["previous_valid_rule"]); previous != nil {
		prior := map[string]any{"rules": []any{previous}}
		if err := subject.ParseRuleConfig(ctx, matcherConfig(prior), parse, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := subject.ParseRuleConfig(ctx, matcherConfig(item.Input), parse, nil); err != nil {
		t.Fatal(err)
	}
	request := str(item.Input["request_binding"])
	for _, raw := range values(item.Input["mutations"]) {
		mutation := mapping(raw)
		if str(mutation["phase"]) == "before_match" {
			request = str(mutation["request_binding"])
		}
	}
	actual := ""
	vm := &matchingVM{matcher: &subject, captured: &actual}
	host, reset := proxytest.NewHostEmulator(proxytest.NewEmulatorOption().WithVMContext(vm))
	defer reset()
	if host.StartPlugin() != types.OnPluginStartStatusOK {
		t.Fatal("matcher emulator unavailable")
	}
	context := host.InitializeHttpContext()
	binding := doc.Bindings[request]
	if err := host.SetProperty([]string{"route_name"}, []byte(binding.Route)); err != nil {
		t.Fatal(err)
	}
	if err := host.SetProperty([]string{"cluster_name"}, []byte(binding.Cluster)); err != nil {
		t.Fatal(err)
	}
	host.CallOnRequestHeaders(context, [][2]string{{":authority", binding.Authority}}, false)
	for _, raw := range values(item.Input["mutations"]) {
		mutation := mapping(raw)
		if str(mutation["phase"]) == "after_match" {
			request = str(mutation["request_binding"])
			binding = doc.Bindings[request]
			if err := host.SetProperty([]string{"route_name"}, []byte(binding.Route)); err != nil {
				t.Fatal(err)
			}
			if err := host.SetProperty([]string{"cluster_name"}, []byte(binding.Cluster)); err != nil {
				t.Fatal(err)
			}
			host.CallOnRequestBody(context, []byte("{}"), true)
		}
	}
	code := ""
	if actual == "" {
		code = "WG_FIXTURE_MANAGED_CONFIG_MISSING"
	} else if actual == "config-old" {
		code = "WG_FIXTURE_BACKUP_RELEASE_MISMATCH"
	} else {
		for _, raw := range values(item.Input["mutations"]) {
			if str(mapping(raw)["phase"]) == "after_match" {
				code = "WG_FIXTURE_MATCHED_SELECTION_MISMATCH"
			}
		}
		if captured := str(item.Input["captured_selection_label"]); captured != "" && captured != doc.Bindings[request].Selection {
			code = "WG_FIXTURE_BINDING_CORRELATION_MISMATCH"
		}
	}
	return actual, code
}
func TestD0MatcherCorpus(t *testing.T) {
	doc, err := Load(FixturePath("matcher-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 11 {
		t.Fatal("matcher catalog shrank")
	}
	for _, item := range doc.Cases {
		t.Run(item.ID, func(t *testing.T) {
			actual, code := observeMatcher(t, item, doc)
			if actual != str(item.Prediction["config_label"]) {
				t.Fatalf("real exported matcher returned %q, frozen expectation %v", actual, item.Prediction["config_label"])
			}
			if code != str(item.Expected["error_code"]) {
				t.Fatalf("fixture correlation code %q, want %v", code, item.Expected["error_code"])
			}
			t.Logf("host_emulator matcher=%q; contract_fixture correlation=%q; production_guard=not_run", actual, code)
		})
	}
}
