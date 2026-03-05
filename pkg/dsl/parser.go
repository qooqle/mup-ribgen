package dsl

import (
	"strconv"
)

// Parser builds a DSLAst from tokens produced by the Lexer.
// On syntax errors it records the error and attempts to recover so that
// multiple errors can be reported in one pass (req 6.2).
type Parser struct {
	tokens  []Token
	pos     int
	errors  ParseErrors
	source  string
}

// Parse parses the DSL source string and returns the AST.
// If syntax errors are found, a ParseErrors slice is returned as the error.
func Parse(input, source string) (*DSLAst, error) {
	l := NewLexer(input, source)
	toks := l.Tokens()

	p := &Parser{tokens: toks, source: source}
	ast := p.parseProgram()
	ast.SourceInfo = SourceInfo{Filename: source}

	if len(p.errors) > 0 {
		return ast, p.errors
	}
	return ast, nil
}

// ParseFile reads a DSL file and returns its AST.
func ParseFile(filename string) (*DSLAst, error) {
	data, err := readFile(filename)
	if err != nil {
		return nil, &ParseError{Message: err.Error(), Filename: filename}
	}
	return Parse(string(data), filename)
}

// --- internal: token stream helpers -----------------------------------------

func (p *Parser) cur() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: EOF}
}

func (p *Parser) peek() Token {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1]
	}
	return Token{Type: EOF}
}

func (p *Parser) advance() Token {
	tok := p.cur()
	p.pos++
	return tok
}

func (p *Parser) skipComments() []string {
	var comments []string
	for p.cur().Type == COMMENT {
		comments = append(comments, p.advance().Literal)
	}
	return comments
}

func (p *Parser) expect(tt TokenType) (Token, bool) {
	if p.cur().Type == tt {
		return p.advance(), true
	}
	tok := p.cur()
	p.errors = append(p.errors, &ParseError{
		Message:  "expected " + tokenTypeName(tt) + ", got " + repr(tok),
		Line:     tok.Line,
		Column:   tok.Column,
		Filename: p.source,
	})
	return tok, false
}

func (p *Parser) errorf(tok Token, msg string) {
	p.errors = append(p.errors, &ParseError{
		Message:  msg,
		Line:     tok.Line,
		Column:   tok.Column,
		Filename: p.source,
	})
}

// skipTo advances until one of the stop token types is found (for recovery).
func (p *Parser) skipTo(stopTypes ...TokenType) {
	for p.cur().Type != EOF {
		for _, st := range stopTypes {
			if p.cur().Type == st {
				return
			}
		}
		p.advance()
	}
}

// --- grammar ----------------------------------------------------------------

func (p *Parser) parseProgram() *DSLAst {
	ast := &DSLAst{}
	for p.cur().Type != EOF {
		p.skipComments()
		switch p.cur().Type {
		case KW_DIALECT:
			p.advance()
			if tok, ok := p.expect(STRING); ok {
				ast.DialectName = tok.Literal
			}
		case KW_VERSION:
			p.advance()
			if tok, ok := p.expect(STRING); ok {
				ast.Version = tok.Literal
			}
		case KW_MAPPING:
			if mb := p.parseMappingBlock(); mb != nil {
				ast.Mappings = append(ast.Mappings, mb)
			}
		case EOF:
			return ast
		default:
			// skip unexpected tokens (error-recovery)
			p.errorf(p.cur(), "unexpected token: "+repr(p.cur()))
			p.advance()
		}
	}
	return ast
}

func (p *Parser) parseMappingBlock() *MappingBlock {
	startTok := p.advance() // consume 'mapping'
	nameTok, ok := p.expect(IDENT)
	if !ok {
		p.skipTo(LBRACE, KW_MAPPING, EOF)
	}
	if _, ok := p.expect(LBRACE); !ok {
		p.skipTo(RBRACE, KW_MAPPING, EOF)
		return nil
	}
	mb := &MappingBlock{Name: nameTok.Literal, Location: SourceLocation{Line: startTok.Line, Column: startTok.Column}}
	for p.cur().Type != RBRACE && p.cur().Type != EOF {
		comments := p.skipComments()
		if p.cur().Type == RBRACE || p.cur().Type == EOF {
			break
		}
		if rule := p.parseRule(comments); rule != nil {
			mb.Rules = append(mb.Rules, rule)
		}
	}
	p.expect(RBRACE)
	return mb
}

