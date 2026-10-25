package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixedBatchAExecutesPublicCases(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	report, err := verifyContracts(root, filepath.Join(root, "testdata/contracts/contracts.lock.json"), "A")
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Executed < 100 || report.Summary.Failed != 0 || report.Status != "passed" {
		t.Fatalf("incomplete public validation: %+v", report)
	}
}

func copiedInputs(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	for _, sub := range []string{"tools/contractcheck/python", "tools/contractcheck/testdata", "testdata/contracts"} {
		if err = filepath.WalkDir(filepath.Join(root, sub), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dest, relative)
			if entry.IsDir() {
				return os.MkdirAll(target, 0700)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, raw, 0600)
		}); err != nil {
			t.Fatal(err)
		}
	}
	return dest
}
func readObject(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err = json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	return d
}
func writeObject(t *testing.T, path string, d any) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestLockRejectsAlteredInputsAtExpectedLayer(t *testing.T) {
	for _, item := range []struct{ name, code string }{{"fixture-byte", "E_DIGEST"}, {"manifest-digest", "E_DIGEST"}, {"missing-common", "E_FILE"}, {"duplicate-urn", "E_DUPLICATE_URN"}, {"network-ref", "E_NETWORK_REF"}, {"unknown-validator", "E_VALIDATOR"}, {"vendor-escape", "E_PATH"}, {"tool-byte", "E_TOOL_DIGEST"}} {
		t.Run(item.name, func(t *testing.T) {
			root := copiedInputs(t)
			lockPath := filepath.Join(root, "testdata/contracts/contracts.lock.json")
			lock := readObject(t, lockPath)
			packages := lock["packages"].([]any)
			pkg := packages[0].(map[string]any)
			manifestPath := filepath.Join(root, pkg["vendor_root"].(string), "bundle-manifest.json")
			manifest := readObject(t, manifestPath)
			entries := manifest["entries"].([]any)
			fixtures := manifest["fixtures"].([]any)
			switch item.name {
			case "fixture-byte":
				f := fixtures[0].(map[string]any)
				p := filepath.Join(filepath.Dir(manifestPath), f["path"].(string))
				raw, _ := os.ReadFile(p)
				if err := os.WriteFile(p, append(raw, ' '), 0600); err != nil {
					t.Fatal(err)
				}
			case "manifest-digest":
				pkg["manifest_sha256"] = strings.Repeat("0", 64)
			case "missing-common":
				if err := os.Remove(filepath.Join(filepath.Dir(manifestPath), "v1/common.schema.json")); err != nil {
					t.Fatal(err)
				}
			case "duplicate-urn":
				other := map[string]any{}
				for k, v := range pkg {
					other[k] = v
				}
				other["bundle_version"] = "conflicting-copy"
				other["vendor_root"] = "testdata/contracts/vendor/conflicting-copy"
				target := filepath.Join(root, other["vendor_root"].(string))
				if err := os.MkdirAll(target, 0700); err != nil {
					t.Fatal(err)
				}
				if err := filepath.WalkDir(filepath.Dir(manifestPath), func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					rel, _ := filepath.Rel(filepath.Dir(manifestPath), path)
					p := filepath.Join(target, rel)
					if entry.IsDir() {
						return os.MkdirAll(p, 0700)
					}
					raw, e := os.ReadFile(path)
					if e != nil {
						return e
					}
					return os.WriteFile(p, raw, 0600)
				}); err != nil {
					t.Fatal(err)
				}
				manifest["bundle_version"] = "conflicting-copy"
				other["manifest_sha256"] = digest(writeObject(t, filepath.Join(target, "bundle-manifest.json"), manifest))
				lock["packages"] = append(packages, other)
			case "network-ref":
				entry := entries[len(entries)-1].(map[string]any)
				p := filepath.Join(filepath.Dir(manifestPath), entry["path"].(string))
				schema := readObject(t, p)
				schema["$ref"] = "https://invalid.example/forbidden"
				entry["sha256"] = digest(writeObject(t, p, schema))
				pkg["manifest_sha256"] = digest(writeObject(t, manifestPath, manifest))
			case "unknown-validator":
				f := fixtures[0].(map[string]any)
				f["validation"] = map[string]any{"kind": "raw_case", "validator_id": "unknown.v1", "selection": map[string]any{"mode": "document"}, "expected_stage": "parse"}
				pkg["manifest_sha256"] = digest(writeObject(t, manifestPath, manifest))
			case "vendor-escape":
				pkg["vendor_root"] = "../forbidden"
			case "tool-byte":
				p := filepath.Join(root, "tools/contractcheck/python/scripts/contracts/validate_bundle.py")
				raw, _ := os.ReadFile(p)
				if err := os.WriteFile(p, append(raw, ' '), 0600); err != nil {
					t.Fatal(err)
				}
			}
			writeObject(t, lockPath, lock)
			_, err := verifyContracts(root, lockPath, "A")
			if err == nil || !strings.Contains(err.Error(), item.code) {
				t.Fatalf("expected %s, got %v", item.code, err)
			}
		})
	}
}
func TestUnavailableBatchIsBlocked(t *testing.T) {
	root, _ := filepath.Abs("../..")
	if _, err := verifyContracts(root, filepath.Join(root, "testdata/contracts/contracts.lock.json"), "C"); err == nil || !strings.Contains(err.Error(), "E_SCOPE_BLOCKED") {
		t.Fatal("missing scope passed", err)
	}
}
