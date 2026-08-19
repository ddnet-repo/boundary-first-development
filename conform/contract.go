// Package conform proves the wire-level Boundary-First Development rules
// against the boundary artifacts of a project: its OpenAPI contract and its
// running API. It knows nothing about the language behind the boundary, and
// it never needs to — that is the point.
//
// The package's own boundary follows the rules it enforces: Run accepts a
// single input struct and returns a result envelope with enumerated error
// codes. Exceptions do not cross it.
package conform

// RulesProven names the BFD rules this build of conform knows how to check.
// It is the answer to "what law is this binary carrying?" — a question an
// installed tool must be able to answer, because a checker that has fallen
// behind the law reports a clean run it is not entitled to report.
//
// Add a rule here in the same change that teaches conform to check it, and
// add its row to CONFORM.md. parity_test.go proves the two agree.
var RulesProven = []string{
	"BFD-2", "BFD-3", "BFD-7", "BFD-8", "BFD-11",
	"BFD-12", "BFD-13", "BFD-17", "BFD-18", "BFD-29",
	"BFD-30", "BFD-31", "BFD-32", "BFD-33",
}

// Finding is one observed violation of a BFD rule.
type Finding struct {
	Rule    string `json:"rule"`    // the rule id, e.g. "BFD-11"
	Where   string `json:"where"`   // where it was observed, e.g. "wire: GET /persons"
	Message string `json:"message"` // for humans; the rule id is the contract
}

// RunInput configures a conformance run. SpecPath runs the static checks,
// BaseURL the wire checks, and setting both runs them together. The toolchain
// tier runs on every run, against RootDir — so a project with neither a spec
// nor a running API still has its lint gate proven.
type RunInput struct {
	SpecPath        string   `json:"specPath"`       // OpenAPI 3.x document, YAML or JSON
	BaseURL         string   `json:"baseUrl"`        // a running API to probe with read-only GETs
	Endpoints       []string `json:"endpoints"`      // extra GET paths to probe, beyond spec discovery
	AuthHeaderName  string   `json:"authHeaderName"` // defaults to "Authorization" when a value is set
	AuthHeaderValue string   `json:"-"`              // never serialized; secrets do not cross boundaries
	TimeoutSeconds  int      `json:"timeoutSeconds"` // per wire request; defaults to 10
	RootDir         string   `json:"rootDir"`        // project root for the toolchain tier; defaults to "."
	Languages       []string `json:"languages"`      // nil: detect from manifests; []: skip the toolchain tier
	RulesRequired   []string `json:"rulesRequired"`  // rules the project demands; a build without them refuses to run

	Workflow WorkflowConfig `json:"workflow"` // branch names for the workflow tier; zero values take the defaults
}

// RunError carries an enumerated code for tool-level failures. Findings are
// not errors; a run that observes violations still returns Ok.
type RunError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RunData is the payload of a completed run.
type RunData struct {
	Findings  []Finding `json:"findings"`  // sorted by rule, then location
	Endpoints []string  `json:"endpoints"` // wire paths actually probed
	Languages []string  `json:"languages"` // toolchain gates actually examined
	Workflow  string    `json:"workflow"`  // production branch the workflow tier judged; "" when unchecked
	Rules     []string  `json:"rules"`     // the law this build carries; see RulesProven
	Notes     []string  `json:"notes"`     // checks skipped or degraded, stated plainly
}

// RunResult is the envelope every call to Run returns.
type RunResult struct {
	Ok    bool     `json:"ok"`
	Data  RunData  `json:"data"`
	Error RunError `json:"error"`
}

// Enumerated error codes for RunError.Code.
const (
	ErrorCodeInputEmpty      = "input_empty"      // no spec, no base URL, and no linted language to check
	ErrorCodeSpecUnreadable  = "spec_unreadable"  // the spec file could not be read
	ErrorCodeSpecInvalid     = "spec_invalid"     // the spec file is not a YAML/JSON mapping
	ErrorCodeWireUnreachable = "wire_unreachable" // no wire request got any response at all
	ErrorCodeRulesStale      = "rules_stale"      // the project requires rules this build cannot check
)
