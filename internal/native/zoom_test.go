package native

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// zoomed presses keys, and returns the zoom chosen and the zoom the window
// shows after the frames that follow.
func zoomed(s *shell, tt *ui.Tester, mods ui.Modifiers, key ui.Key) (chosen, shown float64) {
	tt.Key(mods, key)
	s.drain()
	tt.Frame()
	tt.Frame()
	return s.set.Zoom, s.zoom
}

// TestZoomKeys steps the zoom in and out with its keys, Mod+Shift+= too,
// and back to 100% with Mod+0, from the search box as well, and tells
// each change in a toast.
func TestZoomKeys(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	steps := []struct {
		mods ui.Modifiers
		key  ui.Key
		want float64
	}{
		{ui.Cmd, ui.KeyEqual, 1.1},
		{ui.Cmd | ui.Shift, ui.KeyEqual, 1.25},
		{ui.Cmd, ui.KeyMinus, 1.1},
		{ui.Cmd, ui.Key0, 1},
		{ui.Cmd, ui.KeyMinus, 0.9},
		{ui.Cmd, ui.KeyMinus, 0.8},
		{ui.Cmd, ui.KeyMinus, 0.8},
		{ui.Cmd, ui.Key0, 1},
	}
	for _, st := range steps {
		chosen, shown := zoomed(s, tt, st.mods, st.key)
		if chosen != st.want || math.Abs(shown-st.want) > 1e-6 {
			t.Fatalf("%v+%v: zoom %v, shown %v, want %v", st.mods, st.key, chosen, shown, st.want)
		}
		if want := "Zoom " + percent(st.want); !tt.HasText(want) {
			t.Errorf("no toast %q: %q", want, tt.Texts())
		}
	}
	if s.svc.Preferences.Get().Zoom != 1 {
		t.Errorf("settings.json keeps the zoom %v", s.svc.Preferences.Get().Zoom)
	}

	tt.Click("Search")
	if chosen, _ := zoomed(s, tt, ui.Cmd, ui.KeyEqual); chosen != 1.1 {
		t.Errorf("Mod+= in the search box: zoom %v, want 1.1", chosen)
	}
}

// TestZoomInDialogs leaves the zoom as it is while the command palette is
// open, as the other shortcuts do.
func TestZoomInDialogs(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	s.paletteOpen = true
	tt.Frame()
	if chosen, _ := zoomed(s, tt, ui.Cmd, ui.KeyEqual); chosen != 1 {
		t.Errorf("Mod+= with the palette open zoomed to %v", chosen)
	}
}

// TestZoomRoom zooms a window that has no room for the zoom chosen as far
// as it has room for, says so, and zooms out from what it shows.
func TestZoomRoom(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	s.setZoom(2)
	s.drain()
	tt.Frame()
	tt.Frame()
	// The window MixMusic first opens has room for about 140%.
	if got := percent(s.zoom); got != "139%" {
		t.Errorf("1280×820 at 200%% shows %s, want 139%%", got)
	}
	if !tt.HasText("This window has room for 139%.") {
		t.Errorf("the toast doesn't tell the room: %q", tt.Texts())
	}
	if chosen, shown := zoomed(s, tt, ui.Cmd, ui.KeyMinus); chosen != 1.25 || shown != 1.25 {
		t.Errorf("zooming out from 139%%: zoom %v, shown %v, want 1.25", chosen, shown)
	}

	// Below 100% nothing holds it back, and a window with room shows all.
	if shownZoom(0.8, 0.5) != 0.8 || shownZoom(1.5, 1.2) != 1.2 || shownZoom(1.5, 0.9) != 1 || shownZoom(2, 2.5) != 2 {
		t.Error("shownZoom")
	}
	// Spotify Connect's devices button and the plugins' pages need room.
	s.set.SpotifyPlayback = settings.SpotifyPlaybackConnect
	if r := s.room(1920, 1080); math.Abs(r-1920.0/1050) > 1e-6 {
		t.Errorf("room with the devices button %v", r)
	}
}

// TestZoomKeysGivenAway leaves a key the user gave another command with
// that command, and the zoom's command without it.
func TestZoomKeysGivenAway(t *testing.T) {
	s := newTestShell(t)
	s.set.Shortcuts = map[string]string{"player.mute": "Mod+0"}
	tt := tester(t, s)
	s.setZoom(1.25)
	s.drain()
	tt.Frame()
	if chosen, _ := zoomed(s, tt, ui.Cmd, ui.Key0); chosen != 1.25 {
		t.Errorf("Mod+0, given to Mute, reset the zoom to %v", chosen)
	}
	for _, cmd := range s.commands() {
		if cmd.id == "view.zoomReset" && s.shortcutOf(cmd) != "" {
			t.Errorf("Reset zoom keeps %q", s.shortcutOf(cmd))
		}
	}
	if keys := s.zoomKeys(); strings.Contains(keys, "reset") {
		t.Errorf("the setting names a key to reset: %q", keys)
	}
}

// TestZoomSetting chooses the zoom in Settings › Appearance, whose row
// names the keys.
func TestZoomSetting(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/appearance")
	keys := strings.ReplaceAll(s.zoomKeys(), wordJoiner, "")
	if keys != "Ctrl+= and Ctrl+- change it, Ctrl+0 resets it." && keys != "⌘= and ⌘- change it, ⌘0 resets it." {
		t.Errorf("the zoom's row names the keys %q", keys)
	}
	if !tt.HasText("Makes everything larger or smaller: text, buttons and artwork. " + s.zoomKeys()) {
		t.Errorf("the zoom's row doesn't name its keys: %q", tt.Texts())
	}
}

// TestMiniZoom zooms the mini player with the zoom's keys, its only
// shortcuts.
func TestMiniZoom(t *testing.T) {
	s := newTestShell(t)
	mini := ui.NewTester(s.miniView, miniWidth, miniHeight)
	mini.Frame()
	mini.Key(ui.Cmd, ui.KeyEqual)
	s.drain()
	mini.Frame()
	if s.set.Zoom != 1.1 || s.zoom != 1.1 {
		t.Errorf("Mod+= in the mini player: zoom %v, shown %v", s.set.Zoom, s.zoom)
	}
	mini.Key(ui.Cmd, ui.KeyS)
	s.drain()
	if s.p.State().Shuffle {
		t.Error("Mod+S shuffled in the mini player")
	}
}

// TestRenderZoom draws Settings › Appearance and the mini player zoomed
// to MIXMUSIC_RENDER, for a look.
func TestRenderZoom(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	tt := tester(t, s)
	show(s, tt, "/settings/appearance")
	for _, z := range []float64{0.8, 1, 1.5, 2} {
		s.setZoom(z)
		renderTo(t, s, tt, dir, "zoom-"+percent(z)[:len(percent(z))-1])
	}
	tall := testerOf(t, s, 1280, 2400)
	show(s, tall, "/settings/appearance")
	s.setZoom(1)
	renderTo(t, s, tall, dir, "zoom-setting")
	s.setZoom(1.5)
	mini := ui.NewTester(s.miniView, int(miniWidth*1.5), int(miniHeight*1.5))
	renderTo(t, s, mini, dir, "zoom-mini-150")
}
