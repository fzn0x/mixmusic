package main

// How a stream was found, in Track info (#475): the route that found the
// stream of a YouTube Music track, the arguments of the run of yt-dlp that
// did, whether it was asked for as the user's account, and Copy stream URL,
// which copies the address the engine plays. A bug report about a stream
// that fails, or sounds wrong, needs them.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
)

// streamRows are the rows of Track info that tell how the stream st was
// found: by which route, with which arguments, and as whom.
func streamRows(st *app.TrackStream) []infoRow {
	if st == nil || st.Route == "" {
		return nil
	}
	rows := []infoRow{{locales.TrackInfoFound(), routeLine(st.Route), false}}
	if st.Command != "" {
		rows = append(rows, infoRow{locales.TrackInfoYtDlpRan(), st.Command, false})
	}
	signed := locales.TrackInfoSignedOut()
	if st.AsAccount {
		signed = locales.TrackInfoSignedInYes()
	}
	return append(rows, infoRow{locales.TrackInfoSignedIn(), signed, false})
}

// routeLine tells how a route finds a stream.
func routeLine(r stream.Route) string {
	switch r {
	case stream.RoutePlayerAPI:
		return locales.TrackInfoRoutePlayerAPI()
	case stream.RouteQuick:
		return locales.TrackInfoRouteQuick()
	case stream.RouteFull:
		return locales.TrackInfoRouteFull()
	case stream.RouteAccount:
		return locales.TrackInfoRouteAccount()
	}
	return string(r)
}

// streamCopy shows Copy stream URL, with its note beside it, when MixMusic
// holds the stream of the track st, which the engine plays through its
// proxy. Copying says so, and logs nothing of the address.
func (s *shell) streamCopy(c *ui.Context, st *app.TrackStream) {
	if st == nil || st.URL == "" {
		return
	}
	k := &s.lk
	ui.Row(c).Gap(16).AlignItems(ui.Center).Margin(8, 0, 0).Children(func() {
		if s.button(c, "track-info-copy-url", locales.TrackInfoCopyURL(), false).Clicked() {
			copyText(st.URL)
			s.notes = append(s.notes, notice(locales.TrackInfoCopied()))
		}
		ui.Text(c, locales.TrackInfoStreamNote()).FontSize(k.smallSize()).TextColor(k.muted).Grow(1).Shrink(1).MinWidth(0)
	})
}
