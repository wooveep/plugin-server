package executionfacts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const SchemaVersion = "billing-facts.v1"
const Canonicalization = "billing-json.v1"

// This matches the frozen mf-json.v1 input budget; these typed envelopes cannot
// contain arbitrary JSON trees or floating-point quantities.
const MaxEnvelopeBytes = 1 << 20

// A call cannot retain 128 maximum-sized messages in a Wasm VM. Exceeding
// either retention bound is an explicit Q41 gap, never a business rejection.
const MaxRetainedBytes = 8 << 20

var ErrConflict = errors.New("execution fact identity has different immutable content")
var ErrCapacity = errors.New("execution fact per-call retention capacity exceeded")

// Snapshot owns canonical wire and business bytes. Accessors return copies.
type Snapshot struct {
	envelope       Envelope
	wire, business []byte
	messageType    string
}

func (s Snapshot) MessageType() string   { return s.messageType }
func (s Snapshot) Bytes() []byte         { return append([]byte(nil), s.wire...) }
func (s Snapshot) BusinessBytes() []byte { return append([]byte(nil), s.business...) }
func (s Snapshot) EventID() string       { return s.envelope.EventID }
func (s Snapshot) SourceFactID() string  { return s.envelope.SourceFactID }
func (s Snapshot) Revision() string      { return s.envelope.SourceRevision }
func (s Snapshot) Digest() string        { return s.envelope.PayloadDigest }
func (s Snapshot) ProducerID() string    { return s.envelope.ProducerID }
func (s Snapshot) Envelope() Envelope    { e, _ := cloneEnvelope(s.envelope); return e }

// Freeze validates v1 fields, derives stable source identities if absent, and
// excludes only payload_digest/request_id/trace_id from the business digest.
// It never reads a clock, request headers, current routing state or final usage.
func Freeze(input Envelope) (Snapshot, error) {
	if input.Usage != nil {
		if err := validateUsage(*input.Usage); err != nil {
			return Snapshot{}, err
		}
	}
	e, err := cloneEnvelope(input)
	if err != nil {
		return Snapshot{}, err
	}
	if e.SchemaVersion == "" {
		e.SchemaVersion = SchemaVersion
	}
	if e.PayloadCanonicalization == "" {
		e.PayloadCanonicalization = Canonicalization
	}
	if e.SourceRevision == "" {
		e.SourceRevision = "1"
	}
	if e.EvidenceRefs == nil {
		e.EvidenceRefs = []string{}
	}
	if e.SourceFactID == "" {
		key := []string{e.ProducerID, e.Kind, e.CallID}
		if e.Kind != "call_delivery" {
			key = append(key, e.AttemptID)
		}
		e.SourceFactID = stableID("source", key...)
	}
	if e.EventID == "" {
		e.EventID = stableID("event", e.ProducerID, e.SourceFactID, e.SourceRevision)
	}
	suppliedDigest := e.PayloadDigest
	e.PayloadDigest = ""
	if err := validateEnvelope(e); err != nil {
		return Snapshot{}, err
	}
	raw, err := json.Marshal(e)
	if err != nil || len(raw) > MaxEnvelopeBytes {
		return Snapshot{}, errors.New("execution fact exceeds its input budget")
	}
	var tree map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&tree); err != nil {
		return Snapshot{}, err
	}
	delete(tree, "payload_digest")
	delete(tree, "request_id")
	delete(tree, "trace_id")
	business, err := canonical(tree)
	if err != nil {
		return Snapshot{}, err
	}
	e.PayloadDigest = digestBytes(business)
	if suppliedDigest != "" && suppliedDigest != e.PayloadDigest {
		return Snapshot{}, errors.New("execution fact payload digest mismatch")
	}
	raw, err = json.Marshal(e)
	if err != nil || len(raw) > MaxEnvelopeBytes {
		return Snapshot{}, errors.New("execution fact exceeds its wire budget")
	}
	decoder = json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&tree); err != nil {
		return Snapshot{}, err
	}
	wire, err := canonical(tree)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{envelope: e, wire: wire, business: business, messageType: "observation"}, nil
}

