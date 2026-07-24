// Package conform proves the wire-level Boundary-First Development rules
// against the boundary artifacts of a project: its OpenAPI contract and its
// running API. It knows nothing about the language behind the boundary, and
// it never needs to — that is the point.
//
// The package's own boundary follows the rules it enforces: Run accepts a
// single input struct and returns a result envelope with enumerated error
// codes. Exceptions do not cross it.
package conform

// Finding is one observed violation of a BFD rule.
type Finding struct {
	Rule    string `json:"rule"`    // the rule id, e.g. "BFD-11"
	Where   string `json:"where"`   // where it was observed, e.g. "wire: GET /persons"
	Message string `json:"message"` // for humans; the rule id is the contract
}

// RunInput configures a conformance run. Either SpecPath or BaseURL must be
// set; setting both runs the static and the wire checks together.
type RunInput struct {
	SpecPath        string   `json:"specPath"`       // OpenAPI 3.x document, YAML or JSON
	BaseURL         string   `json:"baseUrl"`        // a running API to probe with read-only GETs
	Endpoints       []string `json:"endpoints"`      // extra GET paths to probe, beyond spec discovery
	AuthHeaderName  string   `json:"authHeaderName"` // defaults to "Authorization" when a value is set
	AuthHeaderValue string   `json:"-"`              // never serialized; secrets do not cross boundaries
	TimeoutSeconds  int      `json:"timeoutSeconds"` // per wire request; defaults to 10
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
	ErrorCodeInputEmpty      = "input_empty"      // neither a spec nor a base URL was given
	ErrorCodeSpecUnreadable  = "spec_unreadable"  // the spec file could not be read
	ErrorCodeSpecInvalid     = "spec_invalid"     // the spec file is not a YAML/JSON mapping
	ErrorCodeWireUnreachable = "wire_unreachable" // no wire request got any response at all
)
