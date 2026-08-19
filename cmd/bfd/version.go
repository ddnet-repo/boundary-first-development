package main

// Knowing a newer bfd exists is the user's problem to have solved for them,
// not to remember. The check reads the Go module proxy — the same index
// `go install` resolves against — at most once a day.
//
// It has no off switch. What it does have are two limits that are facts
// rather than preferences: it never writes into --json or a pipe, because a
// human notice in a machine-readable stream is corruption; and it says
// nothing when the network is unreachable, because that is not news.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	versionProxyURL      = "https://proxy.golang.org/codeberg.org/galaxi/boundary-first-development/@latest"
	versionCheckInterval = 24 * time.Hour
)

// versionInstalled reports the version this binary was built from, or "devel"
// for a local build the proxy knows nothing about.
func versionInstalled() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "devel"
	}
	return info.Main.Version
}

type versionFetchInput struct {
	TimeoutSeconds int
}

type versionFetchResult struct {
	Ok     bool
	Latest string
}

func versionFetch(input versionFetchInput) versionFetchResult {
	client := &http.Client{Timeout: time.Duration(input.TimeoutSeconds) * time.Second}
	response, err := client.Get(versionProxyURL)
	if err != nil {
		return versionFetchResult{}
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return versionFetchResult{}
	}
	// The proxy answers with {"Version": ...}. That casing is Google's, not
	// ours, and BFD-11 governs the boundary we own — so this reads the field
	// untagged, on encoding/json's case-insensitive match, rather than
	// writing a foreign convention into our source.
	var payload struct {
		Version string
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil || payload.Version == "" {
		return versionFetchResult{}
	}
	return versionFetchResult{Ok: true, Latest: payload.Version}
}

type versionCacheEntry struct {
	CheckedAt time.Time `json:"checkedAt"`
	Latest    string    `json:"latest"`
}

func versionCachePath() string {
	directory, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(directory, "bfd", "version-check.json")
}

func versionCacheRead() versionCacheEntry {
	path := versionCachePath()
	if path == "" {
		return versionCacheEntry{}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return versionCacheEntry{}
	}
	var entry versionCacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return versionCacheEntry{}
	}
	return entry
}

func versionCacheWrite(entry versionCacheEntry) {
	path := versionCachePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, encoded, 0o644) // a cache that cannot be written is a check that runs again
}

type versionCompareInput struct {
	Installed string
	Latest    string
}

// versionCompare reports whether Latest is a newer release than Installed.
// Anything it cannot read as a plain vMAJOR.MINOR.PATCH release — "devel", a
// pseudo-version, a pre-release — answers false: a build the proxy does not
// index is not a build to nag about.
func versionCompare(input versionCompareInput) bool {
	installed, installedOk := versionParse(input.Installed)
	latest, latestOk := versionParse(input.Latest)
	if !installedOk || !latestOk {
		return false
	}
	for i := range installed {
		if latest[i] != installed[i] {
			return latest[i] > installed[i]
		}
	}
	return false
}

// versionParse reads "v1.2.3" into its three numbers. A suffix of any kind
// (pre-release, build metadata, pseudo-version timestamp) fails the parse.
func versionParse(version string) ([3]int, bool) {
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	parsed := [3]int{}
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return [3]int{}, false
		}
		parsed[i] = number
	}
	return parsed, true
}

type versionNoticeInput struct {
	Installed string
	Force     bool // "bfd version" asks directly; conform only rides along
	Now       time.Time
}

// versionNoticeFind returns the line to show the user, or "" for silence.
// There is no way to turn this off. Running a checker that has fallen behind
// the law is not a preference (BFD-27).
func versionNoticeFind(input versionNoticeInput) string {
	entry := versionCacheRead()
	if input.Force || input.Now.Sub(entry.CheckedAt) > versionCheckInterval {
		if fetched := versionFetch(versionFetchInput{TimeoutSeconds: 3}); fetched.Ok {
			entry = versionCacheEntry{CheckedAt: input.Now, Latest: fetched.Latest}
			versionCacheWrite(entry)
		}
	}
	if !versionCompare(versionCompareInput{Installed: input.Installed, Latest: entry.Latest}) {
		return ""
	}
	return fmt.Sprintf("bfd %s is available (you have %s) — run \"bfd update\"", entry.Latest, input.Installed)
}

// versionInteractive reports whether stdout is a terminal. On a pipe or in
// CI, the notice is noise in someone's log and stays unprinted.
func versionInteractive() bool {
	info, err := os.Stdout.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
