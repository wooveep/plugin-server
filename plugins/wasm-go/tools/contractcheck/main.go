// contractcheck is a test-only launcher for the pinned full Draft 2020-12 engine.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type summary struct {
	Executed int `json:"executed"`
	Matched  int `json:"matched"`
	Failed   int `json:"failed"`
	Missing  int `json:"missing"`
}
type report struct {
	Status  string            `json:"status"`
	Summary summary           `json:"summary"`
	Results []json.RawMessage `json:"results"`
	Raw     json.RawMessage   `json:"-"`
}
type lockedPackage struct {
	Producer   string   `json:"producer_project"`
	Version    string   `json:"bundle_version"`
	VendorRoot string   `json:"vendor_root"`
	Digest     string   `json:"manifest_sha256"`
	IDs        []string `json:"imported_schema_ids"`
}
type contractLock struct {
	Packages []lockedPackage `json:"packages"`
}
type toolLock struct {
	Files []struct{ Path, SHA256 string } `json:"files"`
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func safeJoin(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", errors.New("E_PATH: relative path required")
	}
	for _, part := range strings.Split(relative, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("E_PATH: unsafe path component")
		}
	}
	p := root
	for _, part := range strings.Split(relative, "/") {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if err != nil {
			return "", fmt.Errorf("E_FILE: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("E_PATH: symlink forbidden")
		}
	}
	return p, nil
}
func verifyTool(root string) error {
	base := filepath.Join(root, "tools/contractcheck")
	raw, err := os.ReadFile(filepath.Join(base, "testdata/tool-lock.json"))
	if err != nil {
		return err
	}
	var lock toolLock
	if err = json.Unmarshal(raw, &lock); err != nil {
		return err
	}
	if len(lock.Files) != 16 {
		return errors.New("E_TOOL_LOCK: fixed tool closure differs")
	}
	seen := map[string]bool{}
	for _, item := range lock.Files {
		if seen[item.Path] {
			return errors.New("E_TOOL_LOCK: duplicate tool path")
		}
		seen[item.Path] = true
		p, err := safeJoin(base, item.Path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if digest(raw) != item.SHA256 {
			return fmt.Errorf("E_TOOL_DIGEST: %s", item.Path)
		}
	}
	return nil
}
func python(root string, args ...string) *exec.Cmd {
	name := os.Getenv("D0_PYTHON")
	if name == "" {
		name = "python3"
	}
	cmd := exec.Command(name, append([]string{"-B"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "PIP_NO_INDEX=1")
	return cmd
}
func verifyContracts(root, lockPath, batch string) (report, error) {
	var got report
	if err := verifyTool(root); err != nil {
		return got, err
	}
	if batch != "A" {
		return got, fmt.Errorf("E_SCOPE_BLOCKED: batch %q is not installed; batch A only", batch)
	}
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return got, fmt.Errorf("E_FILE: %w", err)
	}
	var lock contractLock
	if err = json.Unmarshal(raw, &lock); err != nil {
		return got, err
	}
	ids := map[string]bool{}
	for _, p := range lock.Packages {
		for _, id := range p.IDs {
			ids[id] = true
		}
	}
	if !ids["urn:modelfusion:contracts:v1:common"] || !ids["urn:modelfusion:contracts:v1:execution-dispatch"] {
		return got, errors.New("E_SCOPE_BLOCKED: batch A requires common and execution-dispatch")
	}
	dir, err := os.MkdirTemp("", "wg-contractcheck-")
	if err != nil {
		return got, err
	}
	defer os.RemoveAll(dir)
	reportPath := filepath.Join(dir, "report.json")
	cmd := python(root, filepath.Join(root, "tools/contractcheck/python/scripts/contracts/validate_bundle.py"), "vendor", "--lock", lockPath, "--repo-root", root, "--offline", "--report", reportPath)
	output, runErr := cmd.CombinedOutput()
	raw, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		return got, fmt.Errorf("E_DEPENDENCY: pinned validator did not produce report: %s (%v)", output, runErr)
	}
	if err = json.Unmarshal(raw, &got); err != nil {
		return got, err
	}
	got.Raw = raw
	if runErr != nil || got.Status != "passed" || got.Summary.Executed == 0 || got.Summary.Failed != 0 {
		return got, fmt.Errorf("E_VALIDATION: %s", raw)
	}
	return got, nil
}
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "tools/contractcheck/testdata/tool-lock.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("E_ROOT: run within the WASM checkout")
		}
		dir = parent
	}
}

