package conform

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Run executes a conformance run: the static tier against the spec, the wire
// tier against the running API, or both. Findings are observations, not
// errors — a run that proves violations still returns Ok true.
func Run(input RunInput) RunResult {
	if stale := rulesStaleFind(rulesStaleInput{Required: input.RulesRequired}); len(stale) > 0 {
		return RunResult{Error: RunError{
			Code:    ErrorCodeRulesStale,
			Message: fmt.Sprintf("this build of conform cannot check %s — the project requires law it does not carry; run \"bfd update\"", quoteJoin(stale)),
		}}
	}
	findings := []Finding{}
	notes := []string{}
	report := func(finding Finding) { findings = append(findings, finding) }
	note := func(text string) { notes = append(notes, text) }

	endpoints := []string{}
	if input.SpecPath != "" {
		loaded := specLoad(specLoadInput{Path: input.SpecPath})
		if !loaded.Ok {
			return RunResult{Error: loaded.Error}
		}
		specCheckAll(specCheckInput{Document: loaded.Document, Report: report})
		endpoints = specEndpointsList(specEndpointsInput{Document: loaded.Document})
	}
	for _, extra := range input.Endpoints {
		if !strings.HasPrefix(extra, "/") {
			extra = "/" + extra
		}
		known := false
		for _, existing := range endpoints {
			if existing == extra {
				known = true
			}
		}
		if !known {
			endpoints = append(endpoints, extra)
		}
	}

	probed := []string{}
	if input.BaseURL != "" {
		if len(endpoints) == 0 {
			note("no GET endpoints known (no spec, no configured endpoints) — only the unknown-route probe ran against the wire")
		}
		wired := wireRunAll(wireRunInput{
			BaseURL:         input.BaseURL,
			Endpoints:       endpoints,
			AuthHeaderName:  input.AuthHeaderName,
			AuthHeaderValue: input.AuthHeaderValue,
			TimeoutSeconds:  input.TimeoutSeconds,
			Report:          report,
			Note:            note,
		})
		if !wired.Ok {
			return RunResult{Error: wired.Error}
		}
		probed = endpoints
	}

	rootDir := input.RootDir
	if rootDir == "" {
		rootDir = "."
	}
	toolchain := toolchainCheckAll(toolchainCheckInput{
		RootDir:   rootDir,
		Languages: input.Languages,
		Report:    report,
		Note:      note,
	})
	workflow := workflowCheckAll(workflowCheckInput{
		RootDir: rootDir,
		Config:  input.Workflow,
		Report:  report,
		Note:    note,
	})
	if input.SpecPath == "" && input.BaseURL == "" && len(toolchain.Checked) == 0 && workflow.Production == "" {
		return RunResult{Error: RunError{
			Code:    ErrorCodeInputEmpty,
			Message: "nothing to check: no spec, no base URL, no linted language, and no git history here",
		}}
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Rule != findings[j].Rule {
			return ruleNumber(findings[i].Rule) < ruleNumber(findings[j].Rule)
		}
		return findings[i].Where < findings[j].Where
	})
	return RunResult{Ok: true, Data: RunData{
		Findings:  findings,
		Endpoints: probed,
		Languages: toolchain.Checked,
		Workflow:  workflow.Production,
		Rules:     RulesProven,
		Notes:     notes,
	}}
}

type rulesStaleInput struct {
	Required []string
}

// rulesStaleFind returns the required rules this build cannot check. A stale
// checker that stays silent is worse than no checker: it reports success.
func rulesStaleFind(input rulesStaleInput) []string {
	proven := map[string]bool{}
	for _, rule := range RulesProven {
		proven[rule] = true
	}
	stale := []string{}
	for _, required := range input.Required {
		normalized := strings.ToUpper(strings.TrimSpace(required))
		if normalized != "" && !proven[normalized] {
			stale = append(stale, normalized)
		}
	}
	return stale
}

// quoteJoin renders a name list for humans: `"ok", "data", "error"`.
func quoteJoin(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = strconv.Quote(name)
	}
	return strings.Join(quoted, ", ")
}

func ruleNumber(rule string) int {
	number, err := strconv.Atoi(strings.TrimPrefix(rule, "BFD-"))
	if err != nil {
		return 0
	}
	return number
}
