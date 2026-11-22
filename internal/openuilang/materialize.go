package openuilang

type materializer struct {
	issueSink
	syms      map[string]*node
	unres     []string
	visited   map[string]bool
	unreached map[string]bool
	memo      map[string]any
	depth     int
}

const maxMaterializeDepth = 400

func containsDynamic(value any) bool {
	switch v := value.(type) {
	case *node, *ActionPlan, *Binding:
		return true
	case []any:
		for _, item := range v {
			if containsDynamic(item) {
				return true
			}
		}
	case *Element:
		return v.dynamic
	case *Object:
		for _, key := range v.Keys {
			if containsDynamic(v.Values[key]) {
				return true
			}
		}
	}
	return false
}

func withStatementID(el *Element, id string) *Element {
	if el.StatementID == "" || el.StatementID == id {
		el.StatementID = id
		return el
	}
	copied := *el
	copied.StatementID = id
	return &copied
}

func (m *materializer) withStatement(id string, fn func()) {
	previous := m.statement
	m.statement = id
	defer func() { m.statement = previous }()
	fn()
}

func (m *materializer) resolveRef(name string, exprMode bool) any {
	target, ok := m.syms[name]
	if m.visited[name] || !ok || m.depth > maxMaterializeDepth {
		m.unres = append(m.unres, name)
		if exprMode {
			return &node{k: kPh, name: name}
		}
		return nil
	}
	delete(m.unreached, name)
	if target.k == kComp && isReservedCall(target.name) {
		refType := "query"
		if target.name == "Mutation" {
			refType = "mutation"
		}
		return &node{k: kRuntimeRef, name: name, refType: refType}
	}
	key := "value:" + name
	if exprMode {
		key = "expr:" + name
	}
	if value, ok := m.memo[key]; ok {
		return value
	}
	m.visited[name] = true
	defer delete(m.visited, name)
	var value any
	m.withStatement(name, func() {
		before := len(m.unres)
		if exprMode {
			value = m.expr(target, nil)
		} else {
			value = m.value(target)
			if el, ok := value.(*Element); ok {
				value = withStatementID(el, name)
			}
		}
		if len(m.unres) == before {
			m.memo[key] = value
		}
	})
	return value
}

func nameArgs(args []*node, params []paramDef) []objEntry {
	out := []objEntry{}
	for i := 0; i < len(params) && i < len(args); i++ {
		out = append(out, objEntry{params[i].name, args[i]})
	}
	return out
}

func entryIndex(entries []objEntry, key string) int {
	for i, entry := range entries {
		if entry.key == key {
			return i
		}
	}
	return -1
}

func (m *materializer) mapCallArgs(name string, args []*node, def *callDef) ([]objEntry, bool) {
	if len(args) > len(def.params) {
		m.excess(name, len(def.params), len(args))
	}
	mapped := nameArgs(args, def.params)
	invalid := false
	before := len(m.errors)
	for _, p := range def.params {
		at := entryIndex(mapped, p.name)
		var literal any
		if at >= 0 {
			value, ok := literalValue(mapped[at].value)
			if !ok {
				continue
			}
			literal = value
		}
		if literal == nil {
			if !p.required || p.hasDefault {
				continue
			}
			code := "missing-required"
			if at >= 0 {
				code = "null-required"
			}
			m.required(code, name, "/"+p.name, paramsSignature(name, def.params))
			invalid = true
			continue
		}
		next, bad := m.validate(literal, p.schema, name, "/"+p.name)
		if bad {
			next, bad = resolveInvalid(p.required, p.hasDefault, p.defaultVal)
		}
		if bad {
			invalid = true
		} else if _, drop := next.(undefinedValue); drop {
			mapped = append(mapped[:at:at], mapped[at+1:]...)
		} else {
			mapped[at].value = literalNode(next)
		}
	}
	if def.kind == "action" && len(m.errors) > before {
		invalid = true
	}
	return mapped, !invalid
}

func (m *materializer) call(n *node, args []*node) *node {
	out := *n
	out.args = args
	def := m.lib.calls[n.name]
	if def == nil || def.lazy || def.builtin {
		return &out
	}
	mapped, ok := m.mapCallArgs(n.name, args, def)
	if ok {
		out.mapped, out.hasMap = mapped, true
		return &out
	}
	if def.kind == "action" {
		return &node{k: kComp, name: "Action", args: []*node{{k: kArr, args: []*node{}}}}
	}
	return &node{k: kNull}
}

func (m *materializer) lazyBuiltin(n *node, scoped map[string]bool) *node {
	def := m.lib.calls[n.name]
	if def == nil || def.kind != "function" || !def.lazy || len(n.args) < 3 {
		return nil
	}
	name := ""
	switch n.args[1].k {
	case kRef:
		name = n.args[1].name
	case kStr:
		name = n.args[1].str
	}
	if name == "" {
		return nil
	}
	next := map[string]bool{name: true}
	for key := range scoped {
		next[key] = true
	}
	out := *n
	out.args = make([]*node, len(n.args))
	for i, arg := range n.args {
		switch i {
		case 0:
			out.args[i] = m.expr(arg, scoped)
		case 1:
			out.args[i] = arg
		default:
			out.args[i] = m.expr(arg, next)
		}
	}
	return &out
}

