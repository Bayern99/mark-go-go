package openuilang

import (
	"regexp"
	"strings"
)

type statementKind int

const (
	valueStatement statementKind = iota
	stateStatement
	queryStatement
	mutationStatement
)

type statement struct {
	kind statementKind
	id   string
	expr *node
}

type statementSet struct {
	order []string
	byID  map[string]*statement
}

func newStatementSet() *statementSet { return &statementSet{byID: map[string]*statement{}} }

func (s *statementSet) set(stmt *statement) {
	if _, ok := s.byID[stmt.id]; !ok {
		s.order = append(s.order, stmt.id)
	}
	s.byID[stmt.id] = stmt
}

func (s *statementSet) clone() *statementSet {
	out := &statementSet{order: append([]string(nil), s.order...), byID: map[string]*statement{}}
	for id, stmt := range s.byID {
		out.byID[id] = stmt
	}
	return out
}

type query struct {
	id                   string
	tool, args, defaults *node
	waitFor              []string
}

type Program struct {
	lib        *Library
	Root       *Element
	rootExpr   *node
	Incomplete bool
	Unresolved []string
	Orphans    []string
	Errors     []Error
	State      *Object
	queries    []query
	mutations  map[string]bool
}

var (
	statementStart = regexp.MustCompile(`^\s*\$?[A-Za-z_]\w*\s*=([^=]|$)`)
	fenceLine      = regexp.MustCompile("^[ \t]*```[ \t]*([\\w.+-]*)[ \t]*(.*?)[ \t]*\r?$")
	assignStart    = regexp.MustCompile(`^=([^=]|$)`)
	leadComments   = regexp.MustCompile(`^(\s*(//|#)[^\n]*\n)+`)
)

func startsProgram(block string) bool {
	return statementStart.MatchString(leadComments.ReplaceAllString(block, ""))
}

func stripFences(input string) string {
	if !strings.Contains(input, "```") {
		return input
	}
	var blocks, outside, body []string
	inBlock, script, open := false, false, false
	depth := 0
	scan := func(line string) {
		for i := 0; i < len(line); i++ {
			c := line[i]
			switch {
			case open && c == '\\':
				i++
			case open && c == '"':
				open = false
			case open:
			case c == '"':
				open = true
			case c == '(' || c == '[' || c == '{':
				depth++
			case (c == ')' || c == ']' || c == '}') && depth > 0:
				depth--
			}
		}
	}
	for _, line := range strings.Split(input, "\n") {
		var fence []string
		if !open {
			fence = fenceLine.FindStringSubmatch(line)
		}
		if fence == nil {
			if script {
				continue
			}
			if inBlock {
				body = append(body, line)
			} else {
				outside = append(outside, line)
			}
			if open || depth > 0 || inBlock || statementStart.MatchString(line) {
				scan(line)
			}
			continue
		}
		depth = 0
		tag, rest := fence[1], fence[2]
		if inBlock {
			if !script {
				blocks = append(blocks, strings.Join(body, "\n"))
			}
			inBlock, script, body = false, false, nil
			continue
		}
		if strings.Contains(rest, "```") {
			continue
		}
		inBlock, body = true, nil
		script = strings.EqualFold(tag, "openui-js")
		if statementStart.MatchString(rest) {
			body = []string{rest}
		} else if tag != "" && assignStart.MatchString(rest) {
			body = []string{tag + " " + rest}
		}
	}
	if inBlock && !script {
		blocks = append(blocks, strings.Join(body, "\n"))
	}
	allBlank := true
	for _, block := range blocks {
		if startsProgram(block) {
			return strings.Join(blocks, "\n")
		}
		if strings.TrimSpace(block) != "" {
			allBlank = false
		}
	}
	for _, line := range outside {
		if statementStart.MatchString(line) {
			allBlank = true
			break
		}
	}
	if allBlank {
		return strings.Join(outside, "\n")
	}
	return strings.Join(blocks, "\n")
}

