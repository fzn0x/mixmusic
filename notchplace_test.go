package main

import (
	"testing"

	"github.com/egoist/mygo"
)

// TestHousingOf finds the camera housing of the built-in screen, in MyGo's
// coordinates, wherever the screen is among the displays, and none on
// external displays or screens without one.
func TestHousingOf(t *testing.T) {
	builtIn := mygo.Display{ID: 1, Internal: true, Bounds: mygo.Rectangle{X: -1512, Y: 120, Width: 1512, Height: 982}}
	external := mygo.Display{ID: 2, Primary: true, Bounds: mygo.Rectangle{Width: 2560, Height: 1440}}
	notched := notchScreen{id: 1, left: 662, right: 662, gap: 188, height: 32}
	for _, c := range []struct {
		name     string
		displays []mygo.Display
		screens  []notchScreen
		want     housing
		ok       bool
	}{
		{"built in, left of the primary", []mygo.Display{external, builtIn}, []notchScreen{{id: 2}, notched}, housing{X: -850, Y: 120, W: 188, H: 32}, true},
		{"built in alone", []mygo.Display{{ID: 1, Internal: true, Primary: true, Bounds: mygo.Rectangle{Width: 1512, Height: 982}}},
			[]notchScreen{notched}, housing{X: 662, Y: 0, W: 188, H: 32}, true},
		{"a housing of points between", []mygo.Display{{ID: 1, Internal: true, Bounds: mygo.Rectangle{Width: 1728, Height: 1117}}},
			[]notchScreen{{id: 1, left: 764.5, right: 764.5, gap: 199, height: 37.6}}, housing{X: 765, W: 199, H: 38}, true},
		{"lid closed", []mygo.Display{external}, []notchScreen{{id: 2}}, housing{}, false},
		{"no notch", []mygo.Display{{ID: 1, Internal: true, Bounds: mygo.Rectangle{Width: 1440, Height: 900}}}, []notchScreen{{id: 1}}, housing{}, false},
		{"an external display with areas", []mygo.Display{external}, []notchScreen{{id: 2, left: 1000, right: 1000, gap: 200, height: 32}}, housing{}, false},
		{"no screens told", []mygo.Display{builtIn}, nil, housing{}, false},
		{"another screen's areas", []mygo.Display{builtIn}, []notchScreen{{id: 3, left: 662, right: 662, gap: 188, height: 32}}, housing{}, false},
	} {
		got, ok := housingOf(c.displays, c.screens)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: %+v, %v; want %+v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestNotchSizes sizes the compact player and the open panel from the
// housing, centred on it, the panel about 480 × 190 around a housing 32
// points tall, and wider around a wider one.
func TestNotchSizes(t *testing.T) {
	small := housing{X: 662, W: 180, H: 32}
	if got, want := small.compact(), (mygo.Rectangle{X: 626, Width: 252, Height: 32}); got != want {
		t.Errorf("compact: %+v, not %+v", got, want)
	}
	if got, want := small.open(), (mygo.Rectangle{X: 512, Width: 480, Height: 190}); got != want {
		t.Errorf("open: %+v, not %+v", got, want)
	}
	wide := housing{X: 500, Y: 30, W: 401, H: 38}
	open := wide.open()
	if open.Width <= notchOpenWidth || open.Height != 38+notchBody || open.Y != 30 {
		t.Errorf("open around a wide housing: %+v", open)
	}
	for _, h := range []housing{small, wide} {
		for name, r := range map[string]mygo.Rectangle{"compact": h.compact(), "open": h.open()} {
			if left, right := h.X-r.X, r.X+r.Width-(h.X+h.W); left != right || left <= 0 {
				t.Errorf("%+v: %s %+v isn't centred on the housing: %d left of it, %d right", h, name, r, left, right)
			}
		}
		if room := (h.open().Width - h.W) / 2; room < h.wing()+notchOpenRoom {
			t.Errorf("%+v: the open panel leaves %d beside the housing, not room for its buttons", h, room)
		}
	}
}
