package main

// Settings › Desktop › Windows Firewall (#496): whether other computers
// and cast devices on the network the computer is on reach MixMusic, for
// remote control and casting; why not when they don't; and Allow, which
// lets them in on private networks with an administrator's approval.

import (
	"errors"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/firewall"
)

func init() { addSettings("desktop", 17, (*shell).firewallSettings) }

// The title of the group, and of its row.
const (
	firewallTitle = "Windows Firewall"
	firewallRow   = "Connections from your network"
)

// firewallStale is how long what the firewall does is kept before it's
// read again, at the next frame: the user may answer Windows' question, or
// join another network, meanwhile.
const firewallStale = 5 * time.Second

// networkSettings is the page of Windows' settings that makes a network
// private.
const networkSettings = "ms-settings:network-status"

// firewallView is what the settings know of the firewall, from frame to
// frame.
type firewallView struct {
	state firewall.State
	err   error
	// read is when state was read; zero before.
	read time.Time
	// asked counts the reads asked for (reread), and answered the one the
	// last read answers: one asked for while another runs runs after it.
	asked, answered int
	// reading is set while it's read, and allowing while Allow waits for
	// the administrator's approval and the change.
	reading, allowing bool
}

// known reports whether the firewall was read, without an error.
func (v *firewallView) known() bool { return !v.read.IsZero() && v.err == nil }

// reread has the firewall read again at the next frame, as something may
// have changed it.
func (v *firewallView) reread() { v.asked++ }

// fresh reports whether the firewall was read lately, since the last
// reread.
func (v *firewallView) fresh() bool {
	return !v.read.IsZero() && v.answered == v.asked && time.Since(v.read) < firewallStale
}

// readFirewall reads what the firewall fw does to MixMusic beside the
// main thread, when it wasn't read lately.
func (s *shell) readFirewall(fw *app.Firewall) {
	v := &s.firewall
	if v.reading || v.fresh() {
		return
	}
	v.reading = true
	asked := v.asked
	go func() {
		st, err := fw.Check()
		s.update(func() {
			v.state, v.err, v.read, v.reading, v.answered = st, err, time.Now(), false, asked
		})
	}()
}

// firewallSettings tell what Windows Firewall does to MixMusic, with
// Allow when it keeps MixMusic out by the computer's own rules, and a link
// to Windows' network settings on a public network.
func (s *shell) firewallSettings(c *ui.Context) {
	fw := s.a.Firewall()
	if fw == nil {
		return
	}
	s.readFirewall(fw)
	v := &s.firewall
	s.group(c, firewallTitle, func() {
		s.row(c, firewallRow, firewallAbout(v), func() {
			switch st := v.state; {
			case !v.known():
			case st.CanAllow() || v.allowing:
				label := "allow"
				if v.allowing {
					label = "allowing…"
				}
				b := s.button(c, "firewall-allow", label, true).Label("Allow MixMusic in Windows Firewall")
				if b.Clicked() && !v.allowing {
					s.allowFirewall(fw)
				}
			case st.Public && !st.LetsIn() && !st.Managed && !st.Shielded:
				s.linkOf(c, "network settings", networkSettings)
			}
		}, "windows firewall network connections allow blocked incoming remote control cast public private administrator")
	})
}

// firewallAbout tells what Windows Firewall does to MixMusic on the
// network the computer is on, and why it keeps MixMusic out when it does.
func firewallAbout(v *firewallView) string {
	st := v.state
	switch {
	case v.read.IsZero():
		return "Reading whether Windows Firewall lets other computers and cast devices on this network reach MixMusic…"
	case v.err != nil:
		return "MixMusic couldn't read Windows Firewall: " + v.err.Error() + "."
	case !st.On:
		return "Windows Firewall is off for this network, so other computers and cast devices on it reach MixMusic, for remote control and casting."
	case st.LetsIn():
		return "Windows Firewall lets other computers and cast devices on this network reach MixMusic, for remote control and casting."
	case st.Managed:
		return "Windows Firewall keeps other computers and cast devices on this network from reaching MixMusic, and your organization's policy decides what it lets in. Its administrators can let MixMusic in."
	case st.Shielded:
		return "Windows Firewall blocks every incoming connection on this network, those of the apps it allows too, so other computers and cast devices can't reach MixMusic. Windows Security turns that off, in Firewall & network protection."
	case st.Public:
		return "Windows counts this network as public, so Windows Firewall keeps other computers and cast devices on it from reaching MixMusic. If you trust the network, as the one at home, make it private in Windows' network settings."
	case st.Blocked:
		return "Windows Firewall keeps other computers and cast devices on this network from reaching MixMusic, as it does once its question about MixMusic is answered with Cancel. Allow lets them in on private networks, with an administrator's approval."
	case st.Asks:
		return "Windows Firewall doesn't let other computers and cast devices reach MixMusic yet: it asks the first time remote control or casting starts. Allow lets them in on private networks now, with an administrator's approval."
	}
	return "Windows Firewall keeps other computers and cast devices on this network from reaching MixMusic, and doesn't ask about apps. Allow lets them in on private networks, with an administrator's approval."
}

// firewallKeepsOut reports whether the firewall, read, keeps other
// computers out of this one, for remote control's group to say so.
func (s *shell) firewallKeepsOut() bool {
	return s.a.Firewall() != nil && s.firewall.known() && !s.firewall.state.LetsIn()
}

// allowFirewall lets MixMusic in, with the administrator's approval
// Windows asks for, beside the main thread, and reads the firewall again.
func (s *shell) allowFirewall(fw *app.Firewall) {
	v := &s.firewall
	v.allowing = true
	go func() {
		err := fw.Allow()
		s.update(func() {
			v.allowing = false
			v.reread()
			switch {
			case err == nil:
				s.notes = append(s.notes, notice("Windows Firewall lets MixMusic in on private networks"))
			case errors.Is(err, firewall.ErrDeclined):
				s.notes = append(s.notes, notice("Windows Firewall wasn't changed: "+firewall.ErrDeclined.Error()))
			default:
				s.notes = append(s.notes, noticeOf(err))
			}
		})
	}()
}
