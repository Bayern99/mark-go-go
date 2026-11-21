package openuilang

type nodeKind int

const (
	kComp nodeKind = iota
	kStr
	kNum
	kBool
	kNull
	kArr
	kObj
	kRef
	kPh
	kStateRef
	kRuntimeRef
	kBinOp
	kUnaryOp
	kTernary
	kMember
	kIndex
	kAssign
)

type objEntry struct {
	key   string
	value *node
}

type node struct {
	k       nodeKind
	name    string
	str     string
	num     float64
	boolean bool
	args    []*node
	entries []objEntry
	mapped  []objEntry
	hasMap  bool
	fn      bool
	refType string
	a, b, c *node
}

func isRuntimeExpr(n *node) bool {
	switch n.k {
	case kStateRef, kRuntimeRef, kBinOp, kUnaryOp, kTernary, kMember, kIndex, kAssign:
		return true
	}
	return false
}

func walkAST(root *node, visit func(*node)) {
	seen := map[*node]bool{}
	var walk func(*node)
	walk = func(n *node) {
		if n == nil || seen[n] {
			return
		}
		seen[n] = true
		visit(n)
		for _, child := range n.args {
			walk(child)
		}
		for _, entry := range n.mapped {
			walk(entry.value)
		}
		for _, entry := range n.entries {
			walk(entry.value)
		}
		walk(n.a)
		walk(n.b)
		walk(n.c)
	}
	walk(root)
}

func literalNode(value any) *node {
	switch v := value.(type) {
	case string:
		return &node{k: kStr, str: v}
	case float64:
		return &node{k: kNum, num: v}
	case bool:
		return &node{k: kBool, boolean: v}
	case []any:
		out := &node{k: kArr, args: []*node{}}
		for _, item := range v {
			out.args = append(out.args, literalNode(item))
		}
		return out
	case *Object:
		out := &node{k: kObj}
		for _, key := range v.Keys {
			out.entries = append(out.entries, objEntry{key, literalNode(v.Values[key])})
		}
		return out
	}
	return &node{k: kNull}
}

func literalValue(n *node) (any, bool) {
	switch n.k {
	case kStr:
		return n.str, true
	case kNum:
		return n.num, true
	case kBool:
		return n.boolean, true
	case kNull:
		return nil, true
	case kArr:
		out := []any{}
		for _, item := range n.args {
			value, ok := literalValue(item)
			if !ok {
				return nil, false
			}
			out = append(out, value)
		}
		return out, true
	case kObj:
		out := NewObject()
		for _, entry := range n.entries {
			value, ok := literalValue(entry.value)
			if !ok {
				return nil, false
			}
			out.Set(entry.key, value)
		}
		return out, true
	}
	return nil, false
}
