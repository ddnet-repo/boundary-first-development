package conform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The workflow tier is tested against real repositories: each test builds the
// graph shape it judges with actual git commands. If git is not on the test
// machine, the tier has nothing to prove and the tests say so.

func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	base := []string{"-C", dir,
		"-c", "user.name=conform-test", "-c", "user.email=conform@test.invalid",
		"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false",
	}
	command := exec.Command("git", append(base, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func workflowRepoNew(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q", "-b", "main")
	workflowFileWrite(t, dir, "README.md", "hello\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "root")
	return dir
}

func workflowFileWrite(t *testing.T, dir string, name string, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// workflowGatesPlant satisfies the pipeline and hook checks so a test can
// isolate the graph rules.
func workflowGatesPlant(t *testing.T, dir string) {
	t.Helper()
	workflowFileWrite(t, dir, ".forgejo/workflows/ci.yml", "steps:\n  - run: bfd conform\n")
	workflowFileWrite(t, dir, "lefthook.yml", "pre-commit:\n  commands: {}\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "gates")
}

type workflowFindingsInput struct {
	Dir    string
	Config WorkflowConfig
}

func workflowFindingsRun(t *testing.T, input workflowFindingsInput) []Finding {
	t.Helper()
	findings := []Finding{}
	workflowCheckAll(workflowCheckInput{
		RootDir: input.Dir,
		Config:  input.Config,
		Report:  func(finding Finding) { findings = append(findings, finding) },
		Note:    func(string) {},
	})
	return findings
}

func workflowFindingsByRule(findings []Finding, rule string) []Finding {
	matched := []Finding{}
	for _, finding := range findings {
		if finding.Rule == rule {
			matched = append(matched, finding)
		}
	}
	return matched
}

func TestWorkflowNotARepositoryChecksNothing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	result := workflowCheckAll(workflowCheckInput{
		RootDir: dir,
		Report:  func(Finding) { t.Error("a plain directory has no workflow to judge") },
		Note:    func(string) {},
	})
	if result.Production != "" {
		t.Errorf("expected no production branch judged, got %q", result.Production)
	}
}

func TestWorkflowCleanReleaseFlowHolds(t *testing.T) {
	dir := workflowRepoNew(t)
	workflowGatesPlant(t, dir)
	epoch := gitTest(t, dir, "rev-parse", "HEAD")

	gitTest(t, dir, "checkout", "-q", "-b", "release/v1")
	workflowFileWrite(t, dir, "feature.txt", "work\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "feature")
	gitTest(t, dir, "checkout", "-q", "main")
	gitTest(t, dir, "merge", "-q", "--no-ff", "-m", "Merge release/v1", "release/v1")
	gitTest(t, dir, "tag", "v1.0.0")

	findings := workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}})
	if len(findings) != 0 {
		t.Errorf("a clean release flow must hold, got %+v", findings)
	}
}

func TestWorkflowDirectCommitOnMainIsFound(t *testing.T) {
	dir := workflowRepoNew(t)
	workflowGatesPlant(t, dir)
	epoch := gitTest(t, dir, "rev-parse", "HEAD")
	workflowFileWrite(t, dir, "hotpatch.txt", "oops\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "pushed straight to prod")

	findings := workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}}), "BFD-30")
	if len(findings) != 1 {
		t.Fatalf("expected one aggregated BFD-30 finding, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "1 direct commit(s)") {
		t.Errorf("the finding must count the scars, got %q", findings[0].Message)
	}
}

func TestWorkflowEpochShieldsThePast(t *testing.T) {
	dir := workflowRepoNew(t)
	workflowFileWrite(t, dir, "legacy.txt", "old sins\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "pre-adoption direct commit")
	workflowGatesPlant(t, dir)
	epoch := gitTest(t, dir, "rev-parse", "HEAD")

	findings := workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}})
	if len(findings) != 0 {
		t.Errorf("history before the epoch is not on trial, got %+v", findings)
	}
}

func TestWorkflowUntaggedMergeIsFound(t *testing.T) {
	dir := workflowRepoNew(t)
	workflowGatesPlant(t, dir)
	epoch := gitTest(t, dir, "rev-parse", "HEAD")
	gitTest(t, dir, "checkout", "-q", "-b", "release/v1")
	workflowFileWrite(t, dir, "feature.txt", "work\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "feature")
	gitTest(t, dir, "checkout", "-q", "main")
	gitTest(t, dir, "merge", "-q", "--no-ff", "-m", "Merge release/v1", "release/v1")

	findings := workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}}), "BFD-30")
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "carry no") {
		t.Errorf("an untagged merge into production must be found, got %+v", findings)
	}
}

