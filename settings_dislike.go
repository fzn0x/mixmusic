package main

import "github.com/egoist/mygo/ui"

func init() { addSettings(soundRows, 80, (*shell).skipDislikedRow) }

// skipDislikedRow skips the songs the user dislikes, in Settings ›
// Playback › Sound, and tells where dislikes go.
func (s *shell) skipDislikedRow(c *ui.Context) {
	k := &s.lk
	s.row(c, "Skip songs you dislike", "Songs you dislike don't play when the queue moves on, and radios leave them out.", func() {
		if s.switchOf(c.Key("skip-disliked"), &s.set.SkipDisliked).Label("Skip songs you dislike").Changed() {
			s.save()
		}
	})
	// Where dislikes go, and, once there are any, a link to them.
	note, see := dislikesNote, len(s.svc.Collection.Disliked()) > 0
	if see {
		note += " See the songs you dislike."
	}
	s.found(c, "", note, func() {
		ui.RichText(c).FontSize(k.smallSize()).TextColor(k.muted).Padding(12, 0).MaxWidth(720).Children(func() {
			ui.Text(c, dislikesNote)
			if see {
				ui.Text(c, " ")
				s.linkOf(c, "See the songs you dislike", "/library/disliked")
				ui.Text(c, ".")
			}
		})
	})
}

// dislikesNote is what the note under Skip songs you dislike says of where
// dislikes go.
const dislikesNote = "Signed in to YouTube Music, your dislikes of its songs go there too. Spotify takes no dislikes from apps: those of its tracks stay in MixMusic."
