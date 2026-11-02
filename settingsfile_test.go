package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// opened tells what the tests asked of settings.json: "open" or "show".
var opened = make(chan string, 16)

// fileShell is the main window of an app whose settings.json is told of
// at once when it has a mistake.
func fileShell(t *testing.T) *shell {
	t.Helper()
	s := newTestShellOf(t, func(o *app.Options) { o.ProblemWait = -1 })
	s.main = true
	for {
		select {
		case <-opened:
			continue
		default:
		}
		return s
	}
}

// editSettings saves text to settings.json, as an editor does, and has the
// app read it, as its watch does. The window hears of it as it refreshes,
// which the app has it do once MixMusic runs.
func editSettings(t *testing.T, s *shell, text string) {
	t.Helper()
	if err := store.WriteFile(s.svc.Preferences.File().Path, []byte(text)); err != nil {
		t.Fatal(err)
	}
	s.a.ReloadSettings()
}

// brokenSettings saves text, which has a mistake, to settings.json, and
// waits for the window to be told of it.
func brokenSettings(t testing.TB, s *shell, text string) {
	t.Helper()
	if err := store.WriteFile(s.svc.Preferences.File().Path, []byte(text)); err != nil {
		t.Fatal(err)
	}
	s.a.ReloadSettings()
	for deadline := time.Now().Add(5 * time.Second); s.svc.Preferences.File().Problem == ""; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the mistake wasn't told")
		}
	}
	s.refresh()
}

