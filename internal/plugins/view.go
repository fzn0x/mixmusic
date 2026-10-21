package plugins

import (
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/grafana/sobek"
)

// Node is an element of what a plugin shows: the plain objects of
// api.ui's text, button and the like, as the native interface draws them.
type Node struct {
	// Kind is "text", "heading", "button", "link", "tiles", "tile", "row",
	// "column", "canvas", "image", "spacer", "toggle" or "input".
	Kind string
	// Text is a text's, a heading's, a button's or a link's label, a
	// tile's title, a toggle's label or an input's placeholder.
	Text string
	// Caption is a tile's.
	Caption string
	// Tone is a text's: "muted", "accent", "danger" or ""; and a tile's
	// colour, 1 to 4.
	Tone string
	Size string // a text's: "small", "body", "subtitle" or "title"
	// Primary is set for a filled button; On for a toggle on, and Round
	// for a round image.
	Primary, On, Round bool
	// Path is where a link goes; URL an image's address; Value an input's
	// text.
	Path, URL, Value string
	// Width and Height size a canvas, an image or a spacer, in DIPs; Gap
	// spaces a row's or a column's children.
	Width, Height, Gap float32
	// Animate draws a canvas at every frame.
	Animate bool
	// Handler is the callback of a click, a toggle or an input, for
	// View.Click and the like; 0 for none.
	Handler int
	// Canvas is the index of a canvas among its view's.
	Canvas   int
	Children []Node
}

// View is what a plugin shows in a page or a widget: its render function
// fills it, and the window shows the nodes it set last. It renders while
// a window shows it: Mount and Unmount say when.
type View struct {
	pl     *plugin
	render jsFunc

	mu       sync.Mutex
	nodes    []Node
	canvases []*Canvas

	// Of the loop.
	mounted  bool
	cleanup  jsFunc
	handlers map[int]jsFunc
	next     int
	object   *sobek.Object
}

func (pl *plugin) newView(render jsFunc) *View {
	v := &View{pl: pl, render: render}
	pl.views = append(pl.views, v)
	return v
}

// Nodes returns what the view shows.
func (v *View) Nodes() []Node {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.nodes
}

// Mount renders the view, once a window shows it.
func (v *View) Mount() {
	v.pl.loop.post(func() {
		if v.mounted || v.pl.stopping {
			return
		}
		v.mounted = true
		result, err := v.render(sobek.Undefined(), v.jsObject())
		if err != nil {
			v.failed(jsError(err))
			return
		}
		if fn, ok := sobek.AssertFunction(result); ok {
			v.cleanup = fn
		}
	})
}

// Unmount ends the view's render, once no window shows it.
func (v *View) Unmount() { v.pl.loop.post(v.unmount) }

func (v *View) unmount() {
	if !v.mounted {
		return
	}
	v.mounted = false
	if v.cleanup != nil {
		v.pl.call(v.cleanup)
		v.cleanup = nil
	}
	v.handlers = nil
	v.mu.Lock()
	v.nodes, v.canvases = nil, nil
	v.mu.Unlock()
}

// failed shows why the view's render failed.
func (v *View) failed(err error) {
	v.mu.Lock()
	v.nodes = []Node{{Kind: "text", Text: "This failed: " + err.Error(), Tone: "danger"}}
	v.mu.Unlock()
	v.pl.h.changed()
}

// Click runs the handler of a click on a button, a link or a tile.
func (v *View) Click(handler int) {
	v.pl.loop.post(func() { v.pl.call(v.handlers[handler]) })
}

// Toggle runs the handler of a toggle the user turned on or off.
func (v *View) Toggle(handler int, on bool) {
	v.pl.loop.post(func() { v.pl.call(v.handlers[handler], v.pl.vm.ToValue(on)) })
}

// Submit runs the handler of an input whose text the user submitted.
func (v *View) Submit(handler int, text string) {
	v.pl.loop.post(func() { v.pl.call(v.handlers[handler], v.pl.vm.ToValue(text)) })
}

// Canvas returns the state of a canvas of the view, nil once it's gone.
func (v *View) Canvas(i int) *Canvas {
	v.mu.Lock()
	defer v.mu.Unlock()
	if i < 0 || i >= len(v.canvases) {
		return nil
	}
	return v.canvases[i]
}

// jsObject is the view as the render function gets it: an object of set,
// which replaces what it shows, and clear.
func (v *View) jsObject() *sobek.Object {
	if v.object != nil {
		return v.object
	}
	vm := v.pl.vm
	o := vm.NewObject()
	o.Set("set", func(call sobek.FunctionCall) sobek.Value {
		v.set(call.Arguments)
		return sobek.Undefined()
	})
	o.Set("clear", func() { v.set(nil) })
	v.object = o
	return o
}

