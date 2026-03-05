// dslc – DSL compiler CLI (req 5.3, 6.3, 6.5)
// Usage: dslc <file.dsl> [-pkg <package>] [-out <output.go>]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/qooqle/mup-ribgen/pkg/dsl"
)

func main() {
	pkg := flag.String("pkg", "dialect", "Go package name for generated code")
	out := flag.String("out", "", "output file (default: stdout)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dslc [flags] <file.dsl>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	src := flag.Arg(0)
	code, err := dsl.CompileFile(src, *pkg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dslc: %v\n", err)
		os.Exit(1)
	}

	if *out == "" {
		fmt.Print(code.TransformerImpl)
		return
	}
	if err := os.WriteFile(*out, []byte(code.TransformerImpl), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "dslc: write %q: %v\n", *out, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "dslc: wrote %s (%d bytes)\n", *out, len(code.TransformerImpl))
}
