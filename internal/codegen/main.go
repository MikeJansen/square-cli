// Command codegen generates square-cli's typed models and command tree from
// the vendored Square OpenAPI specification.
//
// Usage:
//
//	go run ./internal/codegen           # uses api/openapi/square-api.json
//	go run ./internal/codegen -spec X   # custom spec path
//
// It writes:
//
//	pkg/square/models_gen.go            # typed request/response/enum models
//	internal/commands/<api>_gen.go      # one cobra command group per API
//	internal/commands/registry_gen.go   # AddCommands wiring
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// ---- OpenAPI subset ----

type Spec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
}

type Info struct {
	Version string `json:"version"`
	Title   string `json:"title"`
}

type Components struct {
	Schemas map[string]*Schema `json:"schemas"`
	XApis   map[string]XApi    `json:"x-apis"`
}

type XApi struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type PathItem map[string]*Operation

type Operation struct {
	Tags          []string            `json:"tags"`
	Summary       string              `json:"summary"`
	OperationID   string              `json:"operationId"`
	Description   string              `json:"description"`
	Deprecated    bool                `json:"deprecated"`
	ReleaseStatus string              `json:"x-release-status"`
	Parameters    []Parameter         `json:"parameters"`
	RequestBody   *RequestBody        `json:"requestBody"`
	Responses     map[string]Response `json:"responses"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description"`
	Required    bool    `json:"required"`
	Schema      *Schema `json:"schema"`
}

type RequestBody struct {
	Required bool                 `json:"required"`
	Content  map[string]MediaType `json:"content"`
}

type Response struct {
	Content map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema *Schema `json:"schema"`
}

type Schema struct {
	Ref                  string             `json:"$ref"`
	Type                 string             `json:"type"`
	Format               string             `json:"format"`
	Description          string             `json:"description"`
	Enum                 []any              `json:"enum"`
	Items                *Schema            `json:"items"`
	Properties           map[string]*Schema `json:"properties"`
	Required             []string           `json:"required"`
	AdditionalProperties json.RawMessage    `json:"additionalProperties"`
}

var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true, "patch": true,
}

// definedSchemas holds the set of schema names actually defined in the spec, so
// that dangling $ref targets can fall back to json.RawMessage.
var definedSchemas = map[string]bool{}

// Documentation slug overrides where the tag name does not kebab-case cleanly.
var docSlugOverride = map[string]string{
	"OAuth": "oauth",
}

func main() {
	specPath := flag.String("spec", "api/openapi/square-api.json", "path to OpenAPI spec")
	root := flag.String("root", ".", "module root directory")
	flag.Parse()

	data, err := os.ReadFile(*specPath)
	if err != nil {
		log.Fatalf("read spec: %v", err)
	}
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		log.Fatalf("parse spec: %v", err)
	}
	for name := range spec.Components.Schemas {
		definedSchemas[name] = true
	}

	g := &generator{spec: &spec}
	g.writeModels(filepath.Join(*root, "pkg", "square", "models_gen.go"))
	g.writeCommands(*root)
	fmt.Printf("codegen complete: %d schemas, %d operations across %d APIs\n",
		len(spec.Components.Schemas), g.opCount, len(g.apiTags()))
}

type generator struct {
	spec    *Spec
	opCount int
}

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

func (g *generator) writeModels(path string) {
	names := make([]string, 0, len(g.spec.Components.Schemas))
	for n := range g.spec.Components.Schemas {
		names = append(names, n)
	}
	sort.Strings(names)

	var b bytes.Buffer
	body := &bytes.Buffer{}
	usesRaw := false

	for _, name := range names {
		s := g.spec.Components.Schemas[name]
		switch {
		case len(s.Enum) > 0 && s.Type == "string":
			writeEnum(body, name, s)
		case s.Properties != nil || s.Type == "object":
			r := writeStruct(body, name, s)
			usesRaw = usesRaw || r
		case s.Type == "array":
			body.WriteString(docComment(s.Description))
			fmt.Fprintf(body, "type %s %s\n\n", name, goType(s.Items, &usesRaw))
		case s.Type == "string" || s.Type == "integer" || s.Type == "number" || s.Type == "boolean":
			body.WriteString(docComment(s.Description))
			fmt.Fprintf(body, "type %s %s\n\n", name, scalarType(s.Type, s.Format))
		default:
			body.WriteString(docComment(s.Description))
			fmt.Fprintf(body, "type %s = json.RawMessage\n\n", name)
			usesRaw = true
		}
	}

	b.WriteString("// Code generated by internal/codegen; DO NOT EDIT.\n\n")
	b.WriteString("// Package square contains typed models for every Square API request,\n")
	b.WriteString("// response, and enum, generated from the official OpenAPI specification.\n")
	b.WriteString("package square\n\n")
	if usesRaw {
		b.WriteString("import \"encoding/json\"\n\n")
	}
	b.Write(body.Bytes())

	writeGoFile(path, b.Bytes())
}

