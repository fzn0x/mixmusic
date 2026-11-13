package main

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/sharecard"
	"github.com/fzn0x/mixmusic/locales"
)

// Your year, a tab of the Stats page, looks back at one calendar year of
// the user's own history: its minutes, top songs, artists and albums, and
// a few moments of it. Save image makes a card of it, as a square or a
// story, with the share cards' renderer (internal/sharecard).

// recapLabel is the name of the tab.
func recapLabel() string { return locales.RecapLabel() }

// recapClock is the time the recap goes by; tests set it.
var recapClock = time.Now

// recapYears lists the years whose recap the user can open, the latest
// first: the past years the history has plays of, and the current year
// from December.
func (s *shell) recapYears() []int {
	now := recapClock()
	var years []int
	for _, y := range s.svc.Collection.RecapYears() {
		if y < now.Year() || y == now.Year() && now.Month() == time.December {
			years = append(years, y)
		}
	}
	return years
}

// recapView is the recap of the year the user chose among years.
func (s *shell) recapView(c *ui.Context, years []int) {
	k := &s.lk
	if s.recapPick >= len(years) {
		s.recapPick = 0
	}
	if len(years) > 1 {
		labels := make([]string, len(years))
		for i, y := range years {
			labels[i] = strconv.Itoa(y)
		}
		ui.Box(c).Margin(0, 0, 24).Children(func() { s.chips(c, labels, nil, &s.recapPick) })
	}
	r := s.svc.Collection.Recap(years[s.recapPick])
	size := k.subtitleSize()
	ui.RichText(c,
		ui.Span{Text: locales.RecapYouPlayed(strconv.Itoa(r.Year)), Size: size, Weight: 300},
		ui.Span{Text: locales.RecapTracksPlayed(r.Plays), Size: size, Weight: 400},
		ui.Span{Text: locales.RecapFor(), Size: size, Weight: 300},
		ui.Span{Text: locales.RecapMinutes(thousands(r.Minutes)), Size: size, Weight: 400},
		ui.Span{Text: locales.RecapPeriod(), Size: size, Weight: 300},
	).MaxWidth(760).Margin(0, 0, 32)

	ui.Row(c).Gap(24).Wrap().AlignItems(ui.Start).Margin(0, 0, 40).Children(func() {
		songs := make([]recapRow, len(r.TopTracks))
		for i, t := range r.TopTracks {
			songs[i] = recapRow{t.Track.Title, names(t.Track.Artists), t.Plays}
		}
		artists := make([]recapRow, len(r.TopArtists))
		for i, a := range r.TopArtists {
			artists[i] = recapRow{a.Artist.Name, "", a.Plays}
		}
		albums := make([]recapRow, len(r.TopAlbums))
		for i, a := range r.TopAlbums {
			albums[i] = recapRow{a.Album.Name, a.Artist, a.Plays}
		}
		s.recapList(c, locales.RecapTopSongs(), songs)
		s.recapList(c, locales.RecapTopArtists(), artists)
		s.recapList(c, locales.RecapTopAlbums(), albums)
	})

	if facts := recapFacts(r); len(facts) > 0 {
		s.sectionTitle(c, locales.RecapMoments())
		ui.Column(c).Gap(8).Margin(0, 0, 40).Children(func() {
			for _, f := range facts {
				ui.Text(c, f).FontSize(k.bodySize()).MaxWidth(760)
			}
		})
	}

	s.sectionTitle(c, locales.RecapShare())
	ui.Row(c).Gap(8).Wrap().Margin(0, 0, 24).Children(func() {
		if s.button(c, "recap-save", locales.RecapSaveImage(), true).Clicked() {
			s.saveCard(recapFileName(r, sharecard.Square), nil, sharecard.Square, s.recapCard(r))
		}
		if s.button(c, "recap-save-story", locales.RecapSaveStory(), false).Clicked() {
			s.saveCard(recapFileName(r, sharecard.Story), nil, sharecard.Story, s.recapCard(r))
		}
	})
}

// recapRow is a row of a ranking: its name, what goes with it and its
// plays.
type recapRow struct {
	name, detail string
	plays        int
}

