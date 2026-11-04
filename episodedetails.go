package main

// An episode's details, where a song shows its lyrics: the side panel, the
// now playing page and the notch player show the show, views, day and
// length of the episode that plays, and its description, whose times play
// it from there and whose links open in the browser.

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// lyricsTitle names what shows where the lyrics do: an episode's details,
// or the lyrics of a song.
func (s *shell) lyricsTitle() string {
	if cur, ok := s.st.Current(); ok && cur.Track.IsEpisode() {
		return "Details"
	}
	return "Lyrics"
}

// lyricsIcon is the icon of the button that shows the lyrics, or an
// episode's details.
func (s *shell) lyricsIcon() string {
	if cur, ok := s.st.Current(); ok && cur.Track.IsEpisode() {
		return "details"
	}
	return "lyrics"
}

// episodeDetailsOf fetches the details of the episode t: Catalog keeps
// them, so a refresh doesn't ask its service again.
func (s *shell) episodeDetailsOf(t media.Track) (string, *loaded[media.EpisodeDetails]) {
	key := "episode-details:" + t.ID
	return key, fetch(s, key, func(ctx context.Context) (media.EpisodeDetails, error) { return s.svc.Catalog.EpisodeDetails(ctx, t) })
}

// openWebPage opens a link of a description in the browser; tests replace
// it.
var openWebPage = func(s *shell, u string) { go s.svc.System.OpenExternal(u) }

// episodeDetails shows the details of the episode t, which plays: what it
// is of, then its description, line by line, the line of the chapter that
// plays lit with the accent. Only the lines near the view are built. large
// sets them on the now playing page; while they load, or when they fail,
// it says so as the lyrics do.
func (s *shell) episodeDetails(c *ui.Context, t media.Track, large bool) {
	k := &s.lk
	where := pick(large, "page", "panel")
	key, l := s.episodeDetailsOf(t)
	if !l.done || l.err != nil {
		s.episodeFacts(c, t, media.EpisodeDetails{})
		if !l.done {
			s.emptyIn(c, "Looking for the details…")
			return
		}
		s.emptyIn(c, l.err.Error())
		// An episode without details stays so; a failure may pass.
		if !errors.Is(l.err, app.ErrNoEpisodeDetails) {
			ui.Row(c).Padding(0, 24, 8).Children(func() {
				if s.button(c, "details-retry-"+where, "try again", false).Clicked() {
					s.forget(key)
				}
			})
		}
		return
	}
	d := l.value
	limit := s.seekLimit(t, d)
	playing := -1
	if at := chapterAt(d.Chapters, s.st.Position+lyricsLead(s.st.Speed), limit); at >= 0 {
		playing = d.Chapters[at].Line
	}
	lines := d.Description
	box := ui.Column(c.Key("details-" + where + "-" + t.ID)).Grow(1).MinHeight(0)
	list := ui.Local(box, "list", func() ui.ListState { return ui.ListState{} })
	box.Children(func() {
		ui.List(c.Key("details-list-"+where), list, 1+max(len(lines), 1), func(i int) {
			switch {
			case i == 0:
				s.episodeFacts(c, t, d)
			case len(lines) == 0:
				s.emptyIn(c, "This episode has no description.")
			case blankLine(lines[i-1]):
				ui.Box(c).Height(8)
			default:
				ink := k.text
				if i-1 == playing {
					ink = k.accent
				}
				s.descriptionLine(c, lines[i-1], ink, limit)
			}
		}).Grow(1).MinHeight(0)
	})
}

// episodeFacts says what show the episode t is of, and its views, day and
// length: those of its details d, or its own while d tells none. The
// show's name opens its page.
func (s *shell) episodeFacts(c *ui.Context, t media.Track, d media.EpisodeDetails) {
	k := &s.lk
	show, day, length, plays := d.Show, d.Released, d.DurationMs, d.Plays
	if e := t.Episode; e != nil {
		if show.Name == "" {
			show = e.Show
		}
		if day == "" {
			day = e.Released
		}
	}
	if length <= 0 {
		length = t.DurationMs
	}
	if plays <= 0 {
		plays = t.Plays
	}
	var facts []string
	if v := playsLabel(media.Track{Video: true, Plays: plays}); v != "" {
		facts = append(facts, v)
	}
	if v := releasedLabel(day, time.Now()); v != "" {
		facts = append(facts, v)
	}
	if length > 0 {
		facts = append(facts, longDuration(ms(length)))
	}
	ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(4, 24, 12).Children(func() {
		sep := ""
		if show.Name != "" {
			if show.ID == "" {
				ui.Text(c, show.Name)
			} else if s.linkOf(c, show.Name, "").Clicked() {
				s.navigate(podcastPath(show.ID))
			}
			sep = " · "
		}
		for _, f := range facts {
			ui.Text(c, sep+f)
			sep = " · "
		}
	})
}