func writeEnum(b *bytes.Buffer, name string, s *Schema) {
	b.WriteString(docComment(s.Description))
	fmt.Fprintf(b, "type %s string\n\n", name)
	fmt.Fprintf(b, "// Enumerated values for %s.\nconst (\n", name)
	seen := map[string]bool{}
	for _, ev := range s.Enum {
		val, ok := ev.(string)
		if !ok {
			continue
		}
		cn := name + pascal(val)
		for seen[cn] {
			cn += "_"
		}
		seen[cn] = true
		fmt.Fprintf(b, "\t%s %s = %q\n", cn, name, val)
	}
	b.WriteString(")\n\n")
}

// writeStruct emits a struct type and reports whether it used json.RawMessage.
func writeStruct(b *bytes.Buffer, name string, s *Schema) bool {
	usesRaw := false
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}
	b.WriteString(docComment(s.Description))
	if len(s.Properties) == 0 {
		fmt.Fprintf(b, "type %s map[string]any\n\n", name)
		return false
	}
	fmt.Fprintf(b, "type %s struct {\n", name)
	propNames := make([]string, 0, len(s.Properties))
	for pn := range s.Properties {
		propNames = append(propNames, pn)
	}
	sort.Strings(propNames)
	usedFields := map[string]bool{}
	for _, pn := range propNames {
		ps := s.Properties[pn]
		field := fieldName(pn)
		for usedFields[field] {
			field += "_"
		}
		usedFields[field] = true
		typ := goType(ps, &usesRaw)
		if !required[pn] && needsPointer(typ) {
			typ = "*" + typ
		}
		if c := shortComment(ps.Description); c != "" {
			fmt.Fprintf(b, "\t// %s\n", c)
		}
		fmt.Fprintf(b, "\t%s %s `json:%q`\n", field, typ, pn+",omitempty")
	}
	b.WriteString("}\n\n")
	return usesRaw
}

// needsPointer reports whether an optional field of the given Go type should be
// a pointer (scalars, enums, and struct refs) rather than left as-is (slices,
// maps, json.RawMessage).
func needsPointer(typ string) bool {
	switch {
	case strings.HasPrefix(typ, "[]"), strings.HasPrefix(typ, "map["):
		return false
	case typ == "json.RawMessage", typ == "any":
		return false
	default:
		return true
	}
}

// goType maps a property/items schema to a Go type expression.
func goType(s *Schema, usesRaw *bool) string {
	if s == nil {
		*usesRaw = true
		return "json.RawMessage"
	}
	if s.Ref != "" {
		name := refName(s.Ref)
		if !definedSchemas[name] {
			*usesRaw = true
			return "json.RawMessage"
		}
		return name
	}
	switch s.Type {
	case "string":
		return "string"
	case "integer":
		return scalarType(s.Type, s.Format)
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		return "[]" + goType(s.Items, usesRaw)
	case "object":
		if elem := additionalPropsType(s, usesRaw); elem != "" {
			return "map[string]" + elem
		}
		if len(s.Properties) > 0 {
			// Inline anonymous objects are rare in this spec; keep them raw.
			*usesRaw = true
			return "json.RawMessage"
		}
		return "map[string]any"
	default:
		*usesRaw = true
		return "json.RawMessage"
	}
}

// additionalPropsType returns the element type for a map schema, or "" if the
// schema is not a map.
func additionalPropsType(s *Schema, usesRaw *bool) string {
	if len(s.AdditionalProperties) == 0 {
		return ""
	}
	var asBool bool
	if err := json.Unmarshal(s.AdditionalProperties, &asBool); err == nil {
		if asBool {
			return "any"
		}
		return ""
	}
	var sub Schema
	if err := json.Unmarshal(s.AdditionalProperties, &sub); err == nil {
		return goType(&sub, usesRaw)
	}
	return "any"
}

