package main

// Lyrics timing (#610). Above synced lyrics, in the lyrics panel and on the
// now playing page, beside synced and plain text, the song's own timing
// moves its lyrics earlier or later: Earlier and Later by lyricsStep a
// press, or a second with Shift, and a field that shows it in words, as
// "0.35 s earlier", and takes it typed in milliseconds. It is kept for the
// lyrics of their source alone (internal/app's lyricsoffset.go). Settings ›
// Playback › Lyrics has Lyrics timing, which moves every song's, for an
// output that sounds late. Every view of the line sung now takes the
// lyrics' time from lyricsAt, and the words lit from lyricsShift: the
// lines and their words keep their own times, which key the romanization
// and the translation.

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// lyricsStep is how far Earlier and Later move the lyrics a press, and
// the slider of Settings a step.
const lyricsStep = settings.LyricsOffsetStep * time.Millisecond

// maxTiming is how far a timing moves lyrics at most, either way.
const maxTiming = settings.MaxLyricsOffset * time.Millisecond

// lyricsShift is how far the lyrics ly of the track t show moved from the
// song's time, earlier above 0: the timing the listener gave the track's
// lyrics of their source, and Settings' timing of every song, which, as a
// delay of the output, grows with the speed, as lyricsLead does.
func (s *shell) lyricsShift(t media.Track, ly lyrics.Lyrics) time.Duration {
	speed := s.st.Speed
	if speed <= 0 {
		speed = 1
	}
	every := time.Duration(float64(ms(s.set.LyricsOffset)) * speed)
	return s.svc.Catalog.LyricsOffset(t, ly) + every
}

// lyricsAt is the time of the lyrics ly of the track t that the line sung
// now lights at: the player's position, lyricsLead ahead, moved by
// lyricsShift.
func (s *shell) lyricsAt(t media.Track, ly lyrics.Lyrics) time.Duration {
	return s.st.Position + lyricsLead(s.st.Speed) + s.lyricsShift(t, ly)
}

// seekLine plays the song from a line of lyrics moved by shift: from its
// time less shift, never before the track's start, so that it is the line
// sung then.
func (s *shell) seekLine(line lyrics.Line, shift time.Duration) {
	s.p.Seek(max(ms(line.TimeMs)-shift, 0))
}

// songTiming is the timing of the synced lyrics ly of the track playing,
// above them, while they show synced: Earlier, the timing, Later, and
// reset while it isn't 0. The arrows and the words between them say what
// it is, with no label, so that it goes on one line with the other
// choices on the now playing page, which leaves the lyrics their room.
func (s *shell) songTiming(c *ui.Context, ly lyrics.Lyrics, where string) {
	cur, ok := s.st.Current()
	if !ok || !ly.Synced || s.set.LyricsPlain {
		return
	}
	t := cur.Track
	d := s.svc.Catalog.LyricsOffset(t, ly)
	set := func(v time.Duration) { s.svc.Catalog.SetLyricsOffset(t, ly, v) }
	// Reset goes under the rest where there's no room beside it.
	ui.Row(c).GapX(4).GapY(8).Wrap().AlignItems(ui.Center).MinWidth(0).Children(func() {
		s.timingStep(c, "lyrics-earlier-"+where, "back", locales.LyricsTimingEarlier(), locales.LyricsTimingEarlierTip(), d, 1, set)
		s.timingField(c, "lyrics-timing-"+where, locales.LyricsTimingField(), d, set).Width(100).FontSize(s.lk.smallSize())
		s.timingStep(c, "lyrics-later-"+where, "forward", locales.LyricsTimingLater(), locales.LyricsTimingLaterTip(), d, -1, set)
		if d != 0 && s.link(c, "lyrics-timing-reset-"+where, locales.LyricsTimingReset()) {
			set(0)
		}
	})
}

// timingStep is a button that moves the timing d of lyrics by lyricsStep,
// or a second with Shift, earlier (dir 1) or later (-1), as far as
// maxTiming, and set takes: its icon, its name, and its tip.
func (s *shell) timingStep(c *ui.Context, key, icon, name, tip string, d time.Duration, dir int, set func(time.Duration)) {
	end := time.Duration(dir) * maxTiming
	b := s.untippedIconButton(c, key, icon, name, false, 18).Tooltip(tip).Disabled(d == end)
	if d == end {
		b.Opacity(0.4)
	}
	if !b.Clicked() {
		return
	}
	step := lyricsStep
	if (b.ClickModifiers()|c.Modifiers())&ui.Shift != 0 {
		step = time.Second
	}
	if dir > 0 {
		set(min(d+step, end))
	} else {
		set(max(d-step, end))
	}
}

