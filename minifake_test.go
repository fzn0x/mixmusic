package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// fakeMini is the mini player's window in the tests: its content, in a
// screen whose work area is work, which the Tester shows at its size once
// what the frames asked of it ran.
type fakeMini struct {
	tt               *ui.Tester
	rect, work       mygo.Rectangle
	queue            []func()
	minW, minH, maxH int
	// moves are the places the window took, in turn.
	moves []mygo.Rectangle
	size  [2]int
	// dirty is set when a frame was asked for, which run draws.
	dirty bool
}

func (f *fakeMini) later(fn func())                        { f.queue = append(f.queue, fn) }
func (f *fakeMini) content() mygo.Rectangle                { return f.rect }
func (f *fakeMini) workArea(mygo.Rectangle) mygo.Rectangle { return f.work }
func (f *fakeMini) limit(minW, minH, maxH int)             { f.minW, f.minH, f.maxH = minW, minH, maxH }
func (f *fakeMini) place(r mygo.Rectangle)                 { f.rect = r; f.moves = append(f.moves, r) }
func (f *fakeMini) invalidate()                            { f.dirty = true }

// run runs what the frames asked of the window, as the main thread does
// after them, and gives the Tester the window's size.
func (f *fakeMini) run() {
	for len(f.queue) > 0 {
		q := f.queue
		f.queue = nil
		for _, fn := range q {
			fn()
		}
	}
	if size := [2]int{f.rect.Width, f.rect.Height}; size != f.size {
		f.size = size
		f.tt.SetSize(size[0], size[1])
	}
	if f.dirty {
		f.dirty = false
		f.tt.Frame()
	}
}

// settle draws frames 16 ms apart, the window doing between them what
// they asked, until it stopped moving.
func (f *fakeMini) settle(t *testing.T, s *shell) {
	t.Helper()
	s.drain()
	f.run()
	for i := 0; s.mini.move != nil || len(f.queue) > 0; i++ {
		if i > 200 {
			t.Fatalf("the mini player's window never stops moving: at %+v, going to %+v", f.rect, s.mini.move.to)
		}
		s.drain()
		f.tt.Advance(16 * time.Millisecond)
		f.run()
	}
	s.drain()
	f.tt.Frame()
	f.run()
}

// workArea is the work area of the screen of the tests: 1920 × 1040 DIPs,
// above a taskbar 40 high.
var workArea = mygo.Rectangle{Width: 1920, Height: 1040}

// miniAt is the mini player of s, its content w × h DIPs at x, y in a
// screen whose work area is workArea, once its first frame took in the
// panel the settings keep.
func miniAt(t *testing.T, s *shell, x, y, w, h int) (*ui.Tester, *fakeMini) {
	t.Helper()
	f := &fakeMini{rect: mygo.Rectangle{X: x, Y: y, Width: w, Height: h}, work: workArea, size: [2]int{w, h}}
	s.mini.win = f
	tt := ui.NewTester(s.miniView, w, h)
	f.tt = tt
	tt.Frame()
	f.settle(t, s)
	return tt, f
}
