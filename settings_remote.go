package main

// Settings › Desktop › Remote control (#353): whether the other computers
// with MixMusic on the network may find this one, pair with it and
// control it, and the computers paired, each with Remove.

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

func init() { addSettings("desktop", 15, (*shell).remoteSettings) }

// remoteTitle is the title of the group, and remoteSwitch of its switch.
const (
	remoteTitle  = "Remote control"
	remoteSwitch = "Allow remote control"
)

// remoteSettings turn remote control on and off, and list the computers
// paired, with Remove.
func (s *shell) remoteSettings(c *ui.Context) {
	rc := s.a.Remote()
	s.group(c, remoteTitle, func() {
		on := rc.On()
		if s.remote.turning != nil {
			on = *s.remote.turning
		}
		s.row(c, remoteSwitch, remoteAbout(rc.Name()), func() {
			sw := s.switchOf(c.Key("remote-control"), &on).Label(remoteSwitch).Disabled(s.remote.turning != nil)
			if sw.Changed() {
				s.setRemote(on)
			}
		}, "remote control another computer laptop desktop network pair phone")
		if fw := s.a.Firewall(); on && fw != nil {
			s.readFirewall(fw)
			if s.firewallKeepsOut() {
				s.hint(c, "Windows Firewall keeps the other computers from reaching this one. Windows Firewall, below, tells why, and how to let them in.")
			}
		}
		paired := rc.Paired()
		if len(paired) == 0 {
			s.hint(c, "No computer is paired yet. To control another computer, choose it in the player bar's devices menu, the cast button's.")
			return
		}
		for _, p := range paired {
			s.row(c, p.Name, pairedAbout(p), func() {
				if s.button(c, "remote-remove-"+p.ID, "remove", false).Label("Remove " + p.Name).Clicked() {
					s.forgetPaired(p)
				}
			}, "paired computer remove forget")
		}
	})
}

// remoteAbout tells what remote control does, of the computer named name.
func remoteAbout(name string) string {
	return "Other computers on this network with MixMusic can find this one, as “" + name +
		"”, pair with it by a code it shows, and control what it plays. What they send goes encrypted, between the two alone."
}

// pairedAbout tells what a computer paired does to this one, and this one
// to it.
func pairedAbout(p app.PairedComputer) string {
	var what []string
	if p.ControlsThis {
		what = append(what, "Can control this computer")
	}
	if p.Controlled {
		what = append(what, "You control it from here")
	}
	about := strings.Join(what, "; ")
	if !p.Paired.IsZero() {
		about += ". Paired " + p.Paired.Local().Format("2 January 2006") + "."
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
			s.notes = append(s.notes, notice("Removed "+p.Name))
		})
	}()
}
