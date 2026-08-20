package conform

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// The workflow tier: BFD-30 through BFD-33 say production advances only by
// tagged merges from release branches, staging is a cherry-picked projection
// that never merges back, and the pipeline that enforces the gates lives in
// the repository. Branch protections are forge configuration and differ on
// every forge — but the thing protections produce is a clean history graph,
// and the graph lives in the repository itself. This tier reads the graph
// with plain git plumbing, so it proves the same law on Codeberg, GitHub,
// GitLab, or a bare clone on a laptop.
//
// The tier judges the present, not the archaeology: the release window is
// production's first-parent line since the most recent version tag on it.
// Shipping a conforming release is what closes the old books — no epoch,
// no grace config, no way to quiet a scar except to ship correctly.

// WorkflowConfig names the branches the workflow rules judge. Zero values
// take the defaults: production "main" (then "master"), releases "release/*",
// staging "staging" plus "staging/*" and "testing/*", tags "v*".
type WorkflowConfig struct {
	Production string   `json:"production" yaml:"production"`
	Release    string   `json:"release" yaml:"release"`
	Staging    []string `json:"staging" yaml:"staging"`
	Tags       string   `json:"tags" yaml:"tags"`
}

type workflowCheckInput struct {
	RootDir string
	Config  WorkflowConfig
	Gated   bool // something exists to gate: a linted language or a spec
	Report  func(finding Finding)
	Note    func(text string)
}

// workflowCheckResult reports what the tier examined: the production branch
// it judged, or "" when there was nothing to judge (not a git repository, or
// no git on PATH). Run needs the difference between "clean" and "unchecked".
type workflowCheckResult struct {
	Production string
}

func workflowCheckAll(input workflowCheckInput) workflowCheckResult {
	if _, err := exec.LookPath("git"); err != nil {
		input.Note("workflow: git is not on PATH — the workflow tier did not run")
		return workflowCheckResult{}
	}
	inside := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-parse", "--is-inside-work-tree"}})
	if !inside.Ok {
		return workflowCheckResult{} // not a repository: nothing to judge
	}

	production := workflowProductionResolve(workflowProductionInput{RootDir: input.RootDir, Configured: input.Config.Production})
	if production == "" {
		wanted := input.Config.Production
		if wanted == "" {
			wanted = "main"
		}
		input.Report(Finding{
			Rule:    "BFD-30",
			Where:   "workflow: branches",
			Message: fmt.Sprintf("no production branch %q found — production is a branch, not a habit", wanted),
		})
		return workflowCheckResult{Production: "(missing)"}
	}

	tags := workflowTagsRead(workflowTagsReadInput{RootDir: input.RootDir, Pattern: workflowDefault(input.Config.Tags, "v*")})
	graph := workflowGraphRead(workflowGraphInput{RootDir: input.RootDir, Production: production, Tagged: tags.ByCommit})
	workflowCheckWindow(workflowWindowCheckInput{Graph: graph, Production: production, Report: input.Report})
	workflowCheckLatestTag(workflowLatestTagCheckInput{Graph: graph, Tags: tags, Production: production, Report: input.Report})
	workflowCheckStaging(workflowStagingCheckInput{
		RootDir: input.RootDir, Graph: graph, Production: production,
		Patterns: workflowStagingPatterns(input.Config.Staging), Report: input.Report,
	})
	if input.Gated {
		// The pipeline and hooks are owed where something exists to gate — a
		// linted language, a contract. A repository of prose owes no pipeline,
		// and a runner is deployment infrastructure, never evidence.
		workflowCheckPipeline(workflowPipelineCheckInput{RootDir: input.RootDir, Report: input.Report})
		workflowCheckHooks(workflowHooksCheckInput{RootDir: input.RootDir, Report: input.Report})
	}
	return workflowCheckResult{Production: production}
}

func workflowDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func workflowStagingPatterns(configured []string) []string {
	if len(configured) > 0 {
		return configured
	}
	return []string{"staging", "staging/*", "testing/*"}
}

// --- git plumbing -----------------------------------------------------------

type gitRunInput struct {
	RootDir string
	Args    []string
}

type gitRunResult struct {
	Ok    bool
	Lines []string
}

// gitRun executes one read-only git command. A failure is a false Ok, never
// an exception — callers decide whether absence is a finding or a skip.
func gitRun(input gitRunInput) gitRunResult {
	command := exec.Command("git", append([]string{"-C", input.RootDir}, input.Args...)...)
	output, err := command.Output()
	if err != nil {
		return gitRunResult{}
	}
	text := strings.TrimSpace(strings.ReplaceAll(string(output), "\r\n", "\n"))
	if text == "" {
		return gitRunResult{Ok: true, Lines: []string{}}
	}
	return gitRunResult{Ok: true, Lines: strings.Split(text, "\n")}
}

type workflowProductionInput struct {
	RootDir    string
	Configured string
}

