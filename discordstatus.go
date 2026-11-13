package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/discord"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// What the Discord status shows is chosen under Show what plays on Discord
// (#352): a preview of the status as Discord's card has it, its two lines,
// and switches for the cover, the service's badge, the time left, a
// button that opens the track, and keeping it while paused.

func init() { addSettings(desktopRows, 5, (*shell).discordStatusRows) }

// The keys of the inputs of the status's lines, whose drafts the preview
// shows as they are typed.
const (
	discordFirstLineKey  = "discord-first-line"
	discordSecondLineKey = "discord-second-line"
)

// discordStatusRows choose what the Discord status shows, while it shows.
func (s *shell) discordStatusRows(c *ui.Context) {
	if !s.set.DiscordPresence {
		return
	}
	shown := s.discordLook()
	s.found(c, locales.DiscordPreview(), discordPreviewWords, func() { s.discordPreview(c, shown) })
	s.discordLineRow(c, discordFirstLineKey, locales.DiscordFirstLine(), &s.set.DiscordFirstLine, settings.DefaultDiscordFirstLine)
	s.discordLineRow(c, discordSecondLineKey, locales.DiscordSecondLine(), &s.set.DiscordSecondLine, settings.DefaultDiscordSecondLine)
	toggle := func(key, title, label, description string, v *bool) {
		s.row(c, title, description, func() {
			if s.switchOf(c.Key(key), v).Label(label).Changed() {
				s.save()
			}
		}, "discord status rich presence")
	}
	toggle("discord-cover", locales.DiscordCover(), locales.DiscordCoverLabel(), locales.DiscordCoverDescription(), &s.set.DiscordCover)
	toggle("discord-badge", locales.DiscordServiceBadge(), locales.DiscordServiceBadgeLabel(), locales.DiscordServiceBadgeDescription(), &s.set.DiscordServiceBadge)
	toggle("discord-time", locales.DiscordTimeLeft(), locales.DiscordTimeLeftLabel(), locales.DiscordTimeLeftDescription(), &s.set.DiscordTimeLeft)
	toggle("discord-listen", locales.DiscordListenButton(), locales.DiscordListenButtonLabel(), locales.DiscordListenButtonDescription(), &s.set.DiscordListenButton)
	toggle("discord-paused", locales.DiscordKeepWhilePaused(), locales.DiscordKeepWhilePausedLabel(), locales.DiscordKeepWhilePausedDescription(), &s.set.DiscordKeepWhilePaused)
	// The preview came before the lines: what was typed since shows in
	// the next frame.
	if typed := s.discordLook(); typed.DiscordFirstLine != shown.DiscordFirstLine || typed.DiscordSecondLine != shown.DiscordSecondLine {
		c.Invalidate()
	}
}

// discordPreviewWords find the preview in the search of the settings.
const discordPreviewWords = "What your Discord profile shows of what plays, as you choose it below. discord status rich presence card"

// discordLook is the settings of the Discord status, with the lines as
// they are typed.
func (s *shell) discordLook() settings.Desktop {
	d := s.set.Desktop
	if dr := s.drafts[discordFirstLineKey]; dr != nil && dr.focused {
		d.DiscordFirstLine = dr.text
	}
	if dr := s.drafts[discordSecondLineKey]; dr != nil && dr.focused {
		d.DiscordSecondLine = dr.text
	}
	return d
}

// discordLineRow is the row of a line of the status: a template, which
// tells the placeholders, and those typed that aren't any.
func (s *shell) discordLineRow(c *ui.Context, key, title string, line *string, def string) {
	k := &s.lk
	text := *line
	if dr := s.drafts[key]; dr != nil && dr.focused {
		text = dr.text
	}
	describe := func() {
		names := make([]string, len(app.DiscordPlaceholders))
		for i, name := range app.DiscordPlaceholders {
			names[i] = "{" + name + "}"
		}
		ui.Text(c, locales.DiscordPlaceholders(strings.Join(names, ", ")))
		if unknown := app.UnknownDiscordPlaceholders(text); len(unknown) > 0 {
			ui.Text(c, discordUnknownNote(unknown)).TextColor(k.danger)
		}
	}
	s.rowWith(c, title, describe, func() {
		if *line != def && s.link(c, key+"-reset", locales.DiscordLineReset()) {
			*line = def
			delete(s.drafts, key)
			s.save()
		}
		in, _ := s.settingInput(c, key, *line, func(v string) {
			*line = v
			s.save()
		})
		in.Placeholder(locales.DiscordLinePlaceholder()).Width(260).MaxLength(settings.DiscordLineMax).Label(locales.DiscordLineLabel(title))
	}, "discord status rich presence template placeholders title artist album service")
}

