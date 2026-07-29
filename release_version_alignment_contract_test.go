package wasmgo_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type releaseVersionSurface struct {
	path     string
	yamlPath []string
}

type documentedReleaseVersion struct {
	path   string
	marker string
}

type pluginReleaseVersionContract struct {
	name               string
	selectedVersion    string
	metadata           []releaseVersionSurface
	artifactURLExample string
	artifactURLDocs    []string
	documentation      []documentedReleaseVersion
}

var releaseSemanticVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func TestAIPluginVersionParserFixture(t *testing.T) {
	t.Parallel()

	const fixtureVersion = "9.8.7"

	yamlFixture := []byte("version: 9.8.7\ninfo:\n  version: 9.8.7\nspec:\n  url: http://plugin-server/plugins/fixture-plugin/9.8.7/plugin.wasm\n")
	for _, test := range []struct {
		name string
		path []string
		want string
	}{
		{name: "root metadata", path: []string{"version"}, want: fixtureVersion},
		{name: "resource metadata", path: []string{"info", "version"}, want: fixtureVersion},
		{name: "artifact URL", path: []string{"spec", "url"}, want: "http://plugin-server/plugins/fixture-plugin/9.8.7/plugin.wasm"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got, err := yamlScalar(yamlFixture, test.path...)
			if err != nil {
				t.Fatalf("parse valid YAML fixture: %v", err)
			}
			if got != test.want {
				t.Fatalf("fixture %s = %q, want %q", strings.Join(test.path, "."), got, test.want)
			}
		})
	}

	artifactVersion, err := artifactURLVersion(
		"http://plugin-server/plugins/fixture-plugin/9.8.7/plugin.wasm",
		"fixture-plugin",
	)
	if err != nil {
		t.Fatalf("parse valid artifact URL fixture: %v", err)
	}
	if artifactVersion != fixtureVersion {
		t.Fatalf("artifact URL fixture version = %q, want %q", artifactVersion, fixtureVersion)
	}

	ociVersion, err := documentedOCIArtifactVersion(
		[]byte("provider: oci://registry.example/providers/qwen:2.0.0\nplugin: oci://registry.example/plugins/fixture-plugin:9.8.7\n"),
		"fixture-plugin",
	)
	if err != nil {
		t.Fatalf("parse valid documented OCI artifact fixture: %v", err)
	}
	if ociVersion != fixtureVersion {
		t.Fatalf("documented OCI artifact fixture version = %q, want %q", ociVersion, fixtureVersion)
	}

	for _, test := range []struct {
		name    string
		content string
		marker  string
	}{
		{
			name:    "Chinese release marker ignores unrelated versions",
			content: "apiVersion: v1\n镜像版本：2.0.0\n> 插件版本：`9.8.7`\n",
			marker:  "插件版本",
		},
		{
			name:    "English release marker ignores unrelated versions",
			content: "API version: 2024.1.0\nBuilder version: 1.24.4\n> Plugin version: `9.8.7`\n",
			marker:  "Plugin version",
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got, err := documentedVersion([]byte(test.content), test.marker)
			if err != nil {
				t.Fatalf("parse valid documentation fixture: %v", err)
			}
			if got != fixtureVersion {
				t.Fatalf("documentation fixture version = %q, want %q", got, fixtureVersion)
			}
		})
	}

	t.Run("root Makefile derives the versioned build path from VERSION", func(t *testing.T) {
		repositoryRoot, err := os.Getwd()
		if err != nil {
			t.Fatalf("get repository root: %v", err)
		}
		fixtureRoot := t.TempDir()
		extensionDir := filepath.Join(fixtureRoot, "extensions", "fixture-plugin")
		buildDir := filepath.Join(fixtureRoot, "build", "plugins")
		versionedBuildDir := filepath.Join(buildDir, "fixture-plugin", fixtureVersion)
		if err := os.MkdirAll(extensionDir, 0o755); err != nil {
			t.Fatalf("create fixture extension directory: %v", err)
		}
		if err := os.MkdirAll(versionedBuildDir, 0o755); err != nil {
			t.Fatalf("create fixture build directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(extensionDir, "VERSION"), []byte(fixtureVersion+"\n"), 0o644); err != nil {
			t.Fatalf("write fixture VERSION: %v", err)
		}
		if err := os.WriteFile(filepath.Join(versionedBuildDir, "plugin.wasm"), []byte("\x00asm"), 0o644); err != nil {
			t.Fatalf("write fixture Wasm artifact: %v", err)
		}

		command := exec.Command(
			"make",
			"--no-print-directory",
			"-f",
			filepath.Join(repositoryRoot, "Makefile"),
			"metadata",
			"PLUGIN=fixture-plugin",
			"EXTENSIONS_DIR="+filepath.Join(fixtureRoot, "extensions"),
			"BUILD_DIR="+buildDir,
		)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("root Makefile rejected valid VERSION-derived fixture path: %v\n%s", err, output)
		}
		if _, err := os.Stat(filepath.Join(versionedBuildDir, "metadata.txt")); err != nil {
			t.Fatalf("root Makefile did not emit metadata beside VERSION-derived artifact: %v", err)
		}
	})
}

