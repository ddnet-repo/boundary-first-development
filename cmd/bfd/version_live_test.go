package main

import (
	"os"
	"testing"
	"time"
)

// The staleness check crosses a real boundary — the Go module proxy — so it
// gets an integration test (BFD-25). It is skipped in short mode and when the
// proxy cannot be reached, because an offline machine is not a broken build.

func TestVersionFetchLive(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: the proxy is a network boundary")
	}
	fetched := versionFetch(versionFetchInput{TimeoutSeconds: 8})
	if !fetched.Ok {
		t.Skip("module proxy unreachable")
	}
	if _, ok := versionParse(fetched.Latest); !ok {
		t.Errorf("proxy returned %q, which is not a plain release version", fetched.Latest)
	}
}

// An old install must actually be told. This is the whole point of the check,
// so it is asserted rather than assumed.
func TestVersionNoticeFindsAnUpgrade(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: the proxy is a network boundary")
	}
	if !versionFetch(versionFetchInput{TimeoutSeconds: 8}).Ok {
		t.Skip("module proxy unreachable")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // never read or write the real cache
	notice := versionNoticeFind(versionNoticeInput{Installed: "v0.0.1", Force: true, Now: time.Now()})
	if notice == "" {
		t.Error("a v0.0.1 install was told nothing, but every published release is newer")
	}
}

func TestVersionCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	stamp := time.Now().UTC().Truncate(time.Second)
	versionCacheWrite(versionCacheEntry{CheckedAt: stamp, Latest: "v9.9.9"})
	entry := versionCacheRead()
	if entry.Latest != "v9.9.9" || !entry.CheckedAt.Equal(stamp) {
		t.Errorf("cache round trip lost data: %+v", entry)
	}
	if _, err := os.Stat(versionCachePath()); err != nil {
		t.Errorf("cache file not written: %v", err)
	}
}
