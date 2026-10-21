package plugins

import (
	"math"
	"sync"

	"github.com/grafana/sobek"
)

// Canvas is a canvas a plugin draws on: its draw function records a frame
// on the plugin's loop, with a context of a part of the web's
// CanvasRenderingContext2D, and the window paints the frame it recorded
// last.
type Canvas struct {
	v *View

	mu      sync.Mutex
	frame   []Op
	pending bool

	// Of the loop.
	draw jsFunc
	rec  *recorder
}

func (c *Canvas) setDraw(draw jsFunc) { c.draw = draw }

// Frame returns the drawing the canvas recorded last.
func (c *Canvas) Frame() []Op {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.frame
}

// Request asks the canvas to draw at a size, in DIPs, unless a drawing
// waits already: a slow plugin draws as often as it can.
func (c *Canvas) Request(width, height float32) {
	c.mu.Lock()
	if c.pending {
		c.mu.Unlock()
		return
	}
	c.pending = true
	c.mu.Unlock()
	pl := c.v.pl
	if !pl.loop.post(func() { c.paint(width, height) }) {
		c.mu.Lock()
		c.pending = false
		c.mu.Unlock()
	}
}

func (c *Canvas) paint(width, height float32) {
	pl := c.v.pl
	var ops []Op
	if c.draw != nil && c.v.mounted && !pl.stopping {
		if c.rec == nil {
			c.rec = &recorder{}
			c.rec.object = c.rec.jsObject(pl.vm)
		}
		c.rec.reset(width, height)
		pl.call(c.draw, c.rec.object, pl.vm.ToValue(width), pl.vm.ToValue(height))
		ops = c.rec.ops
		c.rec.ops = nil
	}
	c.mu.Lock()
	c.frame, c.pending = ops, false
	c.mu.Unlock()
	pl.h.changed()
}

// Op is what a frame of a canvas draws: a path filled or stroked, in a
// CSS colour, at an opacity; its points are in DIPs from the canvas's top
// left corner.
type Op struct {
	Fill  bool
	Path  []Subpath
	Color string
	Alpha float32
	// Width is a stroke's.
	Width float32
}

// Subpath is a line through points, as x, y pairs, closed or not.
type Subpath struct {
	Points []float32
	Closed bool
}

// drawState is what save and restore keep.
type drawState struct {
	// m is the transform: x' = a x + c y + e, y' = b x + d y + f.
	m            [6]float64
	fill, stroke string
	width, alpha float64
	cap          string
}

// recorder records what a draw function draws, as the context it gets.
type recorder struct {
	object        *sobek.Object
	width, height float32
	ops           []Op
	st            drawState
	stack         []drawState
	path          []Subpath
	// x, y is the current point, and sx, sy the start of the subpath,
	// before the transform; at is set while there is a current point.
	x, y, sx, sy float64
	at           bool
}

func (r *recorder) reset(width, height float32) {
	r.width, r.height = width, height
	r.st = drawState{m: [6]float64{1, 0, 0, 1, 0, 0}, fill: "#000000", stroke: "#000000", width: 1, alpha: 1, cap: "butt"}
	r.stack, r.path, r.at = nil, nil, false
	if r.object != nil {
		canvas := r.object.Get("canvas").(*sobek.Object)
		canvas.Set("width", width)
		canvas.Set("height", height)
	}
}

func (r *recorder) point(x, y float64) (float32, float32) {
	m := r.st.m
	return float32(m[0]*x + m[2]*y + m[4]), float32(m[1]*x + m[3]*y + m[5])
}

func (r *recorder) moveTo(x, y float64) {
	px, py := r.point(x, y)
	r.path = append(r.path, Subpath{Points: []float32{px, py}})
	r.x, r.y, r.sx, r.sy, r.at = x, y, x, y, true
}

func (r *recorder) lineTo(x, y float64) {
	if !r.at {
		r.moveTo(x, y)
		return
	}
	px, py := r.point(x, y)
	sp := &r.path[len(r.path)-1]
	sp.Points = append(sp.Points, px, py)
	r.x, r.y = x, y
}

