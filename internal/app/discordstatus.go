package app

import (
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/discord"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The Discord status shows what plays as Settings › Desktop chooses (#352):
// two lines made from templates, the cover, a badge of the track's
// service, the time left, a button that opens the track on its service,
// and, while paused, the track marked as paused or nothing.

// DiscordPlaceholders are the placeholders of the lines of the Discord
// status, which name the track's title, artists, album and service.
var DiscordPlaceholders = []string{"title", "artist", "album", "service"}

// discordBadges is where the badges of the Discord status are, which
// Discord reads: brand/discord's images, from GitHub, so that they show
// whatever the Discord application, the user's own too.
const discordBadges = "https://raw.githubusercontent.com/fzn0x/mixmusic/main/brand/discord/"

// The badges of the Discord status, by the names of their files: the
// services' and the one of a paused track.
const (
	BadgeYouTube  = "youtube-music"
	BadgeSpotify  = "spotify"
	BadgeComputer = "computer"
	BadgePaused   = "paused"
)

// DiscordBadge names the badge that image, an image of a Discord activity,
// is, as BadgeSpotify, or "" for another image, as a cover.
func DiscordBadge(image string) string {
	file, ours := strings.CutPrefix(image, discordBadges)
	name, png := strings.CutSuffix(file, ".png")
	if ours && png && slices.Contains([]string{BadgeYouTube, BadgeSpotify, BadgeComputer, BadgePaused}, name) {
		return name
	}
	return ""
}

// discordBadgeImage is the address of the badge name.
func discordBadgeImage(name string) string { return discordBadges + name + ".png" }

// DiscordActivity is what the Discord status shows of np at now, as d
// chooses: nil while nothing plays, and while it is paused, but where d
// keeps it then. Discord's times are the clock's: at 2×, a track 1 minute
// in started half a minute ago, and ends in half the time left. A live
// stream has no end: Discord tells how long it played.
func DiscordActivity(d settings.Desktop, np NowPlaying, now time.Time) *discord.Activity {
	if np.Track == nil {
		return nil
	}
	playing := np.plays()
	if !playing && !d.DiscordKeepWhilePaused {
		return nil
	}
	t := *np.Track
	a := &discord.Activity{
		Type:    discord.Listening,
		Details: ExpandDiscordLine(d.DiscordFirstLine, t),
		State:   ExpandDiscordLine(d.DiscordSecondLine, t),
		// The member list says "Listening to MixMusic", whether the track
		// is Spotify's or YouTube Music's.
		StatusDisplay: discord.ShowName,
	}
	if playing && d.DiscordTimeLeft {
		a.Start = presenceStart(np, now)
		durationMs := np.DurationMs
		switch {
		case np.Live:
			durationMs = 0
		case durationMs == 0:
			durationMs = t.DurationMs
		}
		if durationMs > 0 {
			a.End = a.Start.Add(time.Duration(float64(durationMs) * float64(time.Millisecond) / np.speed()))
		}
	}
	if d.DiscordCover {
		a.LargeImage = largeArtwork(t.Artwork)
		if t.Album != nil {
			a.LargeText = t.Album.Name
		}
	}
	switch badge := discordServiceBadge(t); {
	case !playing:
		a.SmallImage, a.SmallText = discordBadgeImage(BadgePaused), "Paused"
	case d.DiscordServiceBadge && badge != "":
		a.SmallImage, a.SmallText = discordBadgeImage(badge), discordBadgeText(t)
	}
	// Discord shows the small image on the cover alone: without one, the
	// badge takes its place.
	if a.LargeImage == "" && a.SmallImage != "" {
		a.LargeImage, a.LargeText, a.SmallImage, a.SmallText = a.SmallImage, a.SmallText, "", ""
	}
	if label, link := discordListen(t); d.DiscordListenButton && link != "" {
		a.Buttons = []discord.Button{{Label: label, URL: link}}
	}
	return a
}

// presenceStart is when the track of np would have started, at its
// position, for it to be there now: earlier at a lower speed.
func presenceStart(np NowPlaying, now time.Time) time.Time {
	return now.Add(-time.Duration(float64(np.PositionMs) * float64(time.Millisecond) / np.speed()))
}

// ExpandDiscordLine is a line of the Discord status, template, of t: each
// placeholder, as {title}, in braces, is t's, whatever its case. What
// isn't a placeholder shows as typed, braces too. Where a placeholder has
// nothing to show, as a song of no album, the separators it leaves at the
// line's ends go, as the " · " of "{artist} · {album}".
func ExpandDiscordLine(template string, t media.Track) string {
	var b strings.Builder
	empty := false
	eachDiscordField(template, func(text, name string, field bool) {
		b.WriteString(text)
		v, ok := discordPlaceholder(name, t)
		switch {
		case !field:
		case !ok:
			b.WriteString("{" + name + "}")
		case v == "":
			empty = true
		default:
			b.WriteString(v)
		}
	})
	line := strings.TrimSpace(b.String())
	if empty {
		line = strings.Trim(line, discordSeparators)
	}
	return line
}

// discordSeparators are what may part two placeholders of a line, which a
// placeholder with nothing to show leaves at the line's ends.
const discordSeparators = " \t·•-–—|,;:/~"

// UnknownDiscordPlaceholders returns the names in braces in template that
// are no placeholder, in their order, each once: they show as typed.
func UnknownDiscordPlaceholders(template string) []string {
	var unknown []string
	eachDiscordField(template, func(_, name string, field bool) {
		if _, ok := discordPlaceholder(name, media.Track{}); field && !ok && !slices.Contains(unknown, name) {
			unknown = append(unknown, name)
		}
	})
	return unknown
}

// eachDiscordField calls f with each name in braces of template, and the
// text before it, field set, then with the text after the last, field
// unset. A brace that opens another before it closes, or never closes, is
// text.
func eachDiscordField(template string, f func(text, name string, field bool)) {
	rest := template
	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			break
		}
		end := strings.IndexAny(rest[open+1:], "{}")
		if end < 0 {
			break
		}
		end += open + 1
		if rest[end] == '{' {
			f(rest[:end], "", false)
			rest = rest[end:]
			continue
		}
		f(rest[:open], rest[open+1:end], true)
		rest = rest[end+1:]
	}
	f(rest, "", false)
}

