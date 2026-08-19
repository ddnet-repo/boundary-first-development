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
// GitLab, or a bare clone on a laptop. Evidence, not configuration: an
// escape may skip a hook, but it cannot skip having been recorded.

// WorkflowConfig names the branches the workflow rules judge. Zero values
// take the defaults: production "main" (then "master"), releases "release/*",
// staging "staging" plus "staging/*" and "testing/*", tags "v*". Epoch is the
// revision workflow adoption started at; history before it is not judged, so
// a repository can adopt the law without drowning in its own past.
type WorkflowConfig struct {
	Production string   `json:"production" yaml:"production"`
	Release    string   `json:"release" yaml:"release"`
	Staging    []string `json:"staging" yaml:"staging"`
	Tags       string   `json:"tags" yaml:"tags"`
	Epoch      string   `json:"epoch" yaml:"epoch"`
}

type workflowCheckInput struct {
	RootDir string
	Config  WorkflowConfig
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

	epoch := ""
	if input.Config.Epoch != "" {
		resolved := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-parse", "--verify", "--quiet", input.Config.Epoch + "^{commit}"}})
		if resolved.Ok && len(resolved.Lines) == 1 {
			epoch = resolved.Lines[0]
		} else {
			input.Note(fmt.Sprintf("workflow: epoch %q does not resolve — judging the full history", input.Config.Epoch))
		}
	}

	graph := workflowGraphRead(workflowGraphInput{RootDir: input.RootDir, Production: production, Epoch: epoch})
	workflowCheckProduction(workflowProductionCheckInput{Graph: graph, Production: production, Report: input.Report})
	workflowCheckTags(workflowTagsCheckInput{
		RootDir: input.RootDir, Graph: graph, Production: production,
		Pattern: workflowDefault(input.Config.Tags, "v*"), Epoch: epoch, Report: input.Report,
	})
	workflowCheckStaging(workflowStagingCheckInput{
		RootDir: input.RootDir, Graph: graph, Production: production,
		Patterns: workflowStagingPatterns(input.Config.Staging), Epoch: epoch, Report: input.Report,
	})
	workflowCheckPipeline(workflowPipelineCheckInput{RootDir: input.RootDir, Report: input.Report})
	workflowCheckHooks(workflowHooksCheckInput{RootDir: input.RootDir, Report: input.Report})
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

// workflowGraph is production's first-parent line: the sequence of states
// production has actually been in. Everything the tier judges hangs off it.
type workflowGraph struct {
	FirstParentAll   map[string]bool // full line, for "is this commit a state of production"
	SinceEpoch       []workflowGraphCommit
	SinceEpochMerges []string // shas of merge commits since the epoch
}

type workflowGraphCommit struct {
	Sha     string
	Parents int
}

type workflowGraphInput struct {
	RootDir    string
	Production string
	Epoch      string
}

func workflowGraphRead(input workflowGraphInput) workflowGraph {
	graph := workflowGraph{FirstParentAll: map[string]bool{}}
	full := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-list", "--first-parent", input.Production}})
	for _, sha := range full.Lines {
		graph.FirstParentAll[sha] = true
	}
	args := []string{"rev-list", "--first-parent", "--parents", input.Production}
	if input.Epoch != "" {
		args = append(args, "^"+input.Epoch)
	}
	windowed := gitRun(gitRunInput{RootDir: input.RootDir, Args: args})
	for _, line := range windowed.Lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		commit := workflowGraphCommit{Sha: fields[0], Parents: len(fields) - 1}
		graph.SinceEpoch = append(graph.SinceEpoch, commit)
		if commit.Parents >= 2 {
			graph.SinceEpochMerges = append(graph.SinceEpochMerges, commit.Sha)
		}
	}
	return graph
}

// --- BFD-30: production advances only by tagged merges -----------------------

type workflowProductionCheckInput struct {
	Graph      workflowGraph
	Production string
	Report     func(finding Finding)
}

func workflowCheckProduction(input workflowProductionCheckInput) {
	direct := []string{}
	for _, commit := range input.Graph.SinceEpoch {
		if commit.Parents == 1 { // 0 parents is the root: history starts somewhere
			direct = append(direct, commit.Sha)
		}
	}
	if len(direct) > 0 {
		input.Report(Finding{
			Rule:  "BFD-30",
			Where: "workflow: " + input.Production,
			Message: fmt.Sprintf("%d direct commit(s) on %s since the epoch (latest %.7s) — production advances only by merges from release or hotfix branches",
				len(direct), input.Production, direct[0]),
		})
	}
}

// --- BFD-30/31: merges are tagged, tags live on production -------------------

type workflowTagsCheckInput struct {
	RootDir    string
	Graph      workflowGraph
	Production string
	Pattern    string
	Epoch      string
	Report     func(finding Finding)
}

func workflowCheckTags(input workflowTagsCheckInput) {
	listed := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"tag", "--list", input.Pattern}})
	tagged := map[string]string{} // commit sha -> tag name
	for _, tag := range listed.Lines {
		peeled := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"rev-parse", "--verify", "--quiet", tag + "^{commit}"}})
		if !peeled.Ok || len(peeled.Lines) != 1 {
			continue
		}
		sha := peeled.Lines[0]
		tagged[sha] = tag
		if input.Graph.FirstParentAll[sha] {
			continue
		}
		if input.Epoch != "" {
			ancestor := gitRun(gitRunInput{RootDir: input.RootDir, Args: []string{"merge-base", "--is-ancestor", input.Epoch, sha}})
			if !ancestor.Ok {
				continue // the tag predates adoption; the past is not on trial
			}
		}
		input.Report(Finding{
			Rule:    "BFD-31",
			Where:   "workflow: tag " + tag,
			Message: fmt.Sprintf("version tag %s does not sit on %s's first-parent line — a release that did not ship through the door", tag, input.Production),
		})
	}
	untagged := []string{}
	for _, sha := range input.Graph.SinceEpochMerges {
		if _, ok := tagged[sha]; !ok {
			untagged = append(untagged, sha)
		}
	}
	if len(untagged) > 0 {
		input.Report(Finding{
			Rule:  "BFD-30",
			Where: "workflow: " + input.Production,
			Message: fmt.Sprintf("%d merge(s) into %s since the epoch carry no %q tag (latest %.7s) — every landing on production is a release, and releases have versions",
				len(untagged), input.Production, input.Pattern, untagged[0]),
		})
	}
}

// --- BFD-32: staging is a projection ----------------------------------------

type workflowStagingCheckInput struct {
	RootDir    string
	Graph      workflowGraph
	Production string
	Patterns   []string
	Epoch      string
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

		ownArgs := []string{"rev-list", ref, "^" + input.Production}
		if input.Epoch != "" {
			ownArgs = append(ownArgs, "^"+input.Epoch)
		}
		unrecorded := []string{}
		for _, sha := range gitRun(gitRunInput{RootDir: input.RootDir, Args: ownArgs}).Lines {
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
