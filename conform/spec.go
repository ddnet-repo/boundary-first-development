package conform

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// The static tier: everything provable from the OpenAPI document alone,
// before a single request is made.

type specDocument struct {
	root *yaml.Node
}

type specLoadInput struct {
	Path string
}

type specLoadResult struct {
	Ok       bool
	Document specDocument
	Error    RunError
}

func specLoad(input specLoadInput) specLoadResult {
	raw, err := os.ReadFile(input.Path)
	if err != nil {
		return specLoadResult{Error: RunError{
			Code:    ErrorCodeSpecUnreadable,
			Message: fmt.Sprintf("cannot read spec %q: %v", input.Path, err),
		}}
	}
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		return specLoadResult{Error: RunError{
			Code:    ErrorCodeSpecInvalid,
			Message: fmt.Sprintf("spec %q does not parse: %v", input.Path, err),
		}}
	}
	root := nodeUnwrap(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		return specLoadResult{Error: RunError{
			Code:    ErrorCodeSpecInvalid,
			Message: fmt.Sprintf("spec %q is not a mapping at the top level", input.Path),
		}}
	}
	return specLoadResult{Ok: true, Document: specDocument{root: root}}
}

type specDerefInput struct {
	Document specDocument
	Node     *yaml.Node
	Depth    int
}

// specDeref follows local $ref pointers ("#/components/schemas/x") to the
// schema they name. Unresolvable refs and cycles return nil.
func specDeref(input specDerefInput) *yaml.Node {
	node := nodeUnwrap(input.Node)
	if node == nil || input.Depth > 16 {
		return nil
	}
	ref := nodeGet(nodeGetInput{Node: node, Key: "$ref"})
	if ref == nil {
		return node
	}
	target, local := strings.CutPrefix(ref.Value, "#/")
	if !local {
		return nil // cross-file refs are out of scope for the static tier
	}
	current := input.Document.root
	for _, segment := range strings.Split(target, "/") {
		current = nodeGet(nodeGetInput{Node: current, Key: segment})
		if current == nil {
			return nil
		}
	}
	return specDeref(specDerefInput{Document: input.Document, Node: current, Depth: input.Depth + 1})
}

type specCheckInput struct {
	Document specDocument
	Report   func(finding Finding)
}

func specCheckAll(input specCheckInput) {
	specCheckPaths(input)
	specCheckSchemas(input)
	specCheckSecurity(input)
	specSweepProperties(specSweepInput{
		Document: input.Document,
		Node:     input.Document.root,
		Path:     "",
		Report:   input.Report,
	})
}

var specMethodSet = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true,
}

func specCheckPaths(input specCheckInput) {
	paths := nodeGet(nodeGetInput{Node: input.Document.root, Key: "paths"})
	for _, pathEntry := range nodeEntries(paths) {
		specCheckPathName(specPathNameInput{Path: pathEntry.Key, Report: input.Report})
		pathParams := specParamNames(specParamsInput{Document: input.Document, Owner: pathEntry.Value, Report: input.Report, Where: "spec: " + pathEntry.Key})
		for _, methodEntry := range nodeEntries(pathEntry.Value) {
			if !specMethodSet[methodEntry.Key] {
				continue
			}
			where := fmt.Sprintf("spec: %s %s", strings.ToUpper(methodEntry.Key), pathEntry.Key)
			params := append([]string{}, pathParams...)
			params = append(params, specParamNames(specParamsInput{Document: input.Document, Owner: methodEntry.Value, Report: input.Report, Where: where})...)
			hasUpdatedAfter := false
			for _, name := range params {
				if name == "updatedAfter" {
					hasUpdatedAfter = true
				}
			}
			responses := nodeGet(nodeGetInput{Node: methodEntry.Value, Key: "responses"})
			for _, responseEntry := range nodeEntries(responses) {
				if !strings.HasPrefix(responseEntry.Key, "2") {
					continue
				}
				schema := specResponseSchema(specResponseSchemaInput{Document: input.Document, Response: responseEntry.Value})
				if schema == nil {
					continue
				}
				specCheckEnvelope(specEnvelopeInput{
					Document:        input.Document,
					Schema:          schema,
					Where:           where,
					Method:          methodEntry.Key,
					HasUpdatedAfter: hasUpdatedAfter,
					Report:          input.Report,
				})
			}
		}
	}
}

