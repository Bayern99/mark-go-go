package openuilang

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type evalContext struct {
	lib        *Library
	getState   func(string) any
	resolveRef func(string) any
	report     func(string)
	errors     *[]Error
	depth      int
	budget     *int
	acting     bool
}

const maxEvalDepth = 500

func (c *evalContext) state(name string) any { return orNull(c.getState(name)) }

func (c *evalContext) evaluateAll(list []*node) []any {
	out := make([]any, len(list))
	for i, item := range list {
		out[i] = c.evaluate(item, false)
	}
	return out
}

func (c *evalContext) evaluate(n *node, withSchema bool) any {
	c.depth++
	defer func() { c.depth-- }()
	if n == nil || c.depth > maxEvalDepth {
		return nil
	}
	if c.budget != nil {
		if *c.budget <= 0 {
			return nil
		}
		*c.budget--
	}
	switch n.k {
	case kStr:
		return n.str
	case kNum:
		return n.num
	case kBool:
		return n.boolean
	case kStateRef:
		return c.getState(n.name)
	case kRef, kRuntimeRef:
		return c.resolveRef(n.name)
	case kArr:
		return c.evaluateAll(n.args)
	case kObj:
		out := NewObject()
		for _, entry := range n.entries {
			out.Set(entry.key, c.evaluate(entry.value, false))
		}
		return out
	case kComp:
		return c.call(n, withSchema)
	case kBinOp:
		return c.binary(n)
	case kUnaryOp:
		operand := c.evaluate(n.a, false)
		if n.name == "!" {
			return !truthy(operand)
		}
		return -toNumber(operand)
	case kTernary:
		if truthy(c.evaluate(n.a, false)) {
			return c.evaluate(n.b, false)
		}
		return c.evaluate(n.c, false)
	case kMember:
		object := c.evaluate(n.a, false)
		if list, ok := object.([]any); ok {
			if n.name == "length" {
				return float64(len(list))
			}
			out := make([]any, len(list))
			for i, item := range list {
				out[i] = orNull(member(item, n.name))
			}
			return out
		}
		return member(object, n.name)
	case kIndex:
		object, index := c.evaluate(n.a, false), c.evaluate(n.b, false)
		if isNullish(object) || isNullish(index) {
			return nil
		}
		switch v := object.(type) {
		case []any:
			at := toNumber(index)
			if at >= 0 && at < float64(len(v)) && at == math.Trunc(at) {
				return v[int(at)]
			}
		case *Object:
			value, _ := v.Get(toString(index))
			return value
		}
		return nil
	case kAssign:
		return &Binding{Target: n.name, expr: n.a}
	}
	return nil
}

func member(object any, field string) any {
	switch v := object.(type) {
	case *Object:
		value, _ := v.Get(field)
		return value
	case string:
		if field == "length" {
			return float64(len(utf16.Encode([]rune(v))))
		}
	case []any:
		if field == "length" {
			return float64(len(v))
		}
		if at := stringToNumber(field); field != "" && at >= 0 && at < float64(len(v)) && at == math.Trunc(at) {
			return v[int(at)]
		}
	}
	return nil
}

func (c *evalContext) binary(n *node) any {
	left := c.evaluate(n.a, false)
	switch n.name {
	case "&&":
		if truthy(left) {
			return c.evaluate(n.b, false)
		}
		return left
	case "||":
		if truthy(left) {
			return left
		}
		return c.evaluate(n.b, false)
	}
	right := c.evaluate(n.b, false)
	l, r := toNumber(left), toNumber(right)
	switch n.name {
	case "+":
		_, leftString := left.(string)
		_, rightString := right.(string)
		if leftString || rightString {
			return stringOrEmpty(left) + stringOrEmpty(right)
		}
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		if r == 0 {
			return 0.0
		}
		return l / r
	case "%":
		if r == 0 {
			return 0.0
		}
		return math.Mod(l, r)
	case "==":
		return looseEqual(left, right)
	case "!=":
		return !looseEqual(left, right)
	case ">":
		return l > r
	case "<":
		return l < r
	case ">=":
		return l >= r
	case "<=":
		return l <= r
	}
	return nil
}