const shapeProgram = `import importlib.util,json,sys,pathlib
root=pathlib.Path(sys.argv[1]); lock_path=pathlib.Path(sys.argv[2]); request_path=pathlib.Path(sys.argv[3])
p=root/'tools/contractcheck/python/scripts/contracts/validate_bundle.py'
spec=importlib.util.spec_from_file_location('fixed_d0',p); m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
m.check_environment();lock=m.parse_json(m.read_bytes(lock_path));bundles=[];schemas={};anchors={}
for item in lock['packages']:
 base=m.safe_path(root,item['vendor_root']); mp=m.safe_path(base,'bundle-manifest.json')
 if m.sha256(m.read_bytes(mp))!=item['manifest_sha256']: raise m.ContractError('E_DIGEST','manifest changed')
 b=m.Bundle(mp,defer_refs=True)
 if schemas.keys() & b.schemas.keys(): raise m.ContractError('E_DUPLICATE_URN','duplicate URN')
 schemas.update(b.schemas);anchors.update(b.anchors);bundles.append(b)
registry=m.registry_for(schemas)
for b in bundles:b.schemas,b.anchors,b.registry=schemas,anchors,registry;b.check_references()
requests=m.parse_json(m.read_bytes(request_path))
if not requests:raise m.ContractError('E_NO_CHECKS','empty transport selection')
rows=[]
for request in requests:
 try:
  if 'canonical_raw' in request:
   result=m.canonicalize(request['canonical_raw'].encode('utf-8'))
   if result.decode('utf-8')!=request['canonical_expected'] or m.sha256(result)!=request['canonical_sha256']:raise m.ContractError('WG_FIXTURE_CANONICAL_GOLDEN','body bytes differ from frozen golden')
  else:bundles[0].validate(m.schema_numbers(request['value']),request['schema_ref'])
  actual=None
 except m.ContractError as error:actual=error.code
 rows.append({'case_id':request['case_id'],'error_code':actual})
print(json.dumps(rows))
`

func shapeContracts(root, lockPath, input string) ([]byte, error) {
	if err := verifyTool(root); err != nil {
		return nil, err
	}
	cmd := python(root, "-c", shapeProgram, root, lockPath, input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("E_SHAPE_TOOL: %s", out)
	}
	return out, nil
}
func run(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(out, "verify|fixtures|shape subcommand required")
		return 2
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(out)
	lock := flags.String("lock", "", "fixed lock path")
	batch := flags.String("batch", "A", "installed scope")
	cases := flags.String("cases", "", "fixture input")
	dest := flags.String("report", "", "optional report destination")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	if *lock == "" {
		*lock = filepath.Join(root, "testdata/contracts/contracts.lock.json")
	}
	*lock, err = filepath.Abs(*lock)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	var raw []byte
	switch args[0] {
	case "verify":
		var r report
		r, err = verifyContracts(root, *lock, *batch)
		raw = r.Raw
	case "shape":
		if *cases == "" {
			err = errors.New("E_INPUT: --cases required")
		} else {
			*cases, _ = filepath.Abs(*cases)
			raw, err = shapeContracts(root, *lock, *cases)
		}
	case "fixtures":
		if _, err = verifyContracts(root, *lock, *batch); err == nil {
			if *cases == "" {
				err = errors.New("E_INPUT: --cases required")
			} else {
				*cases, _ = filepath.Abs(*cases)
				if _, e := os.Stat(*cases); e != nil {
					err = fmt.Errorf("E_FILE: %w", e)
				} else {
					cmd := exec.Command("go", "test", "-json", "./internal/executionfixture", "-run", "^TestD0IdentityCorpus$", "-count=1")
					cmd.Dir = root
					cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "D0_FIXTURE_CASES="+*cases, "D0_CONTRACT_LOCK="+*lock)
					raw, err = cmd.CombinedOutput()
					if err == nil && !strings.Contains(string(raw), `"Action":"pass","Package":"github.com/higress-group/wasm-go/internal/executionfixture","Test":"TestD0IdentityCorpus"`) {
						err = errors.New("E_NO_CHECKS: expected identity test did not pass")
					}
				}
			}
		}
	default:
		err = errors.New("E_COMMAND: unknown subcommand")
	}
	if len(raw) > 0 {
		_, _ = out.Write(raw)
		if *dest != "" {
			if e := os.WriteFile(*dest, raw, 0600); e != nil {
				err = e
			}
		}
	}
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	return 0
}
func main() { os.Exit(run(os.Args[1:], os.Stdout)) }