type specPathNameInput struct {
	Path   string
	Report func(finding Finding)
}

func specCheckPathName(input specPathNameInput) {
	for _, segment := range strings.Split(input.Path, "/") {
		if segment == "" || strings.HasPrefix(segment, "{") {
			continue
		}
		if violation, found := namingPluralFind(segment); found {
			input.Report(Finding{
				Rule:    "BFD-13",
				Where:   "spec: path " + input.Path,
				Message: fmt.Sprintf("irregular plural %q in the route — it is %q, you are speaking to computers", violation.Word, violation.Regular),
			})
		}
	}
}

type specParamsInput struct {
	Document specDocument
	Owner    *yaml.Node // a path item or an operation
	Where    string
	Report   func(finding Finding)
}

// specParamNames returns the query-parameter names an owner declares, and
// checks their casing on the way through.
func specParamNames(input specParamsInput) []string {
	names := []string{}
	parameters := nodeGet(nodeGetInput{Node: input.Owner, Key: "parameters"})
	parameters = nodeUnwrap(parameters)
	if parameters == nil || parameters.Kind != yaml.SequenceNode {
		return names
	}
	for _, item := range parameters.Content {
		parameter := specDeref(specDerefInput{Document: input.Document, Node: item})
		name := nodeGet(nodeGetInput{Node: parameter, Key: "name"})
		in := nodeGet(nodeGetInput{Node: parameter, Key: "in"})
		if name == nil || in == nil || in.Value != "query" {
			continue
		}
		names = append(names, name.Value)
		if !namingCamelOk(name.Value) {
			input.Report(Finding{
				Rule:    "BFD-11",
				Where:   input.Where,
				Message: fmt.Sprintf("query parameter %q is not camelCase — the wire toward the frontend speaks camelCase", name.Value),
			})
		}
	}
	return names
}

type specResponseSchemaInput struct {
	Document specDocument
	Response *yaml.Node
}

func specResponseSchema(input specResponseSchemaInput) *yaml.Node {
	response := specDeref(specDerefInput{Document: input.Document, Node: input.Response})
	content := nodeGet(nodeGetInput{Node: response, Key: "content"})
	media := nodeGet(nodeGetInput{Node: content, Key: "application/json"})
	schema := nodeGet(nodeGetInput{Node: media, Key: "schema"})
	if schema == nil {
		return nil
	}
	return specDeref(specDerefInput{Document: input.Document, Node: schema})
}

type specEnvelopeInput struct {
	Document        specDocument
	Schema          *yaml.Node
	Where           string
	Method          string
	HasUpdatedAfter bool
	Report          func(finding Finding)
}

// specCheckEnvelope proves BFD-2/3/7/8 against one declared response schema.
func specCheckEnvelope(input specEnvelopeInput) {
	properties := nodeGet(nodeGetInput{Node: input.Schema, Key: "properties"})
	if properties == nil {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   input.Where,
			Message: "response schema declares no properties — every boundary returns the ok/data/error envelope",
		})
		return
	}
	missing := []string{}
	for _, required := range []string{"ok", "data", "error"} {
		if nodeGet(nodeGetInput{Node: properties, Key: required}) == nil {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   input.Where,
			Message: fmt.Sprintf("response schema is missing %s — every boundary returns the ok/data/error envelope", quoteJoin(missing)),
		})
	}
	okSchema := specDeref(specDerefInput{Document: input.Document, Node: nodeGet(nodeGetInput{Node: properties, Key: "ok"})})
	if okType := nodeGet(nodeGetInput{Node: okSchema, Key: "type"}); okSchema != nil && (okType == nil || okType.Value != "boolean") {
		input.Report(Finding{
			Rule:    "BFD-2",
			Where:   input.Where,
			Message: `envelope field "ok" must be declared type boolean`,
		})
	}
	if nodeGet(nodeGetInput{Node: properties, Key: "serverTime"}) == nil {
		input.Report(Finding{
			Rule:    "BFD-7",
			Where:   input.Where,
			Message: `response schema has no "serverTime" — every API response includes the server's clock`,
		})
	}
	specCheckEnvelopeError(input)
	specCheckEnvelopeData(input)
}

