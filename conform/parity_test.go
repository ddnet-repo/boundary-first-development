package conform

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Parity guards: every surface of this repo carries the same water. These
// tests fail when a material drifts — the canonical presets against the
// toolchain tier's required sets, the repo's own gate against the canonical
// preset, the vendored law against RULES.md, the docs against the features.
// Same-water parity is proven here, not remembered.

func repoRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile("../" + path)
	if err != nil {
		t.Fatalf("cannot read repo material %s: %v", path, err)
	}
	return raw
}

func TestParityRootGateIsCanonicalPreset(t *testing.T) {
	preset := repoRead(t, "lint/golangci.yml")
	rootGate := repoRead(t, ".golangci.yml")
	if !bytes.Equal(preset, rootGate) {
		t.Error(".golangci.yml differs from lint/golangci.yml — the repo's own gate is a copy of the canonical preset; update both together")
	}
}

func TestParityGolangciPresetMatchesRequired(t *testing.T) {
	var config toolchainGolangciFile
	if err := yaml.Unmarshal(repoRead(t, "lint/golangci.yml"), &config); err != nil {
		t.Fatalf("lint/golangci.yml does not parse: %v", err)
	}
	required := map[string]bool{}
	for _, entry := range toolchainGolangciRequired {
		required[entry.Linter] = true
	}
	enabled := map[string]bool{}
	for _, linter := range config.Linters.Enable {
		enabled[linter] = true
		if !required[linter] {
			t.Errorf("preset enables %q but conform does not require it — update toolchainGolangciRequired and LINT.md together", linter)
		}
	}
	for linter := range required {
		if !enabled[linter] {
			t.Errorf("conform requires %q but the preset does not enable it — update lint/golangci.yml and LINT.md together", linter)
		}
	}
}

func TestParityRuffPresetCoversRequired(t *testing.T) {
	var config toolchainRuffConfig
	if err := toml.Unmarshal(repoRead(t, "lint/ruff.toml"), &config); err != nil {
		t.Fatalf("lint/ruff.toml does not parse: %v", err)
	}
	selectors := append(append([]string{}, config.Lint.Select...), config.Lint.ExtendSelect...)
	for _, required := range toolchainRuffRequired {
		if !toolchainSelectorCovered(selectors, required.Selector) {
			t.Errorf("conform requires ruff selector %q but the preset does not cover it — update lint/ruff.toml and LINT.md together", required.Selector)
		}
	}
}

func TestParitySkillVendorsTheLaw(t *testing.T) {
	rules := string(repoRead(t, "RULES.md"))
	skill := string(repoRead(t, "skills/bfd/SKILL.md"))
	marker := "## The Rules"
	start := strings.Index(rules, marker)
	if start < 0 {
		t.Fatalf("RULES.md has no %q section", marker)
	}
	law := strings.TrimSpace(rules[start:])
	if !strings.Contains(skill, law) {
		t.Error("skills/bfd/SKILL.md does not vendor RULES.md verbatim from \"## The Rules\" onward — re-vendor the law and bump the plugin version")
	}
}

// TestParityDocsCarryTheWater pins each feature to the docs that must
// describe it. A hit here means a doc went stale, not that a doc is wrong.
func TestParityDocsCarryTheWater(t *testing.T) {
	expectations := []struct {
		Path     string
		Contains []string
	}{
		{Path: "README.md", Contains: []string{"LINT.md", "CONFORM.md"}},
		{Path: "CONFORM.md", Contains: []string{"BFD-17", "LINT.md", "languages"}},
		{Path: "LINT.md", Contains: []string{"lint/golangci.yml", "lint/ruff.toml", "lint/eslint.config.mjs"}},
		{Path: "AGENTS.md", Contains: []string{"lint/golangci.yml", "lint/ruff.toml", "lint/eslint.config.mjs"}},
		{Path: "skills/bfd/SKILL.md", Contains: []string{"lint/golangci.yml", "lint/ruff.toml", "lint/eslint.config.mjs", "BFD-17"}},
	}
	for _, expectation := range expectations {
		content := string(repoRead(t, expectation.Path))
		for _, needle := range expectation.Contains {
			if !strings.Contains(content, needle) {
				t.Errorf("%s does not mention %q — the material is stale; every surface says the same thing", expectation.Path, needle)
			}
		}
	}
}
