package wasmgo_test

import (
	"bytes"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoWasmReleaseBuildUsesTrimpath(t *testing.T) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}

	roots := []string{
		filepath.Join(t.TempDir(), "source-a"),
		filepath.Join(t.TempDir(), "nested", "source-b"),
	}
	artifacts := make([][]byte, 0, len(roots))
	for _, root := range roots {
		extensionDir := filepath.Join(root, "extensions", "fixture-plugin")
		if err := os.MkdirAll(extensionDir, 0o755); err != nil {
			t.Fatalf("create fixture extension: %v", err)
		}
		fixtures := map[string][]byte{
			filepath.Join(root, "Makefile"):        makefile,
			filepath.Join(extensionDir, "VERSION"): []byte("0.0.1\n"),
			filepath.Join(extensionDir, "go.mod"):  []byte("module example.com/fixture-plugin\n\ngo 1.24.1\n"),
			filepath.Join(extensionDir, "main.go"): []byte("package main\n\nfunc main() {}\n"),
		}
		for path, content := range fixtures {
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Fatalf("write fixture %s: %v", path, err)
			}
		}

		cmd := exec.Command(
			"make",
			"--no-print-directory",
			"build-plugin",
			"PLUGIN=fixture-plugin",
			"WASM_BUILDER=go",
		)
		cmd.Dir = root
		cmd.Env = releaseBuildTestEnvironment(os.Environ())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build fixture in %s: %v\n%s", root, err, output)
		}

		artifactPath := filepath.Join(root, "build", "plugins", "fixture-plugin", "0.0.1", "plugin.wasm")
		artifact, err := os.ReadFile(artifactPath)
		if err != nil {
			t.Fatalf("read fixture artifact: %v", err)
		}
		if len(artifact) < 8 || !bytes.Equal(artifact[:8], []byte("\x00asm\x01\x00\x00\x00")) {
			t.Fatalf("fixture artifact is not a non-empty WebAssembly 1.0 module")
		}
		if bytes.Contains(artifact, []byte(root)) {
			t.Fatalf("fixture artifact contains absolute source path %q", root)
		}
		artifacts = append(artifacts, artifact)
	}

	if !bytes.Equal(artifacts[0], artifacts[1]) {
		t.Fatalf(
			"default Go Wasm builds differ across source paths: %x != %x",
			sha256.Sum256(artifacts[0]),
			sha256.Sum256(artifacts[1]),
		)
	}
}

func releaseBuildTestEnvironment(environment []string) []string {
	blocked := map[string]struct{}{
		"BUILD_DIR":      {},
		"EXTENSIONS_DIR": {},
		"GO":             {},
		"GOENV":          {},
		"GOFLAGS":        {},
		"GO_BUILD_FLAGS": {},
		"GNUMAKEFLAGS":   {},
		"MAKEFLAGS":      {},
		"MAKEOVERRIDES":  {},
		"MFLAGS":         {},
		"PLUGIN":         {},
		"PLUGIN_VERSION": {},
		"WASM_BUILDER":   {},
	}
	filtered := make([]string, 0, len(environment)+2)
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, drop := blocked[name]; drop {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "GOENV=off", "GOFLAGS=")
}
