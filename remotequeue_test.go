package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
)

// controlledShells are two windows on the network: the laptop's, paired
// with the desktop, whose remote control is on, shows the desktop's page.
func controlledShells(t *testing.T) (desk *shell, dt *ui.Tester, lap *shell, lt *ui.Tester, deskID string) {
	t.Helper()
	n := remotetest.New()
	desk = newRemoteShell(t, n, "Desktop")
	lap = newRemoteShell(t, n, "Laptop")
	dt, lt = tester(t, desk), tester(t, lap)
	if err := desk.a.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	deskID = found(t, lap, "Desktop").ID
	show(lap, lt, remotePath(deskID))
	lap.a.Remote().Pair(deskID)
	var code app.RemoteCode
	untilBoth(t, desk, dt, lap, lt, "the code", func() bool {
		c, ok := desk.a.Remote().Code()
		code = c
		return ok && lap.a.Remote().View(deskID).Step == app.RemoteCodeAsked
	})
	lap.a.Remote().EnterCode(deskID, code.Code)
	untilBoth(t, desk, dt, lap, lt, "the session", func() bool {
		v := lap.a.Remote().View(deskID)
		return v.Step == app.RemoteConnected && v.HasState
	})
	return desk, dt, lap, lt, deskID
}

// queueOf is the titles of the tracks of a player's queue.
func queueOf(st player.State) []string {
	var out []string
	for _, it := range st.Queue {
		out = append(out, it.Track.Title)
	}
	return out
}

// TestRemoteQueueRows plays a track of the other computer's queue from
// its page, takes one out by its menu, moves one by dragging it onto
// another, and clears the queue there.
func TestRemoteQueueRows(t *testing.T) {
	desk, dt, lap, lt, _ := controlledShells(t)
	desk.p.PlayTracks(testTracks, 0)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	untilBoth(t, desk, dt, lap, lt, "the queue there", func() bool {
		_, clear := lt.Find("Clear the queue on Desktop")
		return clear && lt.HasText("Glósóli") && lt.HasText("Sigur Rós")
	})
	if lt.HasText("is older") {
		t.Error("the page takes the desktop for an older MixMusic")
	}
	order := func(want ...string) func(player.State) bool {
		return func(st player.State) bool { return slices.Equal(queueOf(st), want) }
	}

	click(t, lap, lt, "Play Strobe on Desktop")
	desk.settle(t, func(st player.State) bool { return st.Index == 2 && st.Playing })
	untilBoth(t, desk, dt, lap, lt, "Strobe there", func() bool { return !lt.HasText("Play Strobe on Desktop") })

	if err := lt.RightClick("Windowlicker"); err != nil {
		t.Fatal(err)
	}
	if got, want := lt.Menu(), []string{"Play", "Remove from the queue"}; !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if err := lt.ChooseMenuItem("Remove from the queue"); err != nil {
		t.Fatal(err)
	}
	lap.drain()
	desk.settle(t, order("Midnight City", "Teardrop", "Strobe", "Archangel", "Glósóli"))
	untilBoth(t, desk, dt, lap, lt, "Windowlicker gone there", func() bool { return !lt.HasText("Windowlicker") })

	// Glósóli dragged onto Archangel takes its place.
	from, ok := lt.Find("Glósóli")
	to, ok2 := lt.Find("Archangel")
	if !ok || !ok2 {
		t.Fatalf("the rows: %q", lt.Texts())
	}
	lt.Press(from.X+from.W/2, from.Y+from.H/2)
	for i := 1; i <= 8; i++ {
		lt.Move(from.X+from.W/2, from.Y+from.H/2+(to.Y-from.Y)*float32(i)/8)
		lt.Frame()
	}
	lt.Release(to.X+to.W/2, to.Y+to.H/2)
	lap.drain()
	lt.Frame()
	desk.settle(t, order("Midnight City", "Teardrop", "Strobe", "Glósóli", "Archangel"))

	click(t, lap, lt, "Clear the queue on Desktop")
	desk.settle(t, order("Midnight City", "Teardrop", "Strobe"))
	untilBoth(t, desk, dt, lap, lt, "nothing next there", func() bool {
		_, clear := lt.Find("Clear the queue on Desktop")
		return !clear && !lt.HasText("Glósóli")
	})
}

// TestRemoteHandOverPage moves what plays here to the other computer from
// its page, which then plays it there; this computer pauses, and tells
// so.
func TestRemoteHandOverPage(t *testing.T) {
	desk, dt, lap, lt, _ := controlledShells(t)
	desk.p.PlayTracks(testTracks[:1], 0)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	mine := []media.Track{
		testTrack("yt:aaaaaaaaaaa", "Paper Boats", "Harbour Lights", "Tides", media.SourceYouTube),
		testTrack("yt:bbbbbbbbbbb", "Low Tide", "Kelp", "Shore", media.SourceYouTube),
	}
	untilBoth(t, desk, dt, lap, lt, "what plays there", func() bool { return lt.HasText("Midnight City") })
	if lt.HasText("On this computer") {
		t.Error("the page offers to move what plays here while nothing does")
	}
	lap.p.PlayTracks(mine, 1)
	lap.settle(t, func(st player.State) bool { return st.Playing })
	untilBoth(t, desk, dt, lap, lt, "what plays here", func() bool {
		return lt.HasText("On this computer") && lt.HasText("Low Tide · Kelp")
	})
	click(t, lap, lt, "move to Desktop")
	desk.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.Title == "Low Tide" && st.Playing && slices.Equal(queueOf(st), []string{"Paper Boats", "Low Tide"})
	})
	lap.settle(t, func(st player.State) bool { return !st.Playing })
	untilBoth(t, desk, dt, lap, lt, "the note", func() bool { return lt.HasText("Moved to Desktop, and paused here") })
	// The page shows it playing there.
	untilBoth(t, desk, dt, lap, lt, "Low Tide there", func() bool { return lt.HasText("Kelp · Shore") })
}

// TestHandedOverNote tells how moving what plays went.
func TestHandedOverNote(t *testing.T) {
	for got, want := range map[app.HandedOver]string{
		{Kept: 3}:                             "Moved to Desk, and paused here",
		{Kept: 2, Left: 1}:                    "Moved to Desk, and paused here, but for 1 track, files of this computer it doesn't have",
		{Kept: 2, Left: 2, CurrentLeft: true}: "Moved to Desk, and paused here, but for 2 tracks, files of this computer it doesn't have: it plays from the next one",
		{Left: 2, CurrentLeft: true}:          "Desk has none of these tracks, files of this computer: they play on here",
	} {
		if note := handedOverNote("Desk", got); note != want {
			t.Errorf("%+v: %q, want %q", got, note, want)
		}
	}
}
