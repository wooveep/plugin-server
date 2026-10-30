package executionfixture

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func contractRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func contractLockPath(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("D0_CONTRACT_LOCK"); path != "" {
		return path
	}
	return filepath.Join(contractRoot(t), "testdata/contracts/contracts.lock.json")
}
func transportTemplate(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(contractLockPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Packages []struct {
			Root string   `json:"vendor_root"`
			IDs  []string `json:"imported_schema_ids"`
		} `json:"packages"`
	}
	if err = decode(raw, &lock); err != nil || len(lock.Packages) == 0 {
		t.Fatal("invalid contract lock", err)
	}
	packageRoot := ""
	for _, item := range lock.Packages {
		for _, id := range item.IDs {
			if id == "urn:modelfusion:contracts:v1:execution-dispatch" {
				packageRoot = item.Root
			}
		}
	}
	if packageRoot == "" {
		t.Fatal("batch A execution package is absent")
	}
	path := filepath.Join(contractRoot(t), packageRoot, "fixtures/mf/execution-dispatch/shape/"+name+".json")
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = decode(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func runShapeBatch(t *testing.T, requests []map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(requests)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "shape.json")
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "shape", "--lock", contractLockPath(t), "--cases", file)
	cmd.Dir = filepath.Join(contractRoot(t), "tools/contractcheck")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixed full-schema engine: %v %s", err, out)
	}
	var rows []struct {
		ID   string `json:"case_id"`
		Code any    `json:"error_code"`
	}
	if err = decode(out, &rows); err != nil {
		t.Fatalf("invalid engine result %s: %v", out, err)
	}
	result := map[string]any{}
	for _, row := range rows {
		result[row.ID] = row.Code
	}
	if len(result) != len(requests) {
		t.Fatal("engine omitted transport validations")
	}
	return result
}
func TestD0IdentityCorpus(t *testing.T) {
	path := os.Getenv("D0_FIXTURE_CASES")
	if path == "" {
		path = FixturePath("identity-body.json")
	}
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Cases) != 13 {
		t.Fatalf("identity catalog shrank: %d", len(doc.Cases))
	}
	var transports []map[string]any
	wanted := map[string]any{}
	for name, sample := range doc.Samples {
		if sample.Raw == "" {
			if sample.CanonicalSHA != sample.RawSHA {
				t.Fatal("raw empty-body digest changed")
			}
			continue
		}
		id := "canonical/" + name
		transports = append(transports, map[string]any{"case_id": id, "canonical_raw": sample.Raw, "canonical_expected": sample.Canonical, "canonical_sha256": sample.CanonicalSHA})
		wanted[id] = nil
	}
	for _, item := range doc.Cases {
		result, runErr := EvaluateIdentity(item.Input, doc.Samples)
		var code any
		if runErr != nil {
			code = runErr.Error()
		}
		if code != item.Expected["error_code"] {
			t.Errorf("%s semantic code=%v, want=%v", item.ID, code, item.Expected["error_code"])
		}
		if phase, exists := item.Expected["next_phase"]; exists && result.Phase != phase {
			t.Errorf("%s phase=%q", item.ID, result.Phase)
		}
		if flag, exists := item.Expected["complete_send_dto_allowed"]; exists && result.CompleteDTO != flag {
			t.Errorf("%s complete-send=%v", item.ID, result.CompleteDTO)
		}
		if runErr == nil && result.CompleteDTO && result.RawSHA != str(item.Expected["raw_body_sha256"]) {
			t.Errorf("%s raw digest mismatch", item.ID)
		}
		identity := transportTemplate(t, "identity-customer")
		identity["call_id"] = str(item.Input["call_label"])
		identity["release_id"] = str(item.Input["release_label"])
		transports = append(transports, map[string]any{"case_id": item.ID + "/identity", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/CallIdentity", "value": identity})
		wanted[item.ID+"/identity"] = nil
		basis := transportTemplate(t, "body-customer")
		if item.Input["basis_sample"] == nil {
			transports = append(transports, map[string]any{"case_id": item.ID + "/send", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/ExecutionContext", "value": identity})
			wanted[item.ID+"/send"] = "E_SCHEMA_SHAPE"
		} else {
			basis["call_id"] = str(item.Input["basis_call_label"])
			basis["release_id"] = str(item.Input["basis_release_label"])
			sample := doc.Samples[str(item.Input["basis_sample"])]
			basis["original_body_digest"] = "sha256:" + sample.CanonicalSHA
			if profile := str(item.Input["normalization_profile"]); profile != "" {
				basis["body_canonicalization"] = profile
			} else {
				delete(basis, "body_canonicalization")
			}
			transports = append(transports, map[string]any{"case_id": item.ID + "/body", "schema_ref": "urn:modelfusion:contracts:v1:execution-dispatch#/$defs/BodyBasis", "value": basis})
			wanted[item.ID+"/body"] = nil
			if _, exists := basis["body_canonicalization"]; !exists {
				wanted[item.ID+"/body"] = "E_SCHEMA_SHAPE"
			}
		}
	}
	for id, actual := range runShapeBatch(t, transports) {
		if actual != wanted[id] {
			t.Errorf("%s transport code %v, want %v", id, actual, wanted[id])
		}
	}
	if doc.Samples["original"].RawSHA == doc.Samples["changed"].RawSHA {
		t.Fatal("payload change lost")
	}
}