// timingField is the timing d of lyrics as a text input named label, as
// wide as its caller sets: in words while it hasn't the focus, as "0.35 s
// earlier", and in milliseconds while it has, earlier above 0 and later
// below. What is typed, set takes as Enter is pressed or the focus
// leaves, within maxTiming either way, as it is: nothing rounds it to a
// step.
func (s *shell) timingField(c *ui.Context, key, label string, d time.Duration, set func(time.Duration)) ui.Element {
	k := &s.lk
	f := s.drafts[key]
	if f == nil {
		f = &draft{}
		s.drafts[key] = f
	}
	if !f.focused {
		f.text = k.sentence(timingWords(d))
	}
	in := ui.TextInput(c.Key(key), &f.text).Label(label).Tooltip(locales.LyricsTimingFieldTip())
	focused := in.Focused()
	switch {
	case focused && !f.focused:
		// The focus came: the timing shows in milliseconds, to type.
		f.text = strconv.Itoa(int(d / time.Millisecond))
		c.AnimationFrame()
	case in.Submitted() || f.focused && !focused:
		if v, ok := parseTiming(f.text); ok {
			if v != d {
				set(v)
			}
			d = v
		}
		if focused {
			f.text = strconv.Itoa(int(d / time.Millisecond))
		} else {
			f.text = k.sentence(timingWords(d))
			c.AnimationFrame()
		}
	}
	f.focused = focused
	return in
}

// parseTiming reads a timing typed in milliseconds, as "350", "-1200" or
// "+80 ms": ok false for text that isn't one. One past maxTiming either
// way stops there.
func parseTiming(text string) (time.Duration, bool) {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "ms"))
	text = strings.ReplaceAll(strings.ReplaceAll(text, "−", "-"), ",", ".")
	v, err := strconv.ParseFloat(text, 64)
	if err != nil || v != v {
		return 0, false
	}
	n := settings.CleanLyricsOffset(int(math.Round(max(min(v, 1e9), -1e9))))
	return ms(n), true
}

// timingWords says how far a timing moves lyrics: "0.35 s earlier", "1 s
// later", or "as timed" for none.
func timingWords(d time.Duration) string {
	if d == 0 {
		return locales.LyricsTimingNone()
	}
	n := float64(d.Abs()) / float64(time.Second)
	sec := i18n.FormatNumber(i18n.Locale(), n, i18n.NumberOptions{MaxFractionDigits: 3})
	if d > 0 {
		return locales.LyricsTimingEarlierBy(sec)
	}
	return locales.LyricsTimingLaterBy(sec)
}

func init() { addSettings(lyricsRows, 27, (*shell).lyricsTimingRow) }

// lyricsTimingRow moves the synced lyrics of every song earlier or later:
// a slider of lyricsStep steps, settings.LyricsOffsetSlider either way,
// earlier at its start and later at its end, as time goes, and the timing
// as a field, which takes one typed as far as maxTiming. The slider keeps
// a timing past its ends where it is, and reset shows while it isn't 0.
func (s *shell) lyricsTimingRow(c *ui.Context) {
	s.row(c, locales.LyricsTimingTitle(), locales.LyricsTimingDescription(), func() {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			d := ms(s.set.LyricsOffset)
			set := func(v time.Duration) {
				s.set.LyricsOffset = int(v / time.Millisecond)
				s.save()
			}
			// Before the slider, which stays where it is as it comes.
			if d != 0 && s.link(c, "lyrics-offset-reset", locales.LyricsTimingReset()) {
				set(0)
				d = 0
			}
			edge := float64(settings.LyricsOffsetSlider)
			x := min(max(-float64(s.set.LyricsOffset), -edge), edge)
			slider := ui.StepSlider(c.Key("lyrics-offset"), &x, -edge, edge, settings.LyricsOffsetStep).Width(200).
				Label(locales.LyricsTimingEvery()).Value(timingWords(d))
			if slider.Changed() {
				set(ms(-int(x)))
				d = ms(-int(x))
			}
			s.timingField(c, "lyrics-offset-ms", locales.LyricsTimingEveryField(), d, set).Width(132)
		})
	}, "offset", "sync", "delay", "latency", "bluetooth", "headphones", "earlier", "later")
}