// descriptionLine is a line of a description: its links to web pages open
// them in the browser, and its times within limit play the episode from
// there, once it plays; the rest is plain text.
func (s *shell) descriptionLine(c *ui.Context, line []media.TextRun, ink ui.Color, limit time.Duration) {
	k := &s.lk
	ui.RichText(c).FontSize(k.bodySize()).TextColor(ink).Padding(2, 24).Children(func() {
		for _, r := range line {
			switch {
			case strings.HasPrefix(r.Link, "https://") || strings.HasPrefix(r.Link, "http://"):
				if s.linkOf(c, r.Text, "").Tooltip(r.Link).Clicked() {
					if u := media.WebLink(r.Link); u != "" {
						openWebPage(s, u)
					}
				}
			case r.Seek && ms(r.AtMs) < limit:
				// An episode that loads isn't moved, as the lyrics don't move a
				// song that loads.
				if s.linkOf(c, r.Text, "").Label("Play from "+r.Text).Clicked() && !s.st.Loading {
					s.p.Seek(ms(r.AtMs))
				}
			default:
				ui.Text(c, r.Text)
			}
		}
	})
}

// blankLine reports whether a line of a description holds only spaces.
func blankLine(line []media.TextRun) bool {
	for _, r := range line {
		if strings.TrimSpace(r.Text) != "" {
			return false
		}
	}
	return true
}

// seekLimit is how far into the episode t, which plays, the times of its
// details d may seek: its length, as the player knows it, or as its
// service tells. Where none is known, and in a live stream, they don't.
func (s *shell) seekLimit(t media.Track, d media.EpisodeDetails) time.Duration {
	switch {
	case s.st.Live:
		return 0
	case s.st.Duration > 0:
		return s.st.Duration
	case t.DurationMs > 0:
		return ms(t.DurationMs)
	}
	return ms(d.DurationMs)
}

// chapterAt returns the index of the chapter of chapters, sorted by time,
// that plays at pos, or -1 before the first: those past limit don't count.
func chapterAt(chapters []media.Chapter, pos, limit time.Duration) int {
	n := sort.Search(len(chapters), func(i int) bool {
		at := ms(chapters[i].AtMs)
		return at > pos || at >= limit
	})
	return n - 1
}

// chapters shows in the notch the chapter of the episode t that plays now,
// with those before and after it, as it shows the line of a song sung now.
// Details that load, or fail, say so as lyrics do.
func (n *notch) chapters(c *ui.Context, t media.Track) {
	s := n.s
	note := func(text string) {
		ui.Text(c, text).FontSize(12).TextColor(notchMuted).MaxLines(3)
	}
	key, l := s.episodeDetailsOf(t)
	switch {
	case !l.done:
		note("Looking for the details…")
		return
	case l.err != nil:
		note(l.err.Error())
		if !errors.Is(l.err, app.ErrNoEpisodeDetails) {
			again := s.buttonBase(c.Key("notch-details-retry")).Padding(4, 10).Radius(12).Border(1, notchMuted).
				AlignSelf(ui.Start).Label("try again").Cursor(ui.CursorPointer)
			n.buttonLook(again, false, false)
			again.Children(func() { ui.Text(c, "try again").FontSize(12).TextColor(notchWhite) })
			if again.Clicked() {
				s.forget(key)
			}
		}
		return
	}
	chapters, limit := l.value.Chapters, s.seekLimit(t, l.value)
	in := chapterAt(chapters, max(limit, 0), limit) + 1
	if in == 0 {
		note("No chapters.")
		return
	}
	at := max(chapterAt(chapters[:in], s.st.Position+lyricsLead(s.st.Speed), limit), 0)
	if at > 0 {
		ui.Text(c, chapters[at-1].Title).FontSize(12).TextColor(notchMuted).SingleLine().MinWidth(0)
	}
	ui.Text(c, chapters[at].Title).FontSize(15).FontWeight(600).TextColor(notchWhite).MaxLines(2).MinWidth(0)
	if at+1 < in {
		ui.Text(c, chapters[at+1].Title).FontSize(12).TextColor(notchMuted).SingleLine().MinWidth(0)
	}
}