func TestAIPluginVersionAlignmentContract(t *testing.T) {
	t.Parallel()

	contracts := []pluginReleaseVersionContract{
		{
			name:            "ai-proxy",
			selectedVersion: "1.0.2",
			metadata: []releaseVersionSurface{
				{path: "extensions/ai-proxy/option.yaml", yamlPath: []string{"version"}},
			},
			artifactURLDocs: []string{
				"docs/plugins/ai/api-consumer/ai-token-ratelimit.md",
				"docs/plugins/ai/api-provider/ai-proxy.md",
				"extensions/ai-proxy/README.md",
				"extensions/ai-proxy/README_EN.md",
				"extensions/ai-token-ratelimit/README.md",
				"extensions/ai-token-ratelimit/README_EN.md",
				"resources/plugins/ai-proxy/README.md",
				"resources/plugins/ai-proxy/README_EN.md",
				"resources/plugins/ai-token-ratelimit/README.md",
				"resources/plugins/ai-token-ratelimit/README_EN.md",
			},
			documentation: releaseDocumentation("ai-proxy"),
		},
		{
			name:               "ai-billing",
			selectedVersion:    "0.1.6",
			artifactURLExample: "extensions/ai-billing/plugin.yaml",
			documentation:      releaseDocumentation("ai-billing"),
		},
		{
			name:               "ai-quota",
			selectedVersion:    "1.0.2",
			artifactURLExample: "extensions/ai-quota/plugin.yaml",
			documentation:      releaseDocumentation("ai-quota"),
		},
		{
			name:            "ai-statistics",
			selectedVersion: "1.0.2",
			documentation:   releaseDocumentation("ai-statistics"),
		},
	}

	for _, contract := range contracts {
		contract := contract
		t.Run(contract.name, func(t *testing.T) {
			t.Parallel()

			versionPath := "extensions/" + contract.name + "/VERSION"
			versionBytes, err := os.ReadFile(versionPath)
			if err != nil {
				t.Errorf("%s authoritative release version: read %s: %v", contract.name, versionPath, err)
			} else {
				assertSelectedReleaseVersion(
					t,
					versionPath+" authoritative release version",
					strings.TrimSpace(string(versionBytes)),
					contract.selectedVersion,
				)
			}

			resourcePath := "resources/plugins/" + contract.name + "/spec.yaml"
			assertYAMLReleaseVersion(
				t,
				resourcePath,
				[]string{"info", "version"},
				contract.selectedVersion,
			)

			for _, metadata := range contract.metadata {
				assertYAMLReleaseVersion(t, metadata.path, metadata.yamlPath, contract.selectedVersion)
			}

			if contract.name == "ai-proxy" {
				data, err := os.ReadFile("extensions/ai-proxy/config/config.go")
				if err != nil {
					t.Errorf("read ai-proxy plugin annotation metadata: %v", err)
				} else {
					version, parseErr := annotationVersion(data)
					if parseErr != nil {
						t.Errorf("extensions/ai-proxy/config/config.go plugin annotation metadata: %v", parseErr)
					} else {
						assertSelectedReleaseVersion(
							t,
							"extensions/ai-proxy/config/config.go @Version",
							version,
							contract.selectedVersion,
						)
					}
				}
			}

			if contract.artifactURLExample != "" {
				data, err := os.ReadFile(contract.artifactURLExample)
				if err != nil {
					t.Errorf("read artifact URL example %s: %v", contract.artifactURLExample, err)
				} else {
					url, parseErr := yamlScalar(data, "spec", "url")
					if parseErr != nil {
						t.Errorf("%s spec.url: %v", contract.artifactURLExample, parseErr)
					} else {
						version, parseErr := artifactURLVersion(url, contract.name)
						if parseErr != nil {
							t.Errorf("%s spec.url: %v", contract.artifactURLExample, parseErr)
						} else {
							assertSelectedReleaseVersion(
								t,
								contract.artifactURLExample+" artifact URL",
								version,
								contract.selectedVersion,
							)
						}
					}
				}
			}

			for _, path := range contract.artifactURLDocs {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("read documented OCI artifact URL %s: %v", path, err)
					continue
				}
				version, parseErr := documentedOCIArtifactVersion(data, contract.name)
				if parseErr != nil {
					t.Errorf("%s documented OCI artifact URL: %v", path, parseErr)
					continue
				}
				assertSelectedReleaseVersion(
					t,
					path+" documented OCI artifact URL",
					version,
					contract.selectedVersion,
				)
			}

			for _, documentation := range contract.documentation {
				data, err := os.ReadFile(documentation.path)
				if err != nil {
					t.Errorf("read release documentation %s: %v", documentation.path, err)
					continue
				}
				version, parseErr := documentedVersion(data, documentation.marker)
				if parseErr != nil {
					t.Errorf("%s documented release version: %v", documentation.path, parseErr)
					continue
				}
				assertSelectedReleaseVersion(
					t,
					documentation.path+" documented release version",
					version,
					contract.selectedVersion,
				)
			}
		})
	}
}

