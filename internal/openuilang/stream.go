package openuilang

import (
	"regexp"
	"strings"
)

type Stream struct {
	lib          *Library
	buf          string
	cleaned      string
	completedEnd int
	completed    *statementSet
	count        int
}

func (l *Library) NewStream() *Stream {
	return &Stream{lib: l, completed: newStatementSet()}
}

func (s *Stream) Push(chunk string) *Program {
	s.buf += chunk
	return s.result()
}

func (s *Stream) Finish() *Program { return s.lib.Parse(s.buf) }

func (s *Stream) refresh() {
	next := preprocess(s.buf)
	if !strings.HasPrefix(next, s.cleaned[:s.completedEnd]) {
		s.completedEnd, s.count = 0, 0
		s.completed = newStatementSet()
	}
	s.cleaned = next
}

func (s *Stream) scanCompleted() int {
	depth, ternary := 0, 0
	var inStr byte
	esc := false
	start := s.completedEnd
	text := s.cleaned
	for i := s.completedEnd; i < len(text); i++ {
		c := text[i]
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
			depth++
		case c == ')' || c == ']' || c == '}':
			depth = max(0, depth-1)
		case c == '?' && depth == 0:
			ternary++
		case c == ':' && depth == 0 && ternary > 0:
			ternary--
		case c == '\n' && depth <= 0 && ternary <= 0:
			peek := i + 1
			for peek < len(text) && strings.IndexByte(" \t\r\n", text[peek]) >= 0 {
				peek++
			}
			if peek < len(text) && text[peek] == '?' {
				continue
			}
			if statement := strings.TrimSpace(text[start:i]); statement != "" {
				s.count += parseStatements(statement, s.completed)
			}
			start, s.completedEnd = i+1, i+1
		}
	}
	return start
}

var trailingState = regexp.MustCompile(`\$\w+$`)

func (s *Stream) result() (program *Program) {
	defer func() {
		if recover() != nil {
			program = s.lib.emptyProgram(false)
			program.Errors = []Error{{Source: "parser", Code: "parse-exception", Message: "the parser failed"}}
		}
	}()
	s.refresh()
	pending := strings.TrimSpace(s.cleaned[s.scanCompleted():])
	closed, incomplete := autoClose(pending)
	var raws []rawStatement
	if pending != "" {
		raws = splitStatements(tokenize(closed))
	}
	if len(raws) == 0 {
		if s.count == 0 {
			return s.lib.emptyProgram(pending == "" || incomplete)
		}
		return s.lib.build(s.completed, incomplete, nil)
	}
	all := s.completed.clone()
	partial := &partialRef{pending: map[*statement]bool{}}
	for _, raw := range raws {
		if _, done := s.completed.byID[raw.id]; done && incomplete {
			continue
		}
		stmt := classify(raw, parseExpression(raw.tokens))
		all.set(stmt)
		partial.pending[stmt] = true
	}
	if tail := trailingState.FindString(s.buf); tail != "" && strings.HasSuffix(pending, tail) {
		before := pending[:len(pending)-len(tail)]
		earlier := false
		for at := strings.Index(before, tail); at >= 0; {
			end := at + len(tail)
			if end >= len(before) || !isWord(before[end]) {
				earlier = true
				break
			}
			next := strings.Index(before[end:], tail)
			if next < 0 {
				break
			}
			at = end + next
		}
		if !earlier {
			partial.name = tail
		}
	}
	return s.lib.build(all, incomplete, partial)
}

type sourceStatement struct {
	id   string
	expr *node
	raw  string
}

func sourceStatements(input string) []sourceStatement {
	var out []sourceStatement
	for _, raw := range splitStatements(tokenize(input)) {
		end := raw.end
		if end < 0 || end > len(input) {
			end = len(input)
		}
		out = append(out, sourceStatement{raw.id, parseExpression(raw.tokens), strings.TrimRight(input[raw.start:end], " \t\r\n\v\f")})
	}
	return out
}

func Names(text string) []string {
	var names []string
	for _, stmt := range sourceStatements(stripFences(text)) {
		names = append(names, stmt.id)
	}
	return names
}

func Merge(existing, patch string) string { return merge(existing, patch, true) }

func Overlay(existing, patch string) string { return merge(existing, patch, false) }

func merge(existing, patch string, prune bool) string {
	current, changes := sourceStatements(existing), sourceStatements(stripFences(patch))
	if len(current) == 0 {
		lines := make([]string, len(changes))
		for i, stmt := range changes {
			lines[i] = stmt.raw
		}
		return strings.Join(lines, "\n")
	}
	if len(changes) == 0 {
		return existing
	}
	merged := map[string]sourceStatement{}
	var order []string
	remove := func(id string) {
		delete(merged, id)
		for i, existing := range order {
			if existing == id {
				order = append(order[:i:i], order[i+1:]...)
				return
			}
		}
	}
	set := func(stmt sourceStatement) {
		if _, ok := merged[stmt.id]; !ok {
			order = append(order, stmt.id)
		}
		merged[stmt.id] = stmt
	}
	for _, stmt := range current {
		set(stmt)
	}
	for _, stmt := range changes {
		if stmt.expr.k == kNull {
			remove(stmt.id)
		} else {
			set(stmt)
		}
	}
	if root, ok := merged["root"]; ok && prune {
		reachable := map[string]bool{"root": true}
		queue := []*node{root.expr}
		for len(queue) > 0 {
			expr := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			walkAST(expr, func(n *node) {
				if n.k != kRef || reachable[n.name] {
					return
				}
				if target, ok := merged[n.name]; ok {
					reachable[n.name] = true
					queue = append(queue, target.expr)
				}
			})
		}
		for _, id := range append([]string(nil), order...) {
			if !reachable[id] && !strings.HasPrefix(id, "$") {
				remove(id)
			}
		}
	}
	lines := make([]string, len(order))
	for i, id := range order {
		lines[i] = merged[id].raw
	}
	return strings.Join(lines, "\n")
}