func (m *materializer) exprs(list []*node, scoped map[string]bool) []*node {
	out := make([]*node, len(list))
	for i, item := range list {
		out[i] = m.expr(item, scoped)
	}
	return out
}

func (m *materializer) expr(n *node, scoped map[string]bool) *node {
	m.depth++
	defer func() { m.depth-- }()
	if n == nil || m.depth > maxMaterializeDepth {
		return &node{k: kNull}
	}
	out := *n
	switch n.k {
	case kRef:
		if scoped[n.name] {
			return n
		}
		return m.resolveRef(n.name, true).(*node)
	case kComp:
		if n.fn && !m.lib.isRuntimeCall(n.name) {
			m.unknownFunction(n.name)
			return &node{k: kNull}
		}
		if lazy := m.lazyBuiltin(n, scoped); lazy != nil {
			return lazy
		}
		args := m.exprs(n.args, scoped)
		if m.lib.isRuntimeCall(n.name) {
			return m.call(n, args)
		}
		out.args = args
		if params, ok := m.lib.components[n.name]; ok {
			out.mapped, out.hasMap = nameArgs(args, params), true
			return &out
		}
		m.unknownComponent(n.name)
		return &out
	case kArr:
		out.args = m.exprs(n.args, scoped)
	case kObj:
		out.entries = make([]objEntry, len(n.entries))
		for i, entry := range n.entries {
			out.entries[i] = objEntry{entry.key, m.expr(entry.value, scoped)}
		}
	case kBinOp, kIndex:
		out.a, out.b = m.expr(n.a, scoped), m.expr(n.b, scoped)
	case kUnaryOp, kMember, kAssign:
		out.a = m.expr(n.a, scoped)
	case kTernary:
		out.a, out.b, out.c = m.expr(n.a, scoped), m.expr(n.b, scoped), m.expr(n.c, scoped)
	default:
		return n
	}
	return &out
}

func (m *materializer) value(n *node) any {
	m.depth++
	defer func() { m.depth-- }()
	if m.depth > maxMaterializeDepth {
		return nil
	}
	switch n.k {
	case kRef:
		return m.resolveRef(n.name, false)
	case kStr:
		return n.str
	case kNum:
		return n.num
	case kBool:
		return n.boolean
	case kNull, kPh:
		return nil
	case kArr:
		items := []any{}
		for _, item := range n.args {
			if item.k == kPh {
				continue
			}
			value := m.value(item)
			if value == nil && (item.k == kComp || item.k == kRef) {
				continue
			}
			items = append(items, value)
		}
		return items
	case kObj:
		out := NewObject()
		for _, entry := range n.entries {
			out.Set(entry.key, m.value(entry.value))
		}
		return out
	case kComp:
		return m.component(n)
	}
	return m.expr(n, nil)
}

func (m *materializer) component(n *node) any {
	name := n.name
	if n.fn && !m.lib.isRuntimeCall(name) {
		m.unknownFunction(name)
		return nil
	}
	if isReservedCall(name) {
		m.issue("inline-reserved", name, "", name+"() must be declared as a top-level statement, not used inline as a value")
		return nil
	}
	if m.lib.isRuntimeCall(name) {
		if lazy := m.lazyBuiltin(n, nil); lazy != nil {
			return lazy
		}
		return m.call(n, m.exprs(n.args, nil))
	}
	params, ok := m.lib.components[name]
	if !ok {
		m.unknownComponent(name)
		return nil
	}
	props := NewObject()
	drop := false
	var drawnWithDefault []int
	for i := 0; i < len(params) && i < len(n.args); i++ {
		param := params[i]
		value := m.value(n.args[i])
		props.Set(param.name, value)
		if param.schema == nil {
			continue
		}
		before := len(m.errors)
		next, bad := m.validate(value, param.schema, name, "/"+param.name)
		if bad {
			next, bad = resolveInvalid(param.required, param.hasDefault, param.defaultVal)
			if bad {
				drop = true
				continue
			}
			if !param.required {
				for at := before; at < len(m.errors); at++ {
					drawnWithDefault = append(drawnWithDefault, at)
				}
			}
		}
		if _, gone := next.(undefinedValue); gone {
			props.Delete(param.name)
		} else {
			props.Set(param.name, next)
		}
	}
	if len(n.args) > len(params) {
		m.excess(name, len(params), len(n.args))
	}
	missing := false
	for _, p := range params {
		value, present := props.Get(p.name)
		if !p.required || (present && value != nil) {
			continue
		}
		if p.hasDefault {
			props.Set(p.name, p.defaultVal)
			continue
		}
		code := "missing-required"
		if present {
			code = "null-required"
		}
		m.required(code, name, "/"+p.name, paramsSignature(name, params))
		missing = true
	}
	if missing || drop {
		return nil
	}
	for _, at := range drawnWithDefault {
		m.errors[at].Degraded = true
	}
	el := &Element{Type: name, Props: props}
	for _, key := range props.Keys {
		if containsDynamic(props.Values[key]) {
			el.dynamic = true
			break
		}
	}
	return el
}
