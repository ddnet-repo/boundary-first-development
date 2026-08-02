package conform

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// The wire tier: everything provable by observing a running API from the
// outside. Read-only GETs, no writes, no knowledge of the implementation.

const wireBodyLimitBytes = 10 << 20

// wireProbePathUnknown is requested to prove the envelope holds even for
// routes that do not exist (BFD-2: exceptions never cross a boundary).
const wireProbePathUnknown = "/bfd-conform-probe-404"

type wireRunInput struct {
	BaseURL         string
	Endpoints       []string
	AuthHeaderName  string
	AuthHeaderValue string
	TimeoutSeconds  int
	Report          func(finding Finding)
	Note            func(text string)
}

type wireRunResult struct {
	Ok    bool
	Error RunError
}

func wireRunAll(input wireRunInput) wireRunResult {
	timeout := input.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}
	client := wireClient{
		baseURL:         strings.TrimRight(input.BaseURL, "/"),
		authHeaderName:  input.AuthHeaderName,
		authHeaderValue: input.AuthHeaderValue,
		http:            &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
	reached := false
	for _, endpoint := range input.Endpoints {
		if wireCheckEndpoint(wireEndpointInput{Client: client, Path: endpoint, Report: input.Report, Note: input.Note}) {
			reached = true
		}
	}
	if wireCheckRouteUnknown(wireEndpointInput{Client: client, Path: wireProbePathUnknown, Report: input.Report, Note: input.Note}) {
		reached = true
	}
	if !reached {
		return wireRunResult{Error: RunError{
			Code:    ErrorCodeWireUnreachable,
			Message: fmt.Sprintf("no request to %s got a response — is the API running?", input.BaseURL),
		}}
	}
	return wireRunResult{Ok: true}
}

type wireClient struct {
	baseURL         string
	authHeaderName  string
	authHeaderValue string
	http            *http.Client
}

type wireFetchInput struct {
	Client wireClient
	Path   string
}

type wireFetchResult struct {
	Ok     bool
	Status int
	Body   []byte
	Error  string
}

func wireFetch(input wireFetchInput) wireFetchResult {
	request, err := http.NewRequest(http.MethodGet, input.Client.baseURL+input.Path, nil)
	if err != nil {
		return wireFetchResult{Error: err.Error()}
	}
	request.Header.Set("Accept", "application/json")
	if input.Client.authHeaderValue != "" {
		headerName := input.Client.authHeaderName
		if headerName == "" {
			headerName = "Authorization"
		}
		request.Header.Set(headerName, input.Client.authHeaderValue)
	}
	response, err := input.Client.http.Do(request)
	if err != nil {
		return wireFetchResult{Error: err.Error()}
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, wireBodyLimitBytes))
	if err != nil {
		return wireFetchResult{Error: err.Error()}
	}
	return wireFetchResult{Ok: true, Status: response.StatusCode, Body: body}
}

type wireEndpointInput struct {
	Client wireClient
	Path   string
	Report func(finding Finding)
	Note   func(text string)
}

// wireCheckEndpoint proves the envelope, casing, UTC, model, and sync rules
// against one live endpoint. It returns whether the wire answered at all.
func wireCheckEndpoint(input wireEndpointInput) bool {
	where := "wire: GET " + input.Path
	if violation, found := namingPluralFind(input.Path); found {
		input.Report(Finding{
			Rule:    "BFD-13",
			Where:   where,
			Message: fmt.Sprintf("irregular plural %q in the route — it is %q", violation.Word, violation.Regular),
		})
	}
	fetched := wireFetch(wireFetchInput{Client: input.Client, Path: input.Path})
	if !fetched.Ok {
		input.Note(fmt.Sprintf("skipped %s: %s", input.Path, fetched.Error))
		return false
	}
	root := wireParseBody(wireParseInput{Fetched: fetched, Where: where, Report: input.Report})
	if root == nil {
		return true
	}
	serverTime := wireCheckEnvelope(wireEnvelopeInput{Root: root, Where: where, Report: input.Report})
	wireCheckBody(wireBodyInput{Root: root, Where: where, Report: input.Report})
	wireCheckData(wireDataInput{Client: input.Client, Root: root, Path: input.Path, Where: where, ServerTime: serverTime, Report: input.Report, Note: input.Note})
	return true
}

type wireParseInput struct {
	Fetched wireFetchResult
	Where   string
	Report  func(finding Finding)
}

