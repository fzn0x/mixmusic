package main

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

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