// arc adds an arc of a circle, as the context's arc does: a line to its
// start, then the arc, in steps short enough not to show.
func (r *recorder) arc(cx, cy, radius, start, end float64, ccw bool) {
	sweep := end - start
	switch {
	case !ccw && sweep >= 2*math.Pi, ccw && sweep <= -2*math.Pi:
		sweep = math.Copysign(2*math.Pi, sweep)
	case !ccw:
		sweep = math.Mod(sweep, 2*math.Pi)
		if sweep < 0 {
			sweep += 2 * math.Pi
		}
	default:
		sweep = math.Mod(sweep, 2*math.Pi)
		if sweep > 0 {
			sweep -= 2 * math.Pi
		}
	}
	steps := max(2, int(math.Ceil(math.Abs(sweep)/(math.Pi/32))))
	for i := 0; i <= steps; i++ {
		a := start + sweep*float64(i)/float64(steps)
		x, y := cx+radius*math.Cos(a), cy+radius*math.Sin(a)
		if i == 0 && !r.at {
			r.moveTo(x, y)
		} else {
			r.lineTo(x, y)
		}
	}
}

func (r *recorder) curve(points func(t float64) (float64, float64)) {
	if !r.at {
		r.moveTo(r.x, r.y)
	}
	const steps = 16
	for i := 1; i <= steps; i++ {
		r.lineTo(points(float64(i) / steps))
	}
}

func (r *recorder) emit(fill bool, path []Subpath) {
	if len(path) == 0 {
		return
	}
	op := Op{Fill: fill, Path: path, Alpha: float32(r.st.alpha)}
	if fill {
		op.Color = r.st.fill
	} else {
		op.Color = r.st.stroke
		m := r.st.m
		op.Width = float32(r.st.width * math.Sqrt(math.Abs(m[0]*m[3]-m[1]*m[2])))
	}
	r.ops = append(r.ops, op)
}

// rect is a closed subpath of a rectangle, transformed.
func (r *recorder) rect(x, y, w, h float64) Subpath {
	var pts []float32
	for _, p := range [][2]float64{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}} {
		px, py := r.point(p[0], p[1])
		pts = append(pts, px, py)
	}
	return Subpath{Points: pts, Closed: true}
}

// copyPath is the current path, which later calls don't change.
func (r *recorder) copyPath() []Subpath {
	out := make([]Subpath, len(r.path))
	for i, sp := range r.path {
		out[i] = Subpath{Points: append([]float32(nil), sp.Points...), Closed: sp.Closed}
	}
	return out
}

func (r *recorder) transform(a, b, c, d, e, f float64) {
	m := r.st.m
	r.st.m = [6]float64{
		m[0]*a + m[2]*b, m[1]*a + m[3]*b,
		m[0]*c + m[2]*d, m[1]*c + m[3]*d,
		m[0]*e + m[2]*f + m[4], m[1]*e + m[3]*f + m[5],
	}
}

