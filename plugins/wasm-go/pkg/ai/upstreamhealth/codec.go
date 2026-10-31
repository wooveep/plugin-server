package upstreamhealth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/higress-group/wasm-go/pkg/ai/balanceidentity"
)

// Console's state_digest uses the common 4 MiB canonical output budget. The
// published JSON body adds only fixed metadata to that closed state tree.
const maxStateBytes = 4_194_304
const MaxSnapshotBytes = maxStateBytes + 4096

func decodeHash(fields []string, config Config, now time.Time) ReadResult {
	if len(fields) != len(HashFields()) || config.validate() != nil || len(fields[3]) > MaxSnapshotBytes {
		return ReadResult{Status: ReadInvalid}
	}
	if fields[0] != config.AuthorityID {
		return ReadResult{Status: ReadWrongAuthority}
	}
	if fields[6] != ProjectionSchema {
		return ReadResult{Status: ReadSchemaMismatch}
	}
	if fields[7] != "true" {
		return ReadResult{Status: ReadIncomplete}
	}
	revision, err := ParseRevision(fields[1])
	if err != nil {
		return ReadResult{Status: ReadInvalid}
	}
	if compareRevision(revision, config.MinimumRevision) < 0 {
		return ReadResult{Status: ReadStale}
	}
	tree, err := parseExact([]byte(fields[3]))
	if err != nil || !snapshotShape(tree) {
		return ReadResult{Status: ReadInvalid}
	}
	var wire snapshotWire
	decoder := json.NewDecoder(bytes.NewBufferString(fields[3]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || !validateWire(wire) {
		return ReadResult{Status: ReadInvalid}
	}
	if wire.AuthorityID != config.AuthorityID {
		return ReadResult{Status: ReadWrongAuthority}
	}
	if wire.ProjectionSchema != ProjectionSchema {
		return ReadResult{Status: ReadSchemaMismatch}
	}
	if !wire.Complete {
		return ReadResult{Status: ReadIncomplete}
	}
	observed, observedOK := utc(wire.ObservedAt)
	validUntil, validOK := utc(wire.ValidUntil)
	hashObserved, hashObservedOK := utc(fields[4])
	hashDeadline, hashDeadlineOK := utc(fields[5])
	if wire.Revision != revision || wire.StateDigest != fields[2] || !observedOK || !validOK || !hashObservedOK || !hashDeadlineOK ||
		!observed.Equal(hashObserved) || !validUntil.Equal(hashDeadline) || !validUntil.After(observed) || now.IsZero() {
		return ReadResult{Status: ReadInvalid}
	}
	object := tree.(map[string]interface{})
	// This is the exact Console state_digest projection, with array order intact.
	digest, err := stateDigest([]interface{}{object["authority_id"], object["revision"], object["credentials"], object["scopes"]})
	if err != nil || digest != wire.StateDigest {
		return ReadResult{Status: ReadInvalid}
	}
	// This is persistent control state, not a lease from a live Console process.
	// Its age is observable through valid_until; only an explicit new revision
	// changes sending eligibility. Structural and revision checks still apply.
	if observed.After(now) {
		return ReadResult{Status: ReadFuture}
	}
	snapshot := &Snapshot{wire: wire, metadata: Metadata{AuthorityID: wire.AuthorityID, Revision: wire.Revision,
		StateDigest: wire.StateDigest, ObservedAt: observed, ValidUntil: validUntil}, raw: []byte(fields[3]), validated: true}
	snapshot.credentialIndex = make(map[string]int, len(wire.Credentials))
	snapshot.scopeIndex = make(map[string]int, len(wire.Scopes))
	snapshot.accountScopes = make(map[string][]int)
	for index, control := range wire.Credentials {
		snapshot.credentialIndex[control.Identity.CredentialRef] = index
	}
	for index, scope := range wire.Scopes {
		snapshot.scopeIndex[scope.Identity.ScopeID] = index
		account := scope.Identity.ProviderAccountID
		snapshot.accountScopes[account] = append(snapshot.accountScopes[account], index)
	}
	return ReadResult{Status: ReadReady, Snapshot: snapshot}
}

func validateWire(wire snapshotWire) bool {
	if !balanceidentity.ValidID(wire.AuthorityID) || !validDigest(wire.StateDigest) {
		return false
	}
	if _, err := ParseRevision(string(wire.Revision)); err != nil {
		return false
	}
	credentials := make(map[string]bool, len(wire.Credentials))
	for _, control := range wire.Credentials {
		if !identity(control.Identity.CredentialRef) || !identity(control.Identity.ProviderAccountID) || !identity(control.Identity.OwnerID) ||
			control.Identity.OwnerKind != "platform" && control.Identity.OwnerKind != "tenant" || credentials[control.Identity.CredentialRef] {
			return false
		}
		for _, value := range []Revision{control.Generation, control.Revision} {
			if _, err := ParseRevision(string(value)); err != nil {
				return false
			}
		}
		credentials[control.Identity.CredentialRef] = true
	}
	scopes := make(map[string]bool, len(wire.Scopes))
	for _, state := range wire.Scopes {
		if !validScope(state.Identity) || scopes[state.Identity.ScopeID] {
			return false
		}
		scopes[state.Identity.ScopeID] = true
		for _, value := range []Revision{state.Revision, state.FailureGeneration, state.RecoveryEpoch} {
			if _, err := ParseRevision(string(value)); err != nil {
				return false
			}
		}
		for _, value := range []*string{state.BlockedUntil, state.RecoveredAt} {
			if value != nil {
				if _, ok := utc(*value); !ok {
					return false
				}
			}
		}
		if state.ActiveClaim != nil && !balanceidentity.ValidID(*state.ActiveClaim) {
			return false
		}
		if state.PermanentReason != nil && !identity(*state.PermanentReason) || state.LastEventID != nil && !identity(*state.LastEventID) {
			return false
		}
	}
	return true
}

func validScope(scope ScopeIdentity) bool {
	if !identity(scope.ScopeID) || !identity(scope.ProviderAccountID) {
		return false
	}
	for _, value := range []*string{scope.RegionEndpointRef, scope.CredentialRef, scope.UpstreamModel, scope.OperationKind, scope.QuotaGroupRef} {
		if value != nil && !identity(*value) {
			return false
		}
	}
	if scope.CredentialGeneration != nil {
		if _, err := ParseRevision(string(*scope.CredentialGeneration)); err != nil {
			return false
		}
	}
	switch scope.Kind {
	case "credential":
		return scope.CredentialRef != nil && scope.CredentialGeneration != nil && scope.UpstreamModel == nil && scope.OperationKind == nil && scope.QuotaGroupRef == nil
	case "account":
		return scope.CredentialRef == nil && scope.CredentialGeneration == nil && scope.UpstreamModel == nil && scope.OperationKind == nil && scope.QuotaGroupRef == nil
	case "model_operation":
		return scope.CredentialRef == nil && scope.CredentialGeneration == nil && scope.RegionEndpointRef != nil && scope.UpstreamModel != nil && scope.OperationKind != nil && scope.QuotaGroupRef == nil
	case "quota_group":
		return scope.CredentialRef == nil && scope.CredentialGeneration == nil && scope.QuotaGroupRef != nil
	}
	return false
}

func validDigest(value string) bool {
	if len(value) != 71 || value[:7] != "sha256:" {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	if err != nil {
		return false
	}
	for _, char := range value[7:] {
		if char >= 'A' && char <= 'F' {
			return false
		}
	}
	return true
}

func exactObject(value interface{}, names string) (map[string]interface{}, bool) {
	object, ok := value.(map[string]interface{})
	keys := splitFields(names)
	if !ok || len(object) != len(keys) {
		return nil, false
	}
	for _, key := range keys {
		if _, found := object[key]; !found {
			return nil, false
		}
	}
	return object, true
}

func splitFields(value string) []string {
	var fields []string
	start := 0
	for index := 0; index <= len(value); index++ {
		if index == len(value) || value[index] == ' ' {
			if index > start {
				fields = append(fields, value[start:index])
			}
			start = index + 1
		}
	}
	return fields
}

func snapshotShape(tree interface{}) bool {
	object, ok := exactObject(tree, "projection_schema authority_id revision complete credentials scopes observed_at valid_until state_digest")
	if !ok {
		return false
	}
	if _, ok := object["complete"].(bool); !ok {
		return false
	}
	credentials, credentialsOK := object["credentials"].([]interface{})
	scopes, scopesOK := object["scopes"].([]interface{})
	if !credentialsOK || !scopesOK {
		return false
	}
	for _, value := range credentials {
		control, ok := exactObject(value, "identity generation revision enabled")
		if !ok {
			return false
		}
		if _, ok := control["enabled"].(bool); !ok {
			return false
		}
		if _, ok := exactObject(control["identity"], "credential_ref provider_account_id owner_kind owner_id"); !ok {
			return false
		}
	}
	for _, value := range scopes {
		state, ok := exactObject(value, "identity revision failure_generation recovery_epoch blocked_until backoff_step permanent_reason recovered_at active_claim last_event_id")
		if !ok {
			return false
		}
		if _, ok := state["backoff_step"].(json.Number); !ok {
			return false
		}
		if _, ok := exactObject(state["identity"], "scope_id kind provider_account_id region_endpoint_ref credential_ref credential_generation upstream_model operation_kind quota_group_ref"); !ok {
			return false
		}
	}
	return true
}

// parseExact rejects ambiguous original bytes before typed decoding. It is
// private to this closed projection, not a new shared JSON/canonical API.
func parseExact(raw []byte) (interface{}, error) {
	if len(raw) > MaxSnapshotBytes || !utf8.Valid(raw) || !validUnicodeEscapes(raw) {
		return nil, errors.New("invalid health JSON encoding or size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var parse func(int) (interface{}, error)
	parse = func(depth int) (interface{}, error) {
		if depth > 128 {
			return nil, errors.New("health JSON nesting exceeds bound")
		}
		token, err := decoder.Token()
		if err != nil {
			return nil, errors.New("incomplete health JSON")
		}
		delimiter, compound := token.(json.Delim)
		if !compound {
			return token, nil
		}
		switch delimiter {
		case '{':
			object := make(map[string]interface{})
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || len(object) >= 10000 {
					return nil, errors.New("invalid health JSON object")
				}
				if _, duplicate := object[name]; duplicate {
					return nil, errors.New("duplicate health JSON key")
				}
				value, err := parse(depth + 1)
				if err != nil {
					return nil, err
				}
				object[name] = value
			}
			if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
				return nil, errors.New("incomplete health JSON object")
			}
			return object, nil
		case '[':
			array := make([]interface{}, 0)
			for decoder.More() {
				if len(array) >= 10000 {
					return nil, errors.New("health JSON array exceeds bound")
				}
				value, err := parse(depth + 1)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			if closing, err := decoder.Token(); err != nil || closing != json.Delim(']') {
				return nil, errors.New("incomplete health JSON array")
			}
			return array, nil
		}
		return nil, errors.New("invalid health JSON token")
	}
	tree, err := parse(0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("trailing health JSON data")
	}
	return tree, nil
}

func validUnicodeEscapes(raw []byte) bool {
	quoted := false
	for index := 0; index < len(raw); index++ {
		if raw[index] == '"' {
			quoted = !quoted
		} else if quoted && raw[index] == '\\' {
			index++
			if index >= len(raw) {
				return false
			}
			if raw[index] != 'u' {
				continue
			}
			if index+4 >= len(raw) {
				return false
			}
			code, err := strconv.ParseUint(string(raw[index+1:index+5]), 16, 16)
			if err != nil || code >= 0xdc00 && code <= 0xdfff {
				return false
			}
			index += 4
			if code >= 0xd800 && code <= 0xdbff {
				if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
					return false
				}
				low, err := strconv.ParseUint(string(raw[index+3:index+7]), 16, 16)
				if err != nil || low < 0xdc00 || low > 0xdfff {
					return false
				}
				index += 6
			}
		}
	}
	return !quoted
}

func stateDigest(payload interface{}) (string, error) {
	canonical, err := canonicalState(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// canonicalState implements mf-json.v1 only for the validated projection's
// strings/bools/null/arrays/objects and unsigned backoff_step integers. Other
// JSON numbers cannot enter this typed wire and are explicitly rejected here.
func canonicalState(payload interface{}) ([]byte, error) {
	var output bytes.Buffer
	var write func(interface{}, int) error
	quoted := func(value string) {
		output.WriteByte('"')
		for _, char := range value {
			switch char {
			case '"', '\\':
				output.WriteByte('\\')
				output.WriteRune(char)
			default:
				if char < 0x20 {
					fmt.Fprintf(&output, "\\u%04x", char)
				} else {
					output.WriteRune(char)
				}
			}
		}
		output.WriteByte('"')
	}
	write = func(value interface{}, depth int) error {
		if depth > 128 || output.Len() > maxStateBytes {
			return errors.New("health canonical state exceeds bound")
		}
		switch value := value.(type) {
		case nil:
			output.WriteString("null")
		case string:
			if !utf8.ValidString(value) {
				return errors.New("invalid health canonical string")
			}
			quoted(value)
		case bool:
			output.WriteString(strconv.FormatBool(value))
		case json.Number:
			number, err := strconv.ParseUint(string(value), 10, 64)
			if err != nil || strconv.FormatUint(number, 10) != string(value) {
				return errors.New("invalid typed health integer")
			}
			output.WriteString(string(value))
		case []interface{}:
			if len(value) > 10000 {
				return errors.New("health canonical array exceeds bound")
			}
			output.WriteByte('[')
			for index, item := range value {
				if index != 0 {
					output.WriteByte(',')
				}
				if err := write(item, depth+1); err != nil {
					return err
				}
			}
			output.WriteByte(']')
		case map[string]interface{}:
			if len(value) > 10000 {
				return errors.New("health canonical object exceeds bound")
			}
			keys := make([]string, 0, len(value))
			for key := range value {
				if !utf8.ValidString(key) {
					return errors.New("invalid health canonical key")
				}
				keys = append(keys, key)
			}
			// UTF-8 byte ordering equals scalar ordering for validated UTF-8.
			sort.Strings(keys)
			output.WriteByte('{')
			for index, key := range keys {
				if index != 0 {
					output.WriteByte(',')
				}
				quoted(key)
				output.WriteByte(':')
				if err := write(value[key], depth+1); err != nil {
					return err
				}
			}
			output.WriteByte('}')
		default:
			return errors.New("unsupported health canonical value")
		}
		if output.Len() > maxStateBytes {
			return errors.New("health canonical state exceeds bound")
		}
		return nil
	}
	if err := write(payload, 0); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