// workflowProductionResolve finds the production branch: the configured name,
// else main, else master — as a local head first, a remote head second.
func workflowProductionResolve(input workflowProductionInput) string {
	candidates := []string{input.Configured}
	if input.Configured == "" {
		candidates = []string{"main", "master"}
	}
	for _, name := range candidates {
		for _, ref := range []string{"refs/heads/" + name, "refs/remotes/origin/" + name} {
			verified := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-parse", "--verify", "--quiet", ref}})
			if verified.Ok && len(verified.Lines) == 1 {
				return name
			}
		}
	}
	return ""
}

// --- tags ---------------------------------------------------------------------

type workflowTags struct {
	ByCommit map[string]string // peeled commit sha -> tag name
	Latest   string            // highest version tag by version sort; "" when none
	LatestAt string            // the commit the latest tag points to
}

type workflowTagsReadInput struct {
	RootDir string
	Pattern string
}

func workflowTagsRead(input workflowTagsReadInput) workflowTags {
	tags := workflowTags{ByCommit: map[string]string{}}
	listed := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"tag", "--list", input.Pattern, "--sort=-v:refname"}})
	for index, tag := range listed.Lines {
		peeled := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-parse", "--verify", "--quiet", tag + "^{commit}"}})
		if !peeled.Ok || len(peeled.Lines) != 1 {
			continue
		}
		tags.ByCommit[peeled.Lines[0]] = tag
		if index == 0 {
			tags.Latest = tag
			tags.LatestAt = peeled.Lines[0]
		}
	}
	return tags
}

// --- the release window -------------------------------------------------------

// workflowGraph is production's first-parent line: the sequence of states
// production has actually been in. The release window is the part of the line
// newer than the most recent version tag on it — what has happened since the
// last release shipped. A repository with no tagged release has an unbounded
// window, because until one release ships through the door the workflow is
// not set up, and the findings should say so.
type workflowGraph struct {
	FirstParentAll map[string]bool // full line, for "is this commit a state of production"
	Window         []workflowGraphCommit
	WindowBase     string // the tag that closes the window; "" when no release ever shipped
}

type workflowGraphCommit struct {
	Sha     string
	Parents int
}

type workflowGraphInput struct {
	RootDir    string
	Production string
	Tagged     map[string]string // peeled commit sha -> tag name
}

func workflowGraphRead(input workflowGraphInput) workflowGraph {
	graph := workflowGraph{FirstParentAll: map[string]bool{}}
	full := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-list", "--first-parent", "--parents", input.Production}})
	open := true
	for _, line := range full.Lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		commit := workflowGraphCommit{Sha: fields[0], Parents: len(fields) - 1}
		graph.FirstParentAll[commit.Sha] = true
		if open {
			if tag, tagged := input.Tagged[commit.Sha]; tagged {
				graph.WindowBase = tag
				open = false
				continue
			}
			graph.Window = append(graph.Window, commit)
		}
	}
	return graph
}

// --- BFD-30: the window holds only tagged merges ------------------------------

type workflowWindowCheckInput struct {
	Graph      workflowGraph
	Production string
	Report     func(finding Finding)
}

func workflowCheckWindow(input workflowWindowCheckInput) {
	direct := []string{}
	merges := []string{}
	for _, commit := range input.Graph.Window {
		switch {
		case commit.Parents == 1: // 0 parents is the root: history starts somewhere
			direct = append(direct, commit.Sha)
		case commit.Parents >= 2: // in the window means newer than any tag: untagged
			merges = append(merges, commit.Sha)
		}
	}
	since := "since the last release"
	if input.Graph.WindowBase == "" {
		since = "and no release has ever shipped through the door"
	}
	if len(direct) > 0 {
		input.Report(Finding{
			Rule:  "BFD-30",
			Where: "workflow: " + input.Production,
			Message: fmt.Sprintf("%d direct commit(s) on %s %s (latest %.7s) — production advances only by merges from release or hotfix branches",
				len(direct), input.Production, since, direct[0]),
		})
	}
	if len(merges) > 0 {
		input.Report(Finding{
			Rule:  "BFD-30",
			Where: "workflow: " + input.Production,
			Message: fmt.Sprintf("%d merge(s) into %s %s carry no version tag (latest %.7s) — every landing on production is a release, and releases have versions",
				len(merges), input.Production, since, merges[0]),
		})
	}
}

// --- BFD-31: the current release is what production says it is ----------------

type workflowLatestTagCheckInput struct {
	Graph      workflowGraph
	Tags       workflowTags
	Production string
	Report     func(finding Finding)
}

func workflowCheckLatestTag(input workflowLatestTagCheckInput) {
	if input.Tags.Latest == "" || input.Graph.FirstParentAll[input.Tags.LatestAt] {
		return
	}
	input.Report(Finding{
		Rule:    "BFD-31",
		Where:   "workflow: tag " + input.Tags.Latest,
		Message: fmt.Sprintf("the current release tag %s does not sit on %s's first-parent line — a release that did not ship through the door", input.Tags.Latest, input.Production),
	})
}

// --- BFD-32: staging is a projection ----------------------------------------

type workflowStagingCheckInput struct {
	RootDir    string
	Graph      workflowGraph
	Production string
	Patterns   []string
	Report     func(finding Finding)
}

