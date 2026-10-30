package executionfixture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

type Case struct {
	ID         string         `json:"case_id"`
	Name       string         `json:"name"`
	Input      map[string]any `json:"input"`
	InputJSON  string         `json:"input_json_utf8"`
	InputSHA   string         `json:"input_sha256"`
	Expected   map[string]any `json:"expected"`
	SourceRefs []string       `json:"source_refs"`
	Prediction map[string]any `json:"current_matcher_static_prediction"`
}
type Sample struct {
	Raw          string `json:"raw_utf8"`
	RawSHA       string `json:"raw_sha256"`
	Canonical    string `json:"canonical_utf8"`
	CanonicalSHA string `json:"canonical_sha256"`
}
type Binding struct {
	Authority string `json:"authority"`
	Route     string `json:"route_name"`
	Cluster   string `json:"cluster_name"`
	Selection string `json:"selection_label"`
	Release   string `json:"release_label"`
}
type Document struct {
	Cases     []Case                     `json:"cases"`
	Samples   map[string]Sample          `json:"samples"`
	Bindings  map[string]Binding         `json:"bindings"`
	Snapshots map[string]json.RawMessage `json:"snapshots"`
	Responses map[string]json.RawMessage `json:"responses"`
}

func Hash(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func decode(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	return d.Decode(target)
}

// Load checks the saved input bytes rather than regenerating their digest from a Go encoder.
func Load(path string) (Document, error) {
	var doc Document
	info, err := os.Lstat(path)
	if err != nil {
		return doc, err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Size() > 1<<20 {
		return doc, fixtureError("INPUT_PATH")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return doc, err
	}
	if err = decode(raw, &doc); err != nil {
		return doc, err
	}
	if len(doc.Cases) == 0 {
		return doc, fixtureError("NO_CHECKS")
	}
	ids := map[string]bool{}
	for _, item := range doc.Cases {
		if item.ID == "" || ids[item.ID] || len(item.SourceRefs) == 0 {
			return doc, fixtureError("CASE_CATALOG")
		}
		ids[item.ID] = true
		if Hash([]byte(item.InputJSON)) != item.InputSHA {
			return doc, fmt.Errorf("WG_FIXTURE_INPUT_DIGEST: %s", item.ID)
		}
		var value map[string]any
		if err = decode([]byte(item.InputJSON), &value); err != nil {
			return doc, err
		}
		if !reflect.DeepEqual(value, item.Input) {
			return doc, fmt.Errorf("WG_FIXTURE_INPUT_BYTES: %s", item.ID)
		}
	}
	for name, sample := range doc.Samples {
		if Hash([]byte(sample.Raw)) != sample.RawSHA {
			return doc, fmt.Errorf("WG_FIXTURE_BODY_DIGEST: %s", name)
		}
	}
	return doc, nil
}
func FixturePath(name string) string {
	if root := os.Getenv("D0_EXECUTION_FIXTURE_ROOT"); root != "" {
		return filepath.Join(root, name)
	}
	return filepath.Join("../../testdata/execution", name)
}
func str(value any) string             { result, _ := value.(string); return result }
func truth(value any) bool             { result, _ := value.(bool); return result }
func number(value any) uint32          { v, _ := value.(json.Number); n, _ := v.Int64(); return uint32(n) }
func values(value any) []any           { result, _ := value.([]any); return result }
func mapping(value any) map[string]any { result, _ := value.(map[string]any); return result }

type IdentityResult struct {
	Phase, RawSHA, CanonicalSHA string
	CompleteDTO                 bool
}

// EvaluateIdentity is a fixture model. It does not implement a gateway trust guard.
func EvaluateIdentity(input map[string]any, samples map[string]Sample) (IdentityResult, error) {
	result := IdentityResult{}
	if str(input["identity_source"]) != "fixture_trusted_writer" {
		return result, fixtureError("UNTRUSTED_IDENTITY_SOURCE")
	}
	if str(input["body_read"]) == "not_attempted" && input["basis_sample"] == nil {
		return IdentityResult{Phase: "collect_body"}, nil
	}
	if str(input["body_read"]) != "ok" {
		return result, fixtureError("ORIGINAL_BODY_READ_FAILED")
	}
	if !truth(input["end_of_stream"]) {
		return result, fixtureError("BODY_INCOMPLETE")
	}
	if input["basis_call_label"] != input["call_label"] {
		return result, fixtureError("BODY_CALL_MISMATCH")
	}
	if input["basis_context_generation"] != input["context_generation"] {
		return result, fixtureError("BODY_GENERATION_MISMATCH")
	}
	if input["basis_release_label"] != input["release_label"] {
		return result, fixtureError("BODY_RELEASE_MISMATCH")
	}
	if str(input["normalization_profile"]) == "" {
		return result, fixtureError("NORMALIZATION_PROFILE_MISSING")
	}
	var raw []byte
	for _, chunk := range values(input["chunks_utf8"]) {
		raw = append(raw, []byte(str(chunk))...)
	}
	body, exists := samples[str(input["body_sample"])]
	if !exists || string(raw) != body.Raw {
		return result, fixtureError("ORIGINAL_BODY_BYTES")
	}
	if input["basis_sample"] != input["body_sample"] {
		return result, fixtureError("TRANSFORMED_AS_ORIGINAL")
	}
	return IdentityResult{Phase: "body_basis_ready", CompleteDTO: true, RawSHA: Hash(raw), CanonicalSHA: body.CanonicalSHA}, nil
}