func (c *evalContext) call(n *node, withSchema bool) any {
	if def := builtinCalls[n.name]; def != nil {
		if def.kind == "action" {
			return c.action(n)
		}
		if n.name == "Each" {
			return c.each(n.args, withSchema)
		}
		return builtinFunction(n.name, c.evaluateAll(n.args))
	}
	if def := c.lib.calls[n.name]; def != nil {
		if def.kind == "action" {
			return c.customAction(n, def)
		}
		return c.function(n, def)
	}
	if !n.hasMap {
		return nil
	}
	props := NewObject()
	for _, entry := range n.mapped {
		value := entry.value
		switch {
		case value.k == kStateRef && withSchema && c.lib.bindable[n.name][entry.key]:
			props.Set(entry.key, &Binding{Target: value.name, expr: &node{k: kStateRef, name: "$value"}})
		case value.k == kStateRef && withSchema:
			props.Set(entry.key, c.getState(value.name))
		case value.k == kStateRef:
			props.Set(entry.key, value)
		default:
			props.Set(entry.key, c.evaluate(value, withSchema))
		}
	}
	if withSchema {
		for _, key := range props.Keys {
			switch v := props.Values[key].(type) {
			case *Element:
				props.Values[key] = c.element(v, true)
			case []any:
				for i, item := range v {
					if el, ok := item.(*Element); ok {
						v[i] = c.element(el, true)
					}
				}
			}
		}
	}
	return &Element{Type: n.name, Props: props, dynamic: true}
}

func (c *evalContext) callArgs(n *node, def *callDef) *Object {
	named := n.mapped
	if !n.hasMap {
		named = nameArgs(n.args, def.params)
	}
	args := NewObject()
	for _, entry := range named {
		value := c.evaluate(entry.value, false)
		at := -1
		for i, p := range def.params {
			if p.name == entry.key {
				at = i
			}
		}
		if isNullish(value) && (at < 0 || !def.params[at].required) {
			continue
		}
		args.Set(entry.key, orNull(value))
	}
	return args
}

func missesRequired(args *Object, def *callDef) bool {
	for _, p := range def.params {
		if value, _ := args.Get(p.name); p.required && value == nil && !p.hasDefault {
			return true
		}
	}
	return false
}

func (c *evalContext) check(value, schema any, name, path string) (any, bool, bool) {
	sink := &issueSink{lib: c.lib}
	checked, invalid := sink.validate(value, schema, name, path)
	for _, e := range sink.errors {
		c.reportError("@" + name + ": " + e.Message)
	}
	return checked, invalid, len(sink.errors) > 0
}

func (c *evalContext) reportError(message string) {
	if c.report != nil {
		c.report(message)
	}
}

func (c *evalContext) function(n *node, def *callDef) any {
	fn := c.lib.functions[n.name]
	if fn == nil {
		c.reportError("@" + n.name + " has no implementation")
		return nil
	}
	if c.lib.actionOnly[n.name] && !c.acting {
		c.reportError("@" + n.name + " gives a new value each time, so it only runs inside an action such as @Set")
		return nil
	}
	raw := c.callArgs(n, def)
	if missesRequired(raw, def) {
		return nil
	}
	checked, invalid, _ := c.check(raw, def.schema, n.name, "")
	args, ok := checked.(*Object)
	if invalid || !ok {
		return nil
	}
	result, err := fn(args)
	if err != nil {
		c.reportError(fmt.Sprintf("@%s threw: %v", n.name, err))
		return nil
	}
	if def.returns == nil {
		return result
	}
	checked, invalid, _ = c.check(result, def.returns, n.name, "/returns")
	if invalid {
		return nil
	}
	return orNull(checked)
}

func (c *evalContext) customAction(n *node, def *callDef) any {
	args := c.callArgs(n, def)
	if missesRequired(args, def) {
		return &ActionPlan{}
	}
	checked, invalid, failed := c.check(args, def.schema, n.name, "")
	params, ok := checked.(*Object)
	if invalid || failed || !ok {
		return &ActionPlan{}
	}
	return &ActionPlan{Steps: []ActionStep{{Type: "custom_action", Name: n.name, Params: params, custom: true}}}
}

