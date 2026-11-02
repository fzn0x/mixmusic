package youtube

import (
	"strconv"
	"strings"
)

// node is a decoded JSON value of an InnerTube response. Its methods walk
// the deeply nested renderers without a type assertion at every level:
// a path that does not exist yields the zero node, whose values are empty.
type node struct{ v any }

// get follows path, made of object keys (string) and array indexes (int).
func (n node) get(path ...any) node {
	cur := n.v
	for _, p := range path {
		switch p := p.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return node{}
			}
			cur = m[p]
		case int:
			a, ok := cur.([]any)
			if !ok {
				return node{}
			}
			if p < 0 {
				p += len(a)
			}
			if p < 0 || p >= len(a) {
				return node{}
			}
			cur = a[p]
		default:
			panic("youtube: bad path element")
		}
	}
	return node{cur}
}

// maxFindDepth bounds how deep find looks, in objects and arrays.
const maxFindDepth = 32

// find returns a value under key anywhere in the node, looking through its
// objects and arrays depth first, or the zero node: for what YouTube nests
// at a depth it changes now and then, as an endpoint inside a dialog's
// button inside a menu. Of several, which it returns is not fixed.
func (n node) find(key string) node { return findIn(n.v, key, maxFindDepth) }

func findIn(v any, key string, depth int) node {
	if depth <= 0 {
		return node{}
	}
	switch v := v.(type) {
	case map[string]any:
		if x, ok := v[key]; ok && x != nil {
			return node{x}
		}
		for _, x := range v {
			if f := findIn(x, key, depth-1); f.exists() {
				return f
			}
		}
	case []any:
		for _, x := range v {
			if f := findIn(x, key, depth-1); f.exists() {
				return f
			}
		}
	}
	return node{}
}

// exists reports whether the node holds a value.
func (n node) exists() bool { return n.v != nil }

// str returns the node as a string, or "".
func (n node) str() string {
	switch v := n.v.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// boolean returns the node as a bool, or false.
func (n node) boolean() bool {
	b, _ := n.v.(bool)
	return b
}

// list returns the elements of an array node.
func (n node) list() []node {
	a, _ := n.v.([]any)
	out := make([]node, len(a))
	for i, v := range a {
		out[i] = node{v}
	}
	return out
}

// renderer returns the name and the value of a node holding one renderer,
// such as {"musicShelfRenderer": {...}}.
func (n node) renderer() (string, node) {
	m, _ := n.v.(map[string]any)
	for k, v := range m {
		if strings.HasSuffix(k, "Renderer") || strings.HasSuffix(k, "Model") {
			return k, node{v}
		}
	}
	return "", node{}
}

// runs returns the runs of a text node: {"runs": [{"text": ...}, ...]}.
func (n node) runs() []node { return n.get("runs").list() }

// text joins the runs of a text node, or returns its simpleText.
func (n node) text() string {
	if s := n.get("simpleText").str(); s != "" {
		return s
	}
	var b strings.Builder
	for _, r := range n.runs() {
		b.WriteString(r.get("text").str())
	}
	return b.String()
}
