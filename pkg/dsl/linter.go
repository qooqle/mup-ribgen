package dsl

import "strings"

// Severity classifies a lint issue.
type Severity int

const (
	SeverityError   Severity = iota // must be fixed
	SeverityWarning                 // best-practice violation
	SeverityInfo                    // informational
)

func (s Severity) String() string {
	return [...]string{"error", "warning", "info"}[s]
}

// LintIssue is a single linter finding.
type LintIssue struct {
	Severity   Severity
	Message    string
	Location   SourceLocation
	Suggestion string
}

// LintReport is the complete output of a Lint run.
type LintReport struct {
	Issues []LintIssue
}

// HasErrors reports whether any issues with SeverityError exist.
func (r *LintReport) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// knownTransforms lists built-in transform functions.
var knownTransforms = map[string]bool{
	"network_to_host_u32": true,
	"to_cidr_prefix":      true,
	"identity":            true,
}

// requiredMappings are the three mapping names that a complete dialect DSL
// must define (best-practice rule).
var requiredMappings = []string{
	"pfcp_establishment_to_state",
	"pfcp_modification_to_delta",
	"state_to_session_info",
}

// Lint validates a parsed DSLAst and returns a LintReport (req 6.5–6.7).
func Lint(ast *DSLAst) *LintReport {
	r := &LintReport{}

	// Error: dialect name is required
	if strings.TrimSpace(ast.DialectName) == "" {
		r.Issues = append(r.Issues, LintIssue{
			Severity:   SeverityError,
			Message:    "dialect name is missing",
			Suggestion: `add 'dialect "MyDialect"' at the top of the file`,
		})
	}

	// Warning: version declaration is recommended
	if ast.Version == "" {
		r.Issues = append(r.Issues, LintIssue{
			Severity:   SeverityWarning,
			Message:    "version declaration is missing",
			Suggestion: `add 'version "1.0"' after the dialect declaration`,
		})
	}

	// Warning: check for the three required mapping blocks
	presentMappings := map[string]bool{}
	for _, mb := range ast.Mappings {
		presentMappings[mb.Name] = true
	}
	for _, name := range requiredMappings {
		if !presentMappings[name] {
			r.Issues = append(r.Issues, LintIssue{
				Severity:   SeverityWarning,
				Message:    "recommended mapping '" + name + "' is not defined",
				Suggestion: "add a 'mapping " + name + " { ... }' block",
			})
		}
	}

	// Per-mapping checks
	for _, mb := range ast.Mappings {
		// Error: mapping must have at least one rule
		if len(mb.Rules) == 0 {
			r.Issues = append(r.Issues, LintIssue{
				Severity:   SeverityError,
				Message:    "mapping '" + mb.Name + "' has no rules",
				Location:   mb.Location,
				Suggestion: "add at least one field mapping rule",
			})
		}

		for _, rule := range mb.Rules {
			// Error: source path must be non-empty
			if len(rule.Source.Segments) == 0 {
				r.Issues = append(r.Issues, LintIssue{
					Severity: SeverityError,
					Message:  "mapping '" + mb.Name + "': rule has empty source path",
					Location: rule.Location,
				})
			}
			// Error: dest path must be non-empty
			if len(rule.Dest.Segments) == 0 {
				r.Issues = append(r.Issues, LintIssue{
					Severity: SeverityError,
					Message:  "mapping '" + mb.Name + "': rule has empty destination path",
					Location: rule.Location,
				})
			}
			// Warning: unknown transform function
			if rule.Transform != nil && !knownTransforms[rule.Transform.Func] {
				r.Issues = append(r.Issues, LintIssue{
					Severity:   SeverityWarning,
					Message:    "unknown transform function '" + rule.Transform.Func + "'",
					Location:   rule.Transform.Location,
					Suggestion: "use one of: network_to_host_u32, to_cidr_prefix, identity",
				})
			}
		}
	}

	return r
}

// LintFile parses filename and lints it.
func LintFile(filename string) (*LintReport, error) {
	ast, err := ParseFile(filename)
	if err != nil {
		return nil, err
	}
	return Lint(ast), nil
}
