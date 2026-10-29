package main

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/bodybasis"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/ai/mfjson"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const managedTextDirectoryField = "_modelfusion_execution_targets"
const managedTextFailureBodyLimit = 64 * 1024

type managedTextSettings struct {
	Retry    *managedTextRetry    `json:"retry,omitempty"`
	Fallback *managedTextFallback `json:"fallback,omitempty"`
}
type managedTextRetry struct {
	MaxRetries     uint32   `json:"max_retries"`
	RetryTimeoutMS uint32   `json:"retry_timeout_ms"`
	RetryOnStatus  []string `json:"retry_on_status"`
}
type managedTextFallback struct {
	Strategy      string              `json:"strategy"`
	Targets       []managedTextTarget `json:"targets"`
	ResponseCodes []string            `json:"response_codes"`
}
type managedTextTarget struct {
	TargetID string `json:"target_id"`
	Weight   uint32 `json:"weight"`
}
type managedTextDirectory struct {
	SchemaVersion string                      `json:"schema_version"`
	Targets       []managedTextDirectoryEntry `json:"targets"`
}
type managedTextDirectoryEntry struct {
	Execution json.RawMessage `json:"execution"`
	Mapper    json.RawMessage `json:"mapper_config"`
	Proxy     json.RawMessage `json:"proxy_config"`
}

// One HTTP invocation owns the chain. Attempts own separate immutable config,
// selection and callbacks; sharing the ledger does not make them one purchase.
type managedTextCall struct {
	current  *managedExecutionState
	attempts []*managedExecutionState
	basis    bodybasis.Snapshot
	// Effective input after the original processors and before protocol conversion.
	// Only its root model is restored/mapped, using the frozen mapper below.
	publicBody                 []byte
	inputByTarget              map[string][]byte
	tried                      map[[2]string]bool
	visited                    map[string]bool
	targetAttempts             map[string]uint32
	maxAttempts, maxHops, hops uint32
	closed, finalStarted       bool
}

func validateManagedTextSettings(c *managedExecutionConfig) error {
	settings := c.settings.TextExecution
	if c.settings.MediaDispatchMode == "managed_operation" {
		if settings != nil {
			return errors.New("managed media cannot enable text retry or fallback")
		}
		return nil
	}
	capability := c.release.Descriptor.Capabilities["text_generate"]
	if settings == nil {
		if capability.SendMode == "observable_retry" {
			return errors.New("observable text execution requires its explicit retry policy")
		}
		return nil
	}
	if settings.Retry == nil && settings.Fallback == nil || c.settings.UpstreamHealth == nil || capability.SendMode != "observable_retry" || capability.MaxAttempts < 2 {
		return errors.New("managed text policy requires its observable attempt and health capability")
	}
	if retry := settings.Retry; retry != nil {
		if retry.MaxRetries == 0 || retry.MaxRetries >= capability.MaxAttempts || retry.RetryTimeoutMS == 0 || retry.RetryTimeoutMS > 600000 || validateManagedStatusPatterns(retry.RetryOnStatus) != nil {
			return errors.New("invalid managed credential retry policy")
		}
	}
	if fallback := settings.Fallback; fallback != nil {
		if fallback.Strategy != "sequence" && fallback.Strategy != "random" || len(fallback.Targets) == 0 || capability.MaxFallbackHops == 0 || validateManagedStatusPatterns(fallback.ResponseCodes) != nil {
			return errors.New("invalid managed fallback policy")
		}
		seen := map[string]bool{}
		for _, target := range fallback.Targets {
			// The original RAND policy is uniform. A weight here is a declaration of
			// that existing semantics, not a second weighted-primary chooser.
			if !managedID(target.TargetID) || target.TargetID == c.release.Descriptor.TargetID || target.Weight != 1 || seen[target.TargetID] {
				return errors.New("invalid managed fallback target directory")
			}
			seen[target.TargetID] = true
		}
	}
	return nil
}