// set replaces what the view shows with the elements given.
func (v *View) set(elements []sobek.Value) {
	if !v.mounted {
		return
	}
	c := converter{v: v, vm: v.pl.vm, handlers: map[int]jsFunc{}}
	nodes := c.nodes(elements)
	v.handlers = c.handlers
	v.mu.Lock()
	v.nodes = nodes
	for i, draw := range c.draws {
		if i < len(v.canvases) {
			v.canvases[i].setDraw(draw)
		} else {
			v.canvases = append(v.canvases, &Canvas{v: v, draw: draw})
		}
	}
	v.canvases = v.canvases[:len(c.draws)]
	v.mu.Unlock()
	v.pl.h.changed()
}

// converter turns the elements a plugin gives into nodes, keeping their
// callbacks.
type converter struct {
	v        *View
	vm       *sobek.Runtime
	handlers map[int]jsFunc
	draws    []jsFunc
}

func (c *converter) nodes(values []sobek.Value) []Node {
	var out []Node
	for _, val := range values {
		out = append(out, c.node(val)...)
	}
	return out
}

func (c *converter) node(val sobek.Value) []Node {
	if val == nil || sobek.IsUndefined(val) || sobek.IsNull(val) {
		return nil
	}
	if b, ok := val.Export().(bool); ok && !b {
		return nil
	}
	obj, ok := val.(*sobek.Object)
	if !ok {
		return []Node{{Kind: "text", Text: val.String()}}
	}
	if obj.ClassName() == "Array" {
		var out []Node
		for i := range obj.Get("length").ToInteger() {
			out = append(out, c.node(obj.Get(strconv.FormatInt(i, 10)))...)
		}
		return out
	}
	str := func(name string) string {
		if v := obj.Get(name); v != nil && !sobek.IsUndefined(v) && !sobek.IsNull(v) {
			return v.String()
		}
		return ""
	}
	num := func(name string) float32 {
		if v := obj.Get(name); v != nil && !sobek.IsUndefined(v) {
			if f := v.ToFloat(); !math.IsNaN(f) && !math.IsInf(f, 0) {
				return float32(max(f, 0))
			}
		}
		return 0
	}
	flag := func(name string) bool {
		v := obj.Get(name)
		return v != nil && v.ToBoolean()
	}
	handler := func(name string) int {
		fn, ok := sobek.AssertFunction(obj.Get(name))
		if !ok {
			return 0
		}
		c.v.next++
		c.handlers[c.v.next] = fn
		return c.v.next
	}
	children := func() []Node {
		v := obj.Get("children")
		if v == nil || sobek.IsUndefined(v) {
			return nil
		}
		return c.node(v)
	}
	n := Node{Kind: str("type")}
	switch n.Kind {
	case "text":
		n.Text, n.Tone, n.Size = str("text"), str("tone"), str("size")
	case "heading":
		n.Text = str("text")
	case "button":
		n.Text, n.Primary, n.Handler = str("label"), flag("primary"), handler("onClick")
	case "link":
		n.Text, n.Path, n.Handler = str("label"), str("path"), handler("onClick")
	case "tiles":
		n.Children = children()
	case "tile":
		n.Text, n.Caption, n.Tone, n.Handler = str("title"), str("caption"), str("tone"), handler("onClick")
	case "row", "column":
		n.Children, n.Gap = children(), num("gap")
	case "canvas":
		draw, ok := sobek.AssertFunction(obj.Get("draw"))
		if !ok {
			panic(c.vm.NewTypeError("a canvas needs a draw function"))
		}
		n.Width, n.Height, n.Animate, n.Canvas = num("width"), num("height"), flag("animate"), len(c.draws)
		if n.Height == 0 {
			n.Height = 240
		}
		c.draws = append(c.draws, draw)
	case "image":
		n.URL, n.Width, n.Round = str("url"), num("size"), flag("round")
	case "spacer":
		n.Height = num("size")
	case "toggle":
		n.Text, n.On, n.Handler = str("label"), flag("on"), handler("onChange")
	case "input":
		n.Text, n.Value, n.Handler = str("placeholder"), str("value"), handler("onSubmit")
	default:
		panic(c.vm.NewTypeError(fmt.Sprintf("MixMusic shows no element of type %q", n.Kind)))
	}
	return []Node{n}
}