// Correct appends a new revision of the same original logical fact.
// The caller supplies newly observed evidence and observed_at; it cannot move
// original ownership/selection/occurred_at or turn a retry into a new purchase.
func Correct(previous Snapshot, replacement Envelope) (Snapshot, error) {
	if len(previous.wire) == 0 || previous.MessageType() != "observation" {
		return Snapshot{}, errors.New("correction has no predecessor")
	}
	old := previous.envelope
	if replacement.SchemaVersion != "" && replacement.SchemaVersion != SchemaVersion {
		return Snapshot{}, errorsForVersion()
	}
	if replacement.PayloadCanonicalization != "" && replacement.PayloadCanonicalization != Canonicalization {
		return Snapshot{}, errorsForVersion()
	}
	if identityKey(old) != identityKey(replacement) {
		return Snapshot{}, ErrConflict
	}
	revision, err := integer(old.SourceRevision, 63)
	if err != nil {
		return Snapshot{}, err
	}
	revision.Add(revision, big.NewInt(1))
	replacement.SchemaVersion = SchemaVersion
	replacement.PayloadCanonicalization = Canonicalization
	replacement.ProducerID = old.ProducerID
	replacement.SourceFactID = old.SourceFactID
	replacement.SourceRevision = revision.String()
	replacement.SupersedesEventID = old.EventID
	replacement.EventID = ""
	replacement.PayloadDigest = ""
	return Freeze(replacement)
}

// Ledger retains source keys only for a bounded call lifetime. The receiver's
// durable uniqueness constraints remain authoritative across contexts/restarts.
type Ledger struct {
	mu                   sync.Mutex
	limit, retainedBytes int
	events, revisions    map[string]Snapshot
}

