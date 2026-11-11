//go:build !windows || !(amd64 || arm64)

package firewall

// Supported is set where Check reads the system's firewall: on Windows.
const Supported = false

// Check is ErrUnsupported: MixMusic reads Windows' firewall alone.
func Check(program string) (State, error) { return State{}, ErrUnsupported }

// AllowHere is ErrUnsupported: MixMusic changes Windows' firewall alone.
func AllowHere(program string) error { return ErrUnsupported }

// Allow is ErrUnsupported: MixMusic changes Windows' firewall alone.
func Allow(program string) error { return ErrUnsupported }
