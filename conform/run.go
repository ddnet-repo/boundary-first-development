package conform

import (
	"sort"
	"strconv"
	"strings"
)

// Run executes a conformance run: the static tier against the spec, the wire
// tier against the running API, or both. Findings are observations, not
// errors — a run that proves violations still returns Ok true.
func Run(input RunInput) RunResult {
	if input.SpecPath == "" && input.BaseURL == "" {
		return RunResult{Error: RunError{
			Code:    ErrorCodeInputEmpty,
			Message: "nothing to check: provide a spec path, a base URL, or both",
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
	toolchainCheckAll(toolchainCheckInput{
		RootDir:   rootDir,
		Languages: input.Languages,
		Report:    report,
		Note:      note,
	})

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Rule != findings[j].Rule {
			return ruleNumber(findings[i].Rule) < ruleNumber(findings[j].Rule)
		}
		return findings[i].Where < findings[j].Where
	})
	return RunResult{Ok: true, Data: RunData{Findings: findings, Endpoints: probed, Notes: notes}}
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
