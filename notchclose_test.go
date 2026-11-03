package main

import (
	"testing"
	"time"
)

// TestCountdown closes the panel once the pointer stayed away for two
// seconds, and not after a hold, which starts it over.
func TestCountdown(t *testing.T) {
	var d countdown
	start := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	if out, left := d.run(start); out || left != notchAway {
		t.Fatalf("a countdown that starts: out %v, %v left", out, left)
	}
	if out, left := d.run(start.Add(1500 * time.Millisecond)); out || left != 500*time.Millisecond {
		t.Errorf("halfway on: out %v, %v left", out, left)
	}
	d.hold()
	if out, _ := d.run(start.Add(2500 * time.Millisecond)); out {
		t.Error("a countdown held, and started again, ran out at once")
	}
	if out, _ := d.run(start.Add(4499 * time.Millisecond)); out {
		t.Error("the countdown ran out early")
	}
	if out, _ := d.run(start.Add(4500 * time.Millisecond)); !out {
		t.Error("the countdown didn't run out")
	}
	if out, left := d.run(start.Add(4600 * time.Millisecond)); out || left != notchAway {
		t.Errorf("after running out, the countdown starts over: out %v, %v left", out, left)
	}
}

// TestKeyHandBack gives the keyboard back at once with no press down, and
// else once the last press the window got is up, as a hand-back then would
// cancel the click.
func TestKeyHandBack(t *testing.T) {
	var k keyHandBack
	if !k.ask() {
		t.Error("with no press down, the keyboard doesn't go back at once")
	}
	if k.release() {
		t.Error("a release with no hand-back waiting gives the keyboard back")
	}

	k.press()
	if k.ask() {
		t.Error("the keyboard goes back while a press is down")
	}
	k.press()
	if k.release() {
		t.Error("the keyboard goes back while another press is down")
	}
	if !k.release() {
		t.Error("the keyboard doesn't go back once the presses are up")
	}
	if k.release() {
		t.Error("the keyboard goes back twice")
	}

	k.press()
	k.ask()
	k.keep()
	if k.release() {
		t.Error("a hand-back called off, as the panel opened again, went on")
	}

	k.press()
	k.ask()
	if !k.lost() {
		t.Error("a hand-back waiting for a release that went elsewhere doesn't go")
	}
	if !k.ask() {
		t.Error("after the presses were lost, the keyboard doesn't go back at once")
	}
}
