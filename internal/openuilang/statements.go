package openuilang

type rawStatement struct {
	id         string
	idKind     tokenKind
	tokens     []token
	start, end int
}

func autoClose(input string) (string, bool) {
	var stack []byte
	var inStr byte
	esc := false
	for i := 0; i < len(input); i++ {
		c := input[i]
		switch {
		case esc:
			esc = false
		case c == '\\' && inStr != 0:
			esc = true
		case inStr != 0:
			if c == inStr {
				inStr = 0
			}
		case c == '"' || c == '\'':
			inStr = c
		case c == '(' || c == '[' || c == '{':
			stack = append(stack, c)
		case c == ')' || c == ']' || c == '}':
			if len(stack) > 0 && stack[len(stack)-1] == opener[c] {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if inStr == 0 && len(stack) == 0 {
		return input, false
	}
	out := []byte(input)
	if inStr != 0 {
		if esc {
			out = append(out, '\\')
		}
		out = append(out, inStr)
	}
	for j := len(stack) - 1; j >= 0; j-- {
		out = append(out, closer[stack[j]])
	}
	return string(out), true
}

var (
	opener = map[byte]byte{')': '(', ']': '[', '}': '{'}
	closer = map[byte]byte{'(': ')', '[': ']', '{': '}'}
)

func splitStatements(tokens []token) []rawStatement {
	var out []rawStatement
	pos := 0
	skipLine := func() {
		for pos < len(tokens) && tokens[pos].t != tNewline && tokens[pos].t != tEOF {
			pos++
		}
	}
	for pos < len(tokens) {
		for pos < len(tokens) && tokens[pos].t == tNewline {
			pos++
		}
		if pos >= len(tokens) || tokens[pos].t == tEOF {
			break
		}
		head := tokens[pos]
		if head.t != tIdent && head.t != tType && head.t != tStateVar {
			skipLine()
			continue
		}
		pos++
		if pos >= len(tokens) || tokens[pos].t != tEquals {
			skipLine()
			continue
		}
		pos++
		var expr []token
		depth, ternary := 0, 0
		for pos < len(tokens) && tokens[pos].t != tEOF {
			kind := tokens[pos].t
			if kind == tNewline && depth <= 0 && ternary <= 0 {
				peek := pos + 1
				for peek < len(tokens) && tokens[peek].t == tNewline {
					peek++
				}
				if peek < len(tokens) && tokens[peek].t == tQuestion {
					pos++
					continue
				}
				break
			}
			if kind == tNewline {
				pos++
				continue
			}
			switch {
			case kind == tLParen || kind == tLBrack || kind == tLBrace:
				depth++
			case (kind == tRParen || kind == tRBrack || kind == tRBrace) && depth > 0:
				depth--
			case kind == tQuestion && depth == 0:
				ternary++
			case kind == tColon && depth == 0 && ternary > 0:
				ternary--
			}
			expr = append(expr, tokens[pos])
			pos++
		}
		if len(expr) > 0 {
			end := -1
			if pos < len(tokens) {
				end = tokens[pos].start
			}
			out = append(out, rawStatement{id: head.s, idKind: head.t, tokens: expr, start: head.start, end: end})
		}
	}
	return out
}
