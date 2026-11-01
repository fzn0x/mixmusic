package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// eventually waits until ok, which the library's writes beside the main
// thread make true.
func eventually(t *testing.T, s *shell, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen", what)
		}
	}
	s.drain()
}

// TestDislike dislikes what plays from the player bar, which moves on at
// once and says so in a toast whose undo goes back to it, and a track of
// the queue from its menu, which the queue then shows it passes over.
func TestDislike(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	first := testTracks[0]
	click(t, s, tt, "Dislike")
	s.settle(t, func(st player.State) bool { return st.Index == 1 && st.Playing })
	eventually(t, s, "the dislike", func() bool { return s.a.IsDisliked(first.ID) })
	tt.Frame()
	if !tt.HasText("Disliked " + first.Title) {
		t.Fatalf("no toast: %q", tt.Texts())
	}
	click(t, s, tt, "undo")
	s.settle(t, func(st player.State) bool { return st.Index == 0 && st.Playing })
	eventually(t, s, "the dislike taken back", func() bool { return !s.a.IsDisliked(first.ID) })
	if q := s.p.State().Queue; q[0].Skip != "" {
		t.Errorf("after undo, %s is passed over: %q", first.Title, q[0].Skip)
	}
	tt.Frame()

	// A track of the queue, from its menu.
	aphex := testTracks[3]
	if err := tt.RightClick(aphex.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Dislike"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	eventually(t, s, "the dislike of "+aphex.Title, func() bool { return s.a.IsDisliked(aphex.ID) })
	s.settle(t, func(st player.State) bool { return st.Queue[3].Skip == player.SkipDisliked })
	tt.Frame()
	if !tt.HasText("Aphex Twin · skipped") {
		t.Errorf("the queue doesn't show %s is passed over: %q", aphex.Title, tt.Texts())
	}
	if err := tt.RightClick(aphex.Title); err != nil {
		t.Fatal(err)
	}
	if menu := tt.Menu(); !slices.Contains(menu, "Remove the dislike") || slices.Contains(menu, "Dislike") {
		t.Errorf("the menu of a disliked track holds %q", menu)
	}
	tt.CloseMenu()
	// The toasts, over the transport, go.
	tt.Advance(15 * time.Second)
	tt.Frame()
	// The player bar's next line names what really plays next.
	click(t, s, tt, "Next")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	click(t, s, tt, "Next")
	s.settle(t, func(st player.State) bool { return st.Index == 2 })
	s.panel = "" // which shows what plays next itself
	tt.Frame()
	if !tt.HasText("next · " + testTracks[4].Title + " — Burial") {
		t.Errorf("the next line doesn't pass over %s: %q", aphex.Title, tt.Texts())
	}
	click(t, s, tt, "Next")
	s.settle(t, func(st player.State) bool { return st.Index == 4 })
}

// TestDislikeCommand dislikes what plays, and takes the dislike back, as
// Ctrl+D (⌘D) does, and a like takes a dislike back from the player too.
func TestDislikeCommand(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks[:2], 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	run := func(id string) {
		t.Helper()
		s.st = s.p.State()
		for _, cmd := range s.commands() {
			if cmd.id == id {
				cmd.run()
				return
			}
		}
		t.Fatalf("no command %s", id)
	}
	for _, cmd := range s.commands() {
		if cmd.id == "player.dislike" && (cmd.title != "Dislike what plays" || cmd.shortcut != "Mod+D") {
			t.Errorf("the command is %q, on %q", cmd.title, cmd.shortcut)
		}
	}
	run("player.dislike")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	eventually(t, s, "the dislike", func() bool { return s.a.IsDisliked(testTracks[0].ID) })
	// The first track is passed over, until a like takes the dislike back.
	s.p.PlayAt(1)
	s.settle(t, func(st player.State) bool { return st.Queue[0].Skip != "" })
	s.setLiked(testTracks[0], true)
	eventually(t, s, "the like", func() bool { return s.a.IsLiked(testTracks[0].ID) && !s.a.IsDisliked(testTracks[0].ID) })
	s.settle(t, func(st player.State) bool { return st.Queue[0].Skip == "" })
	tt.Frame()
}

// TestDislikeMini dislikes what plays from the mini player, beside its
// heart.
func TestDislikeMini(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	click(t, s, tt, "Dislike")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	eventually(t, s, "the dislike", func() bool { return s.a.IsDisliked(testTracks[0].ID) })
	tt.Frame()
	if !tt.HasText(testTracks[1].Title) {
		t.Errorf("the mini player doesn't show what plays now: %q", tt.Texts())
	}
	// Narrow, the artwork gives the buttons their room.
	narrow := ui.NewTester(s.miniView, 340, 112)
	narrow.Frame()
	like, okLike := narrow.Find("Like")
	dislike, okDislike := narrow.Find("Dislike")
	if !okLike || !okDislike || dislike.X+dislike.W > 340 || dislike.X < like.X+like.W {
		t.Errorf("narrow, the heart is at %+v (%v), the thumb at %+v (%v)", like, okLike, dislike, okDislike)
	}
}

// TestDislikedPivot shows the library's disliked pivot once the user
// dislikes a song, with its count, until they take the last dislike back
// from its menu.
func TestDislikedPivot(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/library/disliked")
	if tt.HasText("disliked") {
		t.Fatalf("the library shows a disliked pivot without dislikes: %q", tt.Texts())
	}
	strobe := testTracks[2]
	if err := s.svc.Collection.SetDisliked(context.Background(), strobe, true); err != nil {
		t.Fatal(err)
	}
	show(s, tt, "/library/disliked")
	texts := tt.Texts()
	counted := false
	for i := range len(texts) - 1 {
		counted = counted || texts[i] == "disliked" && texts[i+1] == "1"
	}
	if !counted {
		t.Errorf("no disliked pivot of one song: %q", texts)
	}
	if !tt.HasText(strobe.Title) {
		t.Fatalf("the pivot doesn't list %s: %q", strobe.Title, texts)
	}
	if _, ok := tt.Find("Remove the dislike"); !ok {
		t.Errorf("the disliked song has no thumb: %q", texts)
	}
	if err := tt.RightClick(strobe.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Remove the dislike"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	eventually(t, s, "the dislike taken back", func() bool { return !s.a.IsDisliked(strobe.ID) })
	show(s, tt, "/library/disliked")
	if tt.HasText("disliked") {
		t.Errorf("the pivot stayed once nothing was disliked: %q", tt.Texts())
	}
}

// TestSkipSwitch turns skipping disliked songs off and on in Settings ›
// Playback, which says where dislikes go, and links to them.
func TestSkipSwitch(t *testing.T) {
	s := newTestShell(t)
	if err := s.svc.Collection.SetDisliked(context.Background(), testTracks[0], true); err != nil {
		t.Fatal(err)
	}
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	for _, want := range []string{"Skip songs you dislike", "Songs you dislike don't play when the queue moves on, and radios leave them out.",
		"See the songs you dislike"} {
		if !tt.HasText(want) {
			t.Errorf("Settings › Playback doesn't show %q: %q", want, tt.Texts())
		}
	}
	if !s.svc.Preferences.Get().SkipDisliked {
		t.Fatal("skipping is off at first")
	}
	// The row's title carries the switch's words too: the keyboard reaches
	// the switch, which has them, and the description doesn't.
	const label = "Skip songs you dislike"
	focused := func() bool {
		return tt.Focused(label) && !tt.Focused("Songs you dislike don't play when the queue moves on, and radios leave them out.")
	}
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	for _, want := range []bool{false, true} {
		tt.Key(0, ui.KeySpace)
		s.drain()
		tt.Frame()
		if got := s.svc.Preferences.Get().SkipDisliked; got != want {
			t.Errorf("the switch left skipping %v, not %v", got, want)
		}
	}
}

// TestPlayButtonSkips starts an album's play button at its first song the
// user doesn't dislike, and its shuffle at one of those.
func TestPlayButtonSkips(t *testing.T) {
	s := newTestShell(t)
	album := media.Album{ID: "al-1", Source: media.SourceYouTube, Title: "Mix", Tracks: testTracks[:3]}
	s.loads["album:al-1"] = &loaded[media.Album]{done: true, value: album}
	for _, tr := range testTracks[:2] {
		if err := s.svc.Collection.SetDisliked(context.Background(), tr, true); err != nil {
			t.Fatal(err)
		}
	}
	tt := tester(t, s)
	show(s, tt, "/album/al-1")
	click(t, s, tt, "play")
	s.settle(t, func(st player.State) bool { return st.Index == 2 && st.Playing })
	click(t, s, tt, "shuffle")
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && st.Shuffle && st.Index == 0 && cur.Track.ID == testTracks[2].ID && st.Playing
	})
	// A song the user picks plays, disliked or not.
	s.p.PlayTracks(testTracks[:3], 0)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == testTracks[0].ID && st.Playing
	})
}

// TestUndoNotSkipping takes a dislike back without going back to the song,
// which played on while the settings don't skip disliked songs.
func TestUndoNotSkipping(t *testing.T) {
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.SkipDisliked = false
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	s.p.SettingsChanged(st)
	s.p.PlayTracks(testTracks[:3], 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	click(t, s, tt, "Dislike")
	eventually(t, s, "the dislike", func() bool { return s.a.IsDisliked(testTracks[0].ID) })
	if st := s.p.State(); st.Index != 0 {
		t.Fatalf("not skipping, the dislike moved to %d", st.Index)
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 && st.Playing })
	tt.Frame()
	click(t, s, tt, "undo")
	eventually(t, s, "the dislike taken back", func() bool { return !s.a.IsDisliked(testTracks[0].ID) })
	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	if st := s.p.State(); st.Index != 1 {
		t.Errorf("undo went back to %d", st.Index)
	}
}
