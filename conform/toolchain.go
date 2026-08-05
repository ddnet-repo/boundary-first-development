package conform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// The toolchain tier: BFD-17 says linters run on hooks and nothing merges
// without passing. This tier proves the wiring exists — never by running
// anyone's linter, only by reading the files that configure it. Languages
// are detected from their manifests; frameworks do not exist here.

type toolchainCheckInput struct {
	RootDir   string
	Languages []string // nil: detect from manifests; empty non-nil: tier disabled
	Report    func(finding Finding)
	Note      func(text string)
}

// toolchainLanguageNames are the languages the tier knows how to check.
var toolchainLanguageNames = []string{"go", "python", "js"}

// toolchainCheckResult names the languages whose gate was actually examined.
// A run that checked nothing at all is a run with nothing to say, and Run
// needs to know the difference.
type toolchainCheckResult struct {
	Checked []string
}

func toolchainCheckAll(input toolchainCheckInput) toolchainCheckResult {
	if input.Languages != nil && len(input.Languages) == 0 {
		return toolchainCheckResult{} // explicitly disabled: languages: []
	}
	checks := map[string]func(toolchainCheckInput){
		"go":     toolchainCheckGo,
		"python": toolchainCheckPython,
		"js":     toolchainCheckJs,
	}
	languages := input.Languages
	if languages == nil {
		languages = toolchainLanguagesDetect(toolchainDetectInput{RootDir: input.RootDir})
	}
	checked := []string{}
	for _, language := range languages {
		check, known := checks[language]
		if !known {
			input.Note(fmt.Sprintf("toolchain: unknown language %q in config — known: %s", language, strings.Join(toolchainLanguageNames, ", ")))
			continue
		}
		check(input)
		checked = append(checked, language)
	}
	return toolchainCheckResult{Checked: checked}
}

type toolchainDetectInput struct {
	RootDir string
}