// discordPlaceholder returns what the placeholder name shows of t, and
// whether name is one.
func discordPlaceholder(name string, t media.Track) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "title":
		return t.Title, true
	case "artist":
		return t.ArtistNames(), true
	case "album":
		if t.Album == nil {
			return "", true
		}
		return t.Album.Name, true
	case "service":
		return discordServiceName(t), true
	}
	return "", false
}

// discordSource is the service t comes from, which its ID tells when the
// track doesn't.
func discordSource(t media.Track) media.Source {
	if t.Source != "" {
		return t.Source
	}
	source, _, _ := media.ParseID(t.ID)
	return source
}

// discordServiceName names the service of t, as the Discord status shows
// it to the others.
func discordServiceName(t media.Track) string {
	switch discordSource(t) {
	case media.SourceYouTube:
		return "YouTube Music"
	case media.SourceSpotify:
		return "Spotify"
	case media.SourceFile:
		return "my computer"
	}
	return "MixMusic"
}

// discordBadgeText is what the badge of t's service tells as it is
// hovered.
func discordBadgeText(t media.Track) string {
	if discordSource(t) == media.SourceFile {
		return "A file on my computer"
	}
	return discordServiceName(t)
}

// discordServiceBadge names the badge of t's service, "" for none.
func discordServiceBadge(t media.Track) string {
	switch discordSource(t) {
	case media.SourceYouTube:
		return BadgeYouTube
	case media.SourceSpotify:
		return BadgeSpotify
	case media.SourceFile:
		return BadgeComputer
	}
	return ""
}

// discordListen is the label and the link of the button that opens t on
// its service, or none for a track of no service, as one on this
// computer.
func discordListen(t media.Track) (label, link string) {
	source := discordSource(t)
	switch source {
	case media.SourceYouTube, media.SourceSpotify:
		label = "Listen on " + discordServiceName(t)
	default:
		return "", ""
	}
	if strings.HasPrefix(t.URL, "https://") {
		return label, t.URL
	}
	_, native, err := media.ParseID(t.ID)
	if err != nil {
		return "", ""
	}
	if source == media.SourceYouTube {
		return label, "https://music.youtube.com/watch?v=" + url.QueryEscape(native)
	}
	kind := "track"
	if t.IsEpisode() {
		kind = "episode"
	}
	return label, "https://open.spotify.com/" + kind + "/" + url.PathEscape(native)
}