func NewLedger(limit int) *Ledger {
	return &Ledger{limit: limit, events: map[string]Snapshot{}, revisions: map[string]Snapshot{}}
}
func (l *Ledger) Record(e Envelope) (Snapshot, error) {
	s, err := Freeze(e)
	if err != nil {
		return Snapshot{}, err
	}
	return l.Retain(s)
}
func (l *Ledger) Retain(s Snapshot) (Snapshot, error) {
	if len(s.wire) == 0 {
		return Snapshot{}, errors.New("empty execution fact snapshot")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	event := stableID("event-key", s.ProducerID(), s.EventID())
	revision := stableID("revision-key", s.ProducerID(), s.SourceFactID(), s.Revision())
	for _, existing := range []Snapshot{l.events[event], l.revisions[revision]} {
		if len(existing.wire) != 0 {
			if existing.Digest() != s.Digest() || !bytes.Equal(existing.business, s.business) {
				return Snapshot{}, ErrConflict
			}
			return existing, nil
		}
	}
	size := len(s.wire) + len(s.business)
	if l.limit <= 0 || len(l.events) >= l.limit || size > MaxRetainedBytes-l.retainedBytes {
		return Snapshot{}, ErrCapacity
	}
	l.retainedBytes += size
	l.events[event] = s
	l.revisions[revision] = s
	return s, nil
}

func identityKey(e Envelope) string {
	return stableID("identity", e.ProducerID, e.Kind, e.CallID, e.OperationID, e.AttemptID,
		e.ReleaseID, e.RouteID, e.SalesAssetID, e.TenantID, e.PrincipalID, e.AccountID,
		e.PrincipalKind, e.BillingMode, costOwnerKey(e.CostOwner), e.OperationKind,
		e.TargetID, e.BindingID, e.ProviderID, e.ProviderAccountID, e.ParentAttemptID,
		e.CredentialRef, e.CredentialRevision, e.SelectionDigest, e.UpstreamModel,
		e.SendTransport, e.RecordingOwner, e.Mechanism, e.PublicModel, e.OccurredAt, e.OutboundBodyDigest)
}
func costOwnerKey(c *CostOwner) string {
	if c == nil {
		return ""
	}
	return stableID("cost-owner", c.Kind, c.OwnerID, c.SourceJobID)
}
func stableID(prefix string, values ...string) string {
	items := make([]interface{}, len(values))
	for i, v := range values {
		items[i] = v
	}
	b, _ := canonical(items)
	return prefix + ":" + strings.TrimPrefix(digestBytes(b), "sha256:")
}
func digestBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneEnvelope(e Envelope) (Envelope, error) {
	if err := validStrings(reflect.ValueOf(e)); err != nil {
		return Envelope{}, err
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return Envelope{}, err
	}
	if len(raw) > MaxEnvelopeBytes {
		return Envelope{}, errors.New("execution fact exceeds input budget")
	}
	var copied Envelope
	err = json.Unmarshal(raw, &copied)
	return copied, err
}
func validStrings(value reflect.Value) error {
	budget := MaxEnvelopeBytes
	var visit func(reflect.Value) error
	visit = func(v reflect.Value) error {
		budget -= 8
		if budget < 0 {
			return errors.New("execution fact input budget exceeded")
		}
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !v.IsNil() {
				return visit(v.Elem())
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if err := visit(v.Field(i)); err != nil {
					return err
				}
			}
		case reflect.Slice, reflect.Array:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				budget -= v.Len()
				if budget < 0 {
					return errors.New("raw fact input budget exceeded")
				}
				return nil
			}
			if v.Len() > 10000 {
				return errors.New("execution fact collection budget exceeded")
			}
			for i := 0; i < v.Len(); i++ {
				if err := visit(v.Index(i)); err != nil {
					return err
				}
			}
		case reflect.Map:
			if v.Len() > 10000 {
				return errors.New("execution fact collection budget exceeded")
			}
			iter := v.MapRange()
			for iter.Next() {
				if err := visit(iter.Key()); err != nil {
					return err
				}
				if err := visit(iter.Value()); err != nil {
					return err
				}
			}
		case reflect.String:
			budget -= v.Len()
			if budget < 0 {
				return errors.New("execution fact string budget exceeded")
			}
			if !utf8.ValidString(v.String()) {
				return errors.New("execution fact contains invalid UTF-8")
			}
		}
		return nil
	}
	return visit(value)
}
func id(s string) bool {
	return s != "" && len(s) <= 256 && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
func one(s string, allowed ...string) bool {
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}
func integer(s string, bits int) (*big.Int, error) {
	if s == "" || (len(s) > 1 && s[0] == '0') || len(s) > 39 {
		return nil, errors.New("invalid exact fact integer")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return nil, errors.New("invalid exact fact integer")
		}
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.BitLen() > bits {
		return nil, errors.New("fact integer overflow")
	}
	return n, nil
}
func timestamp(s string) bool {
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil || t.Year() < 1 {
		return false
	}
	_, offset := t.Zone()
	return offset == 0 && (strings.HasSuffix(s, "Z") || strings.HasSuffix(s, "z") || strings.HasSuffix(s, "+00:00"))
}
func validDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	for _, r := range s[7:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func validateIdentity(e Envelope) error {
	for _, v := range []string{e.ProducerID, e.CallID} {
		if !id(v) {
			return errors.New("missing original producer or call identity")
		}
	}
	if !one(e.OperationKind, "text_generate", "image_generate", "video_generate", "task_query") {
		return errors.New("invalid operation kind")
	}
	if e.OperationKind != "text_generate" && !id(e.OperationID) {
		return errors.New("media/query requires original operation")
	}
	if (e.ReleaseID == "") != (e.RouteID == "") {
		return errors.New("release and route must be paired")
	}
	if e.BillingMode == "customer" {
		if e.PrincipalKind != "user" || e.CostOwner != nil {
			return errors.New("customer requires original user ownership")
		}
		for _, v := range []string{e.TenantID, e.PrincipalID, e.AccountID, e.SalesAssetID} {
			if !id(v) {
				return errors.New("missing original customer ownership")
			}
		}
	} else if e.BillingMode == "internal_cost" {
		if e.PrincipalKind != "service" || e.CostOwner == nil || e.AccountID != "" || e.SalesAssetID != "" {
			return errors.New("internal cost cannot claim a sale wallet")
		}
		if !one(e.CostOwner.Kind, "platform", "tenant") || !id(e.CostOwner.OwnerID) || !id(e.CostOwner.SourceJobID) {
			return errors.New("invalid original cost owner")
		}
	} else {
		return errors.New("invalid billing mode")
	}
	if !timestamp(e.OccurredAt) {
		return errors.New("missing original UTC occurred_at")
	}
	return nil
}
func validateAttemptIdentity(e Envelope) error {
	for _, v := range []string{e.AttemptID, e.TargetID, e.BindingID, e.ProviderID, e.ProviderAccountID, e.UpstreamModel} {
		if !id(v) {
			return errors.New("missing original attempt selection")
		}
	}
	if !validDigest(e.SelectionDigest) {
		return errors.New("missing original selection digest")
	}
	if (e.CredentialRef == "") != (e.CredentialRevision == "") {
		return errors.New("credential identity/revision must be paired")
	}
	if e.CredentialRevision != "" {
		n, err := integer(e.CredentialRevision, 128)
		if err != nil || n.Sign() == 0 {
			return errors.New("invalid credential revision")
		}
	}
	if !(e.SendTransport == "router" && e.RecordingOwner == "router_observer" || e.SendTransport == "http_call" && e.RecordingOwner == "ai_proxy") {
		return errors.New("send transport has a different recording owner")
	}
	if !one(e.Mechanism, "router", "credential_retry", "fallback", "native_retry", "task_query") {
		return errors.New("invalid mechanism")
	}
	if one(e.Mechanism, "credential_retry", "fallback", "native_retry") && !id(e.ParentAttemptID) {
		return errors.New("retry/fallback requires original parent attempt")
	}
	if e.ParentAttemptID == e.AttemptID {
		return errors.New("attempt cannot be its own parent")
	}
	return nil
}
func validateEnvelope(e Envelope) error {
	if e.SchemaVersion != SchemaVersion || e.PayloadCanonicalization != Canonicalization {
		return errors.New("unsupported fact schema or canonicalization")
	}
	if err := validateIdentity(e); err != nil {
		return err
	}
	for _, v := range []string{e.EventID, e.SourceFactID} {
		if !id(v) {
			return errors.New("invalid source fact key")
		}
	}
	rev, err := integer(e.SourceRevision, 63)
	if err != nil || rev.Sign() == 0 {
		return errors.New("invalid source revision")
	}
	if (e.SourceRevision == "1") != (e.SupersedesEventID == "") {
		return errors.New("correction must name its predecessor")
	}
	if e.SupersedesEventID != "" && !id(e.SupersedesEventID) {
		return errors.New("invalid predecessor key")
	}
	if !timestamp(e.ObservedAt) {
		return errors.New("invalid original UTC observed_at")
	}
	if e.RetryAfter != "" && !timestamp(e.RetryAfter) {
		return errors.New("invalid original retry_after")
	}
	if len(e.EvidenceRefs) > 256 {
		return errors.New("execution evidence reference budget exceeded")
	}
	for _, v := range e.EvidenceRefs {
		if !id(v) {
			return errors.New("invalid evidence reference")
		}
	}
	for _, v := range []string{e.SelectionDigest, e.OutboundBodyDigest, e.ResponseBodyDigest} {
		if v != "" && !validDigest(v) {
			return errors.New("invalid original body/selection digest")
		}
	}
	switch e.Kind {
	case "upstream_attempt":
		if err := validateAttemptIdentity(e); err != nil {
			return err
		}
		if !one(e.SendState, "not_sent", "handed_off", "observed_sent", "unknown") || !one(e.Acceptance, "not_sent", "rejected_before_acceptance", "accepted", "unknown") || !one(e.ExecutionResult, "pending", "succeeded", "partial", "failed", "unknown") {
			return errors.New("invalid independent attempt outcome")
		}
		if e.HeadersComplete == nil || e.BodyComplete == nil {
			return errors.New("attempt completeness is required")
		}
		if *e.HeadersComplete && e.HTTPStatus == nil {
			return errors.New("complete headers require actual upstream status")
		}
		if *e.BodyComplete && e.ResponseBodyDigest == "" {
			return errors.New("complete body requires original response digest")
		}
		if e.SendState == "not_sent" && e.Acceptance != "not_sent" {
			return errors.New("not_sent cannot claim upstream acceptance")
		}
		if e.Acceptance == "accepted" && len(e.EvidenceRefs) == 0 {
			return errors.New("accepted requires actual evidence references")
		}
	case "call_delivery":
		if !one(e.DeliveryState, "succeeded", "partial", "failed", "cancelled", "unknown") || !one(e.ServedFrom, "upstream", "cache", "local") || !id(e.PublicModel) {
			return errors.New("delivery outcome is incomplete")
		}
		if e.ServedFrom == "cache" && (e.AttemptID != "" || e.TargetID != "" || e.BindingID != "" || e.SendState != "") {
			return errors.New("cache delivery cannot create a purchase attempt")
		}
	case "supporting_observation":
	default:
		return errors.New("invalid observation kind")
	}
	if e.HTTPStatus != nil && (*e.HTTPStatus < 100 || *e.HTTPStatus > 599) {
		return errors.New("invalid actual HTTP status")
	}
	if e.Kind != "supporting_observation" && e.Usage == nil {
		return errors.New("explicit usage evidence status is required")
	}
	if e.Usage != nil {
		return validateUsage(*e.Usage)
	}
	return nil
}
func validateUsage(u Usage) error {
	if len(u.Quantities) > 256 {
		return errors.New("usage meter budget exceeded")
	}
	if !one(u.Unit, "token", "image", "second", "request", "millisecond") {
		return errors.New("invalid usage unit")
	}
	has := len(u.Quantities) > 0
	switch u.Status {
	case "reported":
		if !has || !one(u.Source, "provider", "meter") {
			return errors.New("reported usage needs quantities/source")
		}
	case "estimated":
		if !has || u.Source != "estimator" || u.EstimatorVersion == "" {
			return errors.New("estimated usage needs exact estimator")
		}
	case "known_zero":
		if !has || u.Source != "protocol_zero" || u.ReasonCode == "" {
			return errors.New("known zero needs original protocol reason")
		}
	case "missing", "not_applicable":
		if u.Quantities != nil || u.Source != "none" || u.ReasonCode == "" {
			return errors.New("missing usage cannot fabricate zero quantities")
		}
	default:
		return errors.New("invalid usage status")
	}
	for name, q := range u.Quantities {
		if !id(name) {
			return errors.New("invalid meter name")
		}
		n, err := integer(q, 128)
		if err != nil {
			return err
		}
		if u.Status == "known_zero" && n.Sign() != 0 {
			return errors.New("known zero contains nonzero usage")
		}
	}
	hit, h := u.Quantities["cache_hit"]
	miss, m := u.Quantities["cache_miss"]
	if h != m {
		return errors.New("cache split must be paired")
	}
	if u.Unit == "token" && h {
		a, _ := integer(hit, 128)
		b, _ := integer(miss, 128)
		a.Add(a, b)
		if a.BitLen() > 128 || u.Quantities["input"] != a.String() {
			return errors.New("input must equal exact cache hit plus miss")
		}
	}
	return nil
}

// canonical handles the typed envelope's strings, booleans, arrays and bounded
// integer HTTP status. It is not exported as a general JSON canonicalizer.
func canonical(v interface{}) ([]byte, error) {
	var out bytes.Buffer
	if err := writeCanonical(&out, v); err != nil {
		return nil, err
	}
	if out.Len() > 4*MaxEnvelopeBytes {
		return nil, errors.New("canonical output budget exceeded")
	}
	return out.Bytes(), nil
}
func writeCanonical(out *bytes.Buffer, v interface{}) error {
	switch x := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			writeString(out, k)
			out.WriteByte(':')
			if err := writeCanonical(out, x[k]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	case []interface{}:
		out.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case string:
		writeString(out, x)
	case bool:
		if x {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case json.Number:
		if _, err := strconv.ParseUint(x.String(), 10, 16); err != nil {
			return errors.New("nonintegral fact number")
		}
		out.WriteString(x.String())
	default:
		return errors.New("fact contains null or unsupported canonical value")
	}
	return nil
}
func writeString(out *bytes.Buffer, s string) {
	out.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case r < 32:
			out.WriteString("\\u00")
			out.WriteByte("0123456789abcdef"[byte(r)>>4])
			out.WriteByte("0123456789abcdef"[byte(r)&15])
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
}

func errorsForVersion() error { return errors.New("unsupported original fact schema/canonicalization") }

// FreezeRegistration freezes an independent pre-send registration proposal.
// All selection fields are preserved; no envelope/history/price fields are added.
// This E1 producer profile accepts the frozen text_generate selection only.
func FreezeRegistration(input Registration) (Snapshot, error) {
	if err := validStrings(reflect.ValueOf(input)); err != nil {
		return Snapshot{}, err
	}
	r := input
	if r.SchemaVersion == "" {
		r.SchemaVersion = "execution-registration.v1"
	}
	if r.SchemaVersion != "execution-registration.v1" {
		return Snapshot{}, errors.New("unsupported execution registration schema")
	}
	for _, v := range []string{r.ProducerID, r.CallID, r.AttemptID, r.ReleaseID, r.RouteID, r.TargetID, r.AccountID, r.APIKeyID} {
		if !id(v) {
			return Snapshot{}, errors.New("missing original registration identity")
		}
	}
	n, err := integer(r.CredentialGeneration, 63)
	if err != nil || n.Sign() == 0 {
		return Snapshot{}, errors.New("invalid original credential generation")
	}
	if !timestamp(r.BusinessOccurredAt) {
		return Snapshot{}, errors.New("invalid original registration timestamp")
	}
	selection, err := decodeSelection(r.Selection)
	if err != nil {
		return Snapshot{}, err
	}
	expected, err := SelectionDigest(r.Selection)
	if err != nil {
		return Snapshot{}, err
	}
	if selection["selection_digest"] != expected {
		return Snapshot{}, errors.New("original selection digest mismatch")
	}
	for key, want := range map[string]string{"call_id": r.CallID, "attempt_id": r.AttemptID, "release_id": r.ReleaseID, "target_id": r.TargetID} {
		if selection[key] != want {
			return Snapshot{}, errors.New("registration differs from original selection")
		}
	}
	selectionBytes, err := canonical(selection)
	if err != nil {
		return Snapshot{}, err
	}
	r.Selection = selectionBytes
	if r.RegistrationID == "" {
		r.RegistrationID = stableID("registration", r.ProducerID, r.CallID, r.AttemptID)
	}
	if !id(r.RegistrationID) {
		return Snapshot{}, errors.New("invalid registration key")
	}
	raw, err := json.Marshal(r)
	if err != nil || len(raw) > MaxEnvelopeBytes {
		return Snapshot{}, errors.New("registration exceeds wire budget")
	}
	tree, err := decodeExact(raw)
	if err != nil {
		return Snapshot{}, err
	}
	wire, err := canonical(tree)
	if err != nil {
		return Snapshot{}, err
	}
	metadata := Envelope{ProducerID: r.ProducerID, EventID: r.RegistrationID, SourceFactID: r.RegistrationID, SourceRevision: "1", PayloadDigest: digestBytes(wire)}
	return Snapshot{envelope: metadata, wire: wire, business: wire, messageType: "execution_registration"}, nil
}

func decodeSelection(raw []byte) (map[string]interface{}, error) {
	tree, err := decodeExact(raw)
	if err != nil {
		return nil, err
	}
	object, ok := tree.(map[string]interface{})
	if !ok {
		return nil, errors.New("selection must be a complete object")
	}
	requiredFields := strings.Fields("call_id release_id target_id binding_id attempt_id operation_kind provider_account_id upstream_model cluster_ref credential_ref credential_revision selection_generation selection_digest selection_canonicalization original_body_digest mapper_digest proxy_digest descriptor_digest mechanism send_transport recording_owner")
	allowed := map[string]bool{}
	for _, k := range requiredFields {
		allowed[k] = true
		s, ok := object[k].(string)
		if !ok || s == "" {
			return nil, errors.New("original selection is incomplete")
		}
	}
	for _, k := range strings.Fields("operation_id dispatch_id invocation_id gateway_boot_id parent_attempt_id context_generation recovery_claim_refs") {
		allowed[k] = true
	}
	for k, v := range object {
		if !allowed[k] {
			return nil, errors.New("selection has an undeclared field")
		}
		if k == "recovery_claim_refs" {
			refs, ok := v.([]interface{})
			if !ok || len(refs) == 0 {
				return nil, errors.New("invalid recovery claim references")
			}
			seen := map[string]bool{}
			for _, ref := range refs {
				s, ok := ref.(string)
				if !ok || !id(s) || seen[s] {
					return nil, errors.New("invalid recovery claim references")
				}
				seen[s] = true
			}
			continue
		}
		s, ok := v.(string)
		if !ok || !id(s) {
			return nil, errors.New("invalid exact selection field")
		}
		if strings.HasSuffix(k, "_digest") && !validDigest(s) {
			return nil, errors.New("invalid original selection digest")
		}
		if strings.HasSuffix(k, "_revision") || strings.HasSuffix(k, "_generation") {
			n, err := integer(s, 128)
			if err != nil || n.Sign() == 0 {
				return nil, errors.New("invalid selection revision")
			}
		}
	}
	if object["selection_canonicalization"] != "selection-json.v1" || object["operation_kind"] != "text_generate" {
		return nil, errors.New("registration profile requires frozen text selection")
	}
	if _, ok := object["operation_id"]; ok {
		return nil, errors.New("text selection cannot claim a media operation")
	}
	if _, ok := object["dispatch_id"]; ok {
		return nil, errors.New("text selection cannot claim a media dispatch")
	}
	transport, _ := object["send_transport"].(string)
	owner, _ := object["recording_owner"].(string)
	if !(transport == "router" && owner == "router_observer" || transport == "http_call" && owner == "ai_proxy") {
		return nil, errors.New("selection owner/transport mismatch")
	}
	mechanism, _ := object["mechanism"].(string)
	if !one(mechanism, "router", "credential_retry", "fallback", "native_retry") {
		return nil, errors.New("invalid original text mechanism")
	}
	if one(mechanism, "credential_retry", "fallback", "native_retry") {
		p, ok := object["parent_attempt_id"].(string)
		if !ok || !id(p) || p == object["attempt_id"] {
			return nil, errors.New("retry selection has no original parent")
		}
	}
	return object, nil
}

// decodeExact is bounded and rejects duplicate keys and unpaired Unicode
// surrogates before Go's permissive decoder can replace original evidence.
func decodeExact(raw []byte) (interface{}, error) {
	if len(raw) == 0 || len(raw) > MaxEnvelopeBytes || !utf8.Valid(raw) {
		return nil, errors.New("invalid bounded UTF-8 registration")
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return nil, errors.New("invalid JSON escape")
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return nil, errors.New("invalid Unicode escape")
		}
		n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return nil, errors.New("invalid Unicode escape")
		}
		i += 4
		if n >= 0xDC00 && n <= 0xDFFF {
			return nil, errors.New("unpaired Unicode surrogate")
		}
		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return nil, errors.New("unpaired Unicode surrogate")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xDC00 || low > 0xDFFF {
				return nil, errors.New("unpaired Unicode surrogate")
			}
			i += 6
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var parse func(int) (interface{}, error)
	parse = func(depth int) (interface{}, error) {
		if depth > 128 {
			return nil, errors.New("registration nesting budget exceeded")
		}
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return token, nil
		}
		switch delimiter {
		case '{':
			object := map[string]interface{}{}
			for decoder.More() {
				if len(object) >= 10000 {
					return nil, ErrCapacity
				}
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, errors.New("invalid object key")
				}
				if _, exists := object[name]; exists {
					return nil, errors.New("duplicate registration JSON key")
				}
				v, err := parse(depth + 1)
				if err != nil {
					return nil, err
				}
				object[name] = v
			}
			_, err = decoder.Token()
			return object, err
		case '[':
			array := []interface{}{}
			for decoder.More() {
				if len(array) >= 10000 {
					return nil, ErrCapacity
				}
				v, err := parse(depth + 1)
				if err != nil {
					return nil, err
				}
				array = append(array, v)
			}
			_, err = decoder.Token()
			return array, err
		default:
			return nil, errors.New("invalid JSON delimiter")
		}
	}
	tree, err := parse(0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("registration has trailing JSON")
	}
	return tree, nil
}

