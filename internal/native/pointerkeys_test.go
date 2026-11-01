package native

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

// pressSpace presses Space as a keyboard types it: the key goes down, its
// text comes, and the key goes up.
func pressSpace(s *shell, tt *ui.Tester) {
	tt.TypeKey(0, ui.KeySpace, " ")
	s.drain()
	tt.Frame()
}

// spacePlays presses Space, and fails the test unless the player goes
// from playing to paused, or back.
func spacePlays(t *testing.T, s *shell, tt *ui.Tester, after string) {
	t.Helper()
	was := s.p.State().Playing
	pressSpace(s, tt)
	for deadline := time.Now().Add(5 * time.Second); s.p.State().Playing == was; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("Space after %s didn't play or pause", after)
		}
		s.drain()
	}
	tt.Frame()
}

// spaceStays presses Space, and fails the test if the player goes from
// playing to paused, or back, in a while.
func spaceStays(t *testing.T, s *shell, tt *ui.Tester, after string) {
	t.Helper()
	was := s.p.State().Playing
	pressSpace(s, tt)
	for deadline := time.Now().Add(300 * time.Millisecond); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		s.drain()
		if s.p.State().Playing != was {
			t.Errorf("Space after %s played or paused", after)
			break
		}
	}
	tt.Frame()
}

// playing is a shell playing the test's tracks.
func playing(t *testing.T) *shell {
	t.Helper()
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	return s
}

// tabTo presses Tab until the element labelled label has the focus, and
// not only something holding it: a container holding other has it then.
func tabTo(t *testing.T, tt *ui.Tester, label, other string) {
	t.Helper()
	for range 150 {
		if tt.Focused(label) && (other == "" || !tt.Focused(other)) {
			return
		}
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	}
	t.Fatalf("Tab doesn't reach %q", label)
}

