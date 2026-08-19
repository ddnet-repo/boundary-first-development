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
	Directory string   // the module being checked, relative to RootDir; "" is the root
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
	modules := toolchainModulesFind(toolchainModulesInput{RootDir: input.RootDir})
	if input.Languages != nil {
		modules = toolchainModulesPin(toolchainModulesPinInput{
			Modules:   modules,
			Languages: input.Languages,
			Known:     checks,
			Note:      input.Note,
		})
	}
	checked := []string{}
	seen := map[string]bool{}
	for _, module := range modules {
		checks[module.Language](toolchainCheckInput{
			RootDir:   input.RootDir,
			Directory: module.Directory,
			Report:    input.Report,
			Note:      input.Note,
		})
		if !seen[module.Language] {
			seen[module.Language] = true
			checked = append(checked, module.Language)
		}
	}
	return toolchainCheckResult{Checked: checked}
}

// toolchainModule is one unit of a repository that declares a language by
// carrying that language's manifest.
type toolchainModule struct {
	Directory string // relative to the repo root; "" is the root itself
	Language  string
}

type toolchainModulesInput struct {
	RootDir string
}

// toolchainModuleDepthMax bounds the walk. Four levels reaches the layouts
// monorepos actually use — apps/web, packages/ui, services/api — without
// turning the tier into a filesystem crawl.
const toolchainModuleDepthMax = 4

// toolchainDirectorySkip are directories that hold other people's code or
// build output. Dot-directories are skipped by name.
var toolchainDirectorySkip = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, "venv": true, "__pycache__": true, "testdata": true,
}

