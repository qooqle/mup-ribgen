package dsl

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

// GeneratedCode holds the result of a DSL compilation.
type GeneratedCode struct {
	DialectName     string
	PackageName     string
	TransformerImpl string // Go source code
}

// Compile parses dslSource and generates a Go DialectTransformer implementation.
// The generated code targets the given goPackage (e.g. "dialect").
func Compile(dslSource, sourceName, goPackage string) (*GeneratedCode, error) {
	ast, err := Parse(dslSource, sourceName)
	if err != nil {
		return nil, fmt.Errorf("dsl compile: parse: %w", err)
	}
	return CompileAST(ast, goPackage)
}

// CompileAST generates Go source from an already-parsed DSLAst.
func CompileAST(ast *DSLAst, goPackage string) (*GeneratedCode, error) {
	if ast.DialectName == "" {
		return nil, fmt.Errorf("dsl compile: dialect name is empty")
	}
	typeName := toTypeName(ast.DialectName)

	data := tmplData{
		Package:     goPackage,
		DialectName: ast.DialectName,
		TypeName:    typeName,
	}

	for _, mb := range ast.Mappings {
		switch mb.Name {
		case "pfcp_establishment_to_state":
			data.EstablishmentRules = rulesCode(mb.Rules, "req.Fields", "state")
		case "pfcp_modification_to_delta":
			data.ModificationRules = rulesCode(mb.Rules, "req.Fields", "delta")
		case "state_to_session_info":
			data.SessionInfoRules = rulesCode(mb.Rules, "state", "info")
		}
	}

	var buf bytes.Buffer
	if err := transformerTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("dsl compile: template: %w", err)
	}
	return &GeneratedCode{
		DialectName:     ast.DialectName,
		PackageName:     goPackage,
		TransformerImpl: buf.String(),
	}, nil
}

// --- template data types ----------------------------------------------------

type tmplData struct {
	Package            string
	DialectName        string
	TypeName           string
	EstablishmentRules []ruleStmt
	ModificationRules  []ruleStmt
	SessionInfoRules   []ruleStmt
}

type ruleStmt struct {
	Comment   string
	Code      string // Go statement(s)
}

// --- code generation for rules ----------------------------------------------

func rulesCode(rules []*MappingRule, srcVar, dstVar string) []ruleStmt {
	var stmts []ruleStmt
	for _, r := range rules {
		stmt := ruleStmt{Comment: "// " + r.Source.String() + " -> " + r.Dest.String()}
		stmt.Code = genRuleCode(r, srcVar, dstVar)
		stmts = append(stmts, stmt)
	}
	return stmts
}

// genRuleCode generates a Go snippet that implements a single MappingRule.
// This simplified implementation handles:
//   - Flat paths (no array) → direct field access / struct assignment
//   - Paths with one [*] → for-loop over a slice
func genRuleCode(r *MappingRule, srcVar, dstVar string) string {
	srcHasWildcard := pathHasWildcard(r.Source)
	if !srcHasWildcard {
		return genFlatRule(r, srcVar, dstVar)
	}
	return genLoopRule(r, srcVar, dstVar)
}

// genFlatRule generates code for a simple (non-array) mapping.
func genFlatRule(r *MappingRule, srcVar, dstVar string) string {
	srcAccess := genSrcAccess(r.Source, srcVar, "")
	dstAssign := genDstAssign(r.Dest, dstVar, "v", r.Transform, "")

	var sb strings.Builder
	if r.Condition != nil {
		condPath := genSrcAccess(r.Condition.Path, srcVar, "")
		if r.Condition.NotExist {
			sb.WriteString(fmt.Sprintf("if %s == nil {\n", condPath))
		} else {
			sb.WriteString(fmt.Sprintf("if %s != nil {\n", condPath))
		}
		sb.WriteString(fmt.Sprintf("    v := %s\n    _ = v\n    %s\n}", srcAccess, dstAssign))
	} else {
		sb.WriteString(fmt.Sprintf("if v := %s; v != nil {\n    %s\n}", srcAccess, dstAssign))
	}
	return sb.String()
}

// genLoopRule generates a for-loop for rules with [*] in the source path.
func genLoopRule(r *MappingRule, srcVar, dstVar string) string {
	// Find the wildcard segment index in the source path
	wildcardIdx := -1
	for i, seg := range r.Source.Segments {
		if seg.Index != nil && seg.Index.Kind == IndexWildcard {
			wildcardIdx = i
			break
		}
	}
	if wildcardIdx < 0 {
		return "// (no wildcard found)"
	}
	// Build the full path up to and including the wildcard segment name.
	// Use rt.GetField so intermediate map[string]interface{} traversal is safe.
	arrSegment := r.Source.Segments[wildcardIdx]
	arrPath := append(r.Source.Segments[:wildcardIdx:wildcardIdx], PathSegment{Name: arrSegment.Name})
	arrAccess := fmt.Sprintf("rt.GetField(%s, %s)", srcVar, quotedKeys(arrPath))

	// The inner path (after [*]) is accessed on each item
	innerSrc := FieldPath{Segments: r.Source.Segments[wildcardIdx+1:]}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("if arr, ok := rt.GetArray(%s); ok {\n", arrAccess))
	sb.WriteString("    for _, item := range arr {\n")
	sb.WriteString("        itemFields, _ := item.(map[string]interface{})\n")
	innerAccess := genSrcAccess(innerSrc, "itemFields", "")
	dstAssign := genDstAssign(r.Dest, dstVar, "v", r.Transform, "itemFields")
	sb.WriteString(fmt.Sprintf("        if v := %s; v != nil {\n            %s\n        }\n", innerAccess, dstAssign))
	sb.WriteString("    }\n}")
	return sb.String()
}

