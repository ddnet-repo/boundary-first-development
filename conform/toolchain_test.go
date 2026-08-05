package conform

import (
	"os"
	"path/filepath"
	"testing"
)

// Toolchain-tier tests, at the package boundary like everything else: every
// test calls Run against a temporary project root and asserts on the envelope.

func projectWrite(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("cannot write %s: %v", name, err)
		}
	}
	return root
}

type runToolchainInput struct {
	Root      string
	Languages []string
}

func runToolchain(t *testing.T, input runToolchainInput) RunResult {
	t.Helper()
	return Run(RunInput{SpecPath: "testdata/spec_clean.yaml", RootDir: input.Root, Languages: input.Languages})
}

type requireCountInput struct {
	Result RunResult
	Count  int
}

func requireFindingCount(t *testing.T, input requireCountInput) {
	t.Helper()
	if !input.Result.Ok {
		t.Fatalf("run failed: %s (%s)", input.Result.Error.Message, input.Result.Error.Code)
	}
	if len(input.Result.Data.Findings) != input.Count {
		t.Errorf("expected %d findings, got %+v", input.Count, input.Result.Data.Findings)
	}
}

const golangciConforming = `version: "2"
linters:
  default: none
  enable: [exhaustive, forbidigo, godox, ireturn, nolintlint, revive, tagliatelle]
`

// The toolchain tier is the one tier a project can fail without owning an API
// at all, so it must run when there is no spec and no base URL to pair it with.
func TestRunToolchainRunsWithoutSpecOrWire(t *testing.T) {
	root := projectWrite(t, map[string]string{"go.mod": "module example.test\n"})
	result := Run(RunInput{RootDir: root})
	if !result.Ok {
		t.Fatalf("expected a completed run, got error %+v", result.Error)
	}
	requireRules(t, requireRulesInput{Result: result, Rules: []string{"BFD-17"}})
	if len(result.Data.Languages) != 1 || result.Data.Languages[0] != "go" {
		t.Errorf("expected the go gate to be reported as checked, got %v", result.Data.Languages)
	}
}

func TestRunToolchainGoNoConfig(t *testing.T) {
	root := projectWrite(t, map[string]string{"go.mod": "module example.test\n"})
	requireRules(t, requireRulesInput{Result: runToolchain(t, runToolchainInput{Root: root}), Rules: []string{"BFD-17"}})
}

func TestRunToolchainGoConfigured(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"go.mod":        "module example.test\n",
		".golangci.yml": golangciConforming,
	})
	requireFindingCount(t, requireCountInput{Result: runToolchain(t, runToolchainInput{Root: root}), Count: 0})
}

func TestRunToolchainGoPartial(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"go.mod":        "module example.test\n",
		".golangci.yml": "version: \"2\"\nlinters:\n  enable: [revive]\n",
	})
	result := runToolchain(t, runToolchainInput{Root: root})
	requireRules(t, requireRulesInput{Result: result, Rules: []string{"BFD-17"}})
	requireFindingCount(t, requireCountInput{Result: result, Count: 6}) // exhaustive, forbidigo, godox, ireturn, nolintlint, tagliatelle missing
}

func TestRunToolchainPythonNoRuff(t *testing.T) {
	root := projectWrite(t, map[string]string{"pyproject.toml": "[project]\nname = \"example\"\n"})
	requireRules(t, requireRulesInput{Result: runToolchain(t, runToolchainInput{Root: root}), Rules: []string{"BFD-17"}})
}

func TestRunToolchainPythonRuffAll(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"pyproject.toml": "[project]\nname = \"example\"\n",
		"ruff.toml":      "[lint]\nselect = [\"ALL\"]\n[lint.flake8-bandit]\ncheck-typed-exception = true\n",
	})
	requireFindingCount(t, requireCountInput{Result: runToolchain(t, runToolchainInput{Root: root}), Count: 0})
}

func TestRunToolchainPythonRuffSelectors(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"pyproject.toml": "[project]\nname = \"example\"\n[tool.ruff.lint]\nselect = [\"ANN\", \"DTZ\", \"ERA\", \"FBT\", \"FIX\", \"N\", \"PGH\", \"PL\", \"S\"]\n[tool.ruff.lint.flake8-bandit]\ncheck-typed-exception = true\n",
	})
	requireFindingCount(t, requireCountInput{Result: runToolchain(t, runToolchainInput{Root: root}), Count: 0})
}

// Selecting S110/S112 is not the same as arming them: left at its default,
// flake8-bandit ignores `except SomeError: pass`, which BFD-29 does not.
func TestRunToolchainPythonRuffBanditUnarmed(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"pyproject.toml": "[project]\nname = \"example\"\n[tool.ruff.lint]\nselect = [\"ALL\"]\n",
	})
	result := runToolchain(t, runToolchainInput{Root: root})
	requireRules(t, requireRulesInput{Result: result, Rules: []string{"BFD-17"}})
	requireFindingCount(t, requireCountInput{Result: result, Count: 1})
}

func TestRunToolchainPythonRuffInsufficient(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"pyproject.toml": "[project]\nname = \"example\"\n[tool.ruff.lint]\nselect = [\"E\", \"F\"]\n",
	})
	result := runToolchain(t, runToolchainInput{Root: root})
	requireRules(t, requireRulesInput{Result: result, Rules: []string{"BFD-17"}})
	requireFindingCount(t, requireCountInput{Result: result, Count: 11}) // ten selectors uncovered, plus check-typed-exception unset
}

func TestRunToolchainJsNoConfig(t *testing.T) {
	root := projectWrite(t, map[string]string{"package.json": "{\"name\": \"example\"}\n"})
	requireRules(t, requireRulesInput{Result: runToolchain(t, runToolchainInput{Root: root}), Rules: []string{"BFD-17"}})
}

func TestRunToolchainJsFlatConfig(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"package.json":      "{\"name\": \"example\"}\n",
		"eslint.config.mjs": "export default [];\n",
	})
	requireFindingCount(t, requireCountInput{Result: runToolchain(t, runToolchainInput{Root: root}), Count: 0})
}

func TestRunToolchainJsEmbeddedConfig(t *testing.T) {
	root := projectWrite(t, map[string]string{
		"package.json": "{\"name\": \"example\", \"eslintConfig\": {\"root\": true}}\n",
	})
	requireFindingCount(t, requireCountInput{Result: runToolchain(t, runToolchainInput{Root: root}), Count: 0})
}

func TestRunToolchainDisabled(t *testing.T) {
	root := projectWrite(t, map[string]string{"go.mod": "module example.test\n"})
	requireFindingCount(t, requireCountInput{Result: runToolchain(t, runToolchainInput{Root: root, Languages: []string{}}), Count: 0})
}

func TestRunToolchainExplicitLanguages(t *testing.T) {
	root := projectWrite(t, map[string]string{"package.json": "{\"name\": \"example\"}\n"})
	// explicit [go] means: check go, ignore the detected js
	result := runToolchain(t, runToolchainInput{Root: root, Languages: []string{"go"}})
	requireRules(t, requireRulesInput{Result: result, Rules: []string{"BFD-17"}})
	requireFindingCount(t, requireCountInput{Result: result, Count: 1})
}

func TestRunToolchainUnknownLanguage(t *testing.T) {
	root := projectWrite(t, map[string]string{})
	result := runToolchain(t, runToolchainInput{Root: root, Languages: []string{"cobol"}})
	requireFindingCount(t, requireCountInput{Result: result, Count: 0})
	if len(result.Data.Notes) == 0 {
		t.Errorf("expected a note about the unknown language, got none")
	}
}
