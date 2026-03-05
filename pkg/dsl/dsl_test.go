package dsl_test

// Property 12: DSL parsing (req 6.1)
// Property 13: DSL parse error detection (req 6.2)
// Property 15: DSL round-trip (req 6.4)
// Property 16: DSL Linter syntax check (req 6.5, 6.7)
// Property 17: DSL Linter best-practice check (req 6.6, 6.7)
// Property 11: DSL compilation (req 5.3, 5.4, 5.5)

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/qooqle/mup-ribgen/pkg/dsl"
)

// minimalValidDSL returns the smallest valid DSL text for the given dialect name.
func minimalValidDSL(dialectName string) string {
	return `dialect "` + dialectName + `"
version "1.0"

mapping pfcp_establishment_to_state {
    PFCP.CPFSEID.SEID -> State.SEID
}

mapping pfcp_modification_to_delta {
    PFCP.CPFSEID.SEID -> Delta.SEID
}

mapping state_to_session_info {
    State.SEID -> SessionInfo.SEID
}
`
}

// --- Property 12: DSL Parsing -----------------------------------------------

// Property 12: For any valid DSL source, Parse must succeed and return a
// non-nil DSLAst with the correct DialectName and Version (req 6.1).
func TestProperty12_DSLParsing(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req6.1: valid DSL parses to non-nil AST with correct DialectName", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			src := minimalValidDSL(name)
			ast, err := dsl.Parse(src, "<test>")
			return err == nil && ast != nil && ast.DialectName == name
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
	))

	properties.Property("req6.1: valid DSL preserves Version", prop.ForAll(
		func(version string) bool {
			if version == "" {
				return true
			}
			src := `dialect "Test"
version "` + version + `"
mapping pfcp_establishment_to_state {
    A.B -> C.D
}
`
			ast, err := dsl.Parse(src, "<test>")
			return err == nil && ast != nil && ast.Version == version
		},
		gen.RegexMatch(`[0-9]+\.[0-9]+`),
	))

	properties.Property("req6.1: mapping count is preserved", prop.ForAll(
		func(count int) bool {
			var sb strings.Builder
			sb.WriteString(`dialect "Test"` + "\n")
			for i := 0; i < count; i++ {
				sb.WriteString("mapping mapping_" + string(rune('a'+i)) + " {\n    A.B -> C.D\n}\n")
			}
			ast, err := dsl.Parse(sb.String(), "<test>")
			return err == nil && ast != nil && len(ast.Mappings) == count
		},
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 13: DSL Parse Error Detection ---------------------------------

// Property 13: For any DSL source with a known syntax error, Parse must return
// a non-nil error with line information (req 6.2).
func TestProperty13_DSLParseErrorDetection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Missing closing brace
	properties.Property("req6.2: missing '}' is detected", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			src := `dialect "` + name + `"
mapping foo {
    A.B -> C.D
` // no closing }
			_, err := dsl.Parse(src, "<test>")
			return err != nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
	))

	// Missing arrow
	properties.Property("req6.2: missing '->' is detected", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			src := `dialect "` + name + `"
mapping foo {
    A.B C.D
}
`
			_, err := dsl.Parse(src, "<test>")
			return err != nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
	))

	// Error includes line information
	properties.Property("req6.2: error messages include position info", prop.ForAll(
		func(_ uint8) bool {
			src := `dialect "Test"
mapping foo {
    -> C.D
}
`
			_, err := dsl.Parse(src, "myfile.dsl")
			if err == nil {
				return false
			}
			// error string should contain line number or filename
			s := err.Error()
			return strings.Contains(s, "myfile.dsl") || strings.Contains(s, "line") || strings.Contains(s, "3")
		},
		gen.UInt8(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 15: DSL Round-trip --------------------------------------------

// Property 15: For any valid DSL, Parse → Print → Parse must yield an
// equivalent AST (req 6.4).
func TestProperty15_DSLRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req6.4: parse→print→parse yields equivalent AST", prop.ForAll(
		func(dialectName, mappingName string) bool {
			if dialectName == "" || mappingName == "" {
				return true
			}
			src := `dialect "` + dialectName + `"
version "1.0"

mapping ` + mappingName + ` {
    A.B -> C.D
}
`
			ast1, err := dsl.Parse(src, "<test>")
			if err != nil {
				return false
			}
			printed := dsl.Print(ast1)
			ast2, err := dsl.Parse(printed, "<printed>")
			if err != nil {
				return false
			}
			return ast1.DialectName == ast2.DialectName &&
				ast1.Version == ast2.Version &&
				len(ast1.Mappings) == len(ast2.Mappings)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
		gen.RegexMatch(`[a-z][a-z_]{1,10}`),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 16: DSL Linter Syntax Check -----------------------------------

// Property 16: Lint must report an error when the dialect name is missing (req 6.5, 6.7).
func TestProperty16_DSLLinterSyntaxCheck(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req6.5,6.7: empty dialect name is a lint error", prop.ForAll(
		func(_ uint8) bool {
			// Build an AST with no dialect name
			ast := &dsl.DSLAst{
				DialectName: "",
				Version:     "1.0",
				Mappings:    []*dsl.MappingBlock{{Name: "pfcp_establishment_to_state", Rules: []*dsl.MappingRule{{}}}},
			}
			report := dsl.Lint(ast)
			return report.HasErrors()
		},
		gen.UInt8(),
	))

	properties.Property("req6.5,6.7: empty mapping block is a lint error", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			ast := &dsl.DSLAst{
				DialectName: name,
				Version:     "1.0",
				Mappings: []*dsl.MappingBlock{
					{Name: "pfcp_establishment_to_state", Rules: nil}, // empty
				},
			}
			report := dsl.Lint(ast)
			return report.HasErrors()
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 17: DSL Linter Best-Practice Check ----------------------------

// Property 17: Lint must warn when recommended mappings are absent (req 6.6, 6.7).
func TestProperty17_DSLLinterBestPracticeCheck(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Use a minimal but valid rule so empty-path lint errors don't skew counts.
	validRule := &dsl.MappingRule{
		Source: dsl.FieldPath{Segments: []dsl.PathSegment{{Name: "A"}}},
		Dest:   dsl.FieldPath{Segments: []dsl.PathSegment{{Name: "B"}}},
	}
	properties.Property("req6.6,6.7: missing required mappings generates more warnings than full", prop.ForAll(
		func(name string) bool {
			if name == "" {
				return true
			}
			// Full AST: all three required mappings present.
			fullAST := &dsl.DSLAst{
				DialectName: name, Version: "1.0",
				Mappings: []*dsl.MappingBlock{
					{Name: "pfcp_establishment_to_state", Rules: []*dsl.MappingRule{validRule}},
					{Name: "pfcp_modification_to_delta", Rules: []*dsl.MappingRule{validRule}},
					{Name: "state_to_session_info", Rules: []*dsl.MappingRule{validRule}},
				},
			}
			fullReport := dsl.Lint(fullAST)

			// Partial AST: only one mapping → 2 extra "missing mapping" warnings.
			partialAST := &dsl.DSLAst{
				DialectName: name, Version: "1.0",
				Mappings: []*dsl.MappingBlock{
					{Name: "pfcp_establishment_to_state", Rules: []*dsl.MappingRule{validRule}},
				},
			}
			partialReport := dsl.Lint(partialAST)
			return len(partialReport.Issues) > len(fullReport.Issues)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
	))

	properties.Property("req6.6,6.7: unknown transform is a warning", prop.ForAll(
		func(funcName string) bool {
			if funcName == "" || funcName == "network_to_host_u32" ||
				funcName == "to_cidr_prefix" || funcName == "identity" {
				return true
			}
			ast := &dsl.DSLAst{
				DialectName: "Test",
				Version:     "1.0",
				Mappings: []*dsl.MappingBlock{
					{Name: "pfcp_establishment_to_state", Rules: []*dsl.MappingRule{
						{
							Source: dsl.FieldPath{Segments: []dsl.PathSegment{{Name: "A"}}},
							Dest:   dsl.FieldPath{Segments: []dsl.PathSegment{{Name: "B"}}},
							Transform: &dsl.TransformExpr{Func: funcName},
						},
					}},
				},
			}
			report := dsl.Lint(ast)
			for _, issue := range report.Issues {
				if issue.Severity == dsl.SeverityWarning &&
					strings.Contains(issue.Message, funcName) {
					return true
				}
			}
			return false
		},
		gen.RegexMatch(`[a-z][a-z_]{2,10}`),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// --- Property 11: DSL Compilation -------------------------------------------

// Property 11: Compiled output must be non-empty Go code containing the
// expected type and function names (req 5.3, 5.4, 5.5).
func TestProperty11_DSLCompilation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("req5.3,5.4: compilation produces Go code with transformer type", prop.ForAll(
		func(dialectName string) bool {
			if dialectName == "" {
				return true
			}
			src := minimalValidDSL(dialectName)
			result, err := dsl.Compile(src, "<test>", "dialect")
			if err != nil {
				return false
			}
			code := result.TransformerImpl
			return strings.Contains(code, "EstablishmentToState") &&
				strings.Contains(code, "ModificationToState") &&
				strings.Contains(code, "StateToSessionInfo") &&
				strings.Contains(code, dialectName)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
	))

	properties.Property("req5.4,5.5: dialect name is embedded in generated code", prop.ForAll(
		func(dialectName string) bool {
			if dialectName == "" {
				return true
			}
			src := minimalValidDSL(dialectName)
			result, err := dsl.Compile(src, "<test>", "dialect")
			if err != nil {
				return false
			}
			return result.DialectName == dialectName &&
				strings.Contains(result.TransformerImpl, dialectName)
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 1 }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