// discordUnknownNote tells that names in braces are no placeholders.
func discordUnknownNote(names []string) string {
	braced := make([]string, len(names))
	for i, n := range names {
		braced[i] = "{" + n + "}"
	}
	return locales.DiscordUnknownNote(len(braced), strings.Join(braced, ", "))
}

// discordSample is the track the preview shows while nothing plays.
func discordSample() media.Track {
	return media.Track{ID: "yt:sample", Source: media.SourceYouTube, Title: locales.DiscordSampleTitle(),
		Artists: []media.Ref{{Name: locales.DiscordSampleArtist()}}, Album: &media.Ref{Name: locales.DiscordSampleAlbum()}, DurationMs: 214000}
}

// discordPreview shows what Discord shows of what plays, as d chooses, as
// a card of Discord's: the application's name, the cover with its badge,
// the two lines, the time, and the button. While nothing plays, it shows
// a song of the sample's; while a track is paused that the status doesn't
// keep, it shows it as it will once it plays again.
func (s *shell) discordPreview(c *ui.Context, d settings.Desktop) {
	k := &s.lk
	np, note := s.discordNowPlaying()
	now := time.Now()
	a := app.DiscordActivity(d, np, now)
	if a == nil {
		note = locales.DiscordPreviewPausedNote()
		np.Playing = true
		a = app.DiscordActivity(d, np, now)
	}
	if a == nil {
		return
	}
	name := "MixMusic"
	if d.DiscordAppID != "" {
		name = locales.DiscordYourApplication()
	}
	ui.Column(c).Gap(10).Padding(16, 0).Children(func() {
		ui.Text(c, locales.DiscordPreview()).FontSize(k.bodySize())
		card := ui.Column(c.Key("discord-preview")).Gap(12).Padding(16).Width(360).MaxWidth(360).Shrink(1).
			Background(k.surface2).Radius(k.radius).Label(locales.DiscordPreviewCardLabel())
		card.Children(func() {
			ui.Text(c, locales.DiscordListeningTo(name)).FontSize(k.captionSize()).FontWeight(600).TextColor(k.muted)
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				if a.LargeImage != "" {
					s.discordPicture(c, a)
				}
				ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
					if a.Details != "" {
						ui.Text(c, a.Details).FontSize(k.bodySize()).FontWeight(600).SingleLine()
					}
					if a.State != "" {
						ui.Text(c, a.State).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
					}
					s.discordTimes(c, a, now)
				})
			})
			for i, b := range a.Buttons {
				ui.Box(c.Key("discord-button-"+count(i))).FillWidth().Padding(8, 12).Radius(k.radiusSm).
					Background(k.surface3).Tooltip(b.URL).Children(func() {
					ui.Text(c, b.Label).FontSize(k.smallSize()).TextAlign(ui.Center).FillWidth()
				})
			}
		})
		if note != "" {
			ui.Text(c, note).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(520)
		}
	})
}

// discordNowPlaying is what plays, as the app tells Discord of it, or the
// sample while nothing does, with a note that says so.
func (s *shell) discordNowPlaying() (app.NowPlaying, string) {
	cur, ok := s.st.Current()
	if !ok {
		t := discordSample()
		return app.NowPlaying{Track: &t, Playing: true, PositionMs: 72000},
			locales.DiscordNothingPlays()
	}
	t := cur.Track
	return app.NowPlaying{Track: &t, Playing: s.st.Playing, Loading: s.st.Loading,
		PositionMs: int(s.st.Position / time.Millisecond), DurationMs: int(s.st.Duration / time.Millisecond),
		Live: s.st.Live, Speed: s.st.Speed}, ""
}