func validateManagedStatusPatterns(patterns []string) error {
	if len(patterns) == 0 {
		return errors.New("empty managed response status policy")
	}
	for _, pattern := range patterns {
		if len(pattern) == 0 || len(pattern) > 200 {
			return errors.New("invalid managed response status pattern")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return errors.New("invalid managed response status pattern")
		}
	}
	return nil
}
func managedTextStatusMatches(status uint16, patterns []string) bool {
	actual := strconv.FormatUint(uint64(status), 10)
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, actual); matched {
			return true
		}
	}
	return false
}

func validateManagedTextJSON(settings gjson.Result) error {
	text := settings.Get("text_execution")
	if !text.Exists() {
		return nil
	}
	if !text.IsObject() {
		return errors.New("managed text policy must be a complete object")
	}
	for name, required := range map[string]string{"retry": "max_retries retry_timeout_ms retry_on_status", "fallback": "strategy targets response_codes"} {
		if block := text.Get(name); block.Exists() && !managedRequiredFields(block, required) {
			return errors.New("managed text policy is incomplete")
		}
	}
	return nil
}

// SDK fills this directory after semantic/descriptor hashing. No peer contains
// another directory, so hashing the original configs cannot depend on itself.
func configureManagedTextDirectory(value gjson.Result, origin *managedExecutionConfig) error {
	if err := validateManagedTextJSON(value.Get("managed_execution")); err != nil {
		return err
	}
	directory := value.Get(managedTextDirectoryField)
	if origin.settings.TextExecution == nil {
		if directory.Exists() {
			return errors.New("managed text directory requires its explicit execution policy")
		}
		return nil
	}
	if !directory.IsObject() || !managedRequiredFields(directory, "schema_version targets") {
		return errors.New("managed text execution requires its SDK target directory")
	}
	var raw managedTextDirectory
	if decodeManagedJSON([]byte(directory.Raw), &raw, true) != nil || raw.SchemaVersion != "modelfusion-execution-targets.v1" || len(raw.Targets) == 0 {
		return errors.New("invalid managed text target directory")
	}
	targets := make(map[string]*managedExecutionConfig, len(raw.Targets))
	for _, item := range raw.Targets {
		proxy, execution := gjson.ParseBytes(item.Proxy), gjson.ParseBytes(item.Execution)
		if !proxy.IsObject() || !execution.IsObject() || proxy.Get("_modelfusion_execution").Exists() || proxy.Get(managedTextDirectoryField).Exists() || proxy.Get("path_scope").String() != "bound_target" {
			return errors.New("recursive or unbound managed text target")
		}
		for key := range proxy.Map() {
			if strings.HasPrefix(key, "_match_") {
				return errors.New("managed text semantic config contains match metadata")
			}
		}
		for _, field := range []string{"enable_methods", "enable_path_suffixes"} {
			filter := proxy.Get(field)
			if filter.Exists() && (!filter.IsArray() || len(filter.Array()) != 0) {
				return errors.New("managed text target narrows the bound route")
			}
		}
		if validateManagedCredentialShape(proxy.Get("managed_execution"), execution) != nil || validateManagedTextJSON(proxy.Get("managed_execution")) != nil {
			return errors.New("invalid managed text target credential or policy")
		}
		peer := new(managedExecutionConfig)
		if decodeManagedJSON([]byte(execution.Raw), &peer.release, true) != nil || decodeManagedJSON([]byte(proxy.Get("managed_execution").Raw), &peer.settings, true) != nil || peer.settings.MediaDispatchMode == "managed_operation" || peer.validate() != nil {
			return errors.New("invalid managed text target configuration")
		}
		d := peer.release.Descriptor
		descriptorDigest, err := managedDescriptorDigest([]byte(execution.Get("descriptor").Raw))
		mapperDigest, mapperErr := mfjson.Digest(item.Mapper)
		proxyDigest, proxyErr := mfjson.Digest(item.Proxy)
		if err != nil || mapperErr != nil || proxyErr != nil || descriptorDigest != d.DescriptorDigest || mapperDigest != d.MapperDigest || proxyDigest != d.ProxyDigest {
			return errors.New("managed text target semantic digest differs")
		}
		if peer.release.RouteID != origin.release.RouteID || peer.release.SalesAssetID != origin.release.SalesAssetID || peer.release.PublicModel != origin.release.PublicModel || d.ReleaseID != origin.release.Descriptor.ReleaseID || peer.settings.ProducerID != origin.settings.ProducerID || peer.settings.PrivateStream != origin.settings.PrivateStream || peer.settings.Redis != origin.settings.Redis || peer.settings.InputProtocol != origin.settings.InputProtocol || peer.settings.CallIdentitySource != origin.settings.CallIdentitySource {
			return errors.New("managed fallback changes the original call or fact transport")
		}
		profile, err := newManagedProtocolProfile(peer.settings.InputProtocol, peer.settings.UpstreamProtocol)
		if err != nil || validateManagedTextProvider(proxy, peer) != nil {
			return errors.New("unsupported managed fallback provider configuration")
		}
		peer.profile, peer.mapper, peer.redis = profile, append([]byte(nil), item.Mapper...), origin.redis
		if _, err := managedTextMappedModel(peer); err != nil {
			return err
		}
		peer.health, err = configureManagedHealth(peer)
		if err != nil || peer.health == nil {
			return errors.New("managed text target health authority is unavailable")
		}
		if _, exists := targets[d.TargetID]; exists {
			return errors.New("duplicate managed text target")
		}
		targets[d.TargetID] = peer
	}
	initial, exists := targets[origin.release.Descriptor.TargetID]
	if !exists || initial.release.Descriptor.DescriptorDigest != origin.release.Descriptor.DescriptorDigest {
		return errors.New("managed text directory changes its actual initial target")
	}
	semantic, err := managedTextSemanticJSON(value)
	digest, digestErr := mfjson.Digest(semantic)
	if err != nil || digestErr != nil || digest != origin.release.Descriptor.ProxyDigest {
		return errors.New("managed text directory changes its actual initial configuration")
	}
	// Keep the parser's original initialized handles for the first selected rule.
	origin.mapper = initial.mapper
	targets[origin.release.Descriptor.TargetID] = origin
	for _, target := range targets {
		target.targets = targets
		if policy := target.settings.TextExecution; policy != nil && policy.Fallback != nil {
			for _, fallback := range policy.Fallback.Targets {
				if targets[fallback.TargetID] == nil {
					return errors.New("managed fallback references a missing frozen target")
				}
			}
		}
	}
	return nil
}