// genSrcAccess returns a Go expression that retrieves the value at path.
// For paths without wildcards, srcVar is the starting variable.
func genSrcAccess(path FieldPath, srcVar, _ string) string {
	if len(path.Segments) == 0 {
		return srcVar
	}
	// For struct source (state/info): use direct field access
	if srcVar == "state" || srcVar == "info" || srcVar == "itemFields" {
		return genFieldChain(path.Segments, srcVar)
	}
	// For map sources (req.Fields, delta): use runtime helper
	return fmt.Sprintf(`rt.GetField(%s, %s)`, srcVar, quotedKeys(path.Segments))
}

func genFieldChain(segs []PathSegment, base string) string {
	s := base
	for _, seg := range segs {
		s += `["` + seg.Name + `"]`
	}
	return s
}

func quotedKeys(segs []PathSegment) string {
	keys := make([]string, len(segs))
	for i, seg := range segs {
		keys[i] = `"` + seg.Name + `"`
	}
	return strings.Join(keys, ", ")
}

// genDstAssign returns Go code to assign `valExpr` to the destination path.
func genDstAssign(path FieldPath, dstVar, valExpr string, tx *TransformExpr, _ string) string {
	if len(path.Segments) == 0 {
		return ""
	}
	// Apply transform if present
	if tx != nil {
		args := ""
		if len(tx.Args) > 0 {
			args = ", " + strings.Join(tx.Args, ", ")
		}
		valExpr = fmt.Sprintf("rt.Transform_%s(%s%s)", tx.Func, valExpr, args)
	}
	// Simple struct field assignment (covers State.SEID, SessionInfo.TEID, etc.)
	if len(path.Segments) == 1 {
		return fmt.Sprintf("%s.%s = rt.CoerceField(%s)", dstVar, path.Segments[0].Name, valExpr)
	}
	// Nested / array destinations — emit a map set
	return fmt.Sprintf(`rt.SetField(%s, %s, %s)`, dstVar, quotedKeys(path.Segments), valExpr)
}

func pathHasWildcard(fp FieldPath) bool {
	for _, seg := range fp.Segments {
		if seg.Index != nil && seg.Index.Kind == IndexWildcard {
			return true
		}
	}
	return false
}

// toTypeName converts "Keysight_N9" → "KeysightN9".
func toTypeName(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	})
	var sb strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return sb.String()
}

// --- Go code template -------------------------------------------------------

var transformerTmpl = template.Must(template.New("transformer").Funcs(template.FuncMap{
	"indent": func(s, prefix string) string {
		lines := strings.Split(s, "\n")
		var sb strings.Builder
		for i, l := range lines {
			if i > 0 {
				sb.WriteString("\n")
			}
			if l != "" {
				sb.WriteString(prefix + l)
			}
		}
		return sb.String()
	},
}).Parse(`// Code generated by DSL Compiler. DO NOT EDIT.
// Dialect: {{ .DialectName }}

package {{ .Package }}

import (
	"time"

	"github.com/qooqle/mup-ribgen/pkg/dslruntime"
	"github.com/qooqle/mup-ribgen/pkg/ir"
	"github.com/qooqle/mup-ribgen/pkg/pfcp"
)

// rt is the DSL runtime helper package alias.
var rt = dslruntime.RT{}

func init() {
	// Register this transformer in the global dialect registry (import side-effect).
	_ = rt
}

// {{ .TypeName }}Transformer is the compiled Dialect Transformer for "{{ .DialectName }}".
type {{ .TypeName }}Transformer struct{}

// Name returns the dialect name.
func (t *{{ .TypeName }}Transformer) Name() string { return "{{ .DialectName }}" }

// EstablishmentToState converts a PFCP Session Establishment Request to a new PFCPSessionState.
func (t *{{ .TypeName }}Transformer) EstablishmentToState(req *pfcp.PFCPEstablishmentRequest) (*pfcp.PFCPSessionState, error) {
	state := &pfcp.PFCPSessionState{
		PDRs:         make(map[uint16]*pfcp.PDR),
		FARs:         make(map[uint32]*pfcp.FAR),
		QERs:         make(map[uint32]*pfcp.QER),
		LastModified: time.Now(),
	}
{{ range .EstablishmentRules }}
	{{ .Comment }}
	{{ .Code }}
{{ end }}
	return state, nil
}

// ModificationToState converts a PFCP Session Modification Request to a delta.
func (t *{{ .TypeName }}Transformer) ModificationToState(req *pfcp.PFCPModificationRequest) (*pfcp.PFCPSessionStateDelta, error) {
	delta := &pfcp.PFCPSessionStateDelta{
		UpdatePDRs: make(map[uint16]*pfcp.PDR),
		UpdateFARs: make(map[uint32]*pfcp.FAR),
	}
{{ range .ModificationRules }}
	{{ .Comment }}
	{{ .Code }}
{{ end }}
	return delta, nil
}

// StateToSessionInfo converts a PFCPSessionState to the unified SessionInformation.
func (t *{{ .TypeName }}Transformer) StateToSessionInfo(state *pfcp.PFCPSessionState) (*ir.SessionInformation, error) {
	info := &ir.SessionInformation{
		Source: ir.Mode1PFCP,
	}
{{ range .SessionInfoRules }}
	{{ .Comment }}
	{{ .Code }}
{{ end }}
	return info, nil
}
`))