// SelectionDigest implements frozen selection-json.v1: the entire strict
// ExecutionSelection except selection_digest, including descriptor/body digests.
// It is not a candidate/prepare digest and requires every frozen selection field.
func SelectionDigest(raw []byte) (string, error) {
	tree, err := decodeExact(raw)
	if err != nil {
		return "", err
	}
	object, ok := tree.(map[string]interface{})
	if !ok {
		return "", errors.New("selection must be an object")
	}
	if _, ok := object["selection_digest"]; !ok {
		object["selection_digest"] = "sha256:" + strings.Repeat("0", 64)
	}
	intermediate, err := json.Marshal(object)
	if err != nil {
		return "", err
	}
	checked, err := decodeSelection(intermediate)
	if err != nil {
		return "", err
	}
	delete(checked, "selection_digest")
	canonicalBytes, err := canonical(checked)
	if err != nil {
		return "", err
	}
	return digestBytes(canonicalBytes), nil
}

// CanonicalSelection preserves the full original strict object, after checking
// its declared digest. A caller must keep these bytes for registration/retries.
func CanonicalSelection(raw []byte) ([]byte, error) {
	object, err := decodeSelection(raw)
	if err != nil {
		return nil, err
	}
	expected, err := SelectionDigest(raw)
	if err != nil {
		return nil, err
	}
	if object["selection_digest"] != expected {
		return nil, errors.New("original selection digest mismatch")
	}
	return canonical(object)
}