func managedTextSemanticJSON(value gjson.Result) ([]byte, error) {
	fields := map[string]json.RawMessage{}
	for key, item := range value.Map() {
		if key == "_modelfusion_execution" || key == managedTextDirectoryField || strings.HasPrefix(key, "_match_") {
			continue
		}
		fields[key] = json.RawMessage(item.Raw)
	}
	return json.Marshal(fields)
}

func validateManagedTextProvider(proxy gjson.Result, c *managedExecutionConfig) error {
	provider := proxy.Get("provider")
	if providers := proxy.Get("providers"); providers.Exists() {
		active := proxy.Get("activeProviderId")
		if provider.Exists() || !providers.IsArray() || active.Type != gjson.String || !managedID(active.String()) {
			return errors.New("managed text provider identity differs")
		}
		matches := 0
		for _, item := range providers.Array() {
			if item.Get("id").String() == active.String() {
				provider = item
				matches++
			}
		}
		if matches != 1 {
			return errors.New("managed text provider is absent or ambiguous")
		}
	} else if c.release.SchemaVersion != "modelfusion-execution.v1" || proxy.Get("activeProviderId").Exists() || !provider.IsObject() {
		return errors.New("managed text provider is absent")
	}
	typ := provider.Get("type").String()
	if typ != "openai" && typ != "generic" && (typ != "claude" || c.settings.UpstreamProtocol != managedProtocolMessages) || provider.Get("failover.enabled").Bool() || provider.Get("retryOnFailure.enabled").Bool() || proxy.Get("retryOnFailure.enabled").Bool() {
		return errors.New("managed text provider cannot run another sender")
	}
	// All authentication remains the frozen ref/revision -> secret mapping.
	// The provider's auxiliary directory must describe that same complete set.
	if tokens := provider.Get("apiTokens"); tokens.Exists() {
		members, err := c.credentialMembers()
		if err != nil || !tokens.IsArray() || len(tokens.Array()) != len(members) {
			return errors.New("managed text provider credential directory differs")
		}
		values := map[string]int{}
		for _, member := range members {
			values[member.CredentialValue]++
		}
		for _, token := range tokens.Array() {
			if token.Type != gjson.String || values[token.String()] == 0 {
				return errors.New("managed text provider credential directory differs")
			}
			values[token.String()]--
		}
	}
	return nil
}

