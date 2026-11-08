package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

// discordSwitch turns a switch of the Discord status by its label, and
// returns the settings saved.
func discordSwitch(t *testing.T, s *shell, tt *ui.Tester, label string) {
	t.Helper()
	click(t, s, tt, label)
	s.drain()
	tt.Frame()
}

// TestDiscordStatusRows chooses what the Discord status shows from
// Settings › On the desktop (#352): under Discord's switch and its
// application ID, a preview of the status, its lines and five switches,
// as a new install has them, which show only while Discord shows what
// plays. The preview follows each switch.
func TestDiscordStatusRows(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	var above float32
	for _, text := range []string{"Show what plays on Discord", "Discord application ID", "Preview", "First line", "Second line",
		"Cover", "Service badge", "Time left", "Listen button", "Keep while paused", "Start at login"} {
		r, ok := tt.Find(text)
		if !ok || r.Y <= above {
			t.Fatalf("Settings › Desktop shows %q out of place: %q", text, tt.Texts())
		}
		above = r.Y
	}
	// Nothing plays: the preview shows a song of the sample's, as a new
	// install shows it, a minute and 12 seconds in.
	for _, text := range []string{"Listening to MixMusic", "Song title", "Artist", "1:12", "3:34", "Nothing plays: this is how a song would show."} {
		if _, ok := tt.Find(text); !ok {
			t.Errorf("the preview doesn't show %q: %q", text, tt.Texts())
		}
	}
	for _, text := range []string{"Listen on YouTube Music", "YouTube Music"} {
		if _, ok := tt.Find(text); ok {
			t.Errorf("a new install's preview shows %q", text)
		}
	}

	discordSwitch(t, s, tt, "Show the time left on Discord")
	if got := s.svc.Preferences.Get(); got.DiscordTimeLeft {
		t.Error("the switch didn't save discordTimeLeft")
	}
	if tt.HasText("1:12") || tt.HasText("3:34") {
		t.Errorf("without the time, the preview shows it: %q", tt.Texts())
	}
	discordSwitch(t, s, tt, "Show a Listen button on Discord")
	if !s.svc.Preferences.Get().DiscordListenButton {
		t.Error("the switch didn't save discordListenButton")
	}
	if _, ok := tt.Find("Listen on YouTube Music"); !ok {
		t.Errorf("the preview has no button: %q", tt.Texts())
	}
	// The sample has no cover: its badge takes the cover's place.
	discordSwitch(t, s, tt, "Show the service's badge on Discord")
	if !s.svc.Preferences.Get().DiscordServiceBadge {
		t.Error("the switch didn't save discordServiceBadge")
	}
	if _, ok := tt.Find("YouTube Music"); !ok {
		t.Errorf("the preview has no badge: %q", tt.Texts())
	}
	discordSwitch(t, s, tt, "Show the cover on Discord")
	discordSwitch(t, s, tt, "Keep the Discord status while paused")
	if got := s.svc.Preferences.Get(); got.DiscordCover || !got.DiscordKeepWhilePaused {
		t.Errorf("the switches saved cover %v and kept while paused %v", got.DiscordCover, got.DiscordKeepWhilePaused)
	}

	s.set.DiscordPresence = false
	s.save()
	tt.Frame()
	for _, text := range []string{"Preview", "First line", "Listen button", "Keep while paused"} {
		if _, ok := tt.Find(text); ok {
			t.Errorf("with Discord off, the settings show %q", text)
		}
	}
}

// TestDiscordStatusLines types the lines of the Discord status: the
// preview shows what is typed as it is typed, and tells of what isn't a
// placeholder; Enter saves the line, and reset puts back the title.
func TestDiscordStatusLines(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	click(t, s, tt, "First line on Discord")
	tt.Key(0, ui.KeyEnd)
	for range len("{title}") {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("{artist} · {title} {year}")
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find("Artist · Song title {year}"); !ok {
		t.Errorf("the preview doesn't show the line as it's typed: %q", tt.Texts())
	}
	if !tt.HasText("{year} isn't a placeholder, and shows as typed.") {
		t.Errorf("nothing tells that {year} is no placeholder: %q", tt.Texts())
	}
	if got := s.svc.Preferences.Get().DiscordFirstLine; got != "{title}" {
		t.Errorf("the line was saved as it was typed: %q", got)
	}
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().DiscordFirstLine; got != "{artist} · {title} {year}" {
		t.Errorf("Enter saved the line %q", got)
	}
	reset := ""
	for _, text := range tt.Texts() {
		if strings.EqualFold(text, "reset") {
			reset = text
		}
	}
	if reset == "" {
		t.Fatalf("no reset: %q", tt.Texts())
	}
	click(t, s, tt, reset)
	if got := s.svc.Preferences.Get().DiscordFirstLine; got != "{title}" {
		t.Errorf("reset put back %q", got)
	}
	if _, ok := tt.Find("Song title"); !ok {
		t.Errorf("after reset, the preview shows %q", tt.Texts())
	}
}

// TestDiscordStatusPaused previews a paused track: as Discord will show it
// once it plays again, while the status doesn't keep it, and marked as
// paused, without its times, while it does.
func TestDiscordStatusPaused(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	if !tt.HasText("While paused, your profile shows nothing") {
		t.Errorf("the preview doesn't say that a pause clears the status: %q", tt.Texts())
	}
	if _, ok := tt.Find(testTracks[0].Title); !ok {
		t.Errorf("the preview doesn't show the paused track: %q", tt.Texts())
	}
	discordSwitch(t, s, tt, "Keep the Discord status while paused")
	if tt.HasText("While paused, your profile shows nothing") {
		t.Errorf("kept while paused, the preview says it shows nothing: %q", tt.Texts())
	}
	if _, ok := tt.Find("Paused"); !ok {
		t.Errorf("the preview doesn't mark the track as paused: %q", tt.Texts())
	}
	if _, ok := tt.Find("Time played"); ok {
		t.Errorf("paused, the preview shows the time: %q", tt.Texts())
	}
}

// TestDiscordStatusSearch finds the rows of the Discord status by their
// words.
func TestDiscordStatusSearch(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	for q, want := range map[string]string{"placeholders": "First line", "listen button": "Listen button",
		"keep while paused": "Keep while paused", "rich presence": "Service badge", "discord preview": "Preview"} {
		show(s, tt, settingsPath("", q))
		if _, ok := tt.Find(want); !ok {
			t.Errorf("searching %q doesn't find %q: %q", q, want, tt.Texts())
		}
	}
}