func TestWorkflowTagOffTheLineIsFound(t *testing.T) {
	dir := workflowRepoNew(t)
	workflowGatesPlant(t, dir)
	epoch := gitTest(t, dir, "rev-parse", "HEAD")
	gitTest(t, dir, "checkout", "-q", "-b", "release/v1")
	workflowFileWrite(t, dir, "feature.txt", "work\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "feature")
	gitTest(t, dir, "tag", "v9.9.9") // tagged on the release branch, never shipped
	gitTest(t, dir, "checkout", "-q", "main")

	findings := workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}}), "BFD-31")
	if len(findings) != 1 || !strings.Contains(findings[0].Where, "v9.9.9") {
		t.Errorf("a version tag off production's line must be found, got %+v", findings)
	}
}

func TestWorkflowStagingMergedBackIsFound(t *testing.T) {
	dir := workflowRepoNew(t)
	workflowGatesPlant(t, dir)
	epoch := gitTest(t, dir, "rev-parse", "HEAD")
	gitTest(t, dir, "checkout", "-q", "-b", "staging")
	workflowFileWrite(t, dir, "experiment.txt", "staging only\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "staging experiment")
	gitTest(t, dir, "checkout", "-q", "main")
	gitTest(t, dir, "merge", "-q", "--no-ff", "-m", "Merge staging", "staging")
	gitTest(t, dir, "tag", "v1.0.0") // tagged, so only the staging rule trips

	findings := workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}}), "BFD-32")
	found := false
	for _, finding := range findings {
		if strings.Contains(finding.Message, "merged into main") {
			found = true
		}
	}
	if !found {
		t.Errorf("staging merged into production must be found, got %+v", findings)
	}
}

func TestWorkflowStagingCherryPickProvenance(t *testing.T) {
	dir := workflowRepoNew(t)
	workflowGatesPlant(t, dir)
	epoch := gitTest(t, dir, "rev-parse", "HEAD")
	gitTest(t, dir, "checkout", "-q", "-b", "staging")
	workflowFileWrite(t, dir, "picked.txt", "hand-authored, no provenance\n")
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-q", "-m", "no provenance here")
	gitTest(t, dir, "checkout", "-q", "main")

	findings := workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}}), "BFD-32")
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "cherry-pick") {
		t.Fatalf("staging commits without recorded provenance must be found, got %+v", findings)
	}

	gitTest(t, dir, "checkout", "-q", "staging")
	gitTest(t, dir, "commit", "-q", "--amend", "-m", "no provenance here\n\n(cherry picked from commit 0000000000000000000000000000000000000000)")
	gitTest(t, dir, "checkout", "-q", "main")
	findings = workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir, Config: WorkflowConfig{Epoch: epoch}}), "BFD-32")
	if len(findings) != 0 {
		t.Errorf("a recorded cherry-pick is legal staging provenance, got %+v", findings)
	}
}

func TestWorkflowPipelineMissingAndSilent(t *testing.T) {
	dir := workflowRepoNew(t)
	findings := workflowFindingsRun(t, workflowFindingsInput{Dir: dir})
	ci := workflowFindingsByRule(findings, "BFD-33")
	if len(ci) != 1 || !strings.Contains(ci[0].Message, "no CI configuration") {
		t.Errorf("a repo with no pipeline must be found, got %+v", ci)
	}
	hooks := workflowFindingsByRule(findings, "BFD-17")
	if len(hooks) != 1 {
		t.Errorf("a repo with no hook manager must be found, got %+v", hooks)
	}

	workflowFileWrite(t, dir, ".github/workflows/ci.yml", "steps:\n  - run: echo tests\n")
	ci = workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir}), "BFD-33")
	if len(ci) != 1 || !strings.Contains(ci[0].Message, "bfd conform") {
		t.Errorf("a pipeline that skips the gate must be found, got %+v", ci)
	}

	workflowFileWrite(t, dir, ".github/workflows/ci.yml", "steps:\n  - run: bfd conform\n")
	ci = workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{Dir: dir}), "BFD-33")
	if len(ci) != 0 {
		t.Errorf("a pipeline invoking the gate holds, got %+v", ci)
	}
}

func TestWorkflowProductionBranchMissing(t *testing.T) {
	dir := workflowRepoNew(t)
	findings := workflowFindingsByRule(workflowFindingsRun(t, workflowFindingsInput{
		Dir:    dir,
		Config: WorkflowConfig{Production: "trunk"},
	}), "BFD-30")
	if len(findings) != 1 || !strings.Contains(findings[0].Message, `"trunk"`) {
		t.Errorf("a configured production branch that does not exist must be found, got %+v", findings)
	}
}