func managedTextMappedModel(c *managedExecutionConfig) (string, error) {
	// The existing mapper supports an exact original public-model mapping.
	// Unknown transforms cannot be treated as a reversible model replacement.
	mapper := gjson.ParseBytes(c.mapper)
	if !mapper.IsObject() || !mapper.Get("modelMapping").IsObject() {
		return "", errors.New("managed text mapper is unavailable")
	}
	for key := range mapper.Map() {
		if key != "modelMapping" && key != "capture_body_basis" && key != "consume_body_basis" {
			return "", errors.New("unsupported managed text mapper transform")
		}
	}
	target, exists := mapper.Get("modelMapping").Map()[c.release.PublicModel]
	if !exists || target.Type != gjson.String || target.String() != c.release.Descriptor.UpstreamModel {
		return "", errors.New("managed text mapper differs from its original descriptor")
	}
	return target.String(), nil
}

func mappedManagedTextBody(public []byte, c *managedExecutionConfig) ([]byte, error) {
	target, err := managedTextMappedModel(c)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if decodeManagedJSON(public, &object, false) != nil || object == nil {
		return nil, errors.New("invalid managed text effective input")
	}
	var model string
	if json.Unmarshal(object["model"], &model) != nil || model != c.release.PublicModel {
		return nil, errors.New("managed text effective input model differs")
	}
	object["model"], _ = json.Marshal(target)
	return json.Marshal(object)
}

