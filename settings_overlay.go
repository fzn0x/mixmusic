package main

// Settings › On the desktop › Now playing for overlays (#344): a server
// on this computer that tells what plays to the overlay of a stream, with
// the addresses to copy. It is off at first; internal/overlay serves it.

import (
	"errors"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(desktopRows, 20, (*shell).overlayRows) }

// overlayWords find the overlay rows in the search of the settings.
const overlayWords = "overlay obs stream streaming streamer browser source now playing api server http port twitch"

// overlayRows turn the server of overlays on and off, and, while it's on,
// choose its port and show the addresses to copy.
func (s *shell) overlayRows(c *ui.Context) {
	s.row(c, locales.OverlayTitle(), locales.OverlayAbout(), func() {
		if s.switchOf(c.Key("overlay-server"), &s.set.OverlayServer).Label(locales.OverlayLabel()).Changed() {
			s.save()
		}
	}, overlayWords)
	if !s.set.OverlayServer {
		return
	}
	k := &s.lk
	s.rowWith(c, locales.OverlayPortTitle(), func() {
		ui.Text(c, locales.OverlayPortDescription())
	}, func() {
		if s.set.OverlayPort != settings.DefaultOverlayPort && s.link(c, "overlay-port-reset", locales.OverlayPortReset()) {
			s.set.OverlayPort = settings.DefaultOverlayPort
			delete(s.drafts, "overlay-port")
			s.save()
		}
		in, _ := s.settingInput(c, "overlay-port", strconv.Itoa(s.set.OverlayPort), func(v string) {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1024 || n > 65535 {
				s.notes = append(s.notes, noticeOf(errors.New(locales.OverlayPortRange())))
				delete(s.drafts, "overlay-port")
				return
			}
			s.set.OverlayPort = n
			s.save()
		})
		in.Width(90).MaxLength(5).Label(locales.OverlayPortTitle())
	}, overlayWords)
	st := s.svc.System.OverlayStatus()
	if st.Err != "" {
		s.rowWith(c, locales.OverlayAddressTitle(), func() {
			ui.Text(c, locales.OverlayAddressProblem(st.Err)).TextColor(k.danger)
		}, func() {}, overlayWords)
		return
	}
	if !st.On {
		return
	}
	base := "http://" + st.Addr
	for _, a := range []struct{ key, title, path, what string }{
		{"overlay-page", locales.OverlayBrowserSourceTitle(), "/overlay", locales.OverlayBrowserSourceNote()},
		{"overlay-data", locales.OverlayDataTitle(), "/now-playing", locales.OverlayDataNote()},
	} {
		s.rowWith(c, a.title, func() {
			ui.Text(c, base+a.path).TextColor(k.text)
			ui.Text(c, a.what)
		}, func() {
			if s.button(c, a.key, locales.OverlayCopy(), false).Clicked() {
				copyText(base + a.path)
			}
		}, overlayWords)
	}
}
