package openuilang

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

type tokenKind int

const (
	tNewline tokenKind = iota
	tLParen
	tRParen
	tLBrack
	tRBrack
	tLBrace
	tRBrace
	tComma
	tColon
	tEquals
	tEqEq
	tNotEq
	tNot
	tGreater
	tGreaterEq
	tLess
	tLessEq
	tAnd
	tOr
	tDot
	tQuestion
	tPlus
	tMinus
	tStar
	tSlash
	tPercent
	tStr
	tNum
	tTrue
	tFalse
	tNull
	tIdent
	tType
	tStateVar
	tBuiltinCall
	tEOF
)

type token struct {
	t     tokenKind
	s     string
	n     float64
	start int
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }
func isWord(c byte) bool  { return isAlpha(c) || isDigit(c) }

var punctuation = map[byte]tokenKind{
	'(': tLParen, ')': tRParen, '[': tLBrack, ']': tRBrack, '{': tLBrace, '}': tRBrace,
	',': tComma, ':': tColon, '.': tDot, '?': tQuestion, '+': tPlus, '*': tStar, '%': tPercent,
}

func afterValue(tokens []token) bool {
	if len(tokens) == 0 {
		return false
	}
	switch tokens[len(tokens)-1].t {
	case tNum, tStr, tIdent, tType, tRParen, tRBrack, tTrue, tFalse, tNull, tStateVar, tBuiltinCall:
		return true
	}
	return false
}

func tokenize(src string) []token {
	var tokens []token
	i, n := 0, len(src)
	at := func(j int) byte {
		if j < n {
			return src[j]
		}
		return 0
	}
	pair := func(single, double tokenKind) {
		if at(i+1) == '=' {
			tokens = append(tokens, token{t: double})
			i += 2
			return
		}
		tokens = append(tokens, token{t: single})
		i++
	}
	for i < n {
		for i < n && (src[i] == ' ' || src[i] == '\t' || src[i] == '\r') {
			i++
		}
		if i >= n {
			break
		}
		c := src[i]
		if c == '\n' {
			tokens = append(tokens, token{t: tNewline, start: i})
			i++
			continue
		}
		if kind, ok := punctuation[c]; ok {
			tokens = append(tokens, token{t: kind})
			i++
			continue
		}
		switch c {
		case '=':
			pair(tEquals, tEqEq)
			continue
		case '!':
			pair(tNot, tNotEq)
			continue
		case '>':
			pair(tGreater, tGreaterEq)
			continue
		case '<':
			pair(tLess, tLessEq)
			continue
		case '&', '|':
			kind := tAnd
			if c == '|' {
				kind = tOr
			}
			tokens = append(tokens, token{t: kind})
			if at(i+1) == c {
				i++
			}
			i++
			continue
		case '#':
			for i < n && src[i] != '\n' {
				i++
			}
			continue
		case '/':
			if at(i+1) == '/' {
				for i < n && src[i] != '\n' {
					i++
				}
				continue
			}
			tokens = append(tokens, token{t: tSlash})
			i++
			continue
		case '"':
			start := i
			i++
			closed := false
			for i < n {
				if src[i] == '\\' {
					i += 2
				} else if src[i] == '"' {
					i++
					closed = true
					break
				} else {
					i++
				}
			}
			raw := src[start:min(i, n)]
			i = min(i, n)
			candidate := raw
			if !closed {
				candidate += `"`
			}
			var value string
			if json.Unmarshal([]byte(candidate), &value) != nil {
				value = strings.TrimSuffix(strings.TrimPrefix(raw, `"`), `"`)
			}
			tokens = append(tokens, token{t: tStr, s: value})
			continue
		case '\'':
			i++
			var b strings.Builder
			for i < n {
				if src[i] == '\\' {
					i++
					if i < n {
						switch src[i] {
						case 'n':
							b.WriteByte('\n')
						case 't':
							b.WriteByte('\t')
						default:
							b.WriteByte(src[i])
						}
						i++
					}
				} else if src[i] == '\'' {
					i++
					break
				} else {
					b.WriteByte(src[i])
					i++
				}
			}
			tokens = append(tokens, token{t: tStr, s: b.String()})
			continue
		}
		if c == '-' && (afterValue(tokens) || !isDigit(at(i+1))) {
			tokens = append(tokens, token{t: tMinus})
			i++
			continue
		}
		if isDigit(c) || c == '-' {
			start := i
			if c == '-' {
				i++
			}
			for i < n && isDigit(src[i]) {
				i++
			}
			if at(i) == '.' && isDigit(at(i+1)) {
				i++
				for i < n && isDigit(src[i]) {
					i++
				}
			}
			if at(i) == 'e' || at(i) == 'E' {
				i++
				if at(i) == '+' || at(i) == '-' {
					i++
				}
				for i < n && isDigit(src[i]) {
					i++
				}
			}
			value, err := strconv.ParseFloat(src[start:i], 64)
			if err != nil && !math.IsInf(value, 0) {
				value = math.NaN()
			}
			tokens = append(tokens, token{t: tNum, n: value})
			continue
		}
		if (c == '$' || c == '@') && isAlpha(at(i+1)) {
			start := i
			i++
			for i < n && isWord(src[i]) {
				i++
			}
			if c == '$' {
				tokens = append(tokens, token{t: tStateVar, s: src[start:i], start: start})
			} else {
				tokens = append(tokens, token{t: tBuiltinCall, s: src[start+1 : i]})
			}
			continue
		}
		if isAlpha(c) {
			start := i
			for i < n && isWord(src[i]) {
				i++
			}
			word := src[start:i]
			switch word {
			case "true":
				tokens = append(tokens, token{t: tTrue})
			case "false":
				tokens = append(tokens, token{t: tFalse})
			case "null":
				tokens = append(tokens, token{t: tNull})
			default:
				kind := tIdent
				if c >= 'A' && c <= 'Z' {
					kind = tType
				}
				tokens = append(tokens, token{t: kind, s: word, start: start})
			}
			continue
		}
		i++
	}
	return append(tokens, token{t: tEOF, start: n})
}