func retainManagedTextBody(state *managedExecutionState, basis bodybasis.Snapshot, body []byte) error {
	if state.config.settings.TextExecution == nil {
		return nil
	}
	var object map[string]json.RawMessage
	if decodeManagedJSON(body, &object, false) != nil || object == nil || len(state.config.mapper) == 0 {
		return errors.New("invalid managed text body")
	}
	var actualModel string
	if json.Unmarshal(object["model"], &actualModel) != nil || actualModel != state.config.release.Descriptor.UpstreamModel {
		return errors.New("managed text effective input model differs")
	}
	object["model"], _ = json.Marshal(basis.PublicModel())
	public, err := json.Marshal(object)
	if err != nil {
		return err
	}
	// All effective fields are copied as raw JSON values; only this root model
	// is restored. Runtime bodies keep their existing byte budget rather than
	// being forced through the smaller immutable mf-json configuration budget.
	if _, err := managedTextMappedModel(state.config); err != nil {
		return err
	}
	cap := state.config.release.Descriptor.Capabilities["text_generate"]
	state.text = &managedTextCall{current: state, attempts: []*managedExecutionState{state}, basis: basis, publicBody: public, inputByTarget: map[string][]byte{state.config.release.Descriptor.TargetID: append([]byte(nil), body...)}, tried: map[[2]string]bool{}, visited: map[string]bool{}, targetAttempts: map[string]uint32{}, maxAttempts: cap.MaxAttempts, maxHops: cap.MaxFallbackHops}
	state.selectionGeneration = 1
	return nil
}
func managedTextMechanism(s *managedExecutionState) string {
	if s.mechanism != "" {
		return s.mechanism
	}
	return "router"
}
func managedTextStopped(s *managedExecutionState) bool {
	return s.closed || s.text != nil && (s.text.closed || s.text.finalStarted || s.text.current != s)
}
func beginManagedTextFinal(s *managedExecutionState) bool {
	if managedTextStopped(s) {
		return false
	}
	if s.text != nil {
		s.text.finalStarted = true
	}
	return true
}
func markManagedTextAttempt(s *managedExecutionState) {
	if s.text == nil {
		return
	}
	target := s.config.release.Descriptor.TargetID
	if len(s.text.attempts) == 0 || s.text.attempts[len(s.text.attempts)-1] != s {
		s.text.attempts = append(s.text.attempts, s)
		if s.mechanism == "fallback" {
			s.text.hops++
		}
	}
	s.text.tried[[2]string{target, s.config.release.CredentialRef}] = true
	s.text.visited[target] = true
	s.text.targetAttempts[target]++
}
func managedTextUntried(s *managedExecutionState, members []managedHealthMember) []managedHealthMember {
	if s.text == nil {
		return members
	}
	result := make([]managedHealthMember, 0, len(members))
	for _, member := range members {
		if !s.text.tried[[2]string{member.target.TargetID, member.credential.CredentialRef}] {
			result = append(result, member)
		}
	}
	return result
}

// A complete observed failure is decided only after its immutable FACT is
// retained. Missing status/body, pending/accepted work and closed invocations
// never become a new send merely because a retry pattern would match.
func continueManagedText(ctx wrapper.HttpContext, s *managedExecutionState, outcome executionfacts.AttemptOutcome, headers [][2]string, body []byte, finish func()) bool {
	call := s.text
	if call == nil || managedTextStopped(s) || outcome.HTTPStatus == nil || *outcome.HTTPStatus < 400 || !outcome.HeadersComplete || !outcome.BodyComplete || outcome.ExecutionResult != "failed" || outcome.Acceptance == "accepted" || outcome.ProviderTaskID != "" || !managedTextFailureClassified(s, outcome, headers, body) || len(call.attempts) >= int(call.maxAttempts) || !time.Now().Before(s.admissionDeadline) {
		return false
	}
	policy := s.config.settings.TextExecution
	if policy == nil {
		return false
	}
	fallback := func() { tryManagedTextFallback(ctx, s, *outcome.HTTPStatus, finish) }
	target := s.config.release.Descriptor.TargetID
	if retry := policy.Retry; retry != nil && call.targetAttempts[target] <= retry.MaxRetries && managedTextStatusMatches(*outcome.HTTPStatus, retry.RetryOnStatus) {
		next := newManagedTextAttempt(s, s.config.targets[target], "credential_retry")
		next.config.settings.TimeoutMS = retry.RetryTimeoutMS
		qualifyManagedTextAttempt(ctx, next, fallback, finish)
		return true
	}
	if policy.Fallback != nil && call.hops < call.maxHops && managedTextStatusMatches(*outcome.HTTPStatus, policy.Fallback.ResponseCodes) {
		fallback()
		return true
	}
	return false
}

func newManagedTextAttempt(previous *managedExecutionState, configuration *managedExecutionConfig, mechanism string) *managedExecutionState {
	// Qualification may select another member only in this new request copy.
	copied := *configuration
	state := &managedExecutionState{config: &copied, sourceConfig: configuration, identity: previous.identity, internal: previous.internal, callID: previous.callID, businessUTC: previous.businessUTC, admissionDeadline: previous.admissionDeadline, ledger: previous.ledger, deliveries: previous.deliveries, text: previous.text, mechanism: mechanism, parentAttemptID: previous.attemptID, selectionGeneration: uint32(len(previous.text.attempts) + 1)}
	return state
}