// toolchainLanguagesDetect maps manifest files to languages. A manifest in
// the root is the whole proof; conform never guesses from source trees.
func toolchainLanguagesDetect(input toolchainDetectInput) []string {
	manifests := []struct {
		Language string
		Files    []string
	}{
		{Language: "go", Files: []string{"go.mod"}},
		{Language: "python", Files: []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt"}},
		{Language: "js", Files: []string{"package.json"}},
	}
	detected := []string{}
	for _, manifest := range manifests {
		for _, file := range manifest.Files {
			if toolchainFileExists(toolchainFileInput{RootDir: input.RootDir, Name: file}) {
				detected = append(detected, manifest.Language)
				break
			}
		}
	}
	return detected
}

type toolchainFileInput struct {
	RootDir string
	Name    string
}

func toolchainFileExists(input toolchainFileInput) bool {
	info, err := os.Stat(filepath.Join(input.RootDir, input.Name))
	return err == nil && !info.IsDir()
}

type toolchainFindInput struct {
	RootDir    string
	Candidates []string
}

// toolchainFileFind returns the first existing candidate, or "".
func toolchainFileFind(input toolchainFindInput) string {
	for _, candidate := range input.Candidates {
		if toolchainFileExists(toolchainFileInput{RootDir: input.RootDir, Name: candidate}) {
			return candidate
		}
	}
	return ""
}

// --- go ---

// toolchainGolangciRequired maps each linter the BFD preset enables to the
// rule it proves. A gate that does not enable them does not enforce the law.
var toolchainGolangciRequired = []struct {
	Linter string
	Proves string
}{
	{Linter: "exhaustive", Proves: "BFD-3"},
	{Linter: "forbidigo", Proves: "BFD-12"},
	{Linter: "godox", Proves: "BFD-29"},
	{Linter: "ireturn", Proves: "BFD-16"},
	{Linter: "nolintlint", Proves: "BFD-29"},
	{Linter: "revive", Proves: "BFD-15"},
	{Linter: "tagliatelle", Proves: "BFD-11"},
}

type toolchainGolangciFile struct {
	Linters toolchainGolangciLinters `yaml:"linters" toml:"linters"`
}

type toolchainGolangciLinters struct {
	Default string   `yaml:"default" toml:"default"`
	Enable  []string `yaml:"enable" toml:"enable"`
}

func toolchainCheckGo(input toolchainCheckInput) {
	candidates := []string{".golangci.yml", ".golangci.yaml", ".golangci.json", ".golangci.toml"}
	found := toolchainFileFind(toolchainFindInput{RootDir: input.RootDir, Candidates: candidates})
	if found == "" {
		input.Report(Finding{
			Rule:    "BFD-17",
			Where:   "toolchain: go",
			Message: "no golangci-lint config found — linters run on hooks, nothing merges without passing",
		})
		return
	}
	where := "toolchain: go (" + found + ")"
	raw, err := os.ReadFile(filepath.Join(input.RootDir, found))
	if err != nil {
		input.Note(fmt.Sprintf("toolchain: %s exists but could not be read (%v) — presence is all this run proves", found, err))
		return
	}
	var config toolchainGolangciFile
	if strings.HasSuffix(found, ".toml") {
		err = toml.Unmarshal(raw, &config)
	} else {
		err = yaml.Unmarshal(raw, &config) // JSON is a YAML subset; one parser covers both
	}
	if err != nil {
		input.Note(fmt.Sprintf("toolchain: %s does not parse (%v) — presence is all this run proves", found, err))
		return
	}
	if config.Linters.Default == "all" {
		return // everything is enabled; the gate holds
	}
	enabled := map[string]bool{}
	for _, linter := range config.Linters.Enable {
		enabled[linter] = true
	}
	for _, required := range toolchainGolangciRequired {
		if !enabled[required.Linter] {
			input.Report(Finding{
				Rule:    "BFD-17",
				Where:   where,
				Message: fmt.Sprintf("linter %q is not enabled — it proves %s; the gate must enforce the rules (see LINT.md)", required.Linter, required.Proves),
			})
		}
	}
}

// --- python ---

// toolchainRuffRequired maps each ruff selector the BFD preset selects to the
// rule it proves.
var toolchainRuffRequired = []struct {
	Selector string
	Proves   string
}{
	{Selector: "ANN401", Proves: "BFD-16"},
	{Selector: "DTZ", Proves: "BFD-12"},
	{Selector: "ERA", Proves: "BFD-29"},
	{Selector: "FBT", Proves: "BFD-15"},
	{Selector: "FIX", Proves: "BFD-29"},
	{Selector: "N", Proves: "BFD-11"},
	{Selector: "PGH", Proves: "BFD-29"},
	{Selector: "PLR0913", Proves: "BFD-15"},
	{Selector: "S110", Proves: "BFD-29"},
	{Selector: "S112", Proves: "BFD-29"},
}

type toolchainRuffLint struct {
	Select       []string            `toml:"select"`
	ExtendSelect []string            `toml:"extend-select"`
	Bandit       toolchainRuffBandit `toml:"flake8-bandit"`
}

// toolchainRuffBandit carries the one bandit setting BFD-29 depends on. Left
// at its default, S110 and S112 ignore `except KeyError: pass` — a swallowed
// failure the rule does not distinguish from any other.
type toolchainRuffBandit struct {
	CheckTypedException bool `toml:"check-typed-exception"`
}

type toolchainRuffConfig struct {
	Select       []string            `toml:"select"` // pre-lint-section layout, still read
	ExtendSelect []string            `toml:"extend-select"`
	Bandit       toolchainRuffBandit `toml:"flake8-bandit"`
	Lint         toolchainRuffLint   `toml:"lint"`
}

type toolchainPyprojectFile struct {
	Tool toolchainPyprojectTool `toml:"tool"`
}

type toolchainPyprojectTool struct {
	Ruff toolchainRuffConfig `toml:"ruff"`
}

func toolchainCheckPython(input toolchainCheckInput) {
	found := toolchainFileFind(toolchainFindInput{RootDir: input.RootDir, Candidates: []string{"ruff.toml", ".ruff.toml"}})
	var config toolchainRuffConfig
	if found != "" {
		if _, err := toml.DecodeFile(filepath.Join(input.RootDir, found), &config); err != nil {
			input.Note(fmt.Sprintf("toolchain: %s does not parse (%v) — presence is all this run proves", found, err))
			return
		}
	} else if toolchainFileExists(toolchainFileInput{RootDir: input.RootDir, Name: "pyproject.toml"}) {
		var pyproject toolchainPyprojectFile
		meta, err := toml.DecodeFile(filepath.Join(input.RootDir, "pyproject.toml"), &pyproject)
		if err != nil {
			input.Note(fmt.Sprintf("toolchain: pyproject.toml does not parse (%v) — presence is all this run proves", err))
			return
		}
		if !meta.IsDefined("tool", "ruff") {
			input.Report(Finding{
				Rule:    "BFD-17",
				Where:   "toolchain: python",
				Message: "no ruff config found (ruff.toml, .ruff.toml, or [tool.ruff] in pyproject.toml) — linters run on hooks, nothing merges without passing",
			})
			return
		}
		found = "pyproject.toml"
		config = pyproject.Tool.Ruff
	} else {
		input.Report(Finding{
			Rule:    "BFD-17",
			Where:   "toolchain: python",
			Message: "no ruff config found (ruff.toml, .ruff.toml, or [tool.ruff] in pyproject.toml) — linters run on hooks, nothing merges without passing",
		})
		return
	}
	selectors := append(append(append(append([]string{},
		config.Select...), config.ExtendSelect...), config.Lint.Select...), config.Lint.ExtendSelect...)
	where := "toolchain: python (" + found + ")"
	for _, required := range toolchainRuffRequired {
		if !toolchainSelectorCovered(toolchainSelectorInput{Selectors: selectors, Required: required.Selector}) {
			input.Report(Finding{
				Rule:    "BFD-17",
				Where:   where,
				Message: fmt.Sprintf("ruff selection does not cover %q — it proves %s; the gate must enforce the rules (see LINT.md)", required.Selector, required.Proves),
			})
		}
	}
	if !config.Bandit.CheckTypedException && !config.Lint.Bandit.CheckTypedException {
		input.Report(Finding{
			Rule:    "BFD-17",
			Where:   where,
			Message: `flake8-bandit "check-typed-exception" is not true — without it S110/S112 ignore "except SomeError: pass", and BFD-29 does not (see LINT.md)`,
		})
	}
}

// toolchainSelectorGroups are ruff's documented selectors that span several
// letter prefixes. A bare letter prefix otherwise names exactly one linter —
// "F" is pyflakes and does not cover "FBT".
var toolchainSelectorGroups = map[string][]string{
	"PL": {"PLC", "PLE", "PLR", "PLW"}, // pylint
}

// toolchainSelectorCovered reports whether a required ruff rule or rule group
// is covered by the configured selectors. "ALL" covers everything; a group
// selector covers its member prefixes; otherwise the letter prefixes must
// match exactly and one code must be a prefix of the other ("ANN" covers
// "ANN401"; a lone "FBT001" still counts toward "FBT").
func toolchainSelectorCovered(input toolchainSelectorInput) bool {
	requiredLetters := toolchainSelectorLetters(input.Required)
	for _, selector := range input.Selectors {
		if selector == "ALL" {
			return true
		}
		for _, member := range toolchainSelectorGroups[selector] {
			if member == requiredLetters {
				return true
			}
		}
		if toolchainSelectorLetters(selector) != requiredLetters {
			continue
		}
		if strings.HasPrefix(input.Required, selector) || strings.HasPrefix(selector, input.Required) {
			return true
		}
	}
	return false
}

type toolchainSelectorInput struct {
	Selectors []string
	Required  string
}

func toolchainSelectorLetters(selector string) string {
	for i, r := range selector {
		if r >= '0' && r <= '9' {
			return selector[:i]
		}
	}
	return selector
}

// --- js ---

type toolchainPackageFile struct {
	EslintConfig *toolchainEslintEmbedded `yaml:"eslintConfig"`
}

// toolchainEslintEmbedded only marks that package.json carries an
// "eslintConfig" key; its shape is ESLint's business.
type toolchainEslintEmbedded struct{}

func toolchainCheckJs(input toolchainCheckInput) {
	candidates := []string{
		"eslint.config.js", "eslint.config.mjs", "eslint.config.cjs",
		"eslint.config.ts", "eslint.config.mts", "eslint.config.cts",
		".eslintrc.js", ".eslintrc.cjs", ".eslintrc.json", ".eslintrc.yml", ".eslintrc.yaml",
	}
	found := toolchainFileFind(toolchainFindInput{RootDir: input.RootDir, Candidates: candidates})
	if found == "" && toolchainFileExists(toolchainFileInput{RootDir: input.RootDir, Name: "package.json"}) {
		raw, err := os.ReadFile(filepath.Join(input.RootDir, "package.json"))
		if err == nil {
			var pkg toolchainPackageFile
			if yaml.Unmarshal(raw, &pkg) == nil && pkg.EslintConfig != nil {
				found = "package.json"
			}
		}
	}
	if found == "" {
		input.Report(Finding{
			Rule:    "BFD-17",
			Where:   "toolchain: js",
			Message: "no eslint config found — linters run on hooks, nothing merges without passing",
		})
		return
	}
	input.Note(fmt.Sprintf("toolchain: js — %s found; eslint config is executable, so presence is what this tier proves (rule content stays with review)", found))
}
