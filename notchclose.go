package main

import "time"

// notchAway is how long the pointer stays away from the open panel around
// the notch before it closes.
const notchAway = 2 * time.Second

// countdown closes the open panel around the notch once the pointer stayed
// away from it for notchAway. Each frame asks it again, so a wakeup of a
// countdown called off, or of a panel since closed and opened again,
// closes nothing.
type countdown struct{ at time.Time }

// hold calls the countdown off: the pointer is over the panel, or a drag
// or the keyboard's focus keeps it open.
func (d *countdown) hold() { d.at = time.Time{} }

// run starts the countdown at now unless it runs, and reports whether it
// ran out, or else how long it has left.
func (d *countdown) run(now time.Time) (out bool, left time.Duration) {
	if d.at.IsZero() {
		d.at = now.Add(notchAway)
	}
	if left = d.at.Sub(now); left <= 0 {
		d.at = time.Time{}
		return true, 0
	}
	return false, left
}

// keyHandBack tells when the panel around the notch gives the keyboard
// back to the app the user was in. Not while a press the window got is
// down: giving it back orders the window out and in again, which cancels
// MyGo's click, so a click held a little on the compact player wouldn't
// open it. A frame can come between a press and its release, as when the
// player moves on.
type keyHandBack struct {
	presses int
	pending bool
}

// press counts a press the window got.
func (k *keyHandBack) press() { k.presses++ }

// release counts the end of a press the window got, and reports whether
// the keyboard goes back now: the last press is up, and a hand-back waited
// for it.
func (k *keyHandBack) release() bool {
	if k.presses > 0 {
		k.presses--
	}
	if k.presses == 0 && k.pending {
		k.pending = false
		return true
	}
	return false
}

// lost forgets the presses that are down, as a press outside the window
// tells that their releases went elsewhere, and reports whether a
// hand-back waited for them, which goes now.
func (k *keyHandBack) lost() bool {
	k.presses = 0
	pending := k.pending
	k.pending = false
	return pending
}

// ask asks for the keyboard to go back, and reports whether it goes now;
// else it goes once the presses are up.
func (k *keyHandBack) ask() bool {
	if k.presses > 0 {
		k.pending = true
		return false
	}
	k.pending = false
	return true
}

// keep calls off a hand-back that waits, as the panel opens again and
// takes the keyboard.
func (k *keyHandBack) keep() { k.pending = false }
