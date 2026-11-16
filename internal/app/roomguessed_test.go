package app

import (
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomGuessedTellsTheHost has the room tell its host that someone
// guesses its code, as it takes no code for a while: the windows hear it.
func TestRoomGuessedTellsTheHost(t *testing.T) {
	desk := newRoomApp(t, roomtest.New(), "Desk")
	var mu sync.Mutex
	var notes []string
	t.Cleanup(desk.OnRooms(func(note string) {
		mu.Lock()
		notes = append(notes, note)
		mu.Unlock()
	}))
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	desk.hostedRoom().server.Guessing()
	mu.Lock()
	defer mu.Unlock()
	for _, n := range notes {
		if n == roomGuessed {
			return
		}
	}
	t.Fatalf("the host was told %q", notes)
}
