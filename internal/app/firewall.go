package app

// Windows Firewall (#496): whether other computers and cast devices on the
// network reach MixMusic, for remote control and casting, and letting
// them in with an administrator's approval.

import (
	"os"
	"sync"

	"github.com/fzn0x/mixmusic/internal/firewall"
)

// Firewall reads and changes what the system's firewall does to MixMusic.
type Firewall struct {
	// Check reads what the firewall does to connections from the network
	// the computer is on now to MixMusic.
	Check func() (firewall.State, error)
	// Allow lets MixMusic in on private and domain networks, with an
	// administrator's approval: firewall.ErrDeclined when it wasn't given.
	Allow func() error
}

// systemFirewall is Windows Defender Firewall's, of the executable that
// runs; nil on the other systems, whose firewall MixMusic doesn't read.
var systemFirewall = sync.OnceValue(func() *Firewall {
	if !firewall.Supported {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	return &Firewall{
		Check: func() (firewall.State, error) { return firewall.Check(exe) },
		Allow: func() error { return firewall.Allow(exe) },
	}
})

// Firewall returns the firewall that keeps connections from the network
// from MixMusic, or nil where MixMusic doesn't read it: all but Windows,
// and tests that give none (Options.Firewall).
func (a *App) Firewall() *Firewall {
	if a.opts.Firewall != nil {
		if a.opts.Firewall.Check == nil {
			return nil
		}
		return a.opts.Firewall
	}
	return systemFirewall()
}
