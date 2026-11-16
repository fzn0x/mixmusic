package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/room"
)

// TestRoomBlockedReads tells a computer its host blocked, of the host's
// name, or of a room known by its address alone, whose host's name the
// knock never learned.
func TestRoomBlockedReads(t *testing.T) {
	for name, want := range map[string]string{
		"Hosty":    "Hosty blocked this computer.",
		"The room": "This room blocked this computer.",
		"":         "This room blocked this computer.",
	} {
		if got := roomProblem(name, room.ErrBlocked); got != want {
			t.Errorf("of %q: %q, want %q", name, got, want)
		}
	}
}