// wasOpened waits for the window to ask what of settings.json.
func wasOpened(t *testing.T, want string) {
	t.Helper()
	select {
	case got := <-opened:
		if got != want {
			t.Errorf("asked to %s settings.json, want %s", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("settings.json wasn't asked to %s", want)
	}
}

// TestSettingsFileProblem tells of a mistake in settings.json, at the top
// of the settings and in a toast, until it is fixed.
func TestSettingsFileProblem(t *testing.T) {
	s := fileShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/appearance")
	brokenSettings(t, s, "{\n  \"theme\": \"aurora\",\n}")
	tt.Frame()
	for _, text := range []string{"settings.json has a mistake", "Line 3, column 1: a comma before } has nothing after it. MixMusic keeps the settings it had until you fix it."} {
		if !tt.HasText(text) {
			t.Errorf("no %q: %q", text, tt.Texts())
		}
	}
	if !tt.HasText("Line 3, column 1: a comma before } has nothing after it. MixMusic goes on with the settings it had, and saves what you change here once the file reads again.") {
		t.Errorf("no problem panel: %q", tt.Texts())
	}
	// The other pages of the settings show it too.
	show(s, tt, "/settings/desktop")
	if !tt.HasText("open settings.json") {
		t.Fatalf("no problem panel in Settings › Desktop: %q", tt.Texts())
	}
	click(t, s, tt, "open settings.json")
	wasOpened(t, "open")
	// Fixed, it applies.
	editSettings(t, s, `{"theme": "aurora"}`)
	s.refresh()
	tt.Frame()
	if tt.HasText("open settings.json") || !tt.HasText("Applied settings.json") {
		t.Errorf("after the fix: %q", tt.Texts())
	}
	if s.set.Theme != "aurora" {
		t.Errorf("the theme is %q", s.set.Theme)
	}
}

// TestSettingsFileApplied tells of an edit that applied, and of what of it
// MixMusic didn't take as it was.
func TestSettingsFileApplied(t *testing.T) {
	s := fileShell(t)
	tt := tester(t, s)
	editSettings(t, s, `{"theme": "aurora", "crosfade": 6, "fontScale": 3}`)
	s.refresh()
	tt.Frame()
	for _, text := range []string{"Applied settings.json", "MixMusic doesn't know crosfade: its next save drops it. Put fontScale back in range."} {
		if !tt.HasText(text) {
			t.Errorf("no %q: %q", text, tt.Texts())
		}
	}
	// The look follows, as when the theme is chosen in the settings.
	if aurora := newLook(s.set, s.inv.Themes, isDark(), ""); s.set.Theme != "aurora" || s.lk.bg != aurora.bg {
		t.Errorf("theme %q, the look's ground %v, want %v", s.set.Theme, s.lk.bg, aurora.bg)
	}
}

func TestFileNotes(t *testing.T) {
	for _, c := range []struct {
		r    settings.Report
		want string
	}{
		{settings.Report{}, ""},
		{settings.Report{Unknown: []string{"a", "b", "c"}}, "MixMusic doesn't know a, b and c: its next save drops them."},
		{settings.Report{NotUTF8: true, Adjusted: []string{"zoom"}}, "It isn't UTF-8, so letters outside ASCII read as � in it: save it as UTF-8. Put zoom back in range."},
	} {
		if got := fileNotes(c.r); got != c.want {
			t.Errorf("fileNotes(%+v) = %q, want %q", c.r, got, c.want)
		}
	}
}

// TestSettingsFileRow opens settings.json from Settings › Desktop and from
// the command palette.
func TestSettingsFileRow(t *testing.T) {
	s := fileShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/settings/desktop")
	click(t, s, tt, "open")
	wasOpened(t, "open")
	click(t, s, tt, "show in folder")
	wasOpened(t, "show")
	for _, cmd := range s.commands() {
		if cmd.id == "settings.openFile" {
			if cmd.title != "Open the settings file" {
				t.Errorf("the command is %q", cmd.title)
			}
			cmd.run()
			wasOpened(t, "open")
			return
		}
	}
	t.Error("no command opens the settings file")
}

// TestSettingsFileToastsMainOnly tells of settings.json in the main window
// alone: neither the mini player nor a window opened later.
func TestSettingsFileToastsMainOnly(t *testing.T) {
	s := fileShell(t)
	mini := newShell(s.a, s.p)
	editSettings(t, s, `{"theme": "aurora"}`)
	s.refresh()
	mini.refresh()
	if s.fileNotice == nil {
		t.Error("the main window has nothing to tell")
	}
	if mini.fileNotice != nil {
		t.Errorf("the mini player tells %+v", *mini.fileNotice)
	}
	later := newShell(s.a, s.p)
	later.main = true
	later.refresh()
	if later.fileNotice != nil {
		t.Errorf("a window opened later tells %+v", *later.fileNotice)
	}
}

// TestSaveKeepsAReload changes a setting in a window that hasn't heard of
// an edit yet: the edit stays.
func TestSaveKeepsAReload(t *testing.T) {
	s := fileShell(t)
	editSettings(t, s, `{"theme": "aurora", "crossfade": 6}`)
	// The window's refresh waits; its settings are older than the edit.
	if s.set.Theme == "aurora" {
		t.Fatal("the window heard of the edit already")
	}
	s.set.Density = settings.DensityCompact
	s.save()
	got := s.svc.Preferences.Get()
	if got.Theme != "aurora" || got.Crossfade != 6 || got.Density != settings.DensityCompact {
		t.Errorf("the settings are %q, %d, %q", got.Theme, got.Crossfade, got.Density)
	}
	s.refresh()
	if s.set.Theme != "aurora" || s.set.Density != settings.DensityCompact {
		t.Errorf("the window has %q, %q", s.set.Theme, s.set.Density)
	}
	// And the next change keeps both.
	s.set.FontScale = 1.1
	s.save()
	if got := s.svc.Preferences.Get(); got.Theme != "aurora" || got.FontScale != 1.1 || got.Density != settings.DensityCompact {
		t.Errorf("the settings are %q, %v, %q", got.Theme, got.FontScale, got.Density)
	}
}

// TestSettingsFileFollows does what the pages do around a change made in
// settings.json: the side panel, and what the pages ask again.
func TestSettingsFileFollows(t *testing.T) {
	s := fileShell(t)
	if s.panel != "queue" {
		t.Fatalf("the panel is %q", s.panel)
	}
	s.loads["registry"] = &loaded[int]{done: true}
	editSettings(t, s, `{"panel": "lyrics", "registryUrl": "https://example.com/registry.json", "ytdlpPath": "/opt/yt-dlp", "zoom": 1.25}`)
	s.refresh()
	if s.panel != "lyrics" {
		t.Errorf("the panel is %q", s.panel)
	}
	if _, ok := s.loads["registry"]; ok {
		t.Error("the marketplace keeps the former registry")
	}
	if _, ok := s.loads["tools"].(*loaded[tools.Status]); ok {
		t.Error("the tools stay as they were")
	}
	if !s.zoomToast {
		t.Error("the zoom isn't told")
	}
	editSettings(t, s, `{"panel": "none"}`)
	s.refresh()
	if s.panel != "" {
		t.Errorf("the panel is %q, want none", s.panel)
	}
	// The window's own change of the panel isn't undone by its refresh.
	s.showPanel("queue")
	s.refresh()
	if s.panel != "queue" {
		t.Errorf("the panel is %q, want the queue", s.panel)
	}
}
