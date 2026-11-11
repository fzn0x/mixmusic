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
)

// streamRows are the rows of Track info that tell how the stream st was
// found: by which route, with which arguments, and as whom.
func streamRows(st *app.TrackStream) []infoRow {
	if st == nil || st.Route == "" {
		return nil
	}
	rows := []infoRow{{"Found", routeLine(st.Route), false}}
	if st.Command != "" {
		rows = append(rows, infoRow{"yt-dlp ran", st.Command, false})
	}
	signed := "No: asked for signed out, as no one's account."
	if st.AsAccount {
		signed = "Yes: asked for as your YouTube Music account."
	}
	return append(rows, infoRow{"Signed in", signed, false})
}

// routeLine tells how a route finds a stream.
func routeLine(r stream.Route) string {
	switch r {
	case stream.RoutePlayerAPI:
		return "Quickly, through YouTube's player API, with no yt-dlp"
	case stream.RouteQuick:
		return "With yt-dlp's quick run, as YouTube's visionOS app alone"
	case stream.RouteFull:
		return "With yt-dlp's full run, with its default clients and a JavaScript runtime"
	case stream.RouteAccount:
		return "With yt-dlp's full run, with the session of your YouTube Music account"
	}
	return string(r)
}

// streamURLNote tells what the address Copy stream URL copies is good for.
const streamURLNote = "It stops working after a few hours, and is tied to this network: it may carry your IP address, so share it with care."

// streamCopy shows Copy stream URL, with its note beside it, when MixMusic
// holds the stream of the track st, which the engine plays through its
// proxy. Copying says so, and logs nothing of the address.
func (s *shell) streamCopy(c *ui.Context, st *app.TrackStream) {
	if st == nil || st.URL == "" {
		return
	}
	k := &s.lk
	ui.Row(c).Gap(16).AlignItems(ui.Center).Margin(8, 0, 0).Children(func() {
		if s.button(c, "track-info-copy-url", "copy stream URL", false).Clicked() {
			copyText(st.URL)
			s.notes = append(s.notes, notice("Copied the stream's address: it stops working after a few hours"))
		}
		ui.Text(c, streamURLNote).FontSize(k.smallSize()).TextColor(k.muted).Grow(1).Shrink(1).MinWidth(0)
	})
}
