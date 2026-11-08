package audio

import (
	"errors"
	"testing"
)

// TestReopenOutput starts the output that plays again, as when the user
// chooses another device, and the track plays on from the frame it was at,
// with the channels it had; an engine that plays nothing starts no output,
// and one whose output can't start again stops, as with no device.
func TestReopenOutput(t *testing.T) {
	want := reference(t, cuesFront)
	for _, five := range []bool{false, true} {
		e, dev, _ := newTestEngine(t)
		dev.surround = five
		e.SetSurround(five)
		e.ReopenOutput()
		if dev.starts != 0 {
			t.Fatalf("%d starts with no track", dev.starts)
		}
		if err := e.Load(cuesFront, Cue{}); err != nil {
			t.Fatal(err)
		}
		buffered(t, e, 0, ringFrames/2)
		if err := e.Play(); err != nil {
			t.Fatal(err)
		}
		dev.play(t, 4800)
		e.ReopenOutput()
		if dev.starts != 2 || dev.isStopped() {
			t.Fatalf("5.1 %v: %d starts, stopped %v", five, dev.starts, dev.isStopped())
		}
		wantChannels := 2
		if five {
			wantChannels = 6
		}
		if dev.asked != wantChannels || dev.channels != wantChannels {
			t.Errorf("5.1 %v: asked for %d channels, playing %d", five, dev.asked, dev.channels)
		}
		after := dev.play(t, 4800)
		ch := dev.channels
		for f := range 4800 {
			for c := range 2 {
				if got, w := after[ch*f+c], want[2*(4800+f)+c]; got != w {
					t.Fatalf("5.1 %v: after the output started again, frame %d, channel %d: %v, want %v", five, 4800+f, c, got, w)
				}
			}
		}
	}

	e, dev, rec := newTestEngine(t)
	if err := e.Load(cuesFront, Cue{}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	dev.mu.Lock()
	dev.fail = errors.New("the device is gone")
	dev.mu.Unlock()
	e.ReopenOutput()
	rec.waitFor(t, "error: "+ErrNoOutput.Error())
}
