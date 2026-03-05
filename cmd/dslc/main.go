// dslc – DSL CLI tool (req 5.3, 6.3, 6.5)
//
// Usage:
//
//	dslc compile [-pkg <package>] [-out <file.go>] <file.dsl>  compile DSL to Go
//	dslc lint    <file.dsl>                                     lint DSL file
//	dslc format  [-out <file.dsl>] <file.dsl>                  format DSL file
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/qooqle/mup-ribgen/pkg/dsl"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "compile":
		runCompile(os.Args[2:])
	case "lint":
		runLint(os.Args[2:])
	case "format":
		runFormat(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "dslc: unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `dslc – PFCP Dialect DSL compiler and tools

Usage:
  dslc compile [-pkg <package>] [-out <file.go>] <file.dsl>  compile DSL to Go
  dslc lint    <file.dsl>                                     lint DSL file
  dslc format  [-out <file.dsl>] <file.dsl>                  format DSL file

`)
}

// runCompile compiles a DSL file to a Go Dialect Transformer (req 5.3).
func runCompile(args []string) {
	fs := flag.NewFlagSet("compile", flag.ExitOnError)
	pkg := fs.String("pkg", "dialect", "Go package name for generated code")
	out := fs.String("out", "", "output file (default: stdout)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dslc compile [-pkg <package>] [-out <file.go>] <file.dsl>\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	src := fs.Arg(0)
	code, err := dsl.CompileFile(src, *pkg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dslc compile: %v\n", err)
		os.Exit(1)
	}

	if *out == "" {
		fmt.Print(code.TransformerImpl)
		return
	}
	if err := os.WriteFile(*out, []byte(code.TransformerImpl), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "dslc compile: write %q: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "dslc compile: wrote %s (%d bytes)\n", *out, len(code.TransformerImpl))
}

// runLint lints a DSL file and reports issues (req 6.5).
func runLint(args []string) {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dslc lint <file.dsl>\n")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	src := fs.Arg(0)
	report, err := dsl.LintFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dslc lint: %v\n", err)
		os.Exit(1)
	}

	exitCode := 0
	for _, issue := range report.Issues {
		fmt.Fprintf(os.Stderr, "%s:%d:%d: %s: %s",
			src,
			issue.Location.Line,
			issue.Location.Column,
			issue.Severity.String(),
			issue.Message,
		)
		if issue.Suggestion != "" {
			fmt.Fprintf(os.Stderr, " (suggestion: %s)", issue.Suggestion)
		}
		fmt.Fprintln(os.Stderr)
		if issue.Severity == dsl.SeverityError {
			exitCode = 1
		}
	}
	if len(report.Issues) == 0 {
		fmt.Fprintf(os.Stderr, "dslc lint: %s: OK\n", src)
	}
	os.Exit(exitCode)
}

// runFormat formats a DSL file to its canonical representation (req 6.3).
func runFormat(args []string) {
	fs := flag.NewFlagSet("format", flag.ExitOnError)
	out := fs.String("out", "", "output file (default: stdout)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dslc format [-out <file.dsl>] <file.dsl>\n")
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	src := fs.Arg(0)
	ast, err := dsl.ParseFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dslc format: %v\n", err)
		os.Exit(1)
	}

	formatted := dsl.Print(ast)
	if *out == "" {
		fmt.Print(formatted)
		return
	}
	if err := os.WriteFile(*out, []byte(formatted), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "dslc format: write %q: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "dslc format: wrote %s\n", *out)
}
