package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// shortListText tells how much of a list of the user's library, of noun,
// as "playlists", came from the service of source: "Showing 200 of 250
// playlists".
func shortListText(short *media.ShortList, noun string, source media.Source) string {
	text := "Showing " + thousands(short.Got) + " of " + thousands(short.Total) + " " + noun
	if short.Total <= short.Got {
		text = "Showing the first " + thousands(short.Got) + " " + noun
	}
	if short.Cut {
		text += ", the most " + sourceName(source) + " gives apps"
	}
	return text
}

// shortListLine tells that a list of the user's library, of noun, came in
// part from the service of source, as err does (media.ShortList), with Try
// again, which asks for it again with retry, but for a list that stopped
// at the most the service gives. For another error it shows nothing. It
// reports whether it showed, beside what came.
func (s *shell) shortListLine(c *ui.Context, key, noun string, source media.Source, err error, retry func()) bool {
	short, ok := media.Short(err)
	if !ok {
		return false
	}
	k := &s.lk
	ui.Row(c).Gap(12).Margin(0, 0, 16).AlignItems(ui.Center).Children(func() {
		ui.Text(c, shortListText(short, noun, source)).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
		if !short.Cut && s.button(c, key+"-retry", "try again", false).Clicked() {
			retry()
		}
	})
	return true
}

// shortListItem ends a menu of what a list of the user's library holds,
// which key fetched, with how much of it came, when it came in part, as
// err does: chosen, it asks for the list again.
func (s *shell) shortListItem(m *ui.Menu, key, noun string, source media.Source, err error) {
	short, ok := media.Short(err)
	if !ok {
		return
	}
	text := shortListText(short, noun, source)
	if short.Cut {
		m.Item(text).Disabled(true)
		return
	}
	if m.Item(text + " · Try again").Chosen() {
		s.refetch(key)
	}
}
