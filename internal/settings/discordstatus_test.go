package settings

import (
	"strings"
	"testing"
)

// TestDiscordStatus shows the Discord status as before it could be chosen
// in new installs and in the settings of earlier versions, which lack the
// keys: the title, then the artists, with the cover and the time left, no
// badge or button, and cleared while paused (#352).
func TestDiscordStatus(t *testing.T) {
	for _, file := range []string{`{}`, `{"showOnDiscord":true,"discordAppId":"123456789012345678"}`} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		d := s.Desktop
		if d.DiscordFirstLine != "{title}" || d.DiscordSecondLine != "{artist}" {
			t.Errorf("%s: the lines are %q and %q", file, d.DiscordFirstLine, d.DiscordSecondLine)
		}
		if !d.DiscordCover || !d.DiscordTimeLeft || d.DiscordServiceBadge || d.DiscordListenButton || d.DiscordKeepWhilePaused {
			t.Errorf("%s: cover %v, time %v, badge %v, button %v, kept paused %v", file,
				d.DiscordCover, d.DiscordTimeLeft, d.DiscordServiceBadge, d.DiscordListenButton, d.DiscordKeepWhilePaused)
		}
	}
	s, err := Decode([]byte(`{"discordFirstLine":"","discordCover":false,"discordKeepWhilePaused":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.DiscordFirstLine != "" || s.DiscordSecondLine != "{artist}" || s.DiscordCover || !s.DiscordKeepWhilePaused {
		t.Errorf("the user's choices read as %+v", s.Desktop)
	}
}

// TestDiscordLines keeps a line of the Discord status on one line, and no
// longer than Discord shows, as JavaScript counts its characters.
func TestDiscordLines(t *testing.T) {
	cases := []struct{ in, want string }{
		{"{title}", "{title}"},
		{"  {artist} · {album}  ", "{artist} · {album}"},
		{"{title}\n{artist}\t!", "{title} {artist} !"},
		{"", ""},
		{strings.Repeat("a", 128), strings.Repeat("a", 128)},
		{strings.Repeat("a", 130), strings.Repeat("a", 128)},
		// Two code units each, as Discord counts them.
		{strings.Repeat("🎵", 65), strings.Repeat("🎵", 64)},
		{strings.Repeat("a", 127) + " b", strings.Repeat("a", 127)},
	}
	for _, c := range cases {
		var s Settings
		s.DiscordFirstLine, s.DiscordSecondLine = c.in, c.in
		got := s.Clean()
		if got.DiscordFirstLine != c.want || got.DiscordSecondLine != c.want {
			t.Errorf("%q cleans to %q and %q, want %q", c.in, got.DiscordFirstLine, got.DiscordSecondLine, c.want)
		}
	}
}
