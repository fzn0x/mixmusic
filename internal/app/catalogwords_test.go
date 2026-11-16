package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/locales"
)

// TestWordsOfTheCatalog says, in the pseudo locale, en-XA, which brackets
// every message of the catalogs, what a guest is told of a room, the name
// of a computer that told none, and a music video's timing notes: each
// comes from the catalogs, not the code, and keeps its name and numbers.
func TestWordsOfTheCatalog(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	xa := locales.In("en-XA")
	bracketed := func(s string) bool { return strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") }
	for _, err := range []error{room.ErrWrongCode, room.ErrWait, room.ErrLocked, room.ErrNewCode, room.ErrPreviewEnded,
		room.ErrBusy, room.ErrBlocked, room.ErrRemoved, room.ErrDeclined, room.ErrRoomMoves, room.ErrManyAsk, room.ErrFull,
		room.ErrNotGuest, room.ErrEnded, room.ErrVersion, room.ErrNotSame, errRoomAway, room.ErrFarAddr, room.ErrNotLink,
		room.ErrLost} {
		if got := roomProblem("Desk", err); !bracketed(got) {
			t.Errorf("%v: %q, which no catalog says", err, got)
		}
	}
	if got := roomProblem("Desk", room.ErrFull); !strings.Contains(got, "Desk") {
		t.Errorf("the room's host isn't named: %q", got)
	}
	if got := roomProblem("The room", room.ErrBlocked); got != xa.RoomProblemBlockedUnnamed() {
		t.Errorf("a room known by its address: %q", got)
	}
	if got := farProblem(""); got != xa.RoomProblemFarUnnamed() {
		t.Errorf("a host far and unnamed: %q", got)
	}
	if got, want := roomProblem(room.Unnamed, room.ErrFull), xa.RoomProblemFull(xa.RoomSomeone()); got != want {
		t.Errorf("a host that told no name: %q, want %q", got, want)
	}
	if got := roomProblem("Desk", errors.New("refused")); got != "refused" {
		t.Errorf("an error of no room's: %q, as Go says it", got)
	}
	if got := shownName(room.Unnamed); got != xa.RoomSomeone() {
		t.Errorf("shownName(%q) = %q", room.Unnamed, got)
	}
	if got := shownName("Desk"); got != "Desk" {
		t.Errorf("shownName(Desk) = %q", got)
	}
	for _, note := range []string{timingNote(), timedNote(2900*time.Millisecond, 1), timedNote(2900*time.Millisecond, 1.0026),
		timedNote(-1100*time.Millisecond, 0.997413)} {
		if !bracketed(note) {
			t.Errorf("the note %q, which no catalog says", note)
		}
	}
	if got := timedNote(-1100*time.Millisecond, 0.997413); !strings.Contains(got, "-1.1") || !strings.Contains(got, "0.26") {
		t.Errorf("the note of -1.1 s at 0.26%% faster: %q", got)
	}
}
