package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(soundRows, 80, (*shell).skipDislikedRow) }

// skipDislikedRow skips the songs the user dislikes, in Settings ›
// Playback › Sound, and tells where dislikes go.
func (s *shell) skipDislikedRow(c *ui.Context) {
	k := &s.lk
	s.row(c, locales.DislikeSkip(), locales.DislikeSkipDesc(), func() {
		if s.switchOf(c.Key("skip-disliked"), &s.set.SkipDisliked).Label(locales.DislikeSkip()).Changed() {
			s.save()
		}
	})
	// Where dislikes go, and, once there are any, a link to them.
	note, see := dislikesNote(), len(s.svc.Collection.Disliked()) > 0
	if see {
		note = locales.DislikeNoteSee()
	}
	s.found(c, "", note, func() {
		ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
			ui.Text(c, dislikesNote())
			if see {
				ui.Text(c, " ")
				s.linkOf(c, locales.DislikeSee(), "/library/disliked")
				ui.Text(c, locales.DislikeSeeEnd())
			}
		})
	})
}

// dislikesNote is what the note under Skip songs you dislike says of where
// dislikes go.
func dislikesNote() string { return locales.DislikeNote() }
