package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/room"
)

// TestRoomProblemOfTheLimits tells a guest why the room didn't check its
// code: too many wrong ones from it, or from anyone, or too many at once;
// or why it can't join with a code it proved: the room has a new one.
func TestRoomProblemOfTheLimits(t *testing.T) {
	for err, want := range map[error]string{
		room.ErrWait:    "Too many wrong codes from this computer: try again in a minute.",
		room.ErrBusy:    "Desk's room checks other codes: try again in a moment.",
		room.ErrLocked:  "Desk's room isn't taking codes for a minute: ask Desk for the invite link.",
		room.ErrNewCode: "Desk's room has a new code: ask Desk for it, or for the new invite link.",
	} {
		if got := roomProblem("Desk", err); got != want {
			t.Errorf("%v: %q, want %q", err, got, want)
		}
	}
}
