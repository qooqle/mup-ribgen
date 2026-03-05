package dsl

import (
	"fmt"
	"os"
)

func readFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// CompileFile parses filename and generates a Go DialectTransformer implementation
// targeting goPackage. It is a convenience wrapper for ParseFile + CompileAST.
func CompileFile(filename, goPackage string) (*GeneratedCode, error) {
	ast, err := ParseFile(filename)
	if err != nil {
		return nil, fmt.Errorf("dsl compile %q: %w", filename, err)
	}
	return CompileAST(ast, goPackage)
}
