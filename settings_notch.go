package main

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func init() { addSettings(desktopRows, 10, (*shell).notchRow) }

// notchRow turns the player around the notch on and off, in Settings › On
// the desktop: only on Macs whose screen has a notch.
func (s *shell) notchRow(c *ui.Context) {
	if !hasNotch() {
		return
	}
	const title = "Player around the notch"
	s.row(c, title, notchDescription, func() {
		if s.switchOf(c.Key("notch"), &s.set.NotchPlayer).Label(title).Changed() {
			s.save()
		}
	}, "notch dynamic island menu bar camera")
}

// notchDescription tells what the player around the notch does.
const notchDescription = "While a track is loaded, the black around the camera shows its cover, and bars that move with the music. " +
	"Click it for the player and the lyrics, over any app and on every Space."

// hasNotch reports whether the built-in screen has a camera housing, which
// the settings ask in every frame: tests set it.
var hasNotch = builtInNotch.has

// builtInNotch keeps whether the built-in screen has a camera housing.
var builtInNotch notchKnown

// notchKnown is whether the built-in screen has a camera housing: found
// once, and again once the displays change.
type notchKnown struct {
	once         sync.Once
	asked, found atomic.Bool
}

// has reports whether the built-in screen has a camera housing: never but
// on macOS, once the app runs.
func (k *notchKnown) has() bool {
	if runtime.GOOS != "darwin" || !mygo.App.IsReady() {
		return false
	}
	k.once.Do(func() { mygo.Screen.OnDisplaysChanged(func() { k.asked.Store(false) }) })
	if !k.asked.Load() {
		_, ok := findHousing()
		k.found.Store(ok)
		k.asked.Store(true)
	}
	return k.found.Load()
}
