//go:build !windows

package app

// processes returns none: only Windows hears what another process plays.
func processes() []proc { return nil }