// TestPointerKeys leaves Space to play or pause after a click on MyGo's
// controls, and to the controls after Tab, and a text field takes it as
// typing.
func TestPointerKeys(t *testing.T) {
	s := playing(t)
	pressed, linked := 0, 0
	on, chosen, segment, field := false, "alpha", 0, ""
	view := func(c *ui.Context) {
		s.st = s.p.State()
		c.SetTheme(&s.lk.theme)
		s.runShortcuts(c)
		ui.Column(c).Gap(12).Padding(16).AlignItems(ui.Start).Children(func() {
			if s.buttonBase(c.Key("button")).Size(120, 32).Label("the button").Clicked() {
				pressed++
			}
			s.switchOf(c.Key("switch"), &on).Label("the switch")
			s.selectOf(c.Key("select"), &chosen, []string{"alpha", "beta", "gamma"}).Label("the select")
			s.segmented(c, "segments", "the segments", &segment, "one", "two", "three")
			if s.linkOf(c.Key("link"), "the link", "").Clicked() {
				linked++
			}
			ui.TextInput(c.Key("field"), &field).Label("the field").Width(200)
		})
	}
	tt := ui.NewTester(view, 640, 480)
	tt.Frame()

	click(t, s, tt, "the button")
	spacePlays(t, s, tt, "a click on a button")
	click(t, s, tt, "the switch")
	spacePlays(t, s, tt, "a click on a switch")
	click(t, s, tt, "the select")
	click(t, s, tt, "beta")
	spacePlays(t, s, tt, "choosing in a select")
	click(t, s, tt, "two")
	spacePlays(t, s, tt, "a click on a segment")
	click(t, s, tt, "the link")
	spacePlays(t, s, tt, "a click on a link")
	if pressed != 1 || !on || chosen != "beta" || tt.HasText("gamma") || segment != 1 || linked != 1 {
		t.Errorf("Space acted on what was clicked: button %d, switch %v, select %q (open %v), segment %d, link %d",
			pressed, on, chosen, tt.HasText("gamma"), segment, linked)
	}

	click(t, s, tt, "the field")
	spaceStays(t, s, tt, "a click in a text field")
	if field != " " {
		t.Errorf("Space typed %q in the text field", field)
	}

	// A focus from Tab keeps Space for what it reached.
	tabTo(t, tt, "the button", "")
	spaceStays(t, s, tt, "Tab to a button")
	tabTo(t, tt, "the switch", "")
	spaceStays(t, s, tt, "Tab to a switch")
	tabTo(t, tt, "the select", "")
	spaceStays(t, s, tt, "Tab to a select")
	if pressed != 2 || on || !tt.HasText("gamma") {
		t.Errorf("Space after Tab: button %d, switch %v, select open %v", pressed, on, tt.HasText("gamma"))
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// So does a focus whose ring Enter showed.
	click(t, s, tt, "the button")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	spaceStays(t, s, tt, "Enter on a clicked button")
	if pressed != 5 {
		t.Errorf("the button was pressed %d times, not 5", pressed)
	}

	// Play or pause on another key leaves Space to the controls, as MyGo
	// does; on Enter, it takes Enter after a click. A click hides the ring
	// as it moves the focus, from the field.
	s.set.Shortcuts = map[string]string{"player.toggle": "K"}
	tt.Frame()
	click(t, s, tt, "the field")
	click(t, s, tt, "the button")
	spaceStays(t, s, tt, "a click, with play or pause on K")
	if pressed != 7 {
		t.Errorf("Space doesn't press a clicked button with play or pause on K: pressed %d", pressed)
	}
	s.set.Shortcuts = map[string]string{"player.toggle": "Enter"}
	tt.Frame()
	click(t, s, tt, "the field")
	click(t, s, tt, "the button")
	was := s.p.State().Playing
	tt.Key(0, ui.KeyEnter)
	s.settle(t, func(st player.State) bool { return st.Playing != was })
	tt.Frame()
	if pressed != 8 {
		t.Errorf("Enter pressed a clicked button with play or pause on it: pressed %d", pressed)
	}

	// While a shortcut is being recorded, the keys are the shortcut's.
	s.set.Shortcuts = nil
	s.recording = "player.mute"
	tt.Frame()
	click(t, s, tt, "the field")
	click(t, s, tt, "the button")
	spaceStays(t, s, tt, "a click while recording")
	if pressed != 10 {
		t.Errorf("Space while recording didn't reach the button: pressed %d", pressed)
	}
}

// TestSpaceAfterClicks plays or pauses with Space after a click on the
// window's controls, unless a text field has the keyboard.
func TestSpaceAfterClicks(t *testing.T) {
	t.Run("jump back in", func(t *testing.T) {
		s := playing(t)
		tt := testerOf(t, s, 1600, 1000)
		click(t, s, tt, "Play "+testTracks[0].Title+" again")
		s.settle(t, func(st player.State) bool { return st.Playing })
		s.p.Seek(42 * time.Second)
		s.settle(t, func(st player.State) bool { return st.Position >= 42*time.Second })
		tt.Frame()
		spacePlays(t, s, tt, "a click on jump back in")
		if st := s.p.State(); st.Playing || st.Position < 42*time.Second {
			t.Errorf("Space played the tile again: playing %v at %v", st.Playing, st.Position)
		}
	})
	t.Run("pivot", func(t *testing.T) {
		s := playing(t)
		tt := testerOf(t, s, 1600, 1000)
		click(t, s, tt, "yours")
		spacePlays(t, s, tt, "a click on a pivot")
	})
	t.Run("back to a tile", func(t *testing.T) {
		s := playing(t)
		tt := testerOf(t, s, 1600, 1000)
		for _, back := range []func(){
			func() { click(t, s, tt, "Back") },
			func() { tt.Key(0, ui.KeyBack); tt.Frame() },
		} {
			click(t, s, tt, "Liked songs")
			tt.Advance(time.Second)
			back()
			tt.Advance(time.Second)
			spacePlays(t, s, tt, "going back to the page of a tile clicked")
			tt.Advance(time.Second)
			if got := s.router.Path(); got != "/" {
				t.Errorf("Space went to %q", got)
			}
		}
	})
	t.Run("theme card", func(t *testing.T) {
		s := playing(t)
		tt := testerOf(t, s, 1600, 1400)
		show(s, tt, "/settings/appearance")
		click(t, s, tt, "Aurora")
		spacePlays(t, s, tt, "a click on a theme")
	})
	t.Run("now playing", func(t *testing.T) {
		// The sidebar's tile shows in a window 720 tall and more; the
		// player bar's artwork, labelled as it, in any.
		for _, h := range []int{1000, 700} {
			s := playing(t)
			tt := testerOf(t, s, 1280, h)
			click(t, s, tt, "Open now playing")
			tt.Advance(time.Second)
			spacePlays(t, s, tt, "a click on what plays")
		}
	})
}

// TestSpaceAfterTab leaves Space to a tile Tab reached, which opens it.
func TestSpaceAfterTab(t *testing.T) {
	s := playing(t)
	tt := testerOf(t, s, 1600, 1000)
	tabTo(t, tt, "Liked songs", "jump back in")
	spaceStays(t, s, tt, "Tab to a tile")
	tt.Advance(time.Second)
	if got := s.router.Path(); got != "/library/liked" {
		t.Errorf("Space on the tile Tab reached went to %q", got)
	}
}

// TestMiniSpace leaves the mini player's buttons out of Space's way, as
// the mini player runs no shortcuts (#38).
func TestMiniSpace(t *testing.T) {
	s := playing(t)
	main := testerOf(t, s, 1280, 820)
	mini := ui.NewTester(s.miniView, 420, 132)
	mini.Frame()
	click(t, s, mini, "Next")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	main.Frame()
	mini.Frame()
	spaceStays(t, s, mini, "a click on the mini player's Next")
	if i := s.p.State().Index; i != 1 {
		t.Errorf("Space in the mini player went to track %d", i)
	}
}

// TestRecordSpace records Space as a shortcut after a click on change.
func TestRecordSpace(t *testing.T) {
	s := playing(t)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/shortcuts")
	click(t, s, tt, "change the shortcut of Mute")
	pressSpace(s, tt)
	if s.set.Shortcuts["player.mute"] != "Space" || s.set.Shortcuts["player.toggle"] != "" {
		t.Errorf("Space wasn't recorded: %v", s.set.Shortcuts)
	}
}