func (c *evalContext) action(n *node) any {
	arg := func(i int) any {
		if i < len(n.args) {
			return c.evaluate(n.args[i], false)
		}
		return nil
	}
	plan := &ActionPlan{}
	switch n.name {
	case "Action":
		list, _ := arg(0).([]any)
		for _, item := range list {
			if nested, ok := item.(*ActionPlan); ok {
				plan.Steps = append(plan.Steps, nested.Steps...)
			}
		}
	case "ToAssistant":
		plan.Steps = []ActionStep{{Type: "continue_conversation", Message: stringOrEmpty(arg(0)), Context: orNull(arg(1))}}
	case "OpenUrl":
		plan.Steps = []ActionStep{{Type: "open_url", URL: stringOrEmpty(arg(0))}}
	case "Run":
		if len(n.args) > 0 && n.args[0].k == kRuntimeRef {
			plan.Steps = []ActionStep{{Type: "run", StatementID: n.args[0].name, RefType: n.args[0].refType}}
		}
	case "Set":
		if len(n.args) >= 2 && n.args[0].k == kStateRef {
			plan.Steps = []ActionStep{{Type: "set", Target: n.args[0].name, value: n.args[1]}}
		}
	case "Reset":
		var targets []string
		for _, a := range n.args {
			if a.k == kStateRef {
				targets = append(targets, a.name)
			}
		}
		if len(targets) > 0 {
			plan.Steps = []ActionStep{{Type: "reset", Targets: targets}}
		}
	}
	return plan
}

func substituteRef(n *node, name string, value *node) *node {
	return substituteShared(n, name, value, map[*node]*node{})
}

func substituteShared(n *node, name string, value *node, done map[*node]*node) (result *node) {
	if n == nil {
		return nil
	}
	if previous, ok := done[n]; ok {
		return previous
	}
	defer func() { done[n] = result }()
	out := *n
	sub := func(child *node) *node { return substituteShared(child, name, value, done) }
	subAll := func(list []*node) []*node {
		next := make([]*node, len(list))
		for i, item := range list {
			next[i] = sub(item)
		}
		return next
	}
	subEntries := func(list []objEntry) []objEntry {
		next := make([]objEntry, len(list))
		for i, entry := range list {
			next[i] = objEntry{entry.key, sub(entry.value)}
		}
		return next
	}
	switch n.k {
	case kRef:
		if n.name == name {
			return value
		}
		return n
	case kMember:
		object := sub(n.a)
		if object.k == kObj {
			if at := entryIndex(object.entries, n.name); at >= 0 {
				return object.entries[at].value
			}
		}
		out.a = object
	case kIndex, kBinOp:
		out.a, out.b = sub(n.a), sub(n.b)
	case kUnaryOp, kAssign:
		out.a = sub(n.a)
	case kTernary:
		out.a, out.b, out.c = sub(n.a), sub(n.b), sub(n.c)
	case kArr:
		out.args = subAll(n.args)
	case kObj:
		out.entries = subEntries(n.entries)
	case kComp:
		if n.name == "Each" && len(n.args) >= 3 && (n.args[1].k == kRef && n.args[1].name == name || n.args[1].k == kStr && n.args[1].str == name) {
			out.args = append([]*node{sub(n.args[0])}, n.args[1:]...)
			return &out
		}
		out.args = subAll(n.args)
		if n.hasMap {
			out.mapped = subEntries(n.mapped)
		}
	default:
		return n
	}
	return &out
}

func plainData(value any) bool {
	switch v := value.(type) {
	case nil, string, float64, bool:
		return true
	case []any:
		for _, item := range v {
			if !plainData(item) {
				return false
			}
		}
		return true
	case *Object:
		for _, key := range v.Keys {
			if !plainData(v.Values[key]) {
				return false
			}
		}
		return true
	}
	return false
}

