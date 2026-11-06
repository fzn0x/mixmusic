package app

// Whether a window of MixMusic's is in front, which shows what a
// notification would tell: the notification would only repeat it over the
// window the user looks at (#289).

import "github.com/egoist/mygo"

// focused reports whether a window has the keyboard; tests, which open no
// windows, replace it.
var focused = (*mygo.Window).IsFocused

// inFront reports whether the main window or the mini player has the
// keyboard: both show what plays, and whether it is liked or disliked.
func (a *App) inFront() bool {
	a.mu.Lock()
	main, mini := a.main, a.mini
	a.mu.Unlock()
	return main != nil && focused(main) || mini != nil && focused(mini)
}

// playingInFront is inFront, or the player around the notch has the
// keyboard. That is a panel on macOS, the only system it opens on, which
// never activates MixMusic, but takes the keyboard as its panel opens and
// gives it back as it closes: open, it shows what plays, though not
// whether it is liked or disliked.
func (a *App) playingInFront() bool {
	if a.inFront() {
		return true
	}
	a.notch.mu.Lock()
	notch := a.notch.win
	a.notch.mu.Unlock()
	return notch != nil && focused(notch)
}
