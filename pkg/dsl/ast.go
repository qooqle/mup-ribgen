package dsl

import "fmt"

// DSLAst is the root of a parsed DSL file.
type DSLAst struct {
	DialectName string
	Version     string
	Mappings    []*MappingBlock
	Comments    []string // top-level comments
	SourceInfo  SourceInfo
}

// MappingBlock is a named group of field mapping rules.
type MappingBlock struct {
	Name     string
	Rules    []*MappingRule
	Location SourceLocation
}

// MappingRule maps a source FieldPath to a destination FieldPath with optional
// transform and condition modifiers.
type MappingRule struct {
	Source    FieldPath
	Dest      FieldPath
	Transform *TransformExpr // optional: transform: func(args)
	Condition *ConditionExpr // optional: when: path exists / not exists
	Comment   string         // optional inline comment preceding the rule
	Location  SourceLocation
}

// FieldPath is a dotted, optionally-indexed path expression like
// PFCP.CreatePDR[*].TEID.
type FieldPath struct {
	Segments []PathSegment
}

func (fp FieldPath) String() string {
	var s string
	for i, seg := range fp.Segments {
		if i > 0 {
			s += "."
		}
		s += seg.String()
	}
	return s
}

// PathSegment is one element of a FieldPath, optionally with an index.
type PathSegment struct {
	Name  string
	Index *PathIndex // nil if no bracket
}

func (ps PathSegment) String() string {
	if ps.Index == nil {
		return ps.Name
	}
	return ps.Name + "[" + ps.Index.String() + "]"
}

// PathIndex represents the bracket part of a segment like [*], [key], [0], [].
type PathIndex struct {
	Kind    IndexKind
	Numeric int // used when Kind == IndexNumeric
}

func (pi PathIndex) String() string {
	switch pi.Kind {
	case IndexWildcard:
		return "*"
	case IndexKey:
		return "key"
	case IndexNumeric:
		return fmt.Sprintf("%d", pi.Numeric)
	case IndexEmpty:
		return ""
	}
	return ""
}

// IndexKind enumerates the four kinds of array index.
type IndexKind int

const (
	IndexWildcard IndexKind = iota // [*]
	IndexKey                       // [key]
	IndexNumeric                   // [0], [1], …
	IndexEmpty                     // []
)

// TransformExpr represents a "transform: funcName(args)" modifier.
type TransformExpr struct {
	Func     string
	Args     []string
	Location SourceLocation
}

// ConditionExpr represents a "when: path exists" or "when: path not exists".
type ConditionExpr struct {
	Path     FieldPath
	NotExist bool // true → "not exists"
	Location SourceLocation
}

// SourceInfo holds file-level metadata.
type SourceInfo struct {
	Filename string
}

// SourceLocation pinpoints a token in the source.
type SourceLocation struct {
	Line   int
	Column int
}
