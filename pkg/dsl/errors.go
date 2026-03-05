package dsl

import "fmt"

// ParseError is a syntax error produced by the Parser.
type ParseError struct {
	Message  string
	Line     int
	Column   int
	Filename string
}

func (e *ParseError) Error() string {
	if e.Filename != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.Filename, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("line %d, col %d: %s", e.Line, e.Column, e.Message)
}

// ParseErrors is a slice of ParseError collected during error-recovery parsing.
type ParseErrors []*ParseError

func (pe ParseErrors) Error() string {
	if len(pe) == 0 {
		return "no errors"
	}
	s := fmt.Sprintf("%d parse error(s):\n", len(pe))
	for _, e := range pe {
		s += "  " + e.Error() + "\n"
	}
	return s
}
