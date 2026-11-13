package main

// Settings › Desktop › Remote control (#353): whether the other computers
// with MixMusic on the network may find this one, pair with it and
// control it, and the computers paired, each with Remove.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("desktop", 15, (*shell).remoteSettings) }

// remoteSettings turn remote control on and off, and list the computers
// paired, with Remove.
func (s *shell) remoteSettings(c *ui.Context) {
	rc := s.a.Remote()
	s.group(c, locales.RemoteTitle(), func() {
		on := rc.On()
		if s.remote.turning != nil {
			on = *s.remote.turning
		}
		s.row(c, locales.RemoteSwitch(), locales.RemoteAbout(rc.Name()), func() {
			sw := s.switchOf(c.Key("remote-control"), &on).Label(locales.RemoteSwitch()).Disabled(s.remote.turning != nil)
			if sw.Changed() {
				s.setRemote(on)
			}
		}, "remote control another computer laptop desktop network pair phone")
		if fw := s.a.Firewall(); on && fw != nil {
			s.readFirewall(fw)
			if s.firewallKeepsOut() {
				s.hint(c, locales.RemoteFirewallHint())
			}
		}
		paired := rc.Paired()
		if len(paired) == 0 {
			s.hint(c, locales.RemoteNoPaired())
			return
		}
		for _, p := range paired {
			s.row(c, p.Name, pairedAbout(p), func() {
				if s.button(c, "remote-remove-"+p.ID, locales.RemoteRemove(), false).Label(locales.RemoteRemoveLabel(p.Name)).Clicked() {
					s.forgetPaired(p)
				}
			}, "paired computer remove forget")
		}
	})
}

// pairedAbout tells what a computer paired does to this one, and this one
// to it.
func pairedAbout(p app.PairedComputer) string {
	var about string
	switch {
	case p.ControlsThis && p.Controlled:
		about = locales.RemoteBothWays()
	case p.ControlsThis:
		about = locales.RemoteCanControl()
	case p.Controlled:
		about = locales.RemoteControlled()
	}
	if !p.Paired.IsZero() {
		about = locales.RemotePaired(about, p.Paired.Local().Format("2 January 2006"))
	}
	return about
}

// setRemote turns remote control on or off beside the main thread: the key
// may be made, and the system asked to keep it, the first time.
func (s *shell) setRemote(on bool) {
	s.remote.turning = &on
	go func() {
		err := s.a.Remote().SetOn(on)
		s.update(func() {
			s.remote.turning = nil
			// Listening, MixMusic may have been let in or kept out, by
			// Windows Firewall's question: it's read again.
			s.firewall.reread()
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		})
	}()
}

// forgetPaired removes a computer paired: it may no longer control this
// one, and this one no longer controls it.
func (s *shell) forgetPaired(p app.PairedComputer) {
	go func() {
		err := s.a.Remote().Forget(p.ID)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.notes = append(s.notes, notice(locales.RemoteRemoved(p.Name)))
		})
	}()
}
