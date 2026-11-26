package main

import (
	"log"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings(desktopRows, 21, (*shell).controlRows) }

// controlWords find the programs' rows in the search of the settings.
const controlWords = "control api remote stream deck script curl powershell token http command"

// controlRows let the programs on this computer control MixMusic, in
// Settings › Desktop › Programs on this computer (#591): the switch, the
// token they carry, and an example of each of the two ways to send it.
func (s *shell) controlRows(c *ui.Context) {
	k := &s.lk
	s.row(c, locales.ControlTitle(), locales.ControlAbout(), func() {
		if s.switchOf(c.Key("control-server"), &s.set.Overlay.ControlServer).Label(locales.ControlLabel()).Changed() {
			s.save()
		}
	}, controlWords)
	if !s.set.Overlay.ControlServer {
		return
	}
	s.rowWith(c, locales.ControlTokenTitle(), func() {
		ui.Text(c, locales.ControlTokenDescription())
	}, func() {
		if s.button(c, "control-token-new", locales.ControlTokenNew(), false).Clicked() {
			if err := s.svc.System.NewControlToken(); err != nil {
				log.Printf("mixmusic: control: a new token: %v", err)
				s.notes = append(s.notes, noticeOf(err))
			}
		}
	}, controlWords)
	st := s.svc.System.ControlStatus()
	if !st.On {
		return
	}
	base := "http://" + st.Addr
	s.rowWith(c, locales.ControlAddressTitle(), func() {
		ui.Text(c, base+"/control/state").TextColor(k.text)
		ui.Text(c, locales.ControlAddressNote())
	}, func() {
		for _, a := range []struct{ key, kind, label string }{
			{"control-curl", "curl", locales.ControlCopyCurl()},
			{"control-powershell", "powershell", locales.ControlCopyPowerShell()},
		} {
			if !s.button(c, a.key, a.label, false).Clicked() {
				continue
			}
			ex, err := s.svc.System.ControlExample(a.kind)
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				continue
			}
			copyText(ex)
		}
	}, controlWords)
}
