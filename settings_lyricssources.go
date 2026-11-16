package main

// The sources of lyrics in Settings › Playback › Lyrics: a row a source,
// with its switch and what it is sent, in the order MixMusic asks them
// (settings.Lyrics.SourceOrder). A row drags onto another, which it takes
// the place of, or moves a place up or down with Alt+Up and Alt+Down, as
// Edit home's shelves do (homelayout.go's movedOnto). The last source on
// can't be switched off, and reset to default puts the list back as it
// comes. A change looks the lyrics of the track that plays up again.

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// lyricsSourceText tells the user of a source of lyrics, by its key: its
// name, and what MixMusic sends it.
func lyricsSourceText() map[string]struct{ name, sent string } {
	return map[string]struct{ name, sent string }{
		settings.LyricsBetterLyrics: {"Better Lyrics", locales.LyricsSourceBetterLyricsSent()},
		settings.LyricsUnison:       {"Unison", locales.LyricsSourceUnisonSent()},
		settings.LyricsLRCLIB:       {"LRCLIB", locales.LyricsSourceLRCLIBSent()},
		settings.LyricsYouTubeMusic: {"YouTube Music", locales.LyricsSourceYouTubeMusicSent()},
		settings.LyricsCaptions:     {locales.LyricsSourceCaptionsName(), locales.LyricsSourceCaptionsSent()},
	}
}

// lyricsSourcesAbout describes the list of the sources of lyrics.
func lyricsSourcesAbout() string { return locales.LyricsSourcesAbout() }

// lyricsSourceDrag is a source of lyrics dragged, by its key.
type lyricsSourceDrag struct{ key string }

func init() { addSettings(lyricsRows, 10, (*shell).lyricsSourcesRow) }

// lyricsSourcesRow lists the sources of lyrics, as the package's comment
// tells.
func (s *shell) lyricsSourcesRow(c *ui.Context) {
	k := &s.lk
	order := s.set.SourceOrder()
	words := []string{"reset to default", "order", "move", "switch off"}
	for _, src := range order {
		words = append(words, lyricsSourceText()[src.Key].name, lyricsSourceText()[src.Key].sent)
	}
	if !s.findable(locales.LyricsSourcesTitle(), lyricsSourcesAbout(), words...) {
		return
	}
	s.show(c, func() {
		list := ui.Column(c.Key("lyrics-sources")).Gap(4).Padding(14, 0)
		focus := ui.Local(list, "focus", func() string { return "" })
		list.Children(func() {
			ui.Row(c).Gap(24).Margin(0, 0, 8).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, locales.LyricsSourcesTitle()).FontSize(k.bodySize())
					ui.Text(c, lyricsSourcesAbout()).FontSize(k.smallSize()).TextColor(k.muted)
				})
				ui.Row(c).Shrink(0).Children(func() {
					if s.button(c, "lyrics-sources-reset", locales.LyricsSourcesReset(), false).Disabled(defaultSources(order)).Clicked() {
						s.setLyricsSources(nil)
					}
				})
			})
			on := 0
			for _, src := range order {
				if !src.Off {
					on++
				}
			}
			for i := range order {
				s.lyricsSourceRow(c, i, order, on, focus)
			}
		})
	})
}

