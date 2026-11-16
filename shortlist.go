package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// listCame tells whether a list of the user's library came, whole or in
// part, by the error it came with: none, or a media.ShortList.
func listCame(err error) bool {
	_, short := media.Short(err)
	return err == nil || short
}

// shortListText tells how much of a list of the user's library, of noun,
// as "playlists", came from the service of source: "Showing 200 of 250
// playlists".
func shortListText(short *media.ShortList, noun string, source media.Source) string {
	// YouTube Music gives as many pages as are asked for: MixMusic stops
	// at a bound of its own.
	cut := "none"
	switch {
	case !short.Cut:
	case source == media.SourceYouTube:
		cut = "youtube"
	default:
		cut = "service"
	}
	if short.Total <= short.Got {
		return locales.ShortListFirst(short.Got, noun, cut, sourceName(source))
	}
	return locales.ShortListShowing(short.Got, short.Total, noun, cut, sourceName(source))
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
		if !short.Cut && s.button(c, key+"-retry", locales.WidgetsTryAgain(), false).Clicked() {
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
	m.Separator()
	text := shortListText(short, noun, source)
	if short.Cut {
		m.Item(text).Disabled(true)
		return
	}
	if m.Item(locales.ShortListItemRetry(text)).Chosen() {
		s.refetch(key)
	}
}
