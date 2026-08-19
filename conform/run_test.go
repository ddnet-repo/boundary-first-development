package conform

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Integration tests at the package boundary (BFD-25): every test calls Run
// and asserts on the result envelope. Internals are nobody's business.

func rulesOf(result RunResult) map[string]bool {
	rules := map[string]bool{}
	for _, finding := range result.Data.Findings {
		rules[finding.Rule] = true
	}
	return rules
}

type requireRulesInput struct {
	Result RunResult
	Rules  []string
}

func requireRules(t *testing.T, input requireRulesInput) {
	t.Helper()
	if !input.Result.Ok {
		t.Fatalf("run failed: %s (%s)", input.Result.Error.Message, input.Result.Error.Code)
	}
	rules := rulesOf(input.Result)
	for _, rule := range input.Rules {
		if !rules[rule] {
			t.Errorf("expected a %s finding, got none; findings: %+v", rule, input.Result.Data.Findings)
		}
	}
}

func TestRunSpecClean(t *testing.T) {
	result := Run(RunInput{SpecPath: "testdata/spec_clean.yaml", RootDir: t.TempDir()})
	if !result.Ok {
		t.Fatalf("run failed: %s (%s)", result.Error.Message, result.Error.Code)
	}
	if len(result.Data.Findings) != 0 {
		t.Errorf("clean spec produced findings: %+v", result.Data.Findings)
	}
}

func TestRunSpecViolating(t *testing.T) {
	result := Run(RunInput{SpecPath: "testdata/spec_violating.yaml", RootDir: t.TempDir()})
	requireRules(t, requireRulesInput{Result: result, Rules: []string{
		"BFD-2",  // /people response is not the envelope
		"BFD-3",  // /orders error code has no enum
		"BFD-7",  // no serverTime on /people; people schema has id without status/updatedAt
		"BFD-8",  // /orders list GET without updatedAfter
		"BFD-11", // created_at property
		"BFD-12", // updatedAt on /orders items lacks format date-time
		"BFD-13", // route and schema "people"
		"BFD-18", // no apiKey security scheme
	}})
}

func TestRunSpecMissing(t *testing.T) {
	result := Run(RunInput{SpecPath: "testdata/no_such_spec.yaml"})
	if result.Ok || result.Error.Code != ErrorCodeSpecUnreadable {
		t.Errorf("expected %s, got %+v", ErrorCodeSpecUnreadable, result)
	}
}

// A checker asked for law it does not carry must refuse the run. Reporting a
// clean boundary it was never able to inspect is the one failure a gate must
// not have.
func TestRunRulesStale(t *testing.T) {
	result := Run(RunInput{SpecPath: "testdata/spec_clean.yaml", RulesRequired: []string{"BFD-99"}})
	if result.Ok || result.Error.Code != ErrorCodeRulesStale {
		t.Errorf("expected %s, got %+v", ErrorCodeRulesStale, result)
	}
}

func TestRunRulesCarried(t *testing.T) {
	result := Run(RunInput{SpecPath: "testdata/spec_clean.yaml", RulesRequired: []string{"BFD-29", "bfd-2"}, RootDir: t.TempDir()})
	if !result.Ok {
		t.Fatalf("expected the run to proceed, got error %+v", result.Error)
	}
	if len(result.Data.Rules) == 0 {
		t.Error("a completed run must report the law it carried")
	}
}

func TestRunInputEmpty(t *testing.T) {
	result := Run(RunInput{RootDir: t.TempDir()})
	if result.Ok || result.Error.Code != ErrorCodeInputEmpty {
		t.Errorf("expected %s, got %+v", ErrorCodeInputEmpty, result)
	}
}

func serverConforming() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/persons", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		serverTime := time.Now().UTC().Format(time.RFC3339)
		data := `[{"id":"p1","status":"active","updatedAt":"2026-07-20T10:00:00Z","nameFull":"Ada Lovelace"}]`
		if r.URL.Query().Get("updatedAfter") != "" {
			data = `[]`
		}
		_, _ = fmt.Fprintf(w, `{"ok":true,"data":%s,"error":null,"serverTime":%q}`, data, serverTime)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		serverTime := time.Now().UTC().Format(time.RFC3339)
		_, _ = fmt.Fprintf(w, `{"ok":false,"data":null,"error":{"code":"routeUnknown","message":"no such route"},"serverTime":%q}`, serverTime)
	})
	return httptest.NewServer(mux)
}

func serverViolating() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/people", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"success":true,"items":[{"id":"1","created_at":"2026-07-24T12:00:00+02:00"}]}`)
	})
	return httptest.NewServer(mux) // unknown routes fall through to Go's plain-text 404
}

func TestRunWireClean(t *testing.T) {
	server := serverConforming()
	defer server.Close()
	result := Run(RunInput{BaseURL: server.URL, Endpoints: []string{"/persons"}, RootDir: t.TempDir()})
	if !result.Ok {
		t.Fatalf("run failed: %s (%s)", result.Error.Message, result.Error.Code)
	}
	if len(result.Data.Findings) != 0 {
		t.Errorf("conforming server produced findings: %+v", result.Data.Findings)
	}
}

func TestRunWireViolating(t *testing.T) {
	server := serverViolating()
	defer server.Close()
	result := Run(RunInput{BaseURL: server.URL, Endpoints: []string{"/people"}, RootDir: t.TempDir()})
	requireRules(t, requireRulesInput{Result: result, Rules: []string{
		"BFD-2",  // no envelope on /people, naked 404 on the probe
		"BFD-7",  // no serverTime
		"BFD-11", // created_at key on the wire
		"BFD-12", // +02:00 timestamp
		"BFD-13", // route "people"
	}})
}

func TestRunWireUnreachable(t *testing.T) {
	result := Run(RunInput{BaseURL: "http://127.0.0.1:1", Endpoints: []string{"/persons"}, TimeoutSeconds: 2})
	if result.Ok || result.Error.Code != ErrorCodeWireUnreachable {
		t.Errorf("expected %s, got %+v", ErrorCodeWireUnreachable, result)
	}
}

func TestRunSpecAndWireTogether(t *testing.T) {
	server := serverConforming()
	defer server.Close()
	result := Run(RunInput{SpecPath: "testdata/spec_clean.yaml", BaseURL: server.URL, RootDir: t.TempDir()})
	if !result.Ok {
		t.Fatalf("run failed: %s (%s)", result.Error.Message, result.Error.Code)
	}
	if len(result.Data.Findings) != 0 {
		t.Errorf("clean spec + conforming server produced findings: %+v", result.Data.Findings)
	}
	if len(result.Data.Endpoints) != 1 || result.Data.Endpoints[0] != "/persons" {
		t.Errorf("expected /persons discovered from the spec, got %v", result.Data.Endpoints)
	}
}
