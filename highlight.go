package main

// Jump to the highlight (#548): the moment SponsorBlock tells a video is
// best seen from, which its seek bars mark with a dot (seekparts.go), is a
// button on the now playing page and a command away.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// highlightButton jumps to the highlight of the track that plays, when it
// has one.
func (s *shell) highlightButton(c *ui.Context) {
	if s.st.Highlight <= 0 {
		return
	}
	if s.iconButton(c, "now-highlight", "highlight", locales.PlayerJumpToHighlight(), false, 18).Clicked() {
		s.jumpToHighlight()
	}
}

// jumpToHighlight plays the track that plays from its highlight, or tells
// that it has none.
func (s *shell) jumpToHighlight() {
	if s.st.Highlight <= 0 {
		s.notes = append(s.notes, notice(locales.PlayerNoHighlight()))
		return
	}
	s.p.JumpToHighlight()
}