func specCheckEnvelopeError(input specEnvelopeInput) {
	properties := nodeGet(nodeGetInput{Node: input.Schema, Key: "properties"})
	errorSchema := specDeref(specDerefInput{Document: input.Document, Node: nodeGet(nodeGetInput{Node: properties, Key: "error"})})
	if errorSchema == nil {
		return
	}
	errorProperties := nodeGet(nodeGetInput{Node: errorSchema, Key: "properties"})
	if errorProperties == nil {
		return
	}
	codeSchema := specDeref(specDerefInput{Document: input.Document, Node: nodeGet(nodeGetInput{Node: errorProperties, Key: "code"})})
	if codeSchema == nil {
		input.Report(Finding{
			Rule:    "BFD-3",
			Where:   input.Where,
			Message: `error schema has no "code" — errors carry enumerated codes, not free text`,
		})
		return
	}
	if nodeGet(nodeGetInput{Node: codeSchema, Key: "enum"}) == nil {
		input.Report(Finding{
			Rule:    "BFD-3",
			Where:   input.Where,
			Message: `error "code" declares no enum — the code is the contract, enumerate it`,
		})
	}
}

func specCheckEnvelopeData(input specEnvelopeInput) {
	properties := nodeGet(nodeGetInput{Node: input.Schema, Key: "properties"})
	dataSchema := specDeref(specDerefInput{Document: input.Document, Node: nodeGet(nodeGetInput{Node: properties, Key: "data"})})
	dataType := nodeGet(nodeGetInput{Node: dataSchema, Key: "type"})
	if dataType == nil || dataType.Value != "array" {
		return
	}
	if input.Method == "get" && !input.HasUpdatedAfter {
		input.Report(Finding{
			Rule:    "BFD-8",
			Where:   input.Where,
			Message: `list endpoint declares no "updatedAfter" query parameter — clients sync with ?updatedAfter=<lastServerTime>`,
		})
	}
	items := specDeref(specDerefInput{Document: input.Document, Node: nodeGet(nodeGetInput{Node: dataSchema, Key: "items"})})
	specCheckModel(specModelInput{
		Document: input.Document,
		Schema:   items,
		Where:    input.Where + " data items",
		Report:   input.Report,
	})
}

type specModelInput struct {
	Document specDocument
	Schema   *yaml.Node
	Where    string
	Report   func(finding Finding)
}

// specCheckModel proves BFD-7 on anything that looks like a model: a schema
// with an "id" must carry "status" and "updatedAt".
func specCheckModel(input specModelInput) {
	properties := nodeGet(nodeGetInput{Node: input.Schema, Key: "properties"})
	if properties == nil || nodeGet(nodeGetInput{Node: properties, Key: "id"}) == nil {
		return
	}
	for _, required := range []string{"status", "updatedAt"} {
		if nodeGet(nodeGetInput{Node: properties, Key: required}) == nil {
			input.Report(Finding{
				Rule:    "BFD-7",
				Where:   input.Where,
				Message: fmt.Sprintf("model schema has an \"id\" but no %q — every model carries status and updatedAt", required),
			})
		}
	}
}

func specCheckSchemas(input specCheckInput) {
	components := nodeGet(nodeGetInput{Node: input.Document.root, Key: "components"})
	schemas := nodeGet(nodeGetInput{Node: components, Key: "schemas"})
	for _, schemaEntry := range nodeEntries(schemas) {
		where := "spec: schema " + schemaEntry.Key
		if violation, found := namingPluralFind(schemaEntry.Key); found {
			input.Report(Finding{
				Rule:    "BFD-13",
				Where:   where,
				Message: fmt.Sprintf("irregular plural %q in the schema name — it is %q", violation.Word, violation.Regular),
			})
		}
		specCheckModel(specModelInput{
			Document: input.Document,
			Schema:   specDeref(specDerefInput{Document: input.Document, Node: schemaEntry.Value}),
			Where:    where,
			Report:   input.Report,
		})
	}
}

