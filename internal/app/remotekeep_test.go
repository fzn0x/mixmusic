package app

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// The computer paired with keeps the pairing before it tells the other it
// is paired: these tests end a pairing as that keeping goes.

// TestRemoteNotKept tells a pairing the other computer couldn't keep in
// the interface's words.
func TestRemoteNotKept(t *testing.T) {
	if got := remoteProblem("Desk", remote.ErrNotKept); !strings.Contains(got, "Desk couldn't keep the pairing") {
		t.Errorf("the page says %q", got)
	}
}