func (p *Parser) parseRule(comments []string) *MappingRule {
	comment := ""
	if len(comments) > 0 {
		comment = comments[len(comments)-1]
	}
	src, ok := p.parseFieldPath()
	if !ok {
		p.skipTo(RBRACE, KW_MAPPING, EOF)
		return nil
	}
	if _, ok := p.expect(ARROW); !ok {
		p.skipTo(RBRACE, KW_MAPPING, EOF)
		return nil
	}
	dst, ok := p.parseFieldPath()
	if !ok {
		p.skipTo(RBRACE, KW_MAPPING, EOF)
		return nil
	}
	rule := &MappingRule{
		Source:   src,
		Dest:     dst,
		Comment:  comment,
		Location: SourceLocation{Line: p.cur().Line},
	}
	// optional modifiers: transform: / when:
	for p.cur().Type == KW_TRANSFORM || p.cur().Type == KW_WHEN {
		switch p.cur().Type {
		case KW_TRANSFORM:
			rule.Transform = p.parseTransform()
		case KW_WHEN:
			rule.Condition = p.parseCondition()
		}
	}
	return rule
}

func (p *Parser) parseFieldPath() (FieldPath, bool) {
	var fp FieldPath
	seg, ok := p.parseSegment()
	if !ok {
		return fp, false
	}
	fp.Segments = append(fp.Segments, seg)
	for p.cur().Type == DOT {
		p.advance() // consume '.'
		seg, ok = p.parseSegment()
		if !ok {
			return fp, false
		}
		fp.Segments = append(fp.Segments, seg)
	}
	return fp, true
}

func (p *Parser) parseSegment() (PathSegment, bool) {
	tok := p.cur()
	if tok.Type != IDENT && tok.Type != KW_NOT {
		// Allow keywords that can appear as field names (e.g. nothing currently,
		// but future-proof)
		p.errorf(tok, "expected field name, got "+repr(tok))
		return PathSegment{}, false
	}
	p.advance()
	seg := PathSegment{Name: tok.Literal}
	if p.cur().Type == LBRACK {
		idx, ok := p.parseIndex()
		if !ok {
			return seg, false
		}
		seg.Index = &idx
	}
	return seg, true
}

func (p *Parser) parseIndex() (PathIndex, bool) {
	p.advance() // consume '['
	tok := p.cur()
	var idx PathIndex
	switch {
	case tok.Type == STAR:
		idx.Kind = IndexWildcard
		p.advance()
	case tok.Type == IDENT && tok.Literal == "key":
		idx.Kind = IndexKey
		p.advance()
	case tok.Type == NUMBER:
		n, err := strconv.Atoi(tok.Literal)
		if err != nil {
			p.errorf(tok, "invalid index: "+tok.Literal)
			return idx, false
		}
		idx.Kind = IndexNumeric
		idx.Numeric = n
		p.advance()
	case tok.Type == RBRACK:
		idx.Kind = IndexEmpty
		// don't advance – let the expect below consume it
	default:
		p.errorf(tok, "expected index (*, key, number, or empty), got "+repr(tok))
		p.skipTo(RBRACK, RBRACE, EOF)
	}
	if _, ok := p.expect(RBRACK); !ok {
		return idx, false
	}
	return idx, true
}

func (p *Parser) parseTransform() *TransformExpr {
	loc := SourceLocation{Line: p.cur().Line, Column: p.cur().Column}
	p.advance() // consume 'transform'
	p.expect(COLON)
	funcTok, ok := p.expect(IDENT)
	if !ok {
		return nil
	}
	tx := &TransformExpr{Func: funcTok.Literal, Location: loc}
	if p.cur().Type == LPAREN {
		p.advance() // consume '('
		for p.cur().Type != RPAREN && p.cur().Type != EOF {
			if p.cur().Type == COMMA {
				p.advance()
				continue
			}
			tx.Args = append(tx.Args, p.cur().Literal)
			p.advance()
		}
		p.expect(RPAREN)
	}
	return tx
}

func (p *Parser) parseCondition() *ConditionExpr {
	loc := SourceLocation{Line: p.cur().Line, Column: p.cur().Column}
	p.advance() // consume 'when'
	p.expect(COLON)
	path, ok := p.parseFieldPath()
	if !ok {
		return nil
	}
	cond := &ConditionExpr{Path: path, Location: loc}
	if p.cur().Type == KW_NOT {
		p.advance()
		cond.NotExist = true
	}
	p.expect(KW_EXISTS)
	return cond
}

// --- helpers ----------------------------------------------------------------

func tokenTypeName(tt TokenType) string {
	m := map[TokenType]string{
		IDENT: "identifier", STRING: "string", NUMBER: "number",
		ARROW: "'->'", DOT: "'.'", LBRACK: "'['", RBRACK: "']'",
		LBRACE: "'{'", RBRACE: "'}'", COLON: "':'", LPAREN: "'('", RPAREN: "')'",
		KW_MAPPING: "mapping", KW_EXISTS: "exists", EOF: "EOF",
	}
	if s, ok := m[tt]; ok {
		return s
	}
	return "token"
}

func repr(tok Token) string {
	if tok.Literal != "" {
		return "'" + tok.Literal + "'"
	}
	return tokenTypeName(tok.Type)
}