// specCheckSecurity proves the documented, key-authed Public API surface of
// BFD-18 exists. The App API is allowed to live outside the spec — it moves
// with the product — so only the apiKey scheme is demanded here.
func specCheckSecurity(input specCheckInput) {
	components := nodeGet(nodeGetInput{Node: input.Document.root, Key: "components"})
	schemes := nodeGet(nodeGetInput{Node: components, Key: "securitySchemes"})
	for _, schemeEntry := range nodeEntries(schemes) {
		schemeType := nodeGet(nodeGetInput{Node: schemeEntry.Value, Key: "type"})
		if schemeType != nil && schemeType.Value == "apiKey" {
			return
		}
	}
	input.Report(Finding{
		Rule:    "BFD-18",
		Where:   "spec: components.securitySchemes",
		Message: "no apiKey security scheme declared — the Public API takes an API key and is documented via OpenAPI",
	})
}

type specSweepInput struct {
	Document specDocument
	Node     *yaml.Node
	Path     string
	Report   func(finding Finding)
}

// specSweepProperties walks the whole document and checks every JSON-Schema
// "properties" mapping it meets: property names must be camelCase (BFD-11)
// with regular plurals (BFD-13), and *At fields must be declared RFC3339
// date-times (BFD-12).
func specSweepProperties(input specSweepInput) {
	node := nodeUnwrap(input.Node)
	if node == nil {
		return
	}
	if node.Kind == yaml.SequenceNode {
		for i, item := range node.Content {
			specSweepProperties(specSweepInput{
				Document: input.Document,
				Node:     item,
				Path:     fmt.Sprintf("%s[%d]", input.Path, i),
				Report:   input.Report,
			})
		}
		return
	}
	if node.Kind != yaml.MappingNode {
		return
	}
	for _, entry := range nodeEntries(node) {
		childPath := entry.Key
		if input.Path != "" {
			childPath = input.Path + "." + entry.Key
		}
		if entry.Key == "properties" && nodeUnwrap(entry.Value) != nil && nodeUnwrap(entry.Value).Kind == yaml.MappingNode {
			specSweepPropertyNames(specSweepInput{
				Document: input.Document,
				Node:     entry.Value,
				Path:     childPath,
				Report:   input.Report,
			})
		}
		specSweepProperties(specSweepInput{
			Document: input.Document,
			Node:     entry.Value,
			Path:     childPath,
			Report:   input.Report,
		})
	}
}

func specSweepPropertyNames(input specSweepInput) {
	for _, property := range nodeEntries(input.Node) {
		where := "spec: " + input.Path + "." + property.Key
		if !namingCamelOk(property.Key) {
			input.Report(Finding{
				Rule:    "BFD-11",
				Where:   where,
				Message: fmt.Sprintf("property %q is not camelCase — translation to camelCase happens at the boundary, and the spec is the boundary", property.Key),
			})
		}
		if violation, found := namingPluralFind(property.Key); found {
			input.Report(Finding{
				Rule:    "BFD-13",
				Where:   where,
				Message: fmt.Sprintf("irregular plural %q in the property name — it is %q", violation.Word, violation.Regular),
			})
		}
		if strings.HasSuffix(property.Key, "At") && len(property.Key) > 2 {
			propertySchema := specDeref(specDerefInput{Document: input.Document, Node: property.Value})
			propertyType := nodeGet(nodeGetInput{Node: propertySchema, Key: "type"})
			propertyFormat := nodeGet(nodeGetInput{Node: propertySchema, Key: "format"})
			if propertyType == nil || propertyType.Value != "string" || propertyFormat == nil || propertyFormat.Value != "date-time" {
				input.Report(Finding{
					Rule:    "BFD-12",
					Where:   where,
					Message: fmt.Sprintf("timestamp %q must be declared type string, format date-time — UTC on the wire, always", property.Key),
				})
			}
		}
	}
}

type specEndpointsInput struct {
	Document specDocument
}

// specEndpointsList returns the parameterless GET paths of the spec — the
// endpoints the wire tier can probe read-only without inventing values.
func specEndpointsList(input specEndpointsInput) []string {
	endpoints := []string{}
	paths := nodeGet(nodeGetInput{Node: input.Document.root, Key: "paths"})
	for _, pathEntry := range nodeEntries(paths) {
		if strings.Contains(pathEntry.Key, "{") {
			continue
		}
		if nodeGet(nodeGetInput{Node: pathEntry.Value, Key: "get"}) != nil {
			endpoints = append(endpoints, pathEntry.Key)
		}
	}
	sort.Strings(endpoints)
	return endpoints
}