func wireParseBody(input wireParseInput) *yaml.Node {
	var document yaml.Node
	err := yaml.Unmarshal(input.Fetched.Body, &document)
	root := nodeUnwrap(&document)
	if err != nil || root == nil || root.Kind != yaml.MappingNode {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   input.Where,
			Message: fmt.Sprintf("response (status %d) is not a JSON object — every boundary returns the ok/data/error envelope", input.Fetched.Status),
		})
		return nil
	}
	return root
}

type wireEnvelopeInput struct {
	Root   *yaml.Node
	Where  string
	Report func(finding Finding)
}

// wireCheckEnvelope proves BFD-2 and BFD-7 on a live body and returns the
// parsed serverTime value, if one arrived, for the sync probe.
func wireCheckEnvelope(input wireEnvelopeInput) string {
	missing := []string{}
	for _, required := range []string{"ok", "data", "error"} {
		if nodeGet(nodeGetInput{Node: input.Root, Key: required}) == nil {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   input.Where,
			Message: fmt.Sprintf("response is missing %s — every boundary returns the ok/data/error envelope", quoteJoin(missing)),
		})
	}
	okNode := nodeGet(nodeGetInput{Node: input.Root, Key: "ok"})
	if okNode != nil && nodeScalarKind(okNode) != "bool" {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   input.Where,
			Message: `envelope field "ok" is not a boolean`,
		})
	}
	serverTimeNode := nodeGet(nodeGetInput{Node: input.Root, Key: "serverTime"})
	if serverTimeNode == nil {
		input.Report(Finding{
			Rule:    "BFD-7",
			Where:   input.Where,
			Message: `response has no "serverTime" — every API response includes the server's clock`,
		})
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, serverTimeNode.Value)
	if err != nil {
		input.Report(Finding{
			Rule:    "BFD-12",
			Where:   input.Where,
			Message: fmt.Sprintf("serverTime %q is not RFC3339", serverTimeNode.Value),
		})
		return ""
	}
	if _, offset := parsed.Zone(); offset != 0 {
		input.Report(Finding{
			Rule:    "BFD-12",
			Where:   input.Where,
			Message: fmt.Sprintf("serverTime %q is not UTC — all timestamps are transmitted in UTC", serverTimeNode.Value),
		})
	}
	return serverTimeNode.Value
}

type wireBodyInput struct {
	Root   *yaml.Node
	Where  string
	Report func(finding Finding)
}

// wireCheckBody walks every key and value in a live body: keys must be
// camelCase (BFD-11) with regular plurals (BFD-13), and every RFC3339 value
// anywhere must be UTC (BFD-12). Findings dedupe per key per endpoint.
func wireCheckBody(input wireBodyInput) {
	reported := map[string]bool{}
	once := func(dedupeKey string, finding Finding) {
		if !reported[dedupeKey] {
			reported[dedupeKey] = true
			input.Report(finding)
		}
	}
	nodeWalk(nodeWalkInput{Node: input.Root, OnEvent: func(event nodeWalkEvent) {
		if event.Key != "" {
			if !namingCamelOk(event.Key) {
				once("case:"+event.Key, Finding{
					Rule:    "BFD-11",
					Where:   input.Where,
					Message: fmt.Sprintf("key %q crossed the wire — the frontend boundary speaks camelCase (seen at %s)", event.Key, event.Path),
				})
			}
			if violation, found := namingPluralFind(event.Key); found {
				once("plural:"+violation.Word, Finding{
					Rule:    "BFD-13",
					Where:   input.Where,
					Message: fmt.Sprintf("irregular plural %q in key %q — it is %q", violation.Word, event.Key, violation.Regular),
				})
			}
		}
		if event.Kind == "string" {
			parsed, err := time.Parse(time.RFC3339, event.Value)
			if err != nil {
				return
			}
			if _, offset := parsed.Zone(); offset != 0 {
				once("utc:"+event.Key, Finding{
					Rule:    "BFD-12",
					Where:   input.Where,
					Message: fmt.Sprintf("timestamp %q at %s is not UTC — local time exists only in the display layer", event.Value, event.Path),
				})
			}
		}
	}})
}

type wireDataInput struct {
	Client     wireClient
	Root       *yaml.Node
	Path       string
	Where      string
	ServerTime string
	Report     func(finding Finding)
	Note       func(text string)
}

