package app

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/discord"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The tracks of the Discord status's tests: a song of YouTube Music, with
// its link, one of Spotify, without, and one on this computer, whose cover
// Discord can't read.
var (
	statusYouTube = media.Track{ID: "yt:w3XzgjlKFjc", Source: media.SourceYouTube, Title: "Let It Happen",
		Artists: []media.Ref{{Name: "Tame Impala"}}, Album: &media.Ref{Name: "Currents"}, DurationMs: 467000,
		Artwork: []media.Image{{URL: "https://example.com/544.jpg", Width: 544}},
		URL:     "https://music.youtube.com/watch?v=w3XzgjlKFjc"}
	statusSpotify = media.Track{ID: "sp:4uLU6hMCjMI75M1A2tKUQC", Source: media.SourceSpotify, Title: "Teardrop",
		Artists: []media.Ref{{Name: "Massive Attack"}}, Album: &media.Ref{Name: "Mezzanine"}, DurationMs: 330000,
		Artwork: []media.Image{{URL: "https://i.scdn.co/image/ab67", Width: 640}}}
	statusFile = media.Track{ID: "file:a1b2", Source: media.SourceFile, Title: "Demo",
		Artists: []media.Ref{{Name: "Me"}}, Album: &media.Ref{Name: "Tapes"}, DurationMs: 120000,
		Artwork: []media.Image{{URL: "mixmusic-art://local/a1b2"}}}
)

// statusNow is the clock of the Discord status's tests.
var statusNow = time.Date(2026, 11, 8, 20, 0, 0, 0, time.UTC)

// desktop is the settings of a new install, as change changes them.
func desktop(change func(d *settings.Desktop)) settings.Desktop {
	d := settings.Default().Desktop
	if change != nil {
		change(&d)
	}
	return d
}