func workflowCheckStaging(input workflowStagingCheckInput) {
	listed := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes"}})
	seen := map[string]bool{}
	for _, ref := range listed.Lines {
		name := ref
		if slash := strings.Index(name, "/"); slash >= 0 && !workflowPatternMatch(input.Patterns, name) {
			name = name[slash+1:] // "origin/staging" judges as "staging"
		}
		if !workflowPatternMatch(input.Patterns, name) || seen[name] {
			continue
		}
		seen[name] = true

		tip := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-parse", "--verify", "--quiet", ref}})
		if !tip.Ok || len(tip.Lines) != 1 {
			continue
		}
		merged := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"merge-base", "--is-ancestor", tip.Lines[0], input.Production}})
		if merged.Ok && !input.Graph.FirstParentAll[tip.Lines[0]] {
			input.Report(Finding{
				Rule:    "BFD-32",
				Where:   "workflow: " + name,
				Message: fmt.Sprintf("%s was merged into %s — staging is a projection; it is rebuilt from production, never merged back", name, input.Production),
			})
		}

		unrecorded := []string{}
		for _, sha := range gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-list", ref, "^" + input.Production}}).Lines {
			body := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"show", "-s", "--format=%B", sha}})
			recorded := false
			for _, line := range body.Lines {
				if strings.Contains(line, "(cherry picked from commit ") {
					recorded = true
				}
			}
			if !recorded {
				unrecorded = append(unrecorded, sha)
			}
		}
		if len(unrecorded) > 0 {
			input.Report(Finding{
				Rule:  "BFD-32",
				Where: "workflow: " + name,
				Message: fmt.Sprintf("%d commit(s) on %s carry no recorded cherry-pick provenance (latest %.7s) — staging is populated with git cherry-pick -x",
					len(unrecorded), name, unrecorded[0]),
			})
		}
	}
}

func workflowPatternMatch(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if matched, err := path.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

// --- BFD-33: the pipeline lives in the repository ----------------------------

// workflowPipelineFiles are the CI configurations the tier recognizes, by
// forge and by tool. Recognition is by committed file — the one thing every
// CI system agrees on — never by calling a forge API.
var workflowPipelineFiles = []string{
	".forgejo/workflows", ".gitea/workflows", ".github/workflows",
	".gitlab-ci.yml", ".woodpecker.yml", ".woodpecker",
	"Jenkinsfile", ".circleci/config.yml", "azure-pipelines.yml", "bitbucket-pipelines.yml",
}

type workflowPipelineCheckInput struct {
	RootDir string
	Report  func(finding Finding)
}

func workflowCheckPipeline(input workflowPipelineCheckInput) {
	contents := []string{}
	found := []string{}
	for _, candidate := range workflowPipelineFiles {
		full := filepath.Join(input.RootDir, filepath.FromSlash(candidate))
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			found = append(found, candidate)
			if raw, readErr := os.ReadFile(full); readErr == nil {
				contents = append(contents, string(raw))
			}
			continue
		}
		entries, readErr := os.ReadDir(full)
		if readErr != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
				continue
			}
			found = append(found, candidate+"/"+name)
			if raw, readErr := os.ReadFile(filepath.Join(full, name)); readErr == nil {
				contents = append(contents, string(raw))
			}
		}
	}
	if len(found) == 0 {
		input.Report(Finding{
			Rule:    "BFD-33",
			Where:   "workflow: ci",
			Message: "no CI configuration recognized — the pipeline lives in the repository, and there is no pipeline here",
		})
		return
	}
	for _, content := range contents {
		if strings.Contains(content, "conform") {
			return
		}
	}
	input.Report(Finding{
		Rule:    "BFD-33",
		Where:   "workflow: ci",
		Message: fmt.Sprintf("CI configuration found (%s) but none of it invokes bfd conform — a pipeline that skips the gate is scenery", strings.Join(found, ", ")),
	})
}

// --- BFD-17: the gate runs on hooks ------------------------------------------

// workflowHookFiles are the committed hook-manager configurations the tier
// recognizes. Local git hooks are not committed and so cannot be evidence;
// a hook manager's config is.
var workflowHookFiles = []string{
	"lefthook.yml", ".lefthook.yml", "lefthook.yaml", ".lefthook.yaml", "lefthook.toml",
	".pre-commit-config.yaml", ".pre-commit-config.yml",
	".husky", ".githooks",
}

type workflowHooksCheckInput struct {
	RootDir string
	Report  func(finding Finding)
}

func workflowCheckHooks(input workflowHooksCheckInput) {
	for _, candidate := range workflowHookFiles {
		if _, err := os.Stat(filepath.Join(input.RootDir, candidate)); err == nil {
			return
		}
	}
	input.Report(Finding{
		Rule:    "BFD-17",
		Where:   "workflow: hooks",
		Message: "no hook-manager configuration found (lefthook, pre-commit, husky, .githooks) — the gate runs on hooks, and hooks that exist only on one laptop do not exist",
	})
}
