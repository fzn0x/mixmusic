package main

import (
	"math"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// lyricStyleChoices are the lyrics' styles, in the order of
// settings.LyricStyles, as Settings › Appearance offers them, named in the
// language shown: built on each call, as the language may change.
func lyricStyleChoices() []choice {
	out := make([]choice, len(lyricStyles))
	for i, st := range lyricStyles {
		out[i] = choice{title: st.title(), description: st.about()}
	}
	return out
}

func init() { addSettings("appearance", 35, (*shell).lyricStyleSettings) }

// lyricStyleSettings choose how the full-screen lyrics look: their style, the
// size of their text and whether a ball hops over the words, with a preview
// of what the three make, and show them.
func (s *shell) lyricStyleSettings(c *ui.Context) {
	s.group(c, locales.LyricStyleSettingsGroup(), func() {
		style := slices.Index(settings.LyricStyles, s.set.LyricsStyle)
		options := lyricStyleChoices()
		if s.lk.headings == headingsLight {
			// Tiles' own words are lowercase, as the layout choices'.
			for i, o := range options {
				o.title = s.lk.words(o.title)
				options[i] = o
			}
		}
		if s.choices(c, "lyric-style", locales.LyricStyleSettingsGroup(), &style, options) {
			s.set.LyricsStyle = settings.LyricStyles[style]
			s.save()
		}
		s.found(c, locales.LyricStyleSettingsPreview(), locales.LyricStyleSettingsPreviewDesc(), func() { s.lyricPreview(c) })
		s.row(c, locales.LyricStyleSettingsSize(), locales.LyricStyleSettingsSizePercent(int(s.set.LyricsSize*100+0.5)), func() {
			// The slider goes by whole percents: stepping from 0.6 to 1.8 by 0.1
			// reaches 1.8000000000000003, past its end, which it steps back from.
			pct := math.Round(s.set.LyricsSize * 100)
			if ui.StepSlider(c.Key("lyric-size"), &pct, settings.MinLyricsSize*100, settings.MaxLyricsSize*100, settings.LyricsSizeStep*100).Width(200).Label(locales.LyricStyleSettingsSize()).Changed() {
				s.set.LyricsSize = pct / 100
				s.save()
			}
		}, "font")
		s.row(c, locales.LyricStyleSettingsBall(), locales.LyricStyleSettingsBallDesc(), func() {
			if s.switchOf(c.Key("lyric-ball"), &s.set.LyricsBall).Label(locales.LyricStyleSettingsBallShow()).Changed() {
				s.save()
			}
		}, "karaoke")
		if s.set.LyricsBall {
			// How the ball goes from a line to the next: one motion, or a
			// different one for each line.
			labels, chosen := make([]string, len(ballMotions)), ""
			for i, m := range ballMotions {
				labels[i] = m.name()
				if m.id == s.set.LyricsBallMotion {
					chosen = m.name()
				}
			}
			s.row(c, locales.LyricStyleSettingsBallMotion(), ballMotionOf(s.set.LyricsBallMotion).about(), func() {
				if s.selectOf(c.Key("lyric-ball-motion"), &chosen, labels).Width(200).Label(locales.LyricStyleSettingsBallMotionLabel()).Changed() {
					if i := slices.Index(labels, chosen); i >= 0 {
						s.set.LyricsBallMotion = ballMotions[i].id
						s.save()
					}
				}
			}, "boomerang", "loop", "motion", "animation")
		}
		s.row(c, locales.LyricsFullScreen(), locales.LyricStyleSettingsStageDesc(), func() {
			if s.button(c, "lyric-stage-show", locales.LyricStyleSettingsStageShow(), false).Clicked() {
				s.toggleStage()
			}
		})
	})
}
