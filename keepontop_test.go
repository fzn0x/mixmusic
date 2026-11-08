package main

import (
	"slices"
	"sync"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// keptAbove records what the windows were told of staying on top, in
// place of keepAbove, until the test ends.
func keptAbove(t *testing.T) func() []bool {
	t.Helper()
	var mu sync.Mutex
	var told []bool
	was := keepAbove
	keepAbove = func(_ *mygo.Window, on bool) bool {
		mu.Lock()
		defer mu.Unlock()
		told = append(told, on)
		return true
	}
	t.Cleanup(func() { keepAbove = was })
	return func() []bool {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(told)
	}
}

// TestKeepOnTopRow keeps the main window on top from Settings › On the
// desktop (#350): a switch below Discord's rows and above Start at login,
// off at first, which the keyboard reaches, saved as keepOnTop, and which
// a search for "always on top" finds. The window follows it.
func TestKeepOnTopRow(t *testing.T) {
	told := keptAbove(t)
	s := newTestShell(t)
	s.main = true
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	var above float32
	for _, text := range []string{"Show what plays on Discord", keepOnTopTitle, keepOnTopDescription, "Start at login"} {
		r, ok := tt.Find(text)
		if !ok || r.Y <= above {
			t.Fatalf("Settings › Desktop shows %q out of place: %q", text, tt.Texts())
		}
		above = r.Y
	}
	if s.svc.Preferences.Get().KeepOnTop || len(told()) > 0 {
		t.Fatalf("on at first: told %v", told())
	}
	focused := func() bool { return tt.Focused(keepOnTopTitle) && !tt.Focused(keepOnTopDescription) }
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	for i, want := range []bool{true, false} {
		tt.Key(0, ui.KeySpace)
		s.drain()
		tt.Frame()
		if got := s.svc.Preferences.Get().KeepOnTop; got != want {
			t.Errorf("the switch saved keepOnTop %v, not %v", got, want)
		}
		if got := told(); len(got) != i+1 || got[i] != want {
			t.Errorf("the window was told %v, want %v last", got, want)
		}
	}
	for _, q := range []string{"keep on top", "always on top", "float"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText(keepOnTopTitle) {
			t.Errorf("searching %q doesn't find the row: %q", q, tt.Texts())
		}
	}
}

// TestKeepOnTopFollows keeps the main window on top as the setting changes
// elsewhere, as in settings.json, and only once for each change: the
// frames after tell the window nothing. The mini player's window keeps
// its own options.
func TestKeepOnTopFollows(t *testing.T) {
	told := keptAbove(t)
	s := fileShell(t)
	tt := tester(t, s)
	mini := newShell(s.a, s.p)
	minitt := ui.NewTester(mini.miniView, miniWidth, miniHeight)
	minitt.Frame()
	for _, c := range []struct {
		file string
		want []bool
	}{
		{`{"keepOnTop": true}`, []bool{true}},
		{`{"keepOnTop": true, "theme": "aurora"}`, []bool{true}},
		{`{"keepOnTop": false}`, []bool{true, false}},
	} {
		editSettings(t, s, c.file)
		s.refresh()
		mini.refresh()
		tt.Frame()
		tt.Frame()
		minitt.Frame()
		if got := told(); !slices.Equal(got, c.want) {
			t.Errorf("after %s, the windows were told %v, want %v", c.file, got, c.want)
		}
	}
}

// TestKeepOnTopStart opens the main window on top when the settings keep
// it so, which its first frames don't tell it again.
func TestKeepOnTopStart(t *testing.T) {
	told := keptAbove(t)
	s := fileShell(t)
	editSettings(t, s, `{"keepOnTop": true}`)
	s.refresh()
	if !s.startOnTop() {
		t.Fatal("the window doesn't open on top")
	}
	tt := tester(t, s)
	tt.Frame()
	if got := told(); len(got) > 0 {
		t.Errorf("the open window was told %v", got)
	}
}
