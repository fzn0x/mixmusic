package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// controlledShellsOf is controlledShells of a desktop whose app's options
// set changes.
func controlledShellsOf(t *testing.T, set func(o *app.Options)) (desk *shell, dt *ui.Tester, lap *shell, lt *ui.Tester, deskID string) {
	t.Helper()
	n := remotetest.New()
	desk = newRemoteShellOf(t, n, "Desktop", set)
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

// deskOutputs gives the desktop two sound outputs.
func deskOutputs(o *app.Options) {
	o.OutputDevices = func() ([]app.OutputDevice, error) {
		return []app.OutputDevice{{ID: "sp", Name: "Speakers", Default: true}, {ID: "hp", Name: "Headphones"}}, nil
	}
}

// TestRemoteModesPage shuffles, repeats, mutes and likes on the other
// computer from its page, which shows each as it is there, and chooses
// where it plays among its sound outputs.
func TestRemoteModesPage(t *testing.T) {
	desk, dt, lap, lt, _ := controlledShellsOf(t, deskOutputs)
	desk.p.PlayTracks(testTracks, 0)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	untilBoth(t, desk, dt, lap, lt, "the controls there", func() bool {
		_, shuffle := lt.Find("Shuffle")
		_, like := lt.Find("Like")
		_, mute := lt.Find("Mute")
		_, output := lt.Find("Output device: System default")
		return lt.HasText("Midnight City") && shuffle && like && mute && output
	})
	if lt.HasText("is older") {
		t.Error("the page takes the desktop for an older MixMusic")
	}

	click(t, lap, lt, "Shuffle")
	desk.settle(t, func(st player.State) bool { return st.Shuffle })
	click(t, lap, lt, "Repeat: off")
	desk.settle(t, func(st player.State) bool { return st.Repeat == player.RepeatAll })
	untilBoth(t, desk, dt, lap, lt, "repeat all there", func() bool { _, ok := lt.Find("Repeat: all"); return ok })
	click(t, lap, lt, "Repeat: all")
	desk.settle(t, func(st player.State) bool { return st.Repeat == player.RepeatOne })
	click(t, lap, lt, "Mute")
	desk.settle(t, func(st player.State) bool { return st.Muted })
	untilBoth(t, desk, dt, lap, lt, "muted there", func() bool { _, ok := lt.Find("Unmute"); return ok })
	click(t, lap, lt, "Like")
	untilBoth(t, desk, dt, lap, lt, "liked there", func() bool {
		_, ok := lt.Find("Remove from liked songs")
		return ok && desk.a.IsLiked(testTracks[0].ID)
	})

	if err := lt.Click("Output device: System default"); err != nil {
		t.Fatal(err)
	}
	if got, want := lt.Menu(), []string{"System default", "Speakers", "Headphones"}; !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if err := lt.ChooseMenuItem("Headphones"); err != nil {
		t.Fatal(err)
	}
	lap.drain()
	untilBoth(t, desk, dt, lap, lt, "the headphones chosen there", func() bool {
		_, ok := lt.Find("Output device: Headphones")
		return ok && desk.svc.Preferences.Get().OutputDevice == settings.OutputDevice{ID: "hp", Name: "Headphones"}
	})
}

// TestRemoteMoveHerePage brings what plays on the other computer here from
// its page: this one plays it, the other pauses, and both say so.
func TestRemoteMoveHerePage(t *testing.T) {
	desk, dt, lap, lt, _ := controlledShells(t)
	// The desktop's window tells what remote control says, as a window
	// that started does.
	t.Cleanup(desk.followRemote())
	untilBoth(t, desk, dt, lap, lt, "nothing there", func() bool { return lt.HasText("Nothing plays on Desktop.") })
	if _, ok := lt.Find("move here"); ok {
		t.Error("move here while nothing plays there")
	}
	desk.p.PlayTracks(bringable, 1)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	untilBoth(t, desk, dt, lap, lt, "move here", func() bool { _, ok := lt.Find("move here"); return ok })
	click(t, lap, lt, "move here")
	lap.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.Title == "Low Tide" && st.Playing && len(st.Queue) == 2
	})
	desk.settle(t, func(st player.State) bool { return !st.Playing })
	untilBoth(t, desk, dt, lap, lt, "the notes", func() bool {
		return lt.HasText("Moved here from Desktop, and paused there") && dt.HasText("Moved to Laptop, which asked for it, and paused here")
	})
}

// TestBroughtNote tells how bringing what plays there here went.
func TestBroughtNote(t *testing.T) {
	for got, want := range map[app.HandedOver]string{
		{Kept: 3}:                             "Moved here from Desk, and paused there",
		{Kept: 2, Left: 1}:                    "Moved here from Desk, and paused there, but for 1 track, files of Desk this computer doesn't have",
		{Kept: 2, Left: 2, CurrentLeft: true}: "Moved here from Desk, and paused there, but for 2 tracks, files of Desk this computer doesn't have: it plays from the next one",
		{Left: 2, CurrentLeft: true}:          "This computer has none of the tracks on Desk, files of that computer: they play on there",
	} {
		if note := broughtNote("Desk", got); note != want {
			t.Errorf("%+v: %q, want %q", got, note, want)
		}
	}
}

// TestPseudoLocaleRemoteModes shows the page's new controls, and what
// moving what plays there here tells, in en-XA.
func TestPseudoLocaleRemoteModes(t *testing.T) {
	usePseudoLocale(t)
	desk, dt, lap, lt, _ := controlledShellsOf(t, deskOutputs)
	t.Cleanup(desk.followRemote())
	dt.SetLocale("en-XA")
	lt.SetLocale("en-XA")
	desk.p.PlayTracks(bringable, 0)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	untilBoth(t, desk, dt, lap, lt, "move here", func() bool { _, ok := lt.Find(locales.RemoteMoveHere()); return ok })
	for _, label := range []string{locales.PlayerShuffle(), locales.PlayerRepeatOff(), locales.VolumeMute(), locales.PlayerLike(),
		locales.OutputChoiceButton(locales.OutputChoiceSystemDefault())} {
		if _, ok := lt.Find(label); !ok {
			t.Errorf("no %q in %q", label, lt.Texts())
		}
	}
	names := []string{"Desktop", "Laptop", "Paper Boats", "Harbour Lights", "Tides", "Low Tide", "Kelp", "Shore", "Speakers", "Headphones"}
	checkPseudo(t, "The page of the desktop", lt.Texts(), names...)
	click(t, lap, lt, locales.RemoteMoveHere())
	untilBoth(t, desk, dt, lap, lt, "the notes", func() bool {
		return lt.HasText(locales.RemoteBroughtPaused("Desktop")) && dt.HasText(locales.RemoteNoteBroughtAway("Laptop"))
	})
	checkPseudo(t, "The laptop, once it moved", lt.Texts(), names...)
	checkPseudo(t, "The desktop, once it moved", dt.Texts(), names...)
}

// bringable are tracks of YouTube Music, which another computer plays as
// its own.
var bringable = []media.Track{
	testTrack("yt:aaaaaaaaaaa", "Paper Boats", "Harbour Lights", "Tides", media.SourceYouTube),
	testTrack("yt:bbbbbbbbbbb", "Low Tide", "Kelp", "Shore", media.SourceYouTube),
}