// TestDiscordActivityChoices shows what the Discord status shows of each
// track as each setting chooses (#352): a new install shows the title,
// the artists, the cover and the times, as before they could be chosen.
func TestDiscordActivityChoices(t *testing.T) {
	start := statusNow.Add(-time.Minute)
	paused := discordBadges + "paused.png"
	cases := []struct {
		name   string
		d      settings.Desktop
		track  media.Track
		paused bool
		want   *discord.Activity
	}{
		{"a new install", desktop(nil), statusYouTube, false, &discord.Activity{Type: discord.Listening,
			Details: "Let It Happen", State: "Tame Impala", Start: start, End: start.Add(467 * time.Second),
			LargeImage: "https://example.com/544.jpg", LargeText: "Currents"}},
		{"paused, as a new install", desktop(nil), statusYouTube, true, nil},
		{"kept while paused", desktop(func(d *settings.Desktop) { d.DiscordKeepWhilePaused = true }), statusYouTube, true,
			&discord.Activity{Type: discord.Listening, Details: "Let It Happen", State: "Tame Impala",
				LargeImage: "https://example.com/544.jpg", LargeText: "Currents", SmallImage: paused, SmallText: "Paused"}},
		{"kept while paused, without the cover", desktop(func(d *settings.Desktop) {
			d.DiscordKeepWhilePaused, d.DiscordCover, d.DiscordServiceBadge = true, false, true
		}), statusYouTube, true,
			&discord.Activity{Type: discord.Listening, Details: "Let It Happen", State: "Tame Impala",
				LargeImage: paused, LargeText: "Paused"}},
		{"the service's badge", desktop(func(d *settings.Desktop) { d.DiscordServiceBadge = true }), statusSpotify, false,
			&discord.Activity{Type: discord.Listening, Details: "Teardrop", State: "Massive Attack",
				Start: start, End: start.Add(330 * time.Second), LargeImage: "https://i.scdn.co/image/ab67", LargeText: "Mezzanine",
				SmallImage: discordBadges + "spotify.png", SmallText: "Spotify"}},
		{"the badge without the cover", desktop(func(d *settings.Desktop) { d.DiscordServiceBadge, d.DiscordCover = true, false }), statusYouTube, false,
			&discord.Activity{Type: discord.Listening, Details: "Let It Happen", State: "Tame Impala",
				Start: start, End: start.Add(467 * time.Second), LargeImage: discordBadges + "youtube-music.png", LargeText: "YouTube Music"}},
		{"no cover, no badge", desktop(func(d *settings.Desktop) { d.DiscordCover = false }), statusYouTube, false,
			&discord.Activity{Type: discord.Listening, Details: "Let It Happen", State: "Tame Impala",
				Start: start, End: start.Add(467 * time.Second)}},
		{"no times", desktop(func(d *settings.Desktop) { d.DiscordTimeLeft = false }), statusYouTube, false,
			&discord.Activity{Type: discord.Listening, Details: "Let It Happen", State: "Tame Impala",
				LargeImage: "https://example.com/544.jpg", LargeText: "Currents"}},
		{"listen on YouTube Music", desktop(func(d *settings.Desktop) { d.DiscordListenButton = true }), statusYouTube, false,
			&discord.Activity{Type: discord.Listening, Details: "Let It Happen", State: "Tame Impala",
				Start: start, End: start.Add(467 * time.Second), LargeImage: "https://example.com/544.jpg", LargeText: "Currents",
				Buttons: []discord.Button{{Label: "Listen on YouTube Music", URL: "https://music.youtube.com/watch?v=w3XzgjlKFjc"}}}},
		{"listen on Spotify, from the track's ID", desktop(func(d *settings.Desktop) { d.DiscordListenButton, d.DiscordCover = true, false }), statusSpotify, false,
			&discord.Activity{Type: discord.Listening, Details: "Teardrop", State: "Massive Attack",
				Start: start, End: start.Add(330 * time.Second),
				Buttons: []discord.Button{{Label: "Listen on Spotify", URL: "https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC"}}}},
		{"a file on this computer", desktop(func(d *settings.Desktop) { d.DiscordListenButton, d.DiscordServiceBadge = true, true }), statusFile, false,
			&discord.Activity{Type: discord.Listening, Details: "Demo", State: "Me",
				Start: start, End: start.Add(2 * time.Minute), LargeImage: discordBadges + "computer.png", LargeText: "A file on my computer"}},
		{"the lines chosen", desktop(func(d *settings.Desktop) {
			d.DiscordFirstLine, d.DiscordSecondLine = "{artist} · {title}", "{album} on {service}"
		}), statusSpotify, false,
			&discord.Activity{Type: discord.Listening, Details: "Massive Attack · Teardrop", State: "Mezzanine on Spotify",
				Start: start, End: start.Add(330 * time.Second), LargeImage: "https://i.scdn.co/image/ab67", LargeText: "Mezzanine"}},
		{"empty lines", desktop(func(d *settings.Desktop) { d.DiscordFirstLine, d.DiscordSecondLine = "", "" }), statusYouTube, false,
			&discord.Activity{Type: discord.Listening, Start: start, End: start.Add(467 * time.Second),
				LargeImage: "https://example.com/544.jpg", LargeText: "Currents"}},
	}
	for _, c := range cases {
		track := c.track
		np := NowPlaying{Track: &track, Playing: !c.paused, PositionMs: 60000}
		if c.want != nil {
			c.want.StatusDisplay = discord.ShowName
		}
		if got := DiscordActivity(c.d, np, statusNow); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
	if got := DiscordActivity(desktop(nil), NowPlaying{Playing: true}, statusNow); got != nil {
		t.Errorf("nothing plays, and Discord shows %+v", got)
	}
}

// TestDiscordActivityEveryChoice goes through every choice of the switches,
// for each track, playing and paused: the times show while the track
// plays, where the settings show them; a small image shows only on a large
// one; the paused badge marks a paused track; the button opens a track of a
// service; and the lines are the same whatever the switches.
func TestDiscordActivityEveryChoice(t *testing.T) {
	for bits := range 1 << 5 {
		d := desktop(func(d *settings.Desktop) {
			d.DiscordCover, d.DiscordServiceBadge, d.DiscordTimeLeft = bits&1 != 0, bits&2 != 0, bits&4 != 0
			d.DiscordListenButton, d.DiscordKeepWhilePaused = bits&8 != 0, bits&16 != 0
		})
		for _, track := range []media.Track{statusYouTube, statusSpotify, statusFile} {
			for _, playing := range []bool{true, false} {
				np := NowPlaying{Track: &track, Playing: playing, PositionMs: 1000}
				a := DiscordActivity(d, np, statusNow)
				name := track.ID
				if !playing {
					name += ", paused"
				}
				if !playing && !d.DiscordKeepWhilePaused {
					if a != nil {
						t.Errorf("%+v, %s: Discord shows %+v", d, name, a)
					}
					continue
				}
				if a == nil {
					t.Fatalf("%+v, %s: Discord shows nothing", d, name)
				}
				if a.Details != track.Title || a.State != track.ArtistNames() {
					t.Errorf("%+v, %s: the lines are %q and %q", d, name, a.Details, a.State)
				}
				if times := !a.Start.IsZero() || !a.End.IsZero(); times != (playing && d.DiscordTimeLeft) {
					t.Errorf("%+v, %s: times %v to %v", d, name, a.Start, a.End)
				}
				if a.SmallImage != "" && a.LargeImage == "" {
					t.Errorf("%+v, %s: a badge without a picture", d, name)
				}
				badges := []string{DiscordBadge(a.LargeImage), DiscordBadge(a.SmallImage)}
				if slices.Contains(badges, BadgePaused) == playing {
					t.Errorf("%+v, %s: the badges are %q", d, name, badges)
				}
				if service := playing && d.DiscordServiceBadge; slices.Contains(badges, discordServiceBadge(track)) != service {
					t.Errorf("%+v, %s: the badges are %q", d, name, badges)
				}
				if cover := DiscordBadge(a.LargeImage) == "" && a.LargeImage != ""; cover != (d.DiscordCover && track.ID != statusFile.ID) {
					t.Errorf("%+v, %s: the picture is %q", d, name, a.LargeImage)
				}
				if button := len(a.Buttons) == 1; button != (d.DiscordListenButton && track.ID != statusFile.ID) || len(a.Buttons) > 1 {
					t.Errorf("%+v, %s: the buttons are %+v", d, name, a.Buttons)
				}
			}
		}
	}
}

// TestExpandDiscordLine fills a line's placeholders with the track's, and
// shows what isn't one as typed.
func TestExpandDiscordLine(t *testing.T) {
	single := statusYouTube
	single.Album = nil
	video := media.Track{ID: "yt:v", Title: "Live at the Roundhouse", Artists: []media.Ref{{Name: "A"}, {Name: "B"}}}
	cases := []struct {
		template string
		track    media.Track
		want     string
	}{
		{"{title}", statusYouTube, "Let It Happen"},
		{"{artist}", video, "A, B"},
		{"{artist} · {album}", statusYouTube, "Tame Impala · Currents"},
		{"{artist} · {album}", single, "Tame Impala"},
		{"{album} - {title}", single, "Let It Happen"},
		{"on {service}", statusSpotify, "on Spotify"},
		{"{service}", video, "YouTube Music"},
		{"{service}", statusFile, "my computer"},
		{"{Title} by {ARTIST}", statusYouTube, "Let It Happen by Tame Impala"},
		{"{ title }", statusYouTube, "Let It Happen"},
		{"{year} {title}", statusYouTube, "{year} Let It Happen"},
		{"{title", statusYouTube, "{title"},
		{"}{title}{", statusYouTube, "}Let It Happen{"},
		{"{{title}}", statusYouTube, "{Let It Happen}"},
		{"{a {title}", statusYouTube, "{a Let It Happen"},
		{"{}", statusYouTube, "{}"},
		{"Now playing", statusYouTube, "Now playing"},
		{"", statusYouTube, ""},
		{"{album}", single, ""},
		// The track's own braces stay.
		{"{title}", media.Track{Title: "{artist}"}, "{artist}"},
	}
	for _, c := range cases {
		if got := ExpandDiscordLine(c.template, c.track); got != c.want {
			t.Errorf("%q of %q = %q, want %q", c.template, c.track.Title, got, c.want)
		}
	}
}

// TestUnknownDiscordPlaceholders names what, in braces, isn't a
// placeholder, once each.
func TestUnknownDiscordPlaceholders(t *testing.T) {
	cases := map[string][]string{
		"{title}":                       nil,
		"{Artist} · {album} {service}":  nil,
		"{year} {title} {year} {genre}": {"year", "genre"},
		"{}":                            {""},
		"{title":                        nil,
		"{x {y}":                        {"y"},
	}
	for template, want := range cases {
		if got := UnknownDiscordPlaceholders(template); !slices.Equal(got, want) {
			t.Errorf("%q: %q, want %q", template, got, want)
		}
	}
}

// TestDiscordBadge tells the badges from the other images.
func TestDiscordBadge(t *testing.T) {
	for image, want := range map[string]string{
		discordBadges + "spotify.png":       BadgeSpotify,
		discordBadges + "youtube-music.png": BadgeYouTube,
		discordBadges + "computer.png":      BadgeComputer,
		discordBadges + "paused.png":        BadgePaused,
		discordBadges + "other.png":         "",
		discordBadges + "spotify":           "",
		"https://example.com/spotify.png":   "",
		"":                                  "",
	} {
		if got := DiscordBadge(image); got != want {
			t.Errorf("DiscordBadge(%q) = %q, want %q", image, got, want)
		}
	}
}

// statusDiscord is a Discord that hands the test each activity it shows,
// nil for a Clear.
type statusDiscord struct{ shown chan *discord.Activity }

func (f *statusDiscord) SetActivity(_ context.Context, a discord.Activity) error {
	f.shown <- &a
	return nil
}

func (f *statusDiscord) Clear(context.Context) error {
	f.shown <- nil
	return nil
}

func (f *statusDiscord) Close() error { return nil }

// TestPresenceFollowsTheChoices shows the track again once what the
// status shows is chosen otherwise, without waiting for another track: a
// line, a switch, and keeping the track while paused.
func TestPresenceFollowsTheChoices(t *testing.T) {
	f := &statusDiscord{shown: make(chan *discord.Activity, 8)}
	was := newPresenceClient
	newPresenceClient = func(string) presenceClient { return f }
	t.Cleanup(func() { newPresenceClient = was })
	var p presence
	next := func(what string) (*discord.Activity, bool) {
		t.Helper()
		select {
		case a := <-f.shown:
			for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
				p.mu.Lock()
				busy := p.busy
				p.mu.Unlock()
				if !busy {
					return a, true
				}
				if time.Now().After(deadline) {
					t.Fatalf("%s: Discord's call didn't return", what)
				}
			}
		case <-time.After(100 * time.Millisecond):
			return nil, false
		}
	}
	track := statusYouTube
	plays := NowPlaying{Track: &track, Playing: true}
	paused := NowPlaying{Track: &track}

	p.update(desktop(nil), plays)
	if a, ok := next("playing"); !ok || a == nil || a.Details != "Let It Happen" {
		t.Fatalf("playing, Discord shows %+v", a)
	}
	p.update(desktop(nil), plays)
	if a, ok := next("again"); ok {
		t.Errorf("the same report shows %+v again", a)
	}
	lines := desktop(func(d *settings.Desktop) { d.DiscordFirstLine = "{title} · {album}" })
	p.update(lines, plays)
	if a, ok := next("another line"); !ok || a == nil || a.Details != "Let It Happen · Currents" {
		t.Errorf("after the line changed, Discord shows %+v, %v", a, ok)
	}
	button := desktop(func(d *settings.Desktop) { d.DiscordFirstLine, d.DiscordListenButton = "{title} · {album}", true })
	p.update(button, plays)
	if a, ok := next("a button"); !ok || a == nil || len(a.Buttons) != 1 {
		t.Errorf("after the button was turned on, Discord shows %+v, %v", a, ok)
	}
	p.update(button, paused)
	if a, ok := next("paused"); !ok || a != nil {
		t.Errorf("paused, Discord shows %+v, %v, not nothing", a, ok)
	}
	kept := desktop(func(d *settings.Desktop) { d.DiscordKeepWhilePaused = true })
	p.update(kept, paused)
	if a, ok := next("kept"); !ok || a == nil || a.SmallText != "Paused" || !a.Start.IsZero() {
		t.Errorf("kept while paused, Discord shows %+v, %v", a, ok)
	}
	p.update(kept, paused)
	if a, ok := next("still paused"); ok {
		t.Errorf("still paused, Discord is told %+v again", a)
	}
	p.update(kept, plays)
	if a, ok := next("playing again"); !ok || a == nil || a.SmallImage != "" || a.Start.IsZero() {
		t.Errorf("playing again, Discord shows %+v, %v", a, ok)
	}
	p.update(kept, NowPlaying{})
	if a, ok := next("stopped"); !ok || a != nil {
		t.Errorf("stopped, Discord shows %+v, %v", a, ok)
	}
}
