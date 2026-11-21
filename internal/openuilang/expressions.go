package openuilang

import "strings"

const (
	precTernary = iota + 1
	precOr
	precAnd
	precEq
	precCmp
	precAdd
	precMul
	precUnary
	precMember
)

var infix = map[tokenKind]struct {
	prec int
	op   string
}{
	tQuestion: {precTernary, "?"}, tOr: {precOr, "||"}, tAnd: {precAnd, "&&"},
	tEqEq: {precEq, "=="}, tNotEq: {precEq, "!="},
	tGreater: {precCmp, ">"}, tLess: {precCmp, "<"}, tGreaterEq: {precCmp, ">="}, tLessEq: {precCmp, "<="},
	tPlus: {precAdd, "+"}, tMinus: {precAdd, "-"},
	tStar: {precMul, "*"}, tSlash: {precMul, "/"}, tPercent: {precMul, "%"},
	tDot: {precMember, "."}, tLBrack: {precMember, "["},
}

const maxExpressionDepth = 200

type expressionParser struct {
	tokens []token
	pos    int
	depth  int
}

func parseExpression(tokens []token) *node {
	p := &expressionParser{tokens: tokens}
	return p.expr(0)
}

func (p *expressionParser) cur() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{t: tEOF}
}

func (p *expressionParser) next() tokenKind {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1].t
	}
	return tEOF
}

func (p *expressionParser) eat(kind tokenKind) {
	if p.cur().t == kind {
		p.pos++
	}
}

func (p *expressionParser) expr(minPrec int) *node {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxExpressionDepth {
		p.pos = len(p.tokens)
		return &node{k: kNull}
	}
	left := p.prefix()
	for infix[p.cur().t].prec > minPrec {
		left = p.infix(left)
	}
	return left
}

func (p *expressionParser) key() string {
	tok := p.cur()
	p.pos++
	switch tok.t {
	case tIdent, tType, tStr:
		return tok.s
	case tNum:
		return numberString(tok.n)
	case tStateVar:
		return strings.TrimPrefix(tok.s, "$")
	}
	return "?"
}

func (p *expressionParser) prefix() *node {
	tok := p.cur()
	switch tok.t {
	case tStr:
		p.pos++
		return &node{k: kStr, str: tok.s}
	case tNum:
		p.pos++
		return &node{k: kNum, num: tok.n}
	case tTrue, tFalse:
		p.pos++
		return &node{k: kBool, boolean: tok.t == tTrue}
	case tNull:
		p.pos++
		return &node{k: kNull}
	case tLBrack:
		p.pos++
		return &node{k: kArr, args: p.list(tRBrack)}
	case tLBrace:
		p.pos++
		out := &node{k: kObj}
		for p.cur().t != tRBrace && p.cur().t != tEOF {
			key := p.key()
			p.eat(tColon)
			out.entries = append(out.entries, objEntry{key, p.expr(0)})
			p.eat(tComma)
		}
		p.eat(tRBrace)
		return out
	case tStateVar:
		p.pos++
		if p.cur().t == tEquals {
			p.pos++
			return &node{k: kAssign, name: tok.s, a: p.expr(0)}
		}
		return &node{k: kStateRef, name: tok.s}
	case tType:
		if p.next() == tLParen && (!isBuiltin(tok.s) || tok.s == "Action") {
			return p.call(false)
		}
		p.pos++
		return &node{k: kRef, name: tok.s}
	case tBuiltinCall:
		if p.next() == tLParen {
			return p.call(!isBuiltin(tok.s))
		}
		p.pos++
		return &node{k: kRef, name: tok.s}
	case tIdent:
		p.pos++
		return &node{k: kRef, name: tok.s}
	case tNot, tMinus:
		p.pos++
		op := "!"
		if tok.t == tMinus {
			op = "-"
		}
		return &node{k: kUnaryOp, name: op, a: p.expr(precUnary)}
	case tLParen:
		p.pos++
		inner := p.expr(0)
		p.eat(tRParen)
		return inner
	}
	p.pos++
	return &node{k: kNull}
}

func (p *expressionParser) list(end tokenKind) []*node {
	out := []*node{}
	for p.cur().t != end && p.cur().t != tEOF {
		out = append(out, p.expr(0))
		p.eat(tComma)
	}
	p.eat(end)
	return out
}

func (p *expressionParser) call(fn bool) *node {
	name := p.cur().s
	p.pos += 2
	return &node{k: kComp, name: name, args: p.list(tRParen), fn: fn}
}

func (p *expressionParser) infix(left *node) *node {
	rule := infix[p.cur().t]
	p.pos++
	switch rule.op {
	case "?":
		then := p.expr(0)
		p.eat(tColon)
		return &node{k: kTernary, a: left, b: then, c: p.expr(0)}
	case ".":
		return &node{k: kMember, a: left, name: p.key()}
	case "[":
		index := p.expr(0)
		p.eat(tRBrack)
		return &node{k: kIndex, a: left, b: index}
	}
	return &node{k: kBinOp, name: rule.op, a: left, b: p.expr(rule.prec)}
}