func releaseDocumentation(plugin string) []documentedReleaseVersion {
	return []documentedReleaseVersion{
		{path: "extensions/" + plugin + "/README.md", marker: "插件版本"},
		{path: "extensions/" + plugin + "/README_EN.md", marker: "Plugin version"},
		{path: "resources/plugins/" + plugin + "/README.md", marker: "插件版本"},
		{path: "resources/plugins/" + plugin + "/README_EN.md", marker: "Plugin version"},
	}
}

func assertYAMLReleaseVersion(t *testing.T, path string, yamlPath []string, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read release metadata %s: %v", path, err)
		return
	}
	got, err := yamlScalar(data, yamlPath...)
	if err != nil {
		t.Errorf("%s %s: %v", path, strings.Join(yamlPath, "."), err)
		return
	}
	assertSelectedReleaseVersion(t, path+" "+strings.Join(yamlPath, "."), got, want)
}

func assertSelectedReleaseVersion(t *testing.T, surface string, got string, want string) {
	t.Helper()

	if !releaseSemanticVersionPattern.MatchString(got) {
		t.Errorf("%s = %q, want semantic selected release %q", surface, got, want)
		return
	}
	if got != want {
		t.Errorf("%s = %q, want selected release %q", surface, got, want)
	}
}

func yamlScalar(data []byte, path ...string) (string, error) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("parse YAML: %w", err)
	}

	current := any(root)
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("%s parent is %T, want mapping", strings.Join(path, "."), current)
		}
		next, exists := mapping[key]
		if !exists {
			return "", fmt.Errorf("%s is absent", strings.Join(path, "."))
		}
		current = next
	}
	value, ok := current.(string)
	if !ok {
		return "", fmt.Errorf("%s is %T, want string", strings.Join(path, "."), current)
	}
	return strings.TrimSpace(value), nil
}

func annotationVersion(data []byte) (string, error) {
	const prefix = "// @Version "

	var found []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			found = append(found, strings.TrimSpace(strings.TrimPrefix(line, prefix)))
		}
	}
	if len(found) != 1 {
		return "", fmt.Errorf("found %d @Version annotations, want exactly one", len(found))
	}
	return found[0], nil
}

func artifactURLVersion(url string, plugin string) (string, error) {
	prefix := "/plugins/" + plugin + "/"
	index := strings.Index(url, prefix)
	if index < 0 {
		return "", fmt.Errorf("URL %q does not contain %q", url, prefix)
	}
	remainder := strings.TrimPrefix(url[index:], prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 || parts[1] != "plugin.wasm" || parts[0] == "" {
		return "", fmt.Errorf("URL %q must end with %s<version>/plugin.wasm", url, prefix)
	}
	if !releaseSemanticVersionPattern.MatchString(parts[0]) {
		return "", fmt.Errorf("URL %q contains non-semantic plugin version %q", url, parts[0])
	}
	return parts[0], nil
}

func documentedOCIArtifactVersion(data []byte, plugin string) (string, error) {
	marker := "/plugins/" + plugin + ":"
	var found []string
	for _, field := range strings.Fields(string(data)) {
		field = strings.Trim(field, "`\"'()[],")
		if !strings.HasPrefix(field, "oci://") {
			continue
		}
		index := strings.Index(field, marker)
		if index < 0 {
			continue
		}
		version := strings.Trim(field[index+len(marker):], "`\"'()[],")
		if !releaseSemanticVersionPattern.MatchString(version) {
			return "", fmt.Errorf("URL %q contains non-semantic plugin version %q", field, version)
		}
		found = append(found, version)
	}
	if len(found) != 1 {
		return "", fmt.Errorf(
			"found %d OCI URLs containing %q, want exactly one",
			len(found),
			marker,
		)
	}
	return found[0], nil
}

func documentedVersion(data []byte, marker string) (string, error) {
	prefix := "> " + marker
	if marker == "插件版本" {
		prefix += "：`"
	} else {
		prefix += ": `"
	}

	var found []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "`") {
			continue
		}
		found = append(found, strings.TrimSuffix(strings.TrimPrefix(line, prefix), "`"))
	}
	if len(found) != 1 {
		return "", fmt.Errorf(
			"found %d exact %q markers, want exactly one line %s<version>`",
			len(found),
			marker,
			prefix,
		)
	}
	return found[0], nil
}