// recapList shows a ranking, the first row first.
func (s *shell) recapList(c *ui.Context, title string, rows []recapRow) {
	k := &s.lk
	ui.Column(c).Grow(1).Basis(240).MinWidth(0).Gap(10).Children(func() {
		s.sectionTitle(c, title)
		if len(rows) == 0 {
			ui.Text(c, locales.RecapNothingToRank()).FontSize(k.smallSize()).TextColor(k.muted)
			return
		}
		for i, row := range rows {
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Text(c, strconv.Itoa(i+1)).FontSize(k.bodySize()).TextColor(k.muted).FontFeatures("tnum").Width(20)
				ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
					ui.Text(c, row.name).FontSize(k.bodySize()).SingleLine()
					detail := locales.RecapPlays(row.plays)
					if row.detail != "" {
						detail = locales.RecapDetail(row.detail, row.plays)
					}
					ui.Text(c, detail).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
				})
			})
		}
	})
}

// recapFacts tells the moments of the year in sentences.
func recapFacts(r library.Recap) []string {
	var out []string
	if r.BusiestMonth > 0 {
		out = append(out, locales.RecapBusiest(monthName(time.Date(r.Year, r.BusiestMonth, 1, 0, 0, 0, 0, time.UTC), true), r.BusiestMinutes))
	}
	if r.First != nil {
		out = append(out, locales.RecapBegan(quoted(r.First.Track.Title, names(r.First.Track.Artists)),
			dayMonth(r.First.PlayedAt.In(recapClock().Location()), true)))
	}
	if r.Repeated != nil {
		out = append(out, locales.RecapRepeated(quoted(r.Repeated.Track.Title, names(r.Repeated.Track.Artists)),
			strconv.Itoa(r.Repeated.Plays), recapDay(r.Repeated.Date)))
	}
	if r.NewKnown {
		out = append(out, locales.RecapFound(r.NewArtists))
	}
	return out
}

// quoted writes a song as "Title" by Artist.
func quoted(title, artist string) string {
	if artist == "" {
		return "\u201c" + title + "\u201d"
	}
	return locales.RecapSongBy(title, artist)
}

// recapDay writes a day of the recap as "5 March".
func recapDay(date string) string {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return dayMonth(t, true)
}

// recapFileName is what the card of r in size is called at first: the
// story's name says so, so that both saved in one folder keep apart.
func recapFileName(r library.Recap, size sharecard.Size) string {
	if size == sharecard.Story {
		return fmt.Sprintf("MixMusic %d story", r.Year)
	}
	return fmt.Sprintf("MixMusic %d", r.Year)
}

// recapCard builds the card of r in the colours of the current theme: the
// year's minutes and top five songs on a square, and its top artists and
// albums too on a story, which has the room.
func (s *shell) recapCard(r library.Recap) cardBuilder { return recapCardOf(r, s.lk.cardColors()) }

// recapCardOf is recapCard in colors.
func recapCardOf(r library.Recap, colors sharecard.Colors) cardBuilder {
	return func(_ image.Image, size sharecard.Size) sharecard.Card {
		c := sharecard.Card{
			Size: size, Colors: colors,
			Lines: []sharecard.Line{
				{Text: locales.RecapCardHeading(strconv.Itoa(r.Year)), Size: 34, Weight: 600, Ink: colors.Accent},
				{Text: thousands(r.Minutes), Size: 170, Weight: 700, Gap: 10},
				{Text: locales.RecapCardMinutes(r.Plays), Size: 42, Weight: 400, Ink: colors.Muted, MaxLines: 2},
			},
			Foot: []sharecard.Line{{Text: "MixMusic", Size: 34, Weight: 600, Ink: colors.Accent}},
		}
		section := func(title string, rows []recapRow) {
			if len(rows) == 0 {
				return
			}
			c.Lines = append(c.Lines, sharecard.Line{Text: strings.ToUpper(title), Size: 28, Weight: 600, Ink: colors.Accent, Gap: 44})
			for i, row := range rows {
				text := fmt.Sprintf("%d  %s", i+1, row.name)
				c.Lines = append(c.Lines, sharecard.Line{Text: text, Size: 44, Weight: 600, Gap: 16})
				if row.detail != "" {
					c.Lines = append(c.Lines, sharecard.Line{Text: row.detail, Size: 30, Weight: 400, Ink: colors.Muted})
				}
			}
		}
		var songs, artists, albums []recapRow
		for _, t := range r.TopTracks {
			songs = append(songs, recapRow{t.Track.Title, names(t.Track.Artists), t.Plays})
		}
		section(locales.RecapTopSongs(), songs)
		if size.H > size.W {
			for _, a := range r.TopArtists {
				artists = append(artists, recapRow{name: a.Artist.Name, plays: a.Plays})
			}
			for _, a := range r.TopAlbums {
				albums = append(albums, recapRow{a.Album.Name, a.Artist, a.Plays})
			}
			section(locales.RecapTopArtists(), artists)
			section(locales.RecapTopAlbums(), albums)
		}
		return c
	}
}