var toolchainManifests = []struct {
	Language string
	Files    []string
}{
	{Language: "go", Files: []string{"go.mod"}},
	{Language: "python", Files: []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt"}},
	{Language: "js", Files: []string{"package.json"}},
}

// toolchainModulesFind walks the tree for manifests. A manifest is a module
// declaring that it exists — conform still never guesses from source trees,
// and a monorepo's frontends declare themselves exactly as its backend does,
// which is why looking only at the repo root was never right.
func toolchainModulesFind(input toolchainModulesInput) []toolchainModule {
	modules := []toolchainModule{}
	var walk func(directory string, depth int)
	walk = func(directory string, depth int) {
		base := filepath.Join(input.RootDir, directory)
		for _, manifest := range toolchainManifests {
			for _, file := range manifest.Files {
				if toolchainFileExists(toolchainFileInput{RootDir: base, Name: file}) {
					modules = append(modules, toolchainModule{Directory: directory, Language: manifest.Language})
					break
				}
			}
		}
		if depth >= toolchainModuleDepthMax {
			return
		}
		entries, err := os.ReadDir(base)
		if err != nil {
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || strings.HasPrefix(name, ".") || toolchainDirectorySkip[name] {
				continue
			}
			walk(filepath.ToSlash(filepath.Join(directory, name)), depth+1)
		}
	}
	walk("", 0)
	return modules
}

type toolchainModulesPinInput struct {
	Modules   []toolchainModule
	Languages []string
	Known     map[string]func(toolchainCheckInput)
	Note      func(text string)
}

// toolchainModulesPin applies an explicit languages list: it keeps only the
// named languages, and for a named language with no manifest anywhere it
// still checks the root — declaring "this is a Go project" is a claim conform
// honours even when the manifest is somewhere it cannot see.
func toolchainModulesPin(input toolchainModulesPinInput) []toolchainModule {
	pinned := []toolchainModule{}
	for _, language := range input.Languages {
		if _, known := input.Known[language]; !known {
			input.Note(fmt.Sprintf("toolchain: unknown language %q in config — known: %s", language, strings.Join(toolchainLanguageNames, ", ")))
			continue
		}
		found := false
		for _, module := range input.Modules {
			if module.Language == language {
				pinned = append(pinned, module)
				found = true
			}
		}
		if !found {
			pinned = append(pinned, toolchainModule{Language: language})
		}
	}
	return pinned
}

// toolchainAncestors lists a module's own directory and every ancestor up to
// the repo root, nearest first — the order every linter resolves its config
// in. One config at the top of a workspace covers the packages beneath it.
func toolchainAncestors(directory string) []string {
	directory = filepath.Clean(directory)
	if directory == "." || directory == string(filepath.Separator) {
		return []string{""}
	}
	chain := []string{}
	for {
		chain = append(chain, directory)
		parent := filepath.Dir(directory)
		if parent == "." || parent == directory {
			break
		}
		directory = parent
	}
	return append(chain, "")
}

type toolchainConfigInput struct {
	RootDir    string
	Directory  string
	Candidates []string
}

// toolchainConfigFind returns the nearest matching config, as a path relative
// to the repo root, or "" when no ancestor carries one.
func toolchainConfigFind(input toolchainConfigInput) string {
	for _, directory := range toolchainAncestors(input.Directory) {
		base := filepath.Join(input.RootDir, directory)
		for _, candidate := range input.Candidates {
			if toolchainFileExists(toolchainFileInput{RootDir: base, Name: candidate}) {
				return filepath.Join(directory, candidate)
			}
		}
	}
	return ""
}

type toolchainWhereInput struct {
	Language  string
	Directory string
	Config    string
}

// toolchainWhere labels a finding with the module it belongs to, so a
// monorepo's findings say which package is missing its gate.
func toolchainWhere(input toolchainWhereInput) string {
	where := "toolchain: " + input.Language
	if input.Directory != "" {
		where += " in " + input.Directory
	}
	if input.Config != "" {
		where += " (" + input.Config + ")"
	}
	return where
}

type toolchainFileInput struct {
	RootDir string
	Name    string
}

func toolchainFileExists(input toolchainFileInput) bool {
	info, err := os.Stat(filepath.Join(input.RootDir, input.Name))
	return err == nil && !info.IsDir()
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
	found := toolchainConfigFind(toolchainConfigInput{RootDir: input.RootDir, Directory: input.Directory, Candidates: candidates})
	if found == "" {
		input.Report(Finding{
			Rule:    "BFD-17",
			Where:   toolchainWhere(toolchainWhereInput{Language: "go", Directory: input.Directory}),
			Message: "no golangci-lint config found — linters run on hooks, nothing merges without passing",
		})
		return
	}
	where := toolchainWhere(toolchainWhereInput{Language: "go", Directory: input.Directory, Config: found})
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

type toolchainRuffLocated struct {
	Path   string              // root-relative path of the config that governs this module
	Config toolchainRuffConfig // its parsed contents
	Parsed string              // a note, when a file was found but could not be read
}

// toolchainRuffLocate walks from the module up to the repo root the way ruff
// itself resolves configuration: at each level a ruff.toml wins, otherwise a
// pyproject.toml that actually declares [tool.ruff]. A pyproject without that
// table is not a ruff config, so the search keeps climbing.
func toolchainRuffLocate(input toolchainCheckInput) toolchainRuffLocated {
	for _, directory := range toolchainAncestors(input.Directory) {
		base := filepath.Join(input.RootDir, directory)
		for _, name := range []string{"ruff.toml", ".ruff.toml"} {
			if !toolchainFileExists(toolchainFileInput{RootDir: base, Name: name}) {
				continue
			}
			var config toolchainRuffConfig
			if _, err := toml.DecodeFile(filepath.Join(base, name), &config); err != nil {
				return toolchainRuffLocated{Parsed: fmt.Sprintf("toolchain: %s does not parse (%v) — presence is all this run proves", filepath.Join(directory, name), err)}
			}
			return toolchainRuffLocated{Path: filepath.Join(directory, name), Config: config}
		}
		if !toolchainFileExists(toolchainFileInput{RootDir: base, Name: "pyproject.toml"}) {
			continue
		}
		var pyproject toolchainPyprojectFile
		meta, err := toml.DecodeFile(filepath.Join(base, "pyproject.toml"), &pyproject)
		if err != nil {
			return toolchainRuffLocated{Parsed: fmt.Sprintf("toolchain: %s does not parse (%v) — presence is all this run proves", filepath.Join(directory, "pyproject.toml"), err)}
		}
		if meta.IsDefined("tool", "ruff") {
			return toolchainRuffLocated{Path: filepath.Join(directory, "pyproject.toml"), Config: pyproject.Tool.Ruff}
		}
	}
	return toolchainRuffLocated{}
}

type toolchainPyprojectFile struct {
	Tool toolchainPyprojectTool `toml:"tool"`
}

type toolchainPyprojectTool struct {
	Ruff toolchainRuffConfig `toml:"ruff"`
}

func toolchainCheckPython(input toolchainCheckInput) {
	located := toolchainRuffLocate(input)
	if located.Parsed != "" {
		input.Note(located.Parsed)
		return
	}
	if located.Path == "" {
		input.Report(Finding{
			Rule:    "BFD-17",
			Where:   toolchainWhere(toolchainWhereInput{Language: "python", Directory: input.Directory}),
			Message: "no ruff config found (ruff.toml, .ruff.toml, or [tool.ruff] in pyproject.toml) — linters run on hooks, nothing merges without passing",
		})
		return
	}
	config := located.Config
	selectors := append(append(append(append([]string{},
		config.Select...), config.ExtendSelect...), config.Lint.Select...), config.Lint.ExtendSelect...)
	where := toolchainWhere(toolchainWhereInput{Language: "python", Directory: input.Directory, Config: located.Path})
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
	found := ""
	for _, directory := range toolchainAncestors(input.Directory) {
		base := filepath.Join(input.RootDir, directory)
		for _, candidate := range candidates {
			if toolchainFileExists(toolchainFileInput{RootDir: base, Name: candidate}) {
				found = filepath.Join(directory, candidate)
				break
			}
		}
		if found == "" && toolchainFileExists(toolchainFileInput{RootDir: base, Name: "package.json"}) {
			raw, err := os.ReadFile(filepath.Join(base, "package.json"))
			if err == nil {
				var pkg toolchainPackageFile
				if yaml.Unmarshal(raw, &pkg) == nil && pkg.EslintConfig != nil {
					found = filepath.Join(directory, "package.json")
				}
			}
		}
		if found != "" {
			break
		}
	}
	if found == "" {
		input.Report(Finding{
			Rule:    "BFD-17",
			Where:   toolchainWhere(toolchainWhereInput{Language: "js", Directory: input.Directory}),
			Message: "no eslint config found — linters run on hooks, nothing merges without passing",
		})
		return
	}
	input.Note(fmt.Sprintf("toolchain: js — %s found; eslint config is executable, so presence is what this tier proves (rule content stays with review)", found))
}
