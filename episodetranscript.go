package main

// An episode's transcript, under its details: the captions of its video
// on YouTube, made from speech when it has no other (Catalog.Transcript),
// while YouTube captions is a source of lyrics. Each line shows its time,
// the line said now lit with the accent, and a click on one plays the
// episode from there, as a line of synced lyrics does.

import (
	"context"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// transcriptOf fetches the transcript of the episode t: nothing shows of
// it until it comes, nor when it has none.
func (s *shell) transcriptOf(t media.Track) lyrics.Lyrics {
	l := fetch(s, "transcript:"+t.ID, func(ctx context.Context) (lyrics.Lyrics, error) { return s.svc.Catalog.Transcript(ctx, t) })
	if !l.done || l.err != nil {
		return lyrics.Lyrics{}
	}
	return l.value
}

// transcriptRows is how many rows of an episode's details its transcript
// tr takes: its title and its lines, none without lines.
func transcriptRows(tr lyrics.Lyrics) int {
	if len(tr.Lines) == 0 {
		return 0
	}
	return 1 + len(tr.Lines)
}

// transcriptRow builds row i of the transcript tr: its title, then its
// lines, the one said now lit; those within limit play the episode from
// their time.
func (s *shell) transcriptRow(c *ui.Context, i int, tr lyrics.Lyrics, limit time.Duration) {
	k := &s.lk
	if i == 0 {
		ui.Column(c).Padding(16, 24, 4).Children(func() {
			ui.Text(c, locales.EpisodeTranscriptTitle()).FontSize(k.bodySize()).FontWeight(600)
			ui.Text(c, locales.EpisodeTranscriptSource()).FontSize(k.smallSize()).TextColor(k.muted)
		})
		return
	}
	at := i - 1
	line := tr.Lines[at]
	said := -1
	if tr.Synced {
		pos := s.st.Position + lyricsLead(s.st.Speed)
		for j, l := range tr.Lines {
			if ms(l.TimeMs) > pos {
				break
			}
			said = j
		}
	}
	ink := k.text
	if at == said {
		ink = k.accent
	}
	when := clock(ms(line.TimeMs))
	row := ui.Row(c.Key("transcript-"+strconv.Itoa(at))).Gap(12).Padding(2, 24)
	seeks := tr.Synced && ms(line.TimeMs) < limit
	if seeks {
		row.Label(locales.EpisodePlayFrom(when + ", " + line.Text)).Cursor(ui.CursorPointer).Focusable()
		if row.Clicked() && !s.st.Loading {
			s.p.Seek(ms(line.TimeMs))
		}
	}
	row.Children(func() {
		if tr.Synced {
			ui.Text(c, when).FontSize(k.smallSize()).TextColor(k.muted).Width(48).Shrink(0)
		}
		ui.Text(c, line.Text).FontSize(k.bodySize()).TextColor(ink).Grow(1).MinWidth(0)
	})
}
