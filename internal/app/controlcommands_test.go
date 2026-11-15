package app

import (
	"math"
	"net/url"
	"testing"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// newControlledApp is a quiet app playing A then B, whose main window
// fails the test if it opens: a command from a link or a flag brings no
// window forward.
func newControlledApp(t *testing.T) (*App, *player.Player) {
	t.Helper()
	a := newQuietApp(t, func(o *Options) {
		o.Interface = func(*App, bool) *mygo.Window {
			t.Error("the main window opened for a command of the player")
			return nil
		}
	})
	p := a.GoPlayer()
	p.SetVolume(0.5)
	p.PlayTracks([]media.Track{
		{ID: "yt:a", Source: media.SourceYouTube, Title: "A"},
		{ID: "yt:b", Source: media.SourceYouTube, Title: "B"},
	}, 0)
	waitPlayer(t, p, "A playing", trackPlays("yt:a"))
	return a, p
}

// trackPlays is the state of the track of id playing.
func trackPlays(id string) func(player.State) bool {
	return func(s player.State) bool {
		cur, ok := s.Current()
		return ok && cur.Track.ID == id && s.Playing
	}
}

// parseLink parses a link of the test.
func parseLink(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// TestControlLinks reads the commands of the links: the name alone, a
// slash after it at most, and the volume turned by 5%, as the global
// shortcuts turn it. Like and dislike, a track to play and the pages
// aren't commands.
func TestControlLinks(t *testing.T) {
	for raw, want := range map[string]PlayerCommand{
		"mixmusic://toggle":      {Command: CommandToggle},
		"mixmusic://play":        {Command: CommandPlay},
		"mixmusic://play/":       {Command: CommandPlay},
		"mixmusic://pause":       {Command: CommandPause},
		"mixmusic://next":        {Command: CommandNext},
		"mixmusic://next/":       {Command: CommandNext},
		"mixmusic://previous":    {Command: CommandPrevious},
		"mixmusic://mute":        {Command: CommandMute},
		"mixmusic://shuffle":     {Command: CommandShuffle},
		"mixmusic://repeat":      {Command: CommandRepeat},
		"mixmusic://volume-up":   {Command: CommandVolumeBy, Value: 0.05},
		"mixmusic://volume-down": {Command: CommandVolumeBy, Value: -0.05},
		"MIXMUSIC://next":        {Command: CommandNext},
	} {
		if got, ok := controlOf(parseLink(t, raw)); !ok || got != want {
			t.Errorf("%s: %+v, %v, want %+v", raw, got, ok, want)
		}
	}
	for _, raw := range []string{
		"mixmusic://like",
		"mixmusic://dislike",
		"mixmusic://play/yt:abc",
		"mixmusic://next/yt:abc",
		"mixmusic://library",
		"mixmusic://search?q=next",
		"mixmusic:next",
		"mixmusic://",
		"https://next",
	} {
		if got, ok := controlOf(parseLink(t, raw)); ok {
			t.Errorf("%s is the command %+v", raw, got)
		}
	}
}

// TestControlLinkCommands follows the links as HandleURL does, on every
// system: macOS hands MixMusic the link, Windows and Linux start it again
// with the link, which the start that runs takes. The player does each
// command, and no window opens.
func TestControlLinkCommands(t *testing.T) {
	a, p := newControlledApp(t)
	follow := func(raw string) {
		t.Helper()
		if !a.followControl(parseLink(t, raw)) {
			t.Fatalf("%s isn't a command", raw)
		}
	}
	follow("mixmusic://next")
	waitPlayer(t, p, "B playing", trackPlays("yt:b"))
	follow("mixmusic://previous")
	waitPlayer(t, p, "A playing again", trackPlays("yt:a"))
	follow("mixmusic://pause")
	waitPlayer(t, p, "paused", func(s player.State) bool { return !s.Playing })
	// A toggle is the media key's: another at once is taken for the same
	// press.
	follow("mixmusic://toggle")
	waitPlayer(t, p, "playing after a toggle", func(s player.State) bool { return s.Playing })
	follow("mixmusic://pause")
	waitPlayer(t, p, "paused again", func(s player.State) bool { return !s.Playing })
	follow("mixmusic://play")
	waitPlayer(t, p, "playing", func(s player.State) bool { return s.Playing })
	follow("mixmusic://volume-up")
	waitPlayer(t, p, "5% louder", func(s player.State) bool { return math.Abs(s.Volume-0.55) < 1e-9 })
	follow("mixmusic://volume-down")
	follow("mixmusic://volume-down")
	waitPlayer(t, p, "5% quieter", func(s player.State) bool { return math.Abs(s.Volume-0.45) < 1e-9 })
	follow("mixmusic://mute")
	waitPlayer(t, p, "muted", func(s player.State) bool { return s.Muted })
	follow("mixmusic://shuffle")
	waitPlayer(t, p, "shuffled", func(s player.State) bool { return s.Shuffle })
	follow("mixmusic://repeat")
	waitPlayer(t, p, "repeating all", func(s player.State) bool { return s.Repeat == player.RepeatAll })
	// A page or a track to play isn't a command: HandleURL goes on to them.
	for _, raw := range []string{"mixmusic://library", "mixmusic://play/yt:b"} {
		if a.followControl(parseLink(t, raw)) {
			t.Errorf("%s was taken for a command", raw)
		}
	}
}

// TestSecondStartFlags starts MixMusic again with flags, as a key or a
// script does on every system (on macOS, the program in the bundle): MyGo
// hands the command line to the MixMusic that runs, whose player does
// each command, and no window opens.
func TestSecondStartFlags(t *testing.T) {
	a, p := newControlledApp(t)
	start := func(args ...string) {
		t.Helper()
		if !a.controlled(args) {
			t.Fatalf("%q asked for no command", args)
		}
	}
	start("--next")
	waitPlayer(t, p, "B playing", trackPlays("yt:b"))
	start("--previous")
	waitPlayer(t, p, "A playing again", trackPlays("yt:a"))
	start("--pause")
	waitPlayer(t, p, "paused", func(s player.State) bool { return !s.Playing })
	start("--toggle")
	waitPlayer(t, p, "playing after a toggle", func(s player.State) bool { return s.Playing })
	start("--pause")
	waitPlayer(t, p, "paused again", func(s player.State) bool { return !s.Playing })
	start("--play")
	waitPlayer(t, p, "playing", func(s player.State) bool { return s.Playing })
	// Each flag counts, in its order.
	start("--volume-up", "--volume-up", "--volume-down")
	waitPlayer(t, p, "5% louder", func(s player.State) bool { return math.Abs(s.Volume-0.55) < 1e-9 })
	start("--volume-down", "--volume-down")
	waitPlayer(t, p, "5% quieter", func(s player.State) bool { return math.Abs(s.Volume-0.45) < 1e-9 })
	start("--mute", "--shuffle", "--repeat")
	waitPlayer(t, p, "muted, shuffled, repeating all", func(s player.State) bool {
		return s.Muted && s.Shuffle && s.Repeat == player.RepeatAll
	})
}

// TestHasControlFlag tells the command lines main hands over to the
// MixMusic that runs: each command's flag, anywhere among them, and
// nothing else.
func TestHasControlFlag(t *testing.T) {
	for name := range namedCommands {
		if !HasControlFlag([]string{"--" + name}) {
			t.Errorf("--%s isn't a command's flag", name)
		}
	}
	if !HasControlFlag([]string{"--some-flag", "--next"}) {
		t.Error("--next after another flag isn't one")
	}
	for _, args := range [][]string{nil, {"--like"}, {"--dislike"}, {"--mcp"}, {"--allow-firewall"}, {"mixmusic://next"}, {"next"}, {"-next"}} {
		if HasControlFlag(args) {
			t.Errorf("%q has a command's flag", args)
		}
	}
}

// TestSecondStartLinks starts MixMusic again with a link, as a browser,
// a key or a script does on Windows and Linux. A command's link brings no
// window forward, and its command is HandleURL's, which MyGo hands the
// link to after: done here too, it would be done twice. A page, a track
// to play or no link at all shows the main window, as before.
func TestSecondStartLinks(t *testing.T) {
	a, p := newControlledApp(t)
	for _, args := range [][]string{
		{"mixmusic://pause"},
		{"mixmusic://pause/"},
		{"--some-flag", "mixmusic://pause"},
	} {
		if !a.controlled(args) {
			t.Errorf("%q asked for no command", args)
		}
	}
	// The player does its commands in order: muted, it would have paused
	// before, had the link's command been done here.
	a.command(PlayerCommand{Command: CommandMute})
	waitPlayer(t, p, "muted", func(s player.State) bool { return s.Muted })
	if !p.State().Playing {
		t.Error("the second start did the link's command, which HandleURL does")
	}
	for _, args := range [][]string{
		nil,
		{"mixmusic://library"},
		{"mixmusic://play/yt:b"},
		{"mixmusic://like"},
		{"--like"},
		{"--dislike"},
		{"-next"},
		{"next"},
		{"--Next"},
		{"--mcp"},
	} {
		if a.controlled(args) {
			t.Errorf("%q asked for a command, and would show no window", args)
		}
	}
}