func stripComments(input string) string {
	var inStr byte
	lines := strings.Split(input, "\n")
	for at, line := range lines {
		for i := 0; i < len(line); i++ {
			c := line[i]
			if inStr != 0 {
				if c == '\\' && i+1 < len(line) {
					i++
				} else if c == inStr {
					inStr = 0
				}
				continue
			}
			if c == '"' || c == '\'' {
				inStr = c
				continue
			}
			if c == '#' || (c == '/' && i+1 < len(line) && line[i+1] == '/') {
				lines[at] = strings.TrimRight(line[:i], " \t\r\v\f")
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func preprocess(input string) string {
	return strings.TrimSpace(stripComments(stripFences(strings.TrimSpace(input))))
}

func classify(raw rawStatement, expr *node) *statement {
	kind := valueStatement
	switch {
	case expr.k == kComp && expr.name == "Query":
		kind = queryStatement
	case expr.k == kComp && expr.name == "Mutation":
		kind = mutationStatement
	case raw.idKind == tStateVar:
		kind = stateStatement
	}
	return &statement{kind: kind, id: raw.id, expr: expr}
}

func parseStatements(text string, into *statementSet) int {
	count := 0
	for _, raw := range splitStatements(tokenize(text)) {
		into.set(classify(raw, parseExpression(raw.tokens)))
		count++
	}
	return count
}

func (l *Library) Parse(input string) (program *Program) {
	defer func() {
		if recover() != nil {
			program = l.emptyProgram(false)
			program.Errors = []Error{{Source: "parser", Code: "parse-exception", Message: "the parser failed"}}
		}
	}()
	trimmed := preprocess(input)
	if trimmed == "" {
		return l.emptyProgram(true)
	}
	text, incomplete := autoClose(trimmed)
	statements := newStatementSet()
	if parseStatements(text, statements) == 0 {
		return l.emptyProgram(incomplete)
	}
	return l.build(statements, incomplete, nil)
}

func (l *Library) emptyProgram(incomplete bool) *Program {
	return &Program{lib: l, Incomplete: incomplete, State: NewObject(), mutations: map[string]bool{}, Unresolved: []string{}, Orphans: []string{}}
}

func (l *Library) isComponentCall(n *node) bool {
	return n != nil && n.k == kComp && !n.fn && !isBuiltin(n.name) && !isReservedCall(n.name)
}

type partialRef struct {
	name    string
	pending map[*statement]bool
}

func (l *Library) build(statements *statementSet, incomplete bool, partial *partialRef) *Program {
	entry, fallback := "", false
	if _, ok := statements.byID["root"]; ok {
		entry = "root"
	} else {
		for _, id := range statements.order {
			stmt := statements.byID[id]
			if stmt.kind == valueStatement && l.isComponentCall(stmt.expr) && stmt.expr.name == l.root && l.root != "" {
				entry, fallback = id, true
				break
			}
		}
	}
	var pending map[string]bool
	if incomplete && partial == nil {
		pending = nil
	} else if partial != nil {
		pending = map[string]bool{}
		for id, stmt := range statements.byID {
			pending[id] = partial.pending[stmt]
		}
	}
	m := &materializer{
		syms:       map[string]*node{},
		visited:    map[string]bool{},
		unreached:  map[string]bool{},
		memo:       map[string]any{},
		incomplete: incomplete,
		pending:    pending,
	}
	m.issueSink = issueSink{lib: l, partial: m.partialStatement(entry), statement: entry}
	for _, id := range statements.order {
		stmt := statements.byID[id]
		m.syms[id] = stmt.expr
		if id != entry && stmt.kind == valueStatement {
			m.unreached[id] = true
		}
	}
	program := l.emptyProgram(incomplete)
	if entry != "" {
		switch value := m.value(m.syms[entry]).(type) {
		case *Element:
			program.Root = withStatementID(value, entry)
		case *node:
			found := false
			walkAST(value, func(n *node) { found = found || l.isComponentCall(n) })
			if found {
				program.rootExpr = value
			}
		}
	}
	m.extract(statements, program, partial)
	m.statement = entry
	m.partial = m.partialStatement(entry)
	switch {
	case entry == "":
		message := "No root statement"
		if l.root != "" {
			message += ", and no statement calls the root component " + l.root
		}
		m.issue("no-root", l.root, "", message)
	case program.Root == nil && program.rootExpr == nil && !l.resolvesToComponent(m.syms[entry], m.syms):
		m.issue("no-root", l.root, "", "root does not evaluate to a component")
	case fallback:
		m.issue("no-root", l.root, "", "No root statement. Rendering the first statement that calls the root component "+l.root)
		if program.Root != nil {
			m.errors[len(m.errors)-1].Severity = "warning"
		}
	}
	if entry != "" {
		for _, id := range statements.order {
			if m.unreached[id] {
				program.Orphans = append(program.Orphans, id)
			}
		}
	}
	program.Unresolved = append(program.Unresolved, m.unres...)
	program.Errors = m.errors
	return program
}

func (l *Library) resolvesToComponent(n *node, syms map[string]*node) bool {
	seen := map[string]bool{}
	for n != nil && n.k == kRef && !seen[n.name] {
		seen[n.name] = true
		n = syms[n.name]
	}
	return l.isComponentCall(n)
}

func argAt(args []*node, i int) *node {
	if i < len(args) {
		return args[i]
	}
	return nil
}

func stateRefs(n *node, visit func(string)) {
	walkAST(n, func(current *node) {
		if current.k == kStateRef {
			visit(current.name)
		}
	})
}

func (m *materializer) extract(statements *statementSet, program *Program, partial *partialRef) {
	for _, id := range statements.order {
		stmt := statements.byID[id]
		switch stmt.kind {
		case stateStatement:
			program.State.Set(id, m.stateDefault(m.value(stmt.expr), program.State))
		case queryStatement, mutationStatement:
			var args []*node
			m.withStatement(id, func() { args = m.exprs(stmt.expr.args, nil) })
			if stmt.kind == mutationStatement {
				program.mutations[id] = true
				continue
			}
			q := query{id: id, tool: argAt(args, 0), args: argAt(args, 1), defaults: argAt(args, 2)}
			seen := map[string]bool{}
			for _, source := range []*node{q.tool, q.args} {
				walkAST(source, func(n *node) {
					if n.k == kRuntimeRef && n.refType == "query" && !seen[n.name] {
						seen[n.name] = true
						q.waitFor = append(q.waitFor, n.name)
					}
				})
			}
			program.queries = append(program.queries, q)
		}
	}
	for _, first := range queryCycles(program.queries) {
		m.withStatement(first, func() {
			m.issue("query-cycle", "Query", "", "Queries read each other's results, so none of them can run first. They do not run")
		})
	}
	for _, id := range statements.order {
		stmt := statements.byID[id]
		nodes := []*node{stmt.expr}
		if stmt.kind == queryStatement || stmt.kind == mutationStatement {
			nodes = stmt.expr.args
		}
		for _, n := range nodes {
			stateRefs(n, func(name string) {
				if partial != nil && name == partial.name && partial.pending[stmt] {
					return
				}
				if _, ok := program.State.Get(name); !ok {
					program.State.Set(name, nil)
				}
			})
		}
	}
}

func queryCycles(queries []query) []string {
	order := map[string]int{}
	reads := map[string][]string{}
	for i, q := range queries {
		order[q.id] = i
		reads[q.id] = q.waitFor
	}
	done := map[string]bool{}
	var path, firsts []string
	var visit func(string)
	visit = func(id string) {
		path = append(path, id)
		for _, dep := range reads[id] {
			at := -1
			for i, seen := range path {
				if seen == dep {
					at = i
					break
				}
			}
			if at >= 0 {
				first := path[at]
				for _, member := range path[at:] {
					if order[member] < order[first] {
						first = member
					}
				}
				firsts = append(firsts, first)
			} else if _, known := reads[dep]; known && !done[dep] {
				visit(dep)
			}
		}
		path = path[:len(path)-1]
		done[id] = true
	}
	for _, q := range queries {
		if !done[q.id] {
			visit(q.id)
		}
	}
	return firsts
}

func (m *materializer) stateDefault(value any, declared *Object) any {
	switch v := value.(type) {
	case *node:
		ok := true
		walkAST(v, func(n *node) {
			if n.k == kComp {
				def := builtinCalls[n.name]
				ok = ok && def != nil && def.kind == "function"
			}
		})
		if !ok {
			return nil
		}
		budget := evaluationBudget
		ctx := &evalContext{
			budget:     &budget,
			lib:        m.lib,
			getState:   func(name string) any { value, _ := declared.Get(name); return value },
			resolveRef: func(string) any { return nil },
		}
		return orNull(ctx.evaluate(v, false))
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = m.stateDefault(item, declared)
		}
		return out
	case *Object:
		out := NewObject()
		for _, key := range v.Keys {
			out.Set(key, m.stateDefault(v.Values[key], declared))
		}
		return out
	}
	return value
}
