package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/plugins"
)

// withCommunityPlugins installs the community's plugins in the shell's app
// and runs them, as the main window does. They run as the JavaScript their
// TypeScript compiles to, which `bun run build:plugins` writes: without
// it, the test skips, but fails in CI, which builds it first.
func withCommunityPlugins(t *testing.T, s *shell, ids ...string) {
	t.Helper()
	for _, id := range ids {
		src := filepath.Join("community", "plugins", id)
		if _, err := os.Stat(filepath.Join(src, "index.js")); err != nil {
			if os.Getenv("CI") != "" {
				t.Fatalf("the %s plugin isn't compiled: CI runs bun run build:plugins before the tests", id)
			}
			t.Skipf("the %s plugin isn't compiled: run bun run build:plugins", id)
		}
		dst := filepath.Join(s.svc.System.Info().DataDir, "addons", "plugins", id)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatal(err)
		}
		files, err := os.ReadDir(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			data, err := os.ReadFile(filepath.Join(src, f.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dst, f.Name()), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	st := s.svc.Preferences.Get()
	st.EnabledPlugins = ids
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.svc.Addons.Reload()
	s.refresh()
	s.plugins = plugins.New(s.a, s.p)
	s.plugins.SetUI(pluginUI{s})
	t.Cleanup(s.plugins.Close)
	s.restyle()
	s.syncPlugins()
}

// until draws frames until ok, failing the test after a few seconds.
func until(t *testing.T, s *shell, tt *ui.Tester, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		s.drain()
		tt.Frame()
		if ok() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s; the window shows %q", what, tt.Texts())
		}
	}
}

func TestSleepTimer(t *testing.T) {
	s := newTestShell(t)
	withCommunityPlugins(t, s, "sleep-timer")
	tt := tester(t, s)
	until(t, s, tt, "the sidebar item", func() bool { return tt.HasText("sleep timer") })
	click(t, s, tt, "sleep timer")
	tt.Advance(time.Second)
	until(t, s, tt, "the page", func() bool { return tt.HasText("The sleep timer is off.") })
	if got := s.router.Path(); got != "/plugin/sleep-timer/timer" {
		t.Errorf("the sidebar item went to %q", got)
	}
	click(t, s, tt, "30 min")
	until(t, s, tt, "the timer", func() bool {
		return tt.HasText("The sleep timer is stops in 30:00.") || tt.HasText("The sleep timer is stops in 29:59.")
	})
	// The page draws what the plugin set at once, but its toast waits for
	// the shell's update, which a frame can come before.
	until(t, s, tt, "the timer's toast", func() bool { return tt.HasText("Sleep timer: the music stops in 30 minutes") })
	click(t, s, tt, "cancel")
	until(t, s, tt, "the cancel", func() bool { return tt.HasText("The sleep timer is off.") })

	// Its commands are in the palette.
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	tt.Type("sleep timer: 45")
	tt.Frame()
	if !tt.HasText("Sleep timer: 45 minutes") {
		t.Errorf("the palette doesn't find the plugin's command: %q", tt.Texts())
	}
	// Enter searches what was typed; Down reaches the command.
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	until(t, s, tt, "the command", func() bool { return tt.HasText("Sleep timer: the music stops in 45 minutes") })

	// Leaving the page ends its render.
	show(s, tt, "/")
	until(t, s, tt, "the page to close", func() bool { return len(s.mountedViews) == 0 })
}

func TestSpectrum(t *testing.T) {
	s := newTestShell(t)
	withCommunityPlugins(t, s, "spectrum")
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	// The widget is under the lyrics, below the fold of a window of the
	// usual size, where nothing is drawn.
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/now-playing")
	until(t, s, tt, "the widget", func() bool { return tt.HasText("spectrum") && len(s.plugins.Widgets()) == 1 })
	v := s.plugins.Widgets()[0].View
	until(t, s, tt, "the canvas to draw", func() bool {
		cv := v.Canvas(0)
		return cv != nil && len(cv.Frame()) == 1
	})
	op := v.Canvas(0).Frame()[0]
	if op.Fill || len(op.Path) != 96 || op.Color != hexColor(s.lk.accent) {
		t.Errorf("the spectrum drew %d subpaths in %q, filled %v", len(op.Path), op.Color, op.Fill)
	}
}

// TestRenderPlugins draws the community plugins to MIXMUSIC_RENDER, for a
// look.
func TestRenderPlugins(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s := newTestShell(t)
	withCommunityPlugins(t, s, "sleep-timer", "spectrum")
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	show(s, tt, "/plugin/sleep-timer/timer")
	until(t, s, tt, "the page", func() bool { return tt.HasText("The sleep timer is off.") })
	renderTo(t, s, tt, dir, "plugin-sleep-timer")
	tall := testerOf(t, s, 1280, 1500)
	show(s, tall, "/now-playing")
	until(t, s, tall, "the spectrum", func() bool {
		ws := s.plugins.Widgets()
		return len(ws) == 1 && ws[0].View.Canvas(0) != nil && len(ws[0].View.Canvas(0).Frame()) == 1
	})
	renderTo(t, s, tall, dir, "plugin-spectrum")
}