// jsObject is the context draw functions get.
func (r *recorder) jsObject(vm *sobek.Runtime) *sobek.Object {
	o := vm.NewObject()
	canvas := vm.NewObject()
	o.Set("canvas", canvas)
	method := func(name string, fn func(args []float64)) {
		o.Set(name, func(call sobek.FunctionCall) sobek.Value {
			args := make([]float64, len(call.Arguments))
			for i, a := range call.Arguments {
				args[i] = a.ToFloat()
			}
			fn(append(args, 0, 0, 0, 0, 0, 0, 0, 0))
			return sobek.Undefined()
		})
	}
	property := func(name string, get func() any, set func(sobek.Value)) {
		o.DefineAccessorProperty(name,
			vm.ToValue(func(sobek.FunctionCall) sobek.Value { return vm.ToValue(get()) }),
			vm.ToValue(func(call sobek.FunctionCall) sobek.Value {
				set(call.Argument(0))
				return sobek.Undefined()
			}), sobek.FLAG_FALSE, sobek.FLAG_TRUE)
	}
	property("fillStyle", func() any { return r.st.fill }, func(v sobek.Value) { r.st.fill = v.String() })
	property("strokeStyle", func() any { return r.st.stroke }, func(v sobek.Value) { r.st.stroke = v.String() })
	property("lineWidth", func() any { return r.st.width }, func(v sobek.Value) {
		if w := v.ToFloat(); w > 0 && !math.IsInf(w, 0) {
			r.st.width = w
		}
	})
	property("globalAlpha", func() any { return r.st.alpha }, func(v sobek.Value) {
		if a := v.ToFloat(); a >= 0 && a <= 1 {
			r.st.alpha = a
		}
	})
	property("lineCap", func() any { return r.st.cap }, func(v sobek.Value) { r.st.cap = v.String() })

	method("save", func([]float64) { r.stack = append(r.stack, r.st) })
	method("restore", func([]float64) {
		if n := len(r.stack); n > 0 {
			r.st, r.stack = r.stack[n-1], r.stack[:n-1]
		}
	})
	method("translate", func(a []float64) { r.transform(1, 0, 0, 1, a[0], a[1]) })
	method("scale", func(a []float64) { r.transform(a[0], 0, 0, a[1], 0, 0) })
	method("rotate", func(a []float64) {
		s, c := math.Sincos(a[0])
		r.transform(c, s, -s, c, 0, 0)
	})
	method("transform", func(a []float64) { r.transform(a[0], a[1], a[2], a[3], a[4], a[5]) })
	method("setTransform", func(a []float64) { r.st.m = [6]float64{a[0], a[1], a[2], a[3], a[4], a[5]} })
	method("resetTransform", func([]float64) { r.st.m = [6]float64{1, 0, 0, 1, 0, 0} })
	method("beginPath", func([]float64) { r.path, r.at = nil, false })
	method("moveTo", func(a []float64) { r.moveTo(a[0], a[1]) })
	method("lineTo", func(a []float64) { r.lineTo(a[0], a[1]) })
	method("closePath", func([]float64) {
		if n := len(r.path); n > 0 && r.at {
			// The next subpath starts where the closed one did.
			r.path[n-1].Closed = true
			r.moveTo(r.sx, r.sy)
		}
	})
	method("arc", func(a []float64) { r.arc(a[0], a[1], a[2], a[3], a[4], a[5] != 0) })
	method("rect", func(a []float64) {
		r.path = append(r.path, r.rect(a[0], a[1], a[2], a[3]))
		r.at = false
	})
	method("quadraticCurveTo", func(a []float64) {
		x0, y0 := r.x, r.y
		r.curve(func(t float64) (float64, float64) {
			u := 1 - t
			return u*u*x0 + 2*u*t*a[0] + t*t*a[2], u*u*y0 + 2*u*t*a[1] + t*t*a[3]
		})
	})
	method("bezierCurveTo", func(a []float64) {
		x0, y0 := r.x, r.y
		r.curve(func(t float64) (float64, float64) {
			u := 1 - t
			return u*u*u*x0 + 3*u*u*t*a[0] + 3*u*t*t*a[2] + t*t*t*a[4], u*u*u*y0 + 3*u*u*t*a[1] + 3*u*t*t*a[3] + t*t*t*a[5]
		})
	})
	method("stroke", func([]float64) { r.emit(false, r.copyPath()) })
	method("fill", func([]float64) { r.emit(true, r.copyPath()) })
	method("fillRect", func(a []float64) { r.emit(true, []Subpath{r.rect(a[0], a[1], a[2], a[3])}) })
	method("strokeRect", func(a []float64) { r.emit(false, []Subpath{r.rect(a[0], a[1], a[2], a[3])}) })
	// Every frame starts clear, and clearing the whole canvas, as draw
	// functions do first, drops what was drawn before; clearing a part of
	// it, which only a bitmap could, does nothing.
	method("clearRect", func(a []float64) {
		if r.covers(a[0], a[1], a[2], a[3]) {
			r.ops = r.ops[:0]
		}
	})
	return o
}

// covers reports whether a rectangle, transformed, covers the canvas.
func (r *recorder) covers(x, y, w, h float64) bool {
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, p := range [][2]float64{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}} {
		px, py := r.point(p[0], p[1])
		minX, minY, maxX, maxY = min(minX, px), min(minY, py), max(maxX, px), max(maxY, py)
	}
	return minX <= 0 && minY <= 0 && maxX >= r.width && maxY >= r.height
}
