package dsl

// TokenType identifies the kind of a lexical token.
type TokenType int

const (
	ILLEGAL TokenType = iota
	EOF
	COMMENT // // …

	// Literals
	IDENT  // identifier
	STRING // "…"
	NUMBER // integer

	// Keywords
	KW_DIALECT   // dialect
	KW_VERSION   // version
	KW_MAPPING   // mapping
	KW_TRANSFORM // transform
	KW_WHEN      // when
	KW_EXISTS    // exists
	KW_NOT       // not

	// Operators / punctuation
	ARROW   // ->
	DOT     // .
	LBRACK  // [
	RBRACK  // ]
	LBRACE  // {
	RBRACE  // }
	STAR    // *
	COLON   // :
	LPAREN  // (
	RPAREN  // )
	COMMA   // ,
)

var keywords = map[string]TokenType{
	"dialect":   KW_DIALECT,
	"version":   KW_VERSION,
	"mapping":   KW_MAPPING,
	"transform": KW_TRANSFORM,
	"when":      KW_WHEN,
	"exists":    KW_EXISTS,
	"not":       KW_NOT,
}

// Token is a single lexical unit.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}
