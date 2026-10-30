package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTransportShapesUseFullDraftAndURNFragments(t *testing.T) {
	root, _ := filepath.Abs("../..")
	lockPath := filepath.Join(root, "testdata/contracts/contracts.lock.json")
	lock := readObject(t, lockPath)
	var pkg map[string]any
	for _, item := range lock["packages"].([]any) {
		candidate := item.(map[string]any)
		for _, id := range candidate["imported_schema_ids"].([]any) {
			if id == "urn:modelfusion:contracts:v1:execution-dispatch" {
				pkg = candidate
			}
		}
	}
	if pkg == nil {
		t.Fatal("execution package missing")
	}
	base := filepath.Join(root, pkg["vendor_root"].(string))
	identity := readObject(t, filepath.Join(base, "fixtures/mf/execution-dispatch/shape/identity-customer.json"))
	selection := readObject(t, filepath.Join(base, "fixtures/mf/execution-dispatch/shape/selection-media.json"))
	clone := func(value map[string]any) map[string]any {
		raw, _ := json.Marshal(value)
		var result map[string]any
		_ = json.Unmarshal(raw, &result)
		return result
	}
	missing := clone(identity)
	delete(missing, "call_id")
	revision := clone(selection)
	revision["credential_revision"] = 9007199254740992.0
	digestValue := clone(selection)
	digestValue["original_body_digest"] = 12
	requests := []map[string]any{{"case_id": "valid", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/CallIdentity", "value": identity}, {"case_id": "missing", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/CallIdentity", "value": missing}, {"case_id": "revision", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/ExecutionSelection", "value": revision}, {"case_id": "digest", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/ExecutionSelection", "value": digestValue}, {"case_id": "fragment", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/absent", "value": identity}}
	path := filepath.Join(t.TempDir(), "requests.json")
	raw, _ := json.Marshal(requests)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := shapeContracts(root, lockPath, path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		ID   string  `json:"case_id"`
		Code *string `json:"error_code"`
	}
	if err = json.Unmarshal(output, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rows[0].Code != nil {
		t.Fatal(string(output))
	}
	for _, row := range rows[1:] {
		want := "E_SCHEMA_SHAPE"
		if row.ID == "fragment" {
			want = "E_FRAGMENT"
		}
		if row.Code == nil || *row.Code != want {
			t.Fatal(string(output))
		}
	}
}

func TestDraftIntegerAcrossResourceUsesExactIntegralConversion(t *testing.T) {
	root := copiedInputs(t)
	lockPath := filepath.Join(root, "testdata/contracts/contracts.lock.json")
	lock := readObject(t, lockPath)
	pkg := lock["packages"].([]any)[0].(map[string]any)
	base := filepath.Join(root, pkg["vendor_root"].(string))
	manifestPath := filepath.Join(base, "bundle-manifest.json")
	manifest := readObject(t, manifestPath)
	const id = "urn:wasm:validator-test:integer-root"
	raw := writeObject(t, filepath.Join(base, "v1/validator-test.schema.json"), map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": id, "type": "integer"})
	manifest["entries"] = append(manifest["entries"].([]any), map[string]any{"path": "v1/validator-test.schema.json", "schema_id": id, "sha256": digest(raw)})
	pkg["manifest_sha256"] = digest(writeObject(t, manifestPath, manifest))
	writeObject(t, lockPath, lock)
	input := filepath.Join(t.TempDir(), "request.json")
	raw = []byte(`[{"case_id":"ten","schema_ref":"urn:wasm:validator-test:integer-root","value":10},{"case_id":"big","schema_ref":"urn:wasm:validator-test:integer-root","value":9007199254740993},{"case_id":"fractional","schema_ref":"urn:wasm:validator-test:integer-root","value":10.5}]`)
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := shapeContracts(root, lockPath, input)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Code *string `json:"error_code"`
	}
	if err = json.Unmarshal(output, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Code != nil || rows[1].Code != nil || rows[2].Code == nil || *rows[2].Code != "E_SCHEMA_SHAPE" {
		t.Fatal(string(output))
	}
}