// wireCheckData proves the model rule on list items (BFD-7) and then proves
// the sync contract (BFD-8): re-requesting with ?updatedAfter=<the serverTime
// the server itself just reported> must return an empty list.
func wireCheckData(input wireDataInput) {
	data := nodeUnwrap(nodeGet(nodeGetInput{Node: input.Root, Key: "data"}))
	if data == nil || data.Kind != yaml.SequenceNode {
		return
	}
	reported := map[string]bool{}
	for _, item := range data.Content {
		if nodeGet(nodeGetInput{Node: item, Key: "id"}) == nil {
			continue
		}
		for _, required := range []string{"status", "updatedAt"} {
			if nodeGet(nodeGetInput{Node: item, Key: required}) == nil && !reported[required] {
				reported[required] = true
				input.Report(Finding{
					Rule:    "BFD-7",
					Where:   input.Where,
					Message: fmt.Sprintf("list records have an \"id\" but no %q — every model carries status and updatedAt", required),
				})
			}
		}
	}
	if input.ServerTime == "" {
		return
	}
	probePath := input.Path + "?updatedAfter=" + url.QueryEscape(input.ServerTime)
	fetched := wireFetch(wireFetchInput{Client: input.Client, Path: probePath})
	if !fetched.Ok {
		input.Note(fmt.Sprintf("sync probe skipped for %s: %s", input.Path, fetched.Error))
		return
	}
	syncWhere := fmt.Sprintf("wire: GET %s?updatedAfter=<serverTime>", input.Path)
	var document yaml.Node
	err := yaml.Unmarshal(fetched.Body, &document)
	root := nodeUnwrap(&document)
	if err != nil || root == nil || root.Kind != yaml.MappingNode {
		input.Report(Finding{
			Rule:    "BFD-8",
			Where:   syncWhere,
			Message: fmt.Sprintf("sync request failed (status %d, non-envelope body) — clients sync with ?updatedAfter=<lastServerTime>", fetched.Status),
		})
		return
	}
	syncData := nodeUnwrap(nodeGet(nodeGetInput{Node: root, Key: "data"}))
	if syncData == nil || syncData.Kind != yaml.SequenceNode {
		input.Report(Finding{
			Rule:    "BFD-8",
			Where:   syncWhere,
			Message: "sync response has no data list — the backend returns only records modified since the given time",
		})
		return
	}
	if len(syncData.Content) > 0 {
		input.Report(Finding{
			Rule:    "BFD-8",
			Where:   syncWhere,
			Message: fmt.Sprintf("asked for records modified after the server's own reported clock and got %d back — updatedAfter is not filtering", len(syncData.Content)),
		})
	}
}

// wireCheckRouteUnknown proves BFD-2/BFD-3 for the failure path: a route that
// does not exist must still answer with the envelope and an enumerated code.
func wireCheckRouteUnknown(input wireEndpointInput) bool {
	where := "wire: GET " + input.Path + " (unknown-route probe)"
	fetched := wireFetch(wireFetchInput{Client: input.Client, Path: input.Path})
	if !fetched.Ok {
		input.Note(fmt.Sprintf("unknown-route probe skipped: %s", fetched.Error))
		return false
	}
	var document yaml.Node
	err := yaml.Unmarshal(fetched.Body, &document)
	root := nodeUnwrap(&document)
	if err != nil || root == nil || root.Kind != yaml.MappingNode {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   where,
			Message: fmt.Sprintf("unknown route answers with a naked status-%d body, not the envelope — exceptions never cross a boundary", fetched.Status),
		})
		return true
	}
	okNode := nodeGet(nodeGetInput{Node: root, Key: "ok"})
	if okNode == nil || nodeScalarKind(okNode) != "bool" {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   where,
			Message: `unknown route answers without a boolean "ok" — the envelope must hold even in failure`,
		})
		return true
	}
	if okNode.Value == "true" {
		return true // a catch-all that claims success is not provably broken from here
	}
	errorNode := nodeGet(nodeGetInput{Node: root, Key: "error"})
	codeNode := nodeGet(nodeGetInput{Node: errorNode, Key: "code"})
	if codeNode == nil || codeNode.Value == "" {
		input.Report(Finding{
			Rule:    "BFD-3",
			Where:   where,
			Message: "failure carries no error code — errors carry enumerated codes, not free text",
		})
	}
	return true
}
