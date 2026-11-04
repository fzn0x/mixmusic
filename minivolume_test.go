package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestMiniVolume mutes and unmutes from the mini player's volume button,
// turns the volume with the wheel over it, and sets it with the slider
// that shows above it while the pointer is over either, which goes once
// the pointer left.
func TestMiniVolume(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.p.SetVolume(0.5)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Volume == 0.5 })
	mini := ui.NewTester(s.miniView, miniWidth, miniHeight)
	mini.Frame()
	if _, ok := mini.Find("Volume"); ok {
		t.Error("the mini player's volume slider shows before the pointer came")
	}
	click(t, s, mini, "Mute")
	s.settle(t, func(st player.State) bool { return st.Muted })
	mini.Frame()
	click(t, s, mini, "Unmute")
	s.settle(t, volumeNear(0.5))
	mini.Frame()

	b, ok := mini.Find("Mute")
	if !ok {
		t.Fatalf("no mute button in the mini player: %q", mini.Texts())
	}
	x, y := b.X+b.W/2, b.Y+b.H/2
	mini.Wheel(x, y, 0, -100)
	s.settle(t, volumeNear(0.55))
	mini.Wheel(x, y, 0, 100)
	mini.Wheel(x, y, 0, 100)
	s.settle(t, volumeNear(0.45))
	mini.Frame()

	// The pointer is over the button: the slider shows above it, and sets
	// the volume where it's pressed.
	r, ok := mini.Find("Volume")
	if !ok || r.Y+r.H > b.Y {
		t.Fatalf("hovering the volume button, the slider is at %+v (%v), the button at %+v", r, ok, b)
	}
	mini.Move(r.X+r.W/2, r.Y+r.H/2)
	mini.Frame()
	at := r.X + r.W*0.85
	mini.Press(at, r.Y+r.H/2)
	mini.Release(at, r.Y+r.H/2)
	s.settle(t, func(st player.State) bool { return st.Volume > 0.75 && st.Volume < 0.95 })
	mini.Frame()
	if _, ok := mini.Find("Volume"); !ok {
		t.Error("the slider went with the pointer over it")
	}

	// Away from both, it goes a moment after.
	mini.Move(4, 4)
	mini.Frame()
	if _, ok := mini.Find("Volume"); !ok {
		t.Error("the slider went at once as the pointer left it")
	}
	mini.Advance(time.Second)
	mini.Frame()
	if _, ok := mini.Find("Volume"); ok {
		t.Error("the slider stays once the pointer left it")
	}
}

// TestMiniVolumeKeys reaches the mini player's volume with Tab: the button
// shows the slider, the next Tab goes into it, where the arrows set the
// volume, and Escape closes it until the focus leaves the button.
func TestMiniVolumeKeys(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.p.SetVolume(0.5)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Volume == 0.5 })
	mini := ui.NewTester(s.miniView, miniWidth, miniHeight)
	mini.Frame()
	for range 10 {
		if mini.Focused("Mute") {
			break
		}
		mini.Key(0, ui.KeyTab)
	}
	if !mini.Focused("Mute") {
		t.Fatal("Tab doesn't reach the mini player's volume button")
	}
	mini.Key(0, ui.KeyTab)
	if !mini.Focused("Volume") {
		t.Fatal("Tab from the volume button doesn't go into its slider")
	}
	mini.Key(0, ui.KeyUp)
	s.settle(t, volumeNear(0.51))
	mini.Frame()

	mini.Key(0, ui.KeyEscape)
	mini.Frame()
	if _, ok := mini.Find("Volume"); ok {
		t.Error("Escape doesn't close the volume slider")
	}
	if !mini.Focused("Mute") {
		t.Error("Escape in the volume slider doesn't give the focus back to its button")
	}
	mini.Advance(time.Second)
	mini.Frame()
	if _, ok := mini.Find("Volume"); ok {
		t.Error("the slider closed with Escape came back while its button has the focus")
	}
	mini.Key(ui.Shift, ui.KeyTab)
	mini.Key(0, ui.KeyTab)
	if _, ok := mini.Find("Volume"); !ok {
		t.Error("the slider closed with Escape doesn't come back once the focus left its button and came back")
	}
}

// TestMiniVolumeFits keeps the mini player's controls, its volume button
// too, apart and in the window at its least size, at every zoom, with the
// transport's gap at least before the window's buttons, and its slider in
// the window above the button. At the size it opens at, the artwork still
// takes the window's height.
func TestMiniVolumeFits(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	defer func() { s.set.Zoom = 1 }()
	for _, z := range settings.ZoomSteps {
		s.set.Zoom = z
		w, h := int(miniMinWidth*z+0.5), int(miniMinHeight*z+0.5)
		mini := ui.NewTester(s.miniView, w, h)
		mini.Frame()
		inside := func(r ui.Rect) bool {
			return r.X >= 0 && r.Y >= 0 && r.X+r.W <= float32(w)+0.01 && r.Y+r.H <= float32(h)+0.01
		}
		var before ui.Rect
		for i, label := range []string{"Previous", "Pause", "Next", "Like", "Dislike", "Mute", "Open MixMusic"} {
			r, ok := mini.Find(label)
			if !ok || !inside(r) {
				t.Errorf("at %s, %s is at %+v (%v), out of the window of %dx%d", percent(z), label, r, ok, w, h)
			}
			gap := float32(0)
			if label == "Open MixMusic" {
				gap = 8 * float32(z)
			}
			if i > 0 && r.X < before.X+before.W+gap-0.01 {
				t.Errorf("at %s, %s at %+v runs into what's before it, at %+v", percent(z), label, r, before)
			}
			before = r
		}
		b, _ := mini.Find("Mute")
		mini.Move(b.X+b.W/2, b.Y+b.H/2)
		mini.Frame()
		if r, ok := mini.Find("Volume"); !ok || !inside(r) || r.Y+r.H > b.Y {
			t.Errorf("at %s, the volume slider is at %+v (%v), its button at %+v, in a window of %dx%d", percent(z), r, ok, b, w, h)
		}
	}
	s.set.Zoom = 1
	mini := ui.NewTester(s.miniView, miniWidth, miniHeight)
	mini.Frame()
	if r, _ := mini.Find("Previous"); r.X < miniHeight {
		t.Errorf("at %dx%d, the artwork gives up its room: the transport starts at %v", miniWidth, miniHeight, r.X)
	}
}
