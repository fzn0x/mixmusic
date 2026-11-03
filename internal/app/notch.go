package app

// The player around the MacBook's notch (#46): a window of the root
// package, as the mini player is, which Options.Notch opens. It shows
// while the setting is on, the main window is open, in the tray too, and a
// track is loaded; and only where the built-in screen has a camera
// housing, which Options.Notch finds. It closes with the main window, so
// it alone never keeps MixMusic running.

import (
	"sync"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/player"
)

// notchPlayer is the player around the notch: its window while it shows.
type notchPlayer struct {
	once sync.Once
	mu   sync.Mutex
	win  *mygo.Window
	// none is set once Options.Notch found no housing to show at, until
	// the displays change: it isn't looked for again meanwhile.
	none bool
	// asked is set while placeNotch waits for the main thread.
	asked bool
}

// followNotch shows and hides the player around the notch as the setting,
// the player and the displays change, from the main window's opening.
func (a *App) followNotch() {
	if a.opts.Notch == nil {
		return
	}
	a.notch.once.Do(func() {
		if p := a.goPlayer.Load(); p != nil {
			p.Subscribe(func(e player.Event) {
				if e != player.Progress {
					a.placeNotch()
				}
			})
		}
		// The settings, from a window or from settings.json (#153).
		a.Watch(a.placeNotch)
		mygo.Screen.OnDisplaysChanged(a.notchDisplaysChanged)
	})
	a.placeNotch()
}

// placeNotch has the main thread open or close the player around the
// notch, as notchShows says. It runs from any goroutine, the player's
// too, and returns at once.
func (a *App) placeNotch() {
	n := &a.notch
	n.mu.Lock()
	asked := n.asked
	n.asked = true
	n.mu.Unlock()
	if !asked {
		mygo.App.WhenReady(a.applyNotch)
	}
}

// applyNotch opens or closes the player around the notch, on the main
// thread.
func (a *App) applyNotch() {
	n := &a.notch
	n.mu.Lock()
	n.asked = false
	win, none := n.win, n.none
	n.mu.Unlock()
	a.mu.Lock()
	main := a.main != nil
	a.mu.Unlock()
	p := a.goPlayer.Load()
	shows := p != nil && notchShows(a.settings.Get().NotchPlayer, main, p.State())
	switch {
	case shows && win == nil && !none:
		win = a.opts.Notch(a)
		n.mu.Lock()
		n.win, n.none = win, win == nil
		n.mu.Unlock()
		if win != nil {
			win.OnClosed(func() {
				n.mu.Lock()
				if n.win == win {
					n.win = nil
				}
				n.mu.Unlock()
			})
		}
	case !shows && win != nil:
		a.closeNotch()
	}
}

// notchShows reports whether the player around the notch shows: with the
// setting on, while the main window is open, hidden in the tray too, and
// a track is loaded.
func notchShows(on, main bool, st player.State) bool {
	_, loaded := st.Current()
	return on && main && loaded
}

// closeNotch closes the player around the notch, as the main window
// closes for good.
func (a *App) closeNotch() {
	n := &a.notch
	n.mu.Lock()
	win := n.win
	n.win = nil
	n.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

// notchDisplaysChanged opens the player around the notch again as the
// displays change, a display plugged in or out, the lid closed or the
// resolution changed: at the housing's new place, or not at all where it
// went.
func (a *App) notchDisplaysChanged() {
	a.notch.mu.Lock()
	a.notch.none = false
	a.notch.mu.Unlock()
	a.closeNotch()
	a.placeNotch()
}

// OpenMainWindow shows the main window, opening it again if it was
// closed, at the page of path unless it is "", and leaves the other
// windows as they are, as the player around the notch opens MixMusic.
func (s *System) OpenMainWindow(path string) {
	mygo.App.WhenReady(func() {
		s.a.showMain()
		if path != "" {
			s.a.navigateTo(path)
		}
	})
}
