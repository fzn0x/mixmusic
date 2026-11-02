package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// TestUseThemeFollowingSystem uses a theme from its card in the
// marketplace while the themes follow the computer's appearance: it
// becomes the theme of the appearance the computer is in, which shows it,
// and the card stops offering it. Theme, which the themes don't show then,
// already names it, as when it was used before the user followed the
// system: the card offers it all the same (#174).
func TestUseThemeFollowingSystem(t *testing.T) {
	for _, dark := range []bool{true, false} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			was := isDark
			isDark = func() bool { return dark }
			t.Cleanup(func() { isDark = was })
			s := newTestShell(t)
			st := s.svc.Preferences.Get()
			st.FollowSystem, st.Theme, st.DarkTheme, st.LightTheme = true, "aurora", "midnight", "daylight"
			if _, err := s.svc.Preferences.Set(st); err != nil {
				t.Fatal(err)
			}
			s.refresh()
			s.loads["registry"] = &loaded[addons.Registry]{done: true, value: addons.Registry{
				Themes: []addons.Entry{{ID: "aurora", Name: "Aurora", Version: "1.0.0"}},
			}}
			tt := tester(t, s)
			show(s, tt, "/marketplace/themes")
			if !tt.HasText("use it") {
				t.Fatalf("Aurora's card doesn't offer it, while %s shows: %q", s.set.ActiveTheme(dark), tt.Texts())
			}
			click(t, s, tt, "use it")
			s.drain()
			tt.Frame()

			got := s.svc.Preferences.Get()
			want := settings3{theme: "aurora", dark: "midnight", light: "aurora"}
			if dark {
				want = settings3{theme: "aurora", dark: "aurora", light: "daylight"}
			}
			if have := (settings3{got.Theme, got.DarkTheme, got.LightTheme}); have != want {
				t.Errorf("use it made the themes %+v, want %+v", have, want)
			}
			if active := got.ActiveTheme(dark); active != "aurora" {
				t.Errorf("the theme shown is %q, not aurora", active)
			}
			if tt.HasText("use it") {
				t.Error("the card still offers Aurora, which shows")
			}
		})
	}
}

// settings3 are the three themes of the settings: the one shown when they
// don't follow the system, and the dark and light ones when they do.
type settings3 struct{ theme, dark, light string }
