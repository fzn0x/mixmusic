package main

// Settings › On the desktop › Now playing for overlays (#344): a server
// on this computer that tells what plays to the overlay of a stream, with
// the addresses to copy. It is off at first; internal/overlay serves it.

import (
	"errors"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

func init() { addSettings(desktopRows, 20, (*shell).overlayRows) }

const (
	overlayTitle = "Now playing for overlays"
	overlayAbout = "Tells what plays, as a page for the browser source of OBS or another streaming program, " +
		"and as data for an overlay of your own. It listens on this computer only, and takes no commands."
	overlayLabel = "Serve what plays to overlays"
	overlayWords = "overlay obs stream streaming streamer browser source now playing api server http port twitch"
)

// overlayRows turn the server of overlays on and off, and, while it's on,
// choose its port and show the addresses to copy.
func (s *shell) overlayRows(c *ui.Context) {
	s.row(c, overlayTitle, overlayAbout, func() {
		if s.switchOf(c.Key("overlay-server"), &s.set.OverlayServer).Label(overlayLabel).Changed() {
			s.save()
		}
	}, overlayWords)
	if !s.set.OverlayServer {
		return
	}
	k := &s.lk
	s.rowWith(c, "Overlay port", func() {
		ui.Text(c, "The port the server listens on, 1024 to 65535. ")
		ui.Text(c, "Another port, while it's on, applies at once.")
	}, func() {
		if s.set.OverlayPort != settings.DefaultOverlayPort && s.link(c, "overlay-port-reset", "reset") {
			s.set.OverlayPort = settings.DefaultOverlayPort
			delete(s.drafts, "overlay-port")
			s.save()
		}
		in, _ := s.settingInput(c, "overlay-port", strconv.Itoa(s.set.OverlayPort), func(v string) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1024 || n > 65535 {
				s.notes = append(s.notes, noticeOf(errors.New("the overlay port is a number from 1024 to 65535")))
				delete(s.drafts, "overlay-port")
				return
			}
			s.set.OverlayPort = n
			s.save()
		})
		in.Width(90).MaxLength(5).Label("Overlay port")
	}, overlayWords)
	st := s.svc.System.OverlayStatus()
	if st.Err != "" {
		s.rowWith(c, "Overlay address", func() {
			ui.Text(c, st.Err+" Choose another port above.").TextColor(k.danger)
		}, func() {}, overlayWords)
		return
	}
	if !st.On {
		return
	}
	base := "http://" + st.Addr
	for _, a := range []struct{ key, title, path, what string }{
		{"overlay-page", "Browser source address", "/overlay", "Paste this as the URL of a browser source in OBS."},
		{"overlay-data", "Data address", "/now-playing", "JSON of what plays; /events streams each change, and /cover is the cover."},
	} {
		s.rowWith(c, a.title, func() {
			ui.Text(c, base+a.path).TextColor(k.text)
			ui.Text(c, ". "+a.what)
		}, func() {
			if s.button(c, a.key, "copy", false).Clicked() {
				copyText(base + a.path)
			}
		}, overlayWords)
	}
}
