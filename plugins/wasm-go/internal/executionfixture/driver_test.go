package executionfixture

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestD0DriverRejectsEmptySelection(t *testing.T) {
	out := t.TempDir()
	cmd := exec.Command("bash", "scripts/verify-d0-contracts.sh", "--batch", "A", "--output", out, "--run", "^DoesNotExist$")
	cmd.Dir = contractRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	logs, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("misspelled test selection passed")
	}
	raw, readErr := os.ReadFile(filepath.Join(out, "summary.json"))
	if readErr != nil {
		t.Fatalf("driver did not report its failure: %v %s", readErr, logs)
	}
	var report map[string]any
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report["status"] != "failed" || !strings.Contains(string(raw), "E_NO_CHECKS") {
		t.Fatal(string(raw))
	}
}

func TestD0DriverRejectsSubtestSelection(t *testing.T) {
	out := t.TempDir()
	filter := "^(TestD0Identity|TestD0Matcher|TestD0Snapshot|TestD0Callout|TestD0Schedule|TestHostMatch|TestParseOverrideConfig|TestRuleLevelConfigIsolation)/^DoesNotExist$"
	cmd := exec.Command("bash", "scripts/verify-d0-contracts.sh", "--batch", "A", "--output", out, "--run", filter)
	cmd.Dir = contractRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	logs, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("subtest filter skipped all catalog cases but driver passed")
	}
	raw, readErr := os.ReadFile(filepath.Join(out, "summary.json"))
	if readErr != nil {
		t.Fatalf("driver did not report scope rejection: %v %s", readErr, logs)
	}
	var report struct {
		Status     string            `json:"status"`
		Code       string            `json:"error_code"`
		Assertions []json.RawMessage `json:"assertions"`
	}
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || report.Code != "E_RUN_SCOPE" || len(report.Assertions) != 0 {
		t.Fatal(string(raw))
	}
}

func TestD0DriverRequiresCatalogCasePassEvents(t *testing.T) {
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	realGoJSON, err := json.Marshal(realGo)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := "#!/usr/bin/env python3\nimport json,subprocess,sys\nresult=subprocess.run([" + string(realGoJSON) + "]+sys.argv[1:],stdout=subprocess.PIPE,stderr=subprocess.STDOUT)\nfor line in result.stdout.splitlines(keepends=True):\n try:event=json.loads(line)\n except ValueError:event={}\n if './internal/executionfixture' in sys.argv and event.get('Action')=='pass' and event.get('Test')=='TestD0MatcherCorpus/D0-WG02-MATCH-001':continue\n sys.stdout.buffer.write(line)\nsys.exit(result.returncode)\n"
	if err = os.WriteFile(filepath.Join(bin, "go"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	cmd := exec.Command("bash", "scripts/verify-d0-contracts.sh", "--batch", "A", "--output", out)
	cmd.Dir = contractRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logs, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("driver passed with one frozen catalog case pass event omitted")
	}
	raw, readErr := os.ReadFile(filepath.Join(out, "summary.json"))
	if readErr != nil {
		t.Fatalf("driver did not report missing case: %v %s", readErr, logs)
	}
	var report struct {
		Status     string            `json:"status"`
		Code       string            `json:"error_code"`
		Detail     string            `json:"detail"`
		Assertions []json.RawMessage `json:"assertions"`
	}
	if err = json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" || report.Code != "E_NO_CHECKS" || !strings.Contains(report.Detail, "TestD0MatcherCorpus/D0-WG02-MATCH-001") || len(report.Assertions) != 0 {
		t.Fatal(string(raw))
	}
}

func copiedFixtureData(t *testing.T) string {
	t.Helper()
	dest := t.TempDir()
	for _, name := range []string{"identity-body.json", "matcher-cases.json", "callback-schedules.json"} {
		raw, err := os.ReadFile(FixturePath(name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dest, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dest
}
func TestD0DriverMissingFixtureFailsAtInput(t *testing.T) {
	cases := copiedFixtureData(t)
	if err := os.Remove(filepath.Join(cases, "matcher-cases.json")); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	cmd := exec.Command("bash", "scripts/verify-d0-contracts.sh", "--output", out, "--cases-root", cases)
	cmd.Dir = contractRoot(t)
	logs, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("missing corpus passed")
	}
	raw, err := os.ReadFile(filepath.Join(out, "summary.json"))
	if err != nil {
		t.Fatalf("missing failure report: %v %s", err, logs)
	}
	if !strings.Contains(string(raw), "E_FILE") || !strings.Contains(string(raw), "matcher-cases.json") {
		t.Fatal(string(raw))
	}
}
func TestD0DriverWrongCalloutFails(t *testing.T) {
	cases := copiedFixtureData(t)
	path := filepath.Join(cases, "callback-schedules.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = decode(raw, &doc); err != nil {
		t.Fatal(err)
	}
	first := doc["cases"].([]any)[0].(map[string]any)
	input := first["input"].(map[string]any)
	input["events"].([]any)[1].(map[string]any)["synthetic_callout_id"] = json.Number("999")
	bytes, _ := json.Marshal(input)
	first["input_json_utf8"] = string(bytes)
	first["input_sha256"] = Hash(bytes)
	raw, _ = json.Marshal(doc)
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	cmd := exec.Command("bash", "scripts/verify-d0-contracts.sh", "--output", out, "--cases-root", cases)
	cmd.Dir = contractRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	logs, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("wrong callback ID passed")
	}
	raw, err = os.ReadFile(filepath.Join(out, "fixture-tests.log"))
	if err != nil {
		t.Fatalf("missing concrete fixture failure: %v %s", err, logs)
	}
	if !strings.Contains(string(raw), "D0-WG03-CB-001") || !strings.Contains(string(raw), "WG_FIXTURE_CALLOUT_UNKNOWN") {
		t.Fatal(string(raw))
	}
}

func TestD0DriverPropagatesWrongLock(t *testing.T) {
	raw, err := os.ReadFile(contractLockPath(t))
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]any
	if err = json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	lock["packages"].([]any)[0].(map[string]any)["manifest_sha256"] = strings.Repeat("0", 64)
	raw, _ = json.Marshal(lock)
	bad := filepath.Join(t.TempDir(), "wrong.lock.json")
	if err = os.WriteFile(bad, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	cmd := exec.Command("bash", "scripts/verify-d0-contracts.sh", "--batch", "A", "--output", out, "--lock", bad)
	cmd.Dir = contractRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	logs, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("wrong digest lock passed")
	}
	raw, err = os.ReadFile(filepath.Join(out, "summary.json"))
	if err != nil {
		t.Fatalf("missing failure report %s: %v", logs, err)
	}
	if !strings.Contains(string(raw), "E_DIGEST") {
		t.Fatal(string(raw))
	}
}
