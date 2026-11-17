package main

import (
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleRoomActivity shows the page of a room with activity in
// en-XA: each line of it is one message, which names the guest inside.
func TestPseudoLocaleRoomActivity(t *testing.T) {
	usePseudoLocale(t)
	n := roomtest.New()
	desk := newRoomShell(t, n, "Desk")
	lap := newRoomShell(t, n, "Lap")
	dt, lt := tester(t, desk), tester(t, lap)
	dt.SetLocale("en-XA")
	lt.SetLocale("en-XA")
	if err := desk.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	show(desk, dt, roomsPath)
	joinByLink(t, desk, dt, lap, lt)
	joined := locales.In("en-XA").RoomPageActivityLine(roomActivityWhat(room.Joined), "Lap", "", time.Now())
	start, _, _ := strings.Cut(joined, " · ")
	untilBoth(t, desk, dt, lap, lt, "the activity", func() bool { return hasTextStarting(dt.Texts(), start) })
	checkPseudo(t, "The room's page", dt.Texts(), "Desk", "Lap")
}

// TestRoomActivityLines names each thing the room's activity tells, the
// song a guest added with it, and the time in the language's way.
func TestRoomActivityLines(t *testing.T) {
	at := time.Date(2026, 11, 17, 15, 4, 0, 0, time.Local)
	for what, want := range map[string]string{
		room.Joined: "Lap joined", room.Left: "Lap left", room.Removed: "Lap removed", room.Blocked: "Lap blocked",
		room.Guessed: "Lap tried too many wrong codes", room.Played: "Lap played", room.Paused: "Lap paused",
		room.Skipped: "Lap skipped to the next track", room.WentBack: "Lap went back a track",
		room.Added: "Lap added Teardrop", room.AskedToAdd: "Lap asked to add Teardrop", "something new": "Lap",
	} {
		got := locales.In("en").RoomPageActivityLine(roomActivityWhat(what), "Lap", "Teardrop", at)
		if want += " · 3:04 PM"; strings.ReplaceAll(got, " ", " ") != want {
			t.Errorf("%q says %q, want %q", what, got, want)
		}
	}
}