// discordPicture is the activity's picture, the cover or a badge, with
// the badge at its corner; each tells its text as it is hovered, as on
// Discord.
func (s *shell) discordPicture(c *ui.Context, a *discord.Activity) {
	k := &s.lk
	const size, small = 72, 26
	frame := ui.Box(c.Key("discord-picture")).Size(size+6, size+6).Shrink(0)
	frame.Children(func() {
		pic := ui.Box(c.Key("discord-cover")).Size(size, size).Radius(k.radiusSm).Clip().Background(k.surface3)
		if a.LargeText != "" {
			pic.Tooltip(a.LargeText)
		}
		pic.Children(func() {
			if badge := app.DiscordBadge(a.LargeImage); badge != "" {
				discordBadgeBox(c, "discord-large-badge", badge, a.LargeText, size, false)
				return
			}
			if b := s.art.get(c, a.LargeImage, s.artPixels(size)); b != nil {
				ui.Image(c, b).Size(size, size).Fit(ui.Cover)
			}
		})
		if badge := app.DiscordBadge(a.SmallImage); badge != "" {
			// A ring of the card's colour parts the badge from the cover.
			ring := ui.Box(c.Key("discord-small")).Absolute().Right(0).Bottom(0).Size(small+4, small+4).
				Radius((small + 4) / 2).Background(k.surface2).Padding(2)
			if a.SmallText != "" {
				ring.Tooltip(a.SmallText)
			}
			ring.Children(func() { discordBadgeBox(c, "discord-small-badge", badge, a.SmallText, small, true) })
		}
	})
}

// The colours of the badges, as tools/discordbadges draws them.
var (
	discordBadgeGround = ui.RGB(0x30, 0x30, 0x30)
	discordBadgeInks   = map[string]ui.Color{
		app.BadgeSpotify: ui.RGB(0x1e, 0xd7, 0x60), app.BadgeYouTube: ui.RGB(0xff, 0x40, 0x40),
		app.BadgeComputer: ui.RGB(0xff, 0xff, 0xff), app.BadgePaused: ui.RGB(0xff, 0xff, 0xff),
	}
)

// discordBadgeBox draws the badge name at size, round as Discord shows a
// small image, labelled with its text.
func discordBadgeBox(c *ui.Context, key, name, label string, size float32, round bool) {
	radius := float32(0)
	if round {
		radius = size / 2
	}
	ui.Box(c.Key(key)).Size(size, size).Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, discordBadgeGround, radius)
		ink := discordBadgeInks[name]
		at := func(x0, y0, x1, y1 float32) ui.Rect {
			return ui.Rect{X: r.X + x0*r.W, Y: r.Y + y0*r.H, W: (x1 - x0) * r.W, H: (y1 - y0) * r.H}
		}
		switch name {
		case app.BadgeSpotify:
			p.Fill(at(0.25, 0.25, 0.75, 0.75), ink, r.W/4)
		case app.BadgeYouTube:
			p.Fill(at(0.29, 0.29, 0.71, 0.71), ink, 0)
		case app.BadgeComputer:
			var ring ui.Path
			ring.Circle(r.X+r.W/2, r.Y+r.H/2, 0.235*r.W)
			p.StrokePath(&ring, 0.09*r.W, ink)
			p.Fill(at(0.44, 0.44, 0.56, 0.56), ink, 0.06*r.W)
		case app.BadgePaused:
			p.Fill(at(0.35, 0.29, 0.45, 0.71), ink, 0)
			p.Fill(at(0.55, 0.29, 0.65, 0.71), ink, 0)
		}
	})
}

// discordTimes is the time of the activity at now, as Discord shows it:
// the time played, a bar and the length, or how long it played, of a
// track of no end; nothing without a start.
func (s *shell) discordTimes(c *ui.Context, a *discord.Activity, now time.Time) {
	k := &s.lk
	if a.Start.IsZero() {
		return
	}
	played := now.Sub(a.Start)
	if a.End.IsZero() {
		ui.Text(c, locales.DiscordElapsed(clock(played))).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum")
		return
	}
	length := a.End.Sub(a.Start)
	f := float32(0)
	if length > 0 {
		f = float32(max(0, min(1, played.Seconds()/length.Seconds())))
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(4, 0, 0).Children(func() {
		ui.Text(c, clock(played)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum")
		ui.Box(c).Grow(1).Height(4).Label(locales.DiscordTimeBarLabel()).Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, k.border, r.H/2)
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * f, H: r.H}, k.text, r.H/2)
		})
		ui.Text(c, clock(length)).FontSize(k.captionSize()).TextColor(k.muted).FontFeatures("tnum")
	})
}