func qualifyManagedTextAttempt(ctx wrapper.HttpContext, next *managedExecutionState, unavailable func(), stop func()) {
	call := next.text
	body, exists := call.inputByTarget[next.config.release.Descriptor.TargetID]
	if !exists {
		var err error
		body, err = mappedManagedTextBody(call.publicBody, next.config)
		if err != nil {
			unavailable()
			return
		}
		call.inputByTarget[next.config.release.Descriptor.TargetID] = append([]byte(nil), body...)
	}
	previous := call.current
	call.current = next
	next.healthUnavailable = func(reason string) {
		next.closed = true
		call.current = previous
		if reason == "upstream_health_unavailable" {
			unavailable()
		} else {
			stop()
		}
	}
	// Only a certainly eligible member/committed Consume can reach dispatch.
	// The reserved attempt number is committed to the local chain at dispatch.
	authorizeManagedHealth(ctx, next, call.basis, body)
}

func tryManagedTextFallback(ctx wrapper.HttpContext, previous *managedExecutionState, status uint16, finish func()) {
	call := previous.text
	if call.closed || call.finalStarted {
		return
	}
	call.current = previous
	policy := previous.config.settings.TextExecution
	if policy == nil || policy.Fallback == nil || call.hops >= call.maxHops || !managedTextStatusMatches(status, policy.Fallback.ResponseCodes) {
		finish()
		return
	}
	candidates := make([]managedTextTarget, 0, len(policy.Fallback.Targets))
	for _, target := range policy.Fallback.Targets {
		if !call.visited[target.TargetID] {
			candidates = append(candidates, target)
		}
	}
	var try func()
	try = func() {
		if call.closed || call.finalStarted {
			return
		}
		if len(candidates) == 0 || len(call.attempts) >= int(call.maxAttempts) || !time.Now().Before(previous.admissionDeadline) {
			call.current = previous
			finish()
			return
		}
		index := 0
		if policy.Fallback.Strategy == "random" {
			chosen, err := managedUniformMember(len(candidates))
			if err != nil {
				call.current = previous
				finish()
				return
			}
			index = chosen
		}
		target := candidates[index]
		candidates = append(candidates[:index], candidates[index+1:]...)
		config := previous.config.targets[target.TargetID]
		if config == nil {
			call.current = previous
			finish()
			return
		}
		next := newManagedTextAttempt(previous, config, "fallback")
		qualifyManagedTextAttempt(ctx, next, try, func() { call.current = previous; finish() })
	}
	try()
}

func rejectManagedPreparation(ctx wrapper.HttpContext, state *managedExecutionState, status uint32, reason string) {
	if state.healthUnavailable != nil {
		unavailable := state.healthUnavailable
		state.healthUnavailable = nil
		unavailable(reason)
		return
	}
	rejectManagedRequest(ctx, status, reason)
}

// HTTP completeness and parseable provider failure are separate evidence. A
// truncated JSON error cannot become a classified failure just from its 5xx.
func managedTextFailureClassified(s *managedExecutionState, outcome executionfacts.AttemptOutcome, headers [][2]string, body []byte) bool {
	if len(body) > managedTextFailureBodyLimit {
		return false
	}
	if len(body) == 0 {
		return true
	}
	stream, err := managedProtocolContentType(headers)
	if err != nil {
		return false
	}
	if stream {
		_, err := managedProtocolEvents(body)
		return err == nil && outcome.BodyComplete && outcome.ExecutionResult == "failed"
	}
	canonical, err := mfjson.Canonicalize(body)
	return err == nil && len(canonical) > 0 && canonical[0] == '{'
}