func (c *evalContext) each(args []*node, withSchema bool) any {
	if len(args) < 3 {
		return []any{}
	}
	list, ok := c.evaluate(args[0], false).([]any)
	if !ok {
		return []any{}
	}
	name := ""
	switch args[1].k {
	case kRef:
		name = args[1].name
	case kStr:
		name = args[1].str
	}
	if name == "" {
		return []any{}
	}
	out := make([]any, len(list))
	for i, item := range list {
		template := args[2]
		if plainData(item) {
			template = substituteRef(template, name, literalNode(item))
		}
		child := *c
		child.resolveRef = func(ref string) any {
			if ref == name {
				return item
			}
			return c.resolveRef(ref)
		}
		result := child.evaluate(template, withSchema)
		if el, ok := result.(*Element); ok && withSchema {
			result = child.element(el, true)
		}
		out[i] = result
	}
	return out
}

func (c *evalContext) element(el *Element, inline bool) *Element {
	if !el.dynamic {
		return el
	}
	out := &Element{Type: el.Type, StatementID: el.StatementID, Props: NewObject(), dynamic: true}
	ctx := c
	if !inline {
		scoped := *c
		ctx = &scoped
	}
	for _, key := range el.Props.Keys {
		if !inline {
			ctx.report = func(message string) {
				if c.errors != nil {
					*c.errors = append(*c.errors, Error{Source: "runtime", Code: "runtime-error", Component: el.Type, StatementID: el.StatementID,
						Message: fmt.Sprintf("Evaluating prop %q on %s failed: %s", key, el.Type, message)})
				}
			}
		}
		out.Props.Set(key, ctx.prop(el.Props.Values[key], c.lib.bindable[el.Type][key], inline))
	}
	return out
}

func (c *evalContext) prop(value any, reactive, inline bool) any {
	switch v := value.(type) {
	case *Binding:
		if reactive {
			return v
		}
		return c.state(v.Target)
	case *node:
		if v.k == kStateRef && reactive {
			return &Binding{Target: v.name, expr: &node{k: kStateRef, name: "$value"}}
		}
		switch result := c.evaluate(v, true).(type) {
		case *Element:
			return c.element(result, inline)
		case []any:
			for i, item := range result {
				if el, ok := item.(*Element); ok {
					result[i] = c.element(el, inline)
				}
			}
			return result
		case *Binding:
			if !reactive {
				return c.state(result.Target)
			}
			return result
		case undefinedValue:
			return nil
		default:
			return result
		}
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = c.prop(item, reactive, inline)
		}
		return out
	case *Element:
		return c.element(v, inline)
	case *Object:
		out := NewObject()
		for _, key := range v.Keys {
			out.Set(key, c.prop(v.Values[key], reactive, inline))
		}
		return out
	}
	return value
}

type StateReader func(name string) (any, bool)

const evaluationBudget = 2_000_000

func (p *Program) context(state StateReader, errors *[]Error) *evalContext {
	budget := evaluationBudget
	ctx := &evalContext{lib: p.lib, errors: errors, budget: &budget}
	ctx.getState = func(name string) any {
		if state != nil {
			if value, ok := state(name); ok {
				return value
			}
		}
		value, _ := p.State.Get(name)
		return value
	}
	ctx.resolveRef = func(name string) any {
		for _, q := range p.queries {
			if q.id == name {
				return ctx.evaluate(q.defaults, false)
			}
		}
		if p.mutations[name] {
			idle := NewObject()
			idle.Set("status", "idle")
			idle.Set("data", nil)
			idle.Set("error", nil)
			return idle
		}
		return nil
	}
	return ctx
}

func (p *Program) Evaluate(state StateReader) (root *Element, errors []Error) {
	defer func() {
		if recover() != nil {
			root = nil
			errors = append(errors, Error{Source: "runtime", Code: "runtime-error", StatementID: "root", Message: "evaluation failed"})
		}
	}()
	ctx := p.context(state, &errors)
	root = p.Root
	if root == nil && p.rootExpr != nil {
		root, _ = ctx.evaluate(p.rootExpr, true).(*Element)
		if root == nil {
			errors = append(errors, Error{Source: "runtime", Code: "no-root", StatementID: "root", Message: "root does not evaluate to a component"})
		}
	}
	if root == nil {
		return nil, errors
	}
	return ctx.element(root, false), errors
}

