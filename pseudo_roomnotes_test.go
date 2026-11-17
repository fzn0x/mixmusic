package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleRoomProblem knocks at a room's address typed without
// its port in en-XA: the room's page tells why in the language.
func TestPseudoLocaleRoomProblem(t *testing.T) {
	usePseudoLocale(t)
	n := roomtest.New()
	lap := newRoomShell(t, n, "Lap")
	lt := tester(t, lap)
	lt.SetLocale("en-XA")
	lap.a.Rooms().KnockAt("203.0.113.7", "ABC234")
	show(lap, lt, roomsPath)
	until(t, lap, lt, "the problem", func() bool { return lt.HasText(locales.RoomProblemAddrNoPort()) })
	checkPseudo(t, "The room's page", lt.Texts(), "Lap")
}