func scalarType(t, format string) string {
	switch t {
	case "integer":
		if format == "int32" {
			return "int32"
		}
		return "int64"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	default:
		return "string"
	}
}

func refName(ref string) string {
	i := strings.LastIndex(ref, "/")
	return ref[i+1:]
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (g *generator) apiTags() []string {
	set := map[string]bool{}
	for _, item := range g.spec.Paths {
		for m, op := range item {
			if !httpMethods[m] || op == nil || len(op.Tags) == 0 {
				continue
			}
			set[op.Tags[0]] = true
		}
	}
	tags := make([]string, 0, len(set))
	for t := range set {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}

type endpoint struct {
	method string
	path   string
	op     *Operation
}

func (g *generator) writeCommands(root string) {
	byTag := map[string][]endpoint{}
	for path, item := range g.spec.Paths {
		for m, op := range item {
			if !httpMethods[m] || op == nil || op.OperationID == "" || len(op.Tags) == 0 {
				continue
			}
			tag := op.Tags[0]
			byTag[tag] = append(byTag[tag], endpoint{method: strings.ToUpper(m), path: path, op: op})
			g.opCount++
		}
	}

	tags := g.apiTags()
	for _, tag := range tags {
		eps := byTag[tag]
		sort.Slice(eps, func(i, j int) bool { return eps[i].op.OperationID < eps[j].op.OperationID })
		g.writeTagFile(root, tag, eps)
	}
	g.writeRegistry(root, tags)
}

func (g *generator) writeTagFile(root, tag string, eps []endpoint) {
	var body bytes.Buffer
	usesSquare := false
	usesURL := false

	summary := ""
	if api, ok := g.spec.Components.XApis[tag]; ok {
		summary = api.Summary
	}

	// Group builder.
	fmt.Fprintf(&body, "func new%sGroup() *cobra.Command {\n", tag)
	fmt.Fprintf(&body, "\tgroup := &cobra.Command{\n")
	fmt.Fprintf(&body, "\t\tUse:   %q,\n", groupName(tag))
	fmt.Fprintf(&body, "\t\tShort: %q,\n", oneLine(firstNonEmptyStr(summary, tag+" API")))
	fmt.Fprintf(&body, "\t\tLong:  %q,\n", oneLine(firstNonEmptyStr(summary, tag+" API")))
	fmt.Fprintf(&body, "\t}\n")
	for _, ep := range eps {
		fmt.Fprintf(&body, "\tgroup.AddCommand(new%s%sCmd())\n", tag, ep.op.OperationID)
	}
	fmt.Fprintf(&body, "\treturn group\n}\n\n")

	// Per-endpoint builders.
	for _, ep := range eps {
		g.writeEndpoint(&body, tag, ep, &usesSquare, &usesURL)
	}

	var b bytes.Buffer
	b.WriteString("// Code generated by internal/codegen; DO NOT EDIT.\n\n")
	b.WriteString("package commands\n\n")
	b.WriteString("import (\n")
	if usesURL {
		b.WriteString("\t\"net/url\"\n\n")
	}
	b.WriteString("\t\"github.com/spf13/cobra\"\n")
	if usesSquare {
		b.WriteString("\n\t\"github.com/MikeJansen/square-cli/pkg/square\"\n")
	}
	b.WriteString(")\n\n")
	b.Write(body.Bytes())

	writeGoFile(filepath.Join(root, "internal", "commands", fileBase(tag)+"_gen.go"), b.Bytes())
}

func (g *generator) writeEndpoint(b *bytes.Buffer, tag string, ep endpoint, usesSquare, usesURL *bool) {
	op := ep.op
	docURL := docURL(tag, op.OperationID)

	// Split parameters.
	var pathParams, queryParams []Parameter
	for _, p := range op.Parameters {
		switch p.In {
		case "path":
			pathParams = append(pathParams, p)
		case "query":
			queryParams = append(queryParams, p)
		}
	}
	sort.Slice(queryParams, func(i, j int) bool { return queryParams[i].Name < queryParams[j].Name })

	// Request / response prototypes.
	reqType := bodySchemaRef(op.RequestBody)
	respType := responseSchemaRef(op.Responses)
	hasBody := op.RequestBody != nil
	if reqType != "" || respType != "" {
		*usesSquare = true
	}

	long := op.Summary
	if op.Description != "" {
		long += "\n\n" + op.Description
	}
	long += "\n\nDocumentation: " + docURL

	fmt.Fprintf(b, "func new%s%sCmd() *cobra.Command {\n", tag, op.OperationID)
	fmt.Fprintf(b, "\tcmd := &cobra.Command{\n")
	fmt.Fprintf(b, "\t\tUse:   %q,\n", cmdName(op.OperationID))
	fmt.Fprintf(b, "\t\tShort: %q,\n", oneLine(op.Summary))
	fmt.Fprintf(b, "\t\tLong:  %q,\n", long)
	fmt.Fprintf(b, "\t\tArgs:  cobra.NoArgs,\n")
	if op.Deprecated {
		fmt.Fprintf(b, "\t\tDeprecated: %q,\n", "this endpoint is deprecated by Square")
	}
	// RunE
	fmt.Fprintf(b, "\t\tRunE: func(cmd *cobra.Command, args []string) error {\n")
	*usesURL = true
	fmt.Fprintf(b, "\t\t\tq := url.Values{}\n")
	for _, p := range queryParams {
		fmt.Fprintf(b, "\t\t\t%s(cmd, q, %q, %q)\n", queryHelper(p.Schema), p.Name, flagName(p.Name))
	}
	if len(pathParams) > 0 {
		fmt.Fprintf(b, "\t\t\tpath := BuildPath(cmd, %q, map[string]string{\n", ep.path)
		for _, p := range pathParams {
			fmt.Fprintf(b, "\t\t\t\t%q: %q,\n", p.Name, flagName(p.Name))
		}
		fmt.Fprintf(b, "\t\t\t})\n")
	} else {
		fmt.Fprintf(b, "\t\t\tpath := %q\n", ep.path)
	}
	fmt.Fprintf(b, "\t\t\treturn Execute(cmd, Op{\n")
	fmt.Fprintf(b, "\t\t\t\tMethod:  %q,\n", ep.method)
	fmt.Fprintf(b, "\t\t\t\tPath:    path,\n")
	fmt.Fprintf(b, "\t\t\t\tQuery:   q,\n")
	fmt.Fprintf(b, "\t\t\t\tHasBody: %t,\n", hasBody)
	if reqType != "" {
		fmt.Fprintf(b, "\t\t\t\tRequestProto:  func() any { return new(square.%s) },\n", reqType)
	}
	if respType != "" {
		fmt.Fprintf(b, "\t\t\t\tResponseProto: func() any { return new(square.%s) },\n", respType)
	}
	fmt.Fprintf(b, "\t\t\t\tDocURL: %q,\n", docURL)
	fmt.Fprintf(b, "\t\t\t})\n")
	fmt.Fprintf(b, "\t\t},\n")
	fmt.Fprintf(b, "\t}\n")

	// Flags.
	for _, p := range pathParams {
		fmt.Fprintf(b, "\tcmd.Flags().String(%q, \"\", %q)\n", flagName(p.Name), flagUsage(p.Description)+" (required)")
		fmt.Fprintf(b, "\t_ = cmd.MarkFlagRequired(%q)\n", flagName(p.Name))
	}
	for _, p := range queryParams {
		writeQueryFlag(b, p)
	}
	if hasBody {
		fmt.Fprintf(b, "\tAddBodyFlags(cmd)\n")
	}
	fmt.Fprintf(b, "\treturn cmd\n}\n\n")
}

func writeQueryFlag(b *bytes.Buffer, p Parameter) {
	usage := flagUsage(p.Description)
	name := flagName(p.Name)
	t := ""
	if p.Schema != nil {
		t = p.Schema.Type
	}
	switch t {
	case "array":
		fmt.Fprintf(b, "\tcmd.Flags().StringSlice(%q, nil, %q)\n", name, usage)
	case "integer":
		fmt.Fprintf(b, "\tcmd.Flags().Int64(%q, 0, %q)\n", name, usage)
	case "number":
		fmt.Fprintf(b, "\tcmd.Flags().Float64(%q, 0, %q)\n", name, usage)
	case "boolean":
		fmt.Fprintf(b, "\tcmd.Flags().Bool(%q, false, %q)\n", name, usage)
	default:
		fmt.Fprintf(b, "\tcmd.Flags().String(%q, \"\", %q)\n", name, usage)
	}
	if p.Required {
		fmt.Fprintf(b, "\t_ = cmd.MarkFlagRequired(%q)\n", name)
	}
}

func queryHelper(s *Schema) string {
	if s == nil {
		return "QStr"
	}
	switch s.Type {
	case "array":
		return "QStrList"
	case "integer":
		return "QInt"
	case "number":
		return "QNum"
	case "boolean":
		return "QBool"
	default:
		return "QStr"
	}
}

func (g *generator) writeRegistry(root string, tags []string) {
	var b bytes.Buffer
	b.WriteString("// Code generated by internal/codegen; DO NOT EDIT.\n\n")
	b.WriteString("package commands\n\n")
	b.WriteString("import \"github.com/spf13/cobra\"\n\n")
	b.WriteString("// AddCommands registers every generated Square API command group on root.\n")
	b.WriteString("func AddCommands(root *cobra.Command) {\n")
	for _, tag := range tags {
		fmt.Fprintf(&b, "\troot.AddCommand(new%sGroup())\n", tag)
	}
	b.WriteString("}\n")
	writeGoFile(filepath.Join(root, "internal", "commands", "registry_gen.go"), b.Bytes())
}

func bodySchemaRef(rb *RequestBody) string {
	if rb == nil {
		return ""
	}
	if mt, ok := rb.Content["application/json"]; ok && mt.Schema != nil && mt.Schema.Ref != "" {
		if name := refName(mt.Schema.Ref); definedSchemas[name] {
			return name
		}
	}
	return ""
}

func responseSchemaRef(responses map[string]Response) string {
	for _, code := range []string{"200", "201", "202"} {
		if r, ok := responses[code]; ok {
			if mt, ok := r.Content["application/json"]; ok && mt.Schema != nil && mt.Schema.Ref != "" {
				if name := refName(mt.Schema.Ref); definedSchemas[name] {
					return name
				}
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Naming & formatting helpers
// ---------------------------------------------------------------------------

func docURL(tag, operationID string) string {
	slug, ok := docSlugOverride[tag]
	if !ok {
		slug = kebab(tag)
	}
	return "https://developer.squareup.com/reference/square/" + slug + "-api/" + kebab(operationID)
}

func groupName(tag string) string {
	if s, ok := docSlugOverride[tag]; ok {
		return s
	}
	return kebab(tag)
}

func cmdName(operationID string) string {
	return kebab(operationID)
}

func fileBase(tag string) string {
	return strings.ReplaceAll(groupName(tag), "-", "_")
}

func flagName(paramName string) string {
	return strings.ReplaceAll(paramName, "_", "-")
}

// pascal converts a token (snake_case or PascalCase) to PascalCase, preserving
// existing internal capitalization.
func pascal(s string) string {
	var out strings.Builder
	upNext := true
	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' || r == '.' || r == '/' {
			upNext = true
			continue
		}
		if upNext {
			out.WriteRune(unicode.ToUpper(r))
			upNext = false
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func fieldName(jsonName string) string {
	f := pascal(jsonName)
	if f == "" {
		return "Field"
	}
	if r := []rune(f)[0]; unicode.IsDigit(r) {
		f = "N" + f
	}
	return f
}

// kebab converts PascalCase/camelCase/snake_case to kebab-case.
func kebab(s string) string {
	runes := []rune(s)
	var out []rune
	for i, r := range runes {
		if r == '_' || r == ' ' || r == '.' {
			out = append(out, '-')
			continue
		}
		if unicode.IsUpper(r) {
			prevLowerOrDigit := i > 0 && (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]))
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1]) && i > 0 && unicode.IsUpper(runes[i-1])
			if prevLowerOrDigit || nextLower {
				out = append(out, '-')
			}
			out = append(out, unicode.ToLower(r))
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func docComment(string) string {
	// Keep models terse; a full description bloats the file without helping.
	return ""
}

// flagUsage renders a concise, single-line flag description.
func flagUsage(desc string) string {
	c := oneLine(desc)
	if len(c) > 140 {
		c = strings.TrimSpace(c[:137]) + "..."
	}
	return c
}

func shortComment(desc string) string {
	c := oneLine(desc)
	if len(c) > 110 {
		c = c[:107] + "..."
	}
	return c
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	// Backticks in pflag usage strings are interpreted as the value placeholder
	// name, so strip Markdown code ticks from single-line usage text.
	s = strings.ReplaceAll(s, "`", "")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func writeGoFile(path string, src []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", path, err)
	}
	formatted, err := format.Source(src)
	if err != nil {
		// Write unformatted for debugging, then fail.
		_ = os.WriteFile(path+".broken", src, 0o644)
		log.Fatalf("format %s: %v (wrote %s.broken)", path, err, path)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
}
