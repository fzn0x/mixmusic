package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/room"
)

// TestRoomProblemOfTheLimits tells a guest why the room didn't check its
// code: too many wrong ones, or too many at once.
func TestRoomProblemOfTheLimits(t *testing.T) {
	for err, want := range map[error]string{
		room.ErrWait: "Too many wrong codes: try again in a minute.",
		room.ErrBusy: "Desk's room checks other codes: try again in a moment.",
	} {
		if got := roomProblem("Desk", err); got != want {
			t.Errorf("%v: %q, want %q", err, got, want)
		}
	}
}
