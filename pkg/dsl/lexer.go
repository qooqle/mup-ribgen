package dsl

import (
	"strings"
	"unicode"
)

// Lexer tokenises a DSL source string.
type Lexer struct {
	input  []rune
	pos    int
	line   int
	col    int
	source string // filename for error messages
}

// NewLexer creates a Lexer for the given source string.
func NewLexer(input, source string) *Lexer {
	return &Lexer{input: []rune(input), pos: 0, line: 1, col: 1, source: source}
}

// Tokens scans all tokens until EOF and returns them.
func (l *Lexer) Tokens() []Token {
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}
	return tokens
}

// Next returns the next token, skipping whitespace.
// Comments are returned as COMMENT tokens (not skipped) so the pretty-printer
// can preserve them.
func (l *Lexer) Next() Token {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return Token{Type: EOF, Literal: "", Line: l.line, Column: l.col}
	}
	ch := l.input[l.pos]

	// comment
	if ch == '/' && l.peek(1) == '/' {
		return l.readComment()
	}
	// string literal
	if ch == '"' {
		return l.readString()
	}
	// number
	if ch >= '0' && ch <= '9' {
		return l.readNumber()
	}
	// identifier / keyword
	if ch == '_' || unicode.IsLetter(ch) {
		return l.readIdent()
	}
	// operators
	switch ch {
	case '-':
		if l.peek(1) == '>' {
			return l.consume2(ARROW, "->")
		}
		return l.consume1(ILLEGAL, string(ch))
	case '.':
		return l.consume1(DOT, ".")
	case '[':
		return l.consume1(LBRACK, "[")
	case ']':
		return l.consume1(RBRACK, "]")
	case '{':
		return l.consume1(LBRACE, "{")
	case '}':
		return l.consume1(RBRACE, "}")
	case '*':
		return l.consume1(STAR, "*")
	case ':':
		return l.consume1(COLON, ":")
	case '(':
		return l.consume1(LPAREN, "(")
	case ')':
		return l.consume1(RPAREN, ")")
	case ',':
		return l.consume1(COMMA, ",")
	}
	return l.consume1(ILLEGAL, string(ch))
}

// --- internal helpers -------------------------------------------------------

func (l *Lexer) peek(offset int) rune {
	i := l.pos + offset
	if i >= len(l.input) {
		return 0
	}
	return l.input[i]
}

func (l *Lexer) advance() {
	if l.pos < len(l.input) {
		if l.input[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *Lexer) consume1(tt TokenType, lit string) Token {
	tok := Token{Type: tt, Literal: lit, Line: l.line, Column: l.col}
	l.advance()
	return tok
}

func (l *Lexer) consume2(tt TokenType, lit string) Token {
	tok := Token{Type: tt, Literal: lit, Line: l.line, Column: l.col}
	l.advance()
	l.advance()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.advance()
	}
}

func (l *Lexer) readComment() Token {
	line, col := l.line, l.col
	var sb strings.Builder
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		sb.WriteRune(l.input[l.pos])
		l.advance()
	}
	return Token{Type: COMMENT, Literal: sb.String(), Line: line, Column: col}
}

func (l *Lexer) readString() Token {
	line, col := l.line, l.col
	l.advance() // consume opening "
	var sb strings.Builder
	for l.pos < len(l.input) && l.input[l.pos] != '"' {
		if l.input[l.pos] == '\n' {
			break // unterminated string
		}
		sb.WriteRune(l.input[l.pos])
		l.advance()
	}
	if l.pos < len(l.input) {
		l.advance() // consume closing "
	}
	return Token{Type: STRING, Literal: sb.String(), Line: line, Column: col}
}

func (l *Lexer) readNumber() Token {
	line, col := l.line, l.col
	var sb strings.Builder
	for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
		sb.WriteRune(l.input[l.pos])
		l.advance()
	}
	return Token{Type: NUMBER, Literal: sb.String(), Line: line, Column: col}
}

func (l *Lexer) readIdent() Token {
	line, col := l.line, l.col
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '_' || unicode.IsLetter(ch) || (ch >= '0' && ch <= '9') {
			sb.WriteRune(ch)
			l.advance()
		} else {
			break
		}
	}
	lit := sb.String()
	tt, isKW := keywords[lit]
	if !isKW {
		tt = IDENT
	}
	return Token{Type: tt, Literal: lit, Line: line, Column: col}
}
