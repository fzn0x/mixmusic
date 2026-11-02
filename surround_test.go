package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
)

// TestSurroundButton shows the 5.1 button by the player only while the
// track that plays has 5.1 surround audio, and turns the setting on and
// off with it, as Settings › Playback's switch does.
func TestSurroundButton(t *testing.T) {
	surroundTracks.Store(testTracks[2].ID, true)
	t.Cleanup(func() { surroundTracks.Delete(testTracks[2].ID) })
	s := newTestShell(t)
	tt := tester(t, s)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool {
		return st.Playing && st.Source != nil && st.Source.Track.ID == testTracks[0].ID
	})
	tt.Frame()
	if tt.HasText("5.1") {
		t.Fatal("the 5.1 button shows for a track of no 5.1 audio")
	}
	s.p.PlayAt(2)
	s.settle(t, func(st player.State) bool { return st.Playing && hasSurround(st) })
	tt.Frame()
	if !tt.HasText("5.1") {
		t.Fatalf("no 5.1 button for a track of 5.1 audio: %q", tt.Texts())
	}
	if s.svc.Preferences.Get().Surround {
		t.Fatal("5.1 surround is on at first")
	}
	click(t, s, tt, "5.1 surround: off")
	if !s.svc.Preferences.Get().Surround {
		t.Fatal("the button didn't turn 5.1 surround on")
	}
	tt.Frame()
	click(t, s, tt, "5.1 surround: on")
	if s.svc.Preferences.Get().Surround {
		t.Fatal("the button didn't turn 5.1 surround off")
	}
}

// TestSurroundSetting turns 5.1 surround on in Settings › Playback, where
// it tells what it does.
func TestSurroundSetting(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	if !tt.HasText("Play 5.1 surround when a track has it") || !tt.HasText(surroundDescription) {
		t.Fatalf("no 5.1 surround row: %q", tt.Texts())
	}
	// The row's title shows its label too: the switch is to its right,
	// beside the title or halfway down the row.
	title, _ := tt.Find("Play 5.1 surround when a track has it")
	desc, _ := tt.Find(surroundDescription)
	for _, want := range []bool{true, false} {
		for x := title.X + title.W; x < 1280 && s.svc.Preferences.Get().Surround != want; x += 6 {
			for _, y := range []float32{title.Y + title.H/2, (title.Y + desc.Y + desc.H) / 2} {
				if s.svc.Preferences.Get().Surround != want {
					tt.ClickAt(x, y)
					s.drain()
					tt.Frame()
				}
			}
		}
		if got := s.svc.Preferences.Get().Surround; got != want {
			t.Errorf("the switch left 5.1 surround %v", got)
		}
	}
}
