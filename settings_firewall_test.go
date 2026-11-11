package main

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/firewall"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
)

// fakeFirewall is a firewall whose state a test sets, which Allow lets
// MixMusic through, or fails with allowErr.
type fakeFirewall struct {
	mu       sync.Mutex
	state    firewall.State
	allowErr error
	allows   int
}

func (f *fakeFirewall) app() *app.Firewall { return &app.Firewall{Check: f.check, Allow: f.allow} }

func (f *fakeFirewall) check() (firewall.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, nil
}

func (f *fakeFirewall) allow() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allows++
	if f.allowErr == nil {
		f.state = firewall.State{On: true, Allowed: true, Asks: true}
	}
	return f.allowErr
}

func (f *fakeFirewall) set(s firewall.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = s
}

// firewallShell is a test shell whose firewall is f, at Settings ›
// Desktop.
func firewallShell(t *testing.T, f *fakeFirewall) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShellOf(t, func(o *app.Options) { o.Firewall = f.app() })
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	return s, tt
}

// firewallShows waits until the firewall's row tells about.
func firewallShows(t *testing.T, s *shell, tt *ui.Tester, about string) {
	t.Helper()
	until(t, s, tt, "the firewall's row tells "+about, func() bool { return tt.HasText(firewallTitle) && tt.HasText(about) })
}

// reread has the settings read the firewall again at the next frame.
func reread(s *shell) { s.firewall.reread() }

// TestFirewallAllow lets MixMusic in when Cancel kept it out: the row
// tells why, Allow asks the firewall to let it in, a toast says so, and
// the row tells it's let in, without Allow then.
func TestFirewallAllow(t *testing.T) {
	f := &fakeFirewall{state: firewall.State{On: true, Blocked: true, Asks: true}}
	s, tt := firewallShell(t, f)
	firewallShows(t, s, tt, "as it does once its question about MixMusic is answered with Cancel")
	click(t, s, tt, "Allow MixMusic in Windows Firewall")
	until(t, s, tt, "Allow", func() bool { return tt.HasText("Windows Firewall lets MixMusic in on private networks") })
	firewallShows(t, s, tt, "Windows Firewall lets other computers and cast devices on this network reach MixMusic")
	if f.allows != 1 || tt.HasText("Allow MixMusic in Windows Firewall") {
		t.Errorf("Allow ran %d times, and shows %v after", f.allows, tt.HasText("Allow MixMusic in Windows Firewall"))
	}
}

// TestFirewallAllowDeclined changes nothing when the administrator's
// approval isn't given, and says so; Allow stays.
func TestFirewallAllowDeclined(t *testing.T) {
	f := &fakeFirewall{state: firewall.State{On: true, Asks: true}, allowErr: firewall.ErrDeclined}
	s, tt := firewallShell(t, f)
	firewallShows(t, s, tt, "it asks the first time remote control or casting starts")
	click(t, s, tt, "Allow MixMusic in Windows Firewall")
	until(t, s, tt, "Allow declined", func() bool {
		return tt.HasText("Windows Firewall wasn't changed: the administrator's approval wasn't given")
	})
	f.allowErr = errors.New("Windows Firewall wasn't changed (code 1)")
	until(t, s, tt, "Allow again", func() bool { return !s.firewall.allowing && tt.HasText("Allow MixMusic in Windows Firewall") })
	click(t, s, tt, "Allow MixMusic in Windows Firewall")
	until(t, s, tt, "Allow failed", func() bool { return tt.HasText("Windows Firewall wasn't changed (code 1)") })
}

// TestFirewallPublic tells that a public network keeps MixMusic out, and
// links Windows' network settings, without Allow, which covers private
// networks alone.
func TestFirewallPublic(t *testing.T) {
	f := &fakeFirewall{state: firewall.State{On: true, Public: true, Asks: true}}
	s, tt := firewallShell(t, f)
	firewallShows(t, s, tt, "Windows counts this network as public")
	if tt.HasText("Allow MixMusic in Windows Firewall") {
		t.Error("Allow shows on a public network")
	}
	click(t, s, tt, "network settings")
	tt.Frame()
	if got := tt.OpenedURLs(); !slices.Contains(got, networkSettings) {
		t.Errorf("opened %q, want %q", got, networkSettings)
	}
}

// TestFirewallStates tells each state of the firewall, with Allow only
// where it would let MixMusic in.
func TestFirewallStates(t *testing.T) {
	f := &fakeFirewall{}
	s, tt := firewallShell(t, f)
	for _, tc := range []struct {
		state firewall.State
		about string
		allow bool
	}{
		{firewall.State{Asks: true}, "Windows Firewall is off for this network", false},
		{firewall.State{On: true, Allowed: true, Asks: true}, "Windows Firewall lets other computers and cast devices on this network reach MixMusic", false},
		{firewall.State{On: true, Managed: true, Asks: true}, "your organization's policy decides what it lets in", false},
		{firewall.State{On: true, Allowed: true, Shielded: true, Asks: true}, "Windows Firewall blocks every incoming connection on this network", false},
		{firewall.State{On: true}, "and doesn't ask about apps", true},
		{firewall.State{On: true, Blocked: true, Asks: true}, "answered with Cancel", true},
	} {
		f.set(tc.state)
		reread(s)
		firewallShows(t, s, tt, tc.about)
		if got := tt.HasText("Allow MixMusic in Windows Firewall"); got != tc.allow {
			t.Errorf("%+v: Allow shows %v, want %v", tc.state, got, tc.allow)
		}
	}
	// The search of the settings finds it.
	show(s, tt, "/settings?q=firewall")
	until(t, s, tt, "the search", func() bool { return tt.HasText(firewallRow) })
}

// TestFirewallNone shows no Windows Firewall where MixMusic doesn't read
// the firewall.
func TestFirewallNone(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	if tt.HasText(firewallTitle) || tt.HasText(firewallRow) {
		t.Errorf("Settings › Desktop shows the firewall: %q", tt.Texts())
	}
}

// TestRemoteFirewallHint says in remote control's group, while it's on,
// that the firewall keeps the other computers out, and no longer once
// MixMusic is let in.
func TestRemoteFirewallHint(t *testing.T) {
	f := &fakeFirewall{state: firewall.State{On: true, Blocked: true, Asks: true}}
	s := newRemoteShellOf(t, remotetest.New(), "Desktop", func(o *app.Options) { o.Firewall = f.app() })
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	const hint = "Windows Firewall keeps the other computers from reaching this one."
	firewallShows(t, s, tt, "answered with Cancel")
	if tt.HasText(hint) {
		t.Error("the hint shows while remote control is off")
	}
	toggle(t, s, tt, remoteSwitch, remoteAbout("Desktop"))
	until(t, s, tt, "the hint", func() bool { return s.a.Remote().On() && tt.HasText(hint) })
	f.set(firewall.State{On: true, Allowed: true, Asks: true})
	reread(s)
	until(t, s, tt, "no hint", func() bool { return !tt.HasText(hint) })
}