func (p *Program) EvaluateExpression(step ActionStep, state StateReader) (value any) {
	defer func() {
		if recover() != nil {
			value = nil
		}
	}()
	context := p.context(state, nil)
	context.acting = true
	return orNull(context.evaluate(step.value, false))
}

func resolveField(object any, path string) any {
	if path == "" || isNullish(object) {
		return nil
	}
	current := object
	for _, part := range strings.Split(path, ".") {
		if isNullish(current) {
			return nil
		}
		current = member(current, part)
	}
	return current
}

func elementwise(value any, fn func(float64) float64) any {
	if list, ok := value.([]any); ok {
		out := make([]any, len(list))
		for i, item := range list {
			out[i] = elementwise(item, fn)
		}
		return out
	}
	return fn(toNumber(value))
}

func sortsAsNumber(value any) bool {
	switch v := value.(type) {
	case float64:
		return true
	case string:
		return v != "" && !math.IsNaN(stringToNumber(v))
	}
	return false
}

var collator = collate.New(language.Und)

func builtinFunction(name string, args []any) any {
	arg := func(i int) any {
		if i < len(args) {
			return args[i]
		}
		return nil
	}
	list, isList := arg(0).([]any)
	switch name {
	case "Count":
		return float64(len(list))
	case "First":
		if len(list) > 0 {
			return orNull(list[0])
		}
		return nil
	case "Last":
		if len(list) > 0 {
			return orNull(list[len(list)-1])
		}
		return nil
	case "Sum", "Avg", "Min", "Max":
		if len(list) == 0 {
			return 0.0
		}
		total := toNumber(list[0])
		for _, item := range list[1:] {
			switch name {
			case "Min":
				total = math.Min(total, toNumber(item))
			case "Max":
				total = math.Max(total, toNumber(item))
			default:
				total += toNumber(item)
			}
		}
		if name == "Avg" {
			return total / float64(len(list))
		}
		return total
	case "Sort":
		if !isList {
			return arg(0)
		}
		field := stringOrEmpty(arg(1))
		desc := stringOrEmpty(arg(2)) == "desc"
		out := append([]any{}, list...)
		key := func(item any) any {
			if field == "" {
				return item
			}
			return resolveField(item, field)
		}
		sort.SliceStable(out, func(i, j int) bool {
			a, b := key(out[i]), key(out[j])
			var diff float64
			if sortsAsNumber(a) && sortsAsNumber(b) {
				diff = toNumber(a) - toNumber(b)
			} else {
				diff = float64(collator.CompareString(stringOrEmpty(a), stringOrEmpty(b)))
			}
			if desc {
				return diff > 0
			}
			return diff < 0
		})
		return out
	case "Filter":
		out := []any{}
		field := stringOrEmpty(arg(1))
		op := "=="
		if !isNullish(arg(2)) {
			op = toString(arg(2))
		}
		want := arg(3)
		for _, item := range list {
			value := item
			if field != "" {
				value = resolveField(item, field)
			}
			keep := false
			switch op {
			case "==":
				keep = looseEqual(value, want)
			case "!=":
				keep = !looseEqual(value, want)
			case ">":
				keep = toNumber(value) > toNumber(want)
			case "<":
				keep = toNumber(value) < toNumber(want)
			case ">=":
				keep = toNumber(value) >= toNumber(want)
			case "<=":
				keep = toNumber(value) <= toNumber(want)
			case "contains":
				keep = strings.Contains(stringOrEmpty(value), stringOrEmpty(want))
			}
			if keep {
				out = append(out, item)
			}
		}
		return out
	case "Round":
		factor := math.Pow(10, toNumber(arg(1)))
		return elementwise(arg(0), func(n float64) float64 { return math.Floor(n*factor+0.5) / factor })
	case "Abs":
		return elementwise(arg(0), math.Abs)
	case "Floor":
		return elementwise(arg(0), math.Floor)
	case "Ceil":
		return elementwise(arg(0), math.Ceil)
	}
	return nil
}