// lyricsSourceRow is the row of the source at i of order, of which on are
// on: it drags onto another, and moves with Alt+Up and Alt+Down while it
// has the focus, which focus tells it takes once it moved.
func (s *shell) lyricsSourceRow(c *ui.Context, i int, order []settings.LyricsSource, on int, focus *string) {
	k := &s.lk
	src := order[i]
	text := lyricsSourceText()[src.Key]
	state := locales.LyricsSourceOn()
	if src.Off {
		state = locales.LyricsSourceOff()
	}
	row := ui.Row(c.Key("lyrics-source-"+src.Key)).MinHeight(56).Padding(8, 8, 8, 16).Gap(12).Radius(k.radiusSm).
		Background(k.surface2).Label(locales.LyricsSourceRowLabel(text.name, state)).Focusable().Cursor(ui.CursorGrab)
	if row.Hovered() {
		row.Background(k.surface2.Mix(k.text, 0.06))
	}
	row.Drag(lyricsSourceDrag{src.Key})
	if row.Dragging() {
		row.Opacity(0.4)
	}
	if d, over := ui.DragOver[lyricsSourceDrag](row); over && d.key != src.Key {
		// The accent's line shows where it goes: below, from above, and
		// above, from below.
		if shelfAt(order, d.key) < i {
			row.InsetShadow(0, -2, 0, 0, k.accent)
		} else {
			row.InsetShadow(0, 2, 0, 0, k.accent)
		}
	}
	if d, ok := ui.Drop[lyricsSourceDrag](row); ok && d.key != src.Key {
		s.setLyricsSources(movedOnto(order, d.key, src.Key))
	}
	step := func(to int) {
		if to >= 0 && to < len(order) {
			s.setLyricsSources(movedOnto(order, src.Key, order[to].Key))
			*focus = src.Key
		}
	}
	row.OnShortcut(ui.Alt, ui.KeyUp, func() { step(i - 1) })
	row.OnShortcut(ui.Alt, ui.KeyDown, func() { step(i + 1) })
	if *focus == src.Key {
		*focus = ""
		row.Focus().ScrollIntoView()
	}
	row.Children(func() {
		s.grip(c)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ink := k.text
			if src.Off {
				ink = k.muted
			}
			ui.Text(c, text.name).FontSize(k.bodySize()).TextColor(ink).SingleLine().MinWidth(0)
			ui.Text(c, text.sent).FontSize(k.smallSize()).TextColor(k.muted)
		})
		lit := !src.Off
		// The last source on stays on: lyrics come from somewhere.
		last := lit && on == 1
		label := text.name
		if last {
			label = locales.LyricsSourceLastOn(text.name)
		}
		if s.switchOf(c.Key("lyrics-source-on-"+src.Key), &lit).Label(label).Disabled(last).Changed() && !last {
			next := slices.Clone(order)
			next[i].Off = !lit
			s.setLyricsSources(next)
		}
	})
}

// defaultSources reports whether order is the sources' own: every one on,
// in their own order.
func defaultSources(order []settings.LyricsSource) bool {
	if len(order) != len(settings.LyricsSourceKeys) {
		return false
	}
	for i, src := range order {
		if src.Off || src.Key != settings.LyricsSourceKeys[i] {
			return false
		}
	}
	return true
}

// setLyricsSources keeps order as the sources of lyrics, nil as they
// come, and looks the lyrics of the track that plays up again.
func (s *shell) setLyricsSources(order []settings.LyricsSource) {
	if order == nil {
		order = []settings.LyricsSource{}
	}
	s.set.LyricsSources = order
	s.save()
	s.lyricsAgain()
}

// lyricsAgain has the lyrics of the track that plays, and the transcript
// of an episode, looked up again, as the sources of lyrics changed.
func (s *shell) lyricsAgain() {
	if cur, ok := s.st.Current(); ok {
		s.forget("lyrics:" + cur.Track.ID)
		s.forget("transcript:" + cur.Track.ID)
	}
}

func init() { addSettings(lyricsRows, 20, (*shell).preferWordsRow) }

// preferWordsRow has lyrics timed word by word come first.
func (s *shell) preferWordsRow(c *ui.Context) {
	s.row(c, locales.LyricsPreferWords(), locales.LyricsPreferWordsDescription(), func() {
		if s.switchOf(c.Key("lyrics-prefer-words"), &s.set.LyricsPreferWords).Label(locales.LyricsPreferWords()).Changed() {
			s.save()
			s.lyricsAgain()
		}
	}, "word by word", "karaoke")
}

func init() { addSettings(lyricsRows, 30, (*shell).captionLanguageRow) }

// captionLanguageRow chooses the language of the captions lyrics come
// from: the video's own, or one Translate lyrics to offers.
func (s *shell) captionLanguageRow(c *ui.Context) {
	s.row(c, locales.LyricsCaptionLanguage(), locales.LyricsCaptionLanguageDescription(), func() {
		options := make([]string, 0, len(translateLanguages)+1)
		options = append(options, locales.LyricsCaptionLanguageOriginal())
		chosen := options[0]
		for _, l := range translateLanguages {
			options = append(options, l.name())
			if l.code == s.set.LyricsCaptionLanguage {
				chosen = l.name()
			}
		}
		if s.selectOf(c.Key("caption-language"), &chosen, options).Width(220).Label(locales.LyricsCaptionLanguage()).Changed() {
			s.set.LyricsCaptionLanguage = ""
			if i := slices.Index(options, chosen); i > 0 {
				s.set.LyricsCaptionLanguage = translateLanguages[i-1].code
			}
			s.save()
			s.lyricsAgain()
		}
	}, "subtitles", "captions")
}
