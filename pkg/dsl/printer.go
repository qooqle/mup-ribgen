package dsl

import (
	"os"
	"strings"
)

// Print returns the canonical formatted text of the DSL AST (req 6.3).
// Indentation is 4 spaces. Comments are preserved.
func Print(ast *DSLAst) string {
	var sb strings.Builder
	sb.WriteString("dialect " + quote(ast.DialectName) + "\n")
	if ast.Version != "" {
		sb.WriteString("version " + quote(ast.Version) + "\n")
	}
	for _, mb := range ast.Mappings {
		sb.WriteString("\n")
		sb.WriteString("mapping " + mb.Name + " {\n")
		for _, rule := range mb.Rules {
			if rule.Comment != "" {
				sb.WriteString("    " + rule.Comment + "\n")
			}
			sb.WriteString("    " + rule.Source.String() + " -> " + rule.Dest.String() + "\n")
			if rule.Transform != nil {
				sb.WriteString("        transform: " + rule.Transform.Func)
				if len(rule.Transform.Args) > 0 {
					sb.WriteString("(" + strings.Join(rule.Transform.Args, ", ") + ")")
				}
				sb.WriteString("\n")
			}
			if rule.Condition != nil {
				sb.WriteString("        when: " + rule.Condition.Path.String())
				if rule.Condition.NotExist {
					sb.WriteString(" not exists")
				} else {
					sb.WriteString(" exists")
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n")
	}
	return sb.String()
}

// PrintToFile writes the formatted DSL to the given file path.
func PrintToFile(ast *DSLAst, filename string) error {
	return os.WriteFile(filename, []byte(Print(ast)), 0o644)
}

func quote(s string) string { return `"` + s + `"` }
