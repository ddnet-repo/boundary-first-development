package main

import "testing"

// versionCompare is a pure function, so it gets a unit test (BFD-25). The
// network path around it is not one, and is not tested here.

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		Installed string
		Latest    string
		Newer     bool
	}{
		{Installed: "v0.2.0", Latest: "v0.3.0", Newer: true},
		{Installed: "v0.2.0", Latest: "v0.2.1", Newer: true},
		{Installed: "v0.2.0", Latest: "v1.0.0", Newer: true},
		{Installed: "v0.9.0", Latest: "v0.10.0", Newer: true}, // not a string comparison
		{Installed: "v0.2.0", Latest: "v0.2.0", Newer: false},
		{Installed: "v0.3.0", Latest: "v0.2.0", Newer: false},
		{Installed: "v1.0.0", Latest: "v0.9.9", Newer: false},
		{Installed: "devel", Latest: "v0.3.0", Newer: false},                          // a local build is not stale
		{Installed: "v0.2.1-0.20260804120000-abcdef", Latest: "v0.3.0", Newer: false}, // pseudo-version: unknown ground
		{Installed: "v0.2.0", Latest: "", Newer: false},                               // no answer from the proxy
		{Installed: "v0.2.0", Latest: "v0.3.0-rc1", Newer: false},                     // pre-releases are not upgrades
	}
	for _, testCase := range cases {
		got := versionCompare(versionCompareInput{Installed: testCase.Installed, Latest: testCase.Latest})
		if got != testCase.Newer {
			t.Errorf("versionCompare(%q -> %q) = %v, want %v", testCase.Installed, testCase.Latest, got, testCase.Newer)
		}
	}
}
