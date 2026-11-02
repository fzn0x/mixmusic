package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/portal"
)

// Inside a Flatpak's sandbox, the autostart entry MyGo writes lands in the
// sandbox's own config folder, which the desktop never reads, and a
// desktop that watches the apps running without a window, as GNOME does,
// may stop MixMusic once it is closed to the tray: the desktop's
// Background portal decides both (#79). Every request of it writes or
// removes the autostart entry, so each one carries the Start at login
// choice.

var (
	// inFlatpak tells whether MixMusic runs in a Flatpak, whose sandbox
	// has /.flatpak-info at its root.
	inFlatpak = sync.OnceValue(func() bool {
		_, err := os.Stat("/.flatpak-info")
		return err == nil
	})
	// askBackground asks the Background portal to let MixMusic run in the
	// background, and to start it at login or not.
	askBackground = func(autostart bool) (portal.Answer, error) {
		ctx, cancel := context.WithTimeout(context.Background(), backgroundWait)
		defer cancel()
		reason := "MixMusic plays on in the tray when you close its window"
		if autostart {
			reason = "MixMusic starts when you log in, as you chose in its settings"
		}
		return portal.RequestBackground(ctx, portal.Background{Reason: reason, Autostart: autostart, Commandline: loginCommand})
	}
	// setOpenAtLogin and openAtLogin are MyGo's, which in a Flatpak keep
	// the choice in the sandbox's autostart entry.
	setOpenAtLogin = func(open bool) error { return mygo.App.SetOpenAtLogin(open) }
	openAtLogin    = func() bool { return mygo.App.OpenAtLogin() }
	// background asks the portal one request at a time, so that each
	// carries the Start at login choice of the one before.
	background sync.Mutex
)

// backgroundWait is how long a request of the Background portal waits for
// the desktop, which may ask the user.
const backgroundWait = 2 * time.Minute

// loginCommand starts MixMusic at login, in the tray: the Flatpak's command,
// which the portal runs in its sandbox, with the argument of MyGo's own
// autostart entries, which mygo.App.WasOpenedAtLogin tells of.
var loginCommand = []string{"mixmusic", "--mygo-opened-at-login"}

// setLogin makes MixMusic start when the user logs in, or not: in a
// Flatpak, through the Background portal, and then in MyGo's entry, which
// OpenAtLogin reads back. When the desktop refuses, the entry stays as it
// was.
func setLogin(open bool) error {
	if !inFlatpak() {
		return setOpenAtLogin(open)
	}
	background.Lock()
	defer background.Unlock()
	answer, err := askBackground(open)
	switch {
	case errors.Is(err, portal.ErrNoBackground):
		if open {
			return errors.New("this desktop has no Background portal, which starts MixMusic at login from its Flatpak: add MixMusic to the apps your desktop starts at login instead")
		}
		// Nothing the portal wrote to remove.
	case err != nil:
		return fmt.Errorf("asking the desktop to start MixMusic at login: %w", err)
	case answer.Autostart != open && open:
		return errors.New("the desktop didn't let MixMusic start at login: allow it to run in the background in your system's settings for apps, then turn this on again")
	case answer.Autostart != open:
		return errors.New("the desktop still starts MixMusic at login")
	}
	return setOpenAtLogin(open)
}

// keepInBackground asks the desktop to let MixMusic play on in the tray,
// with no window, in a Flatpak. It waits for the answer.
func keepInBackground() error {
	if !inFlatpak() {
		return nil
	}
	background.Lock()
	defer background.Unlock()
	answer, err := askBackground(openAtLogin())
	switch {
	case errors.Is(err, portal.ErrNoBackground):
		return nil // nothing stops apps without a window there
	case err != nil:
		return fmt.Errorf("asking the desktop to let MixMusic run in the background: %w", err)
	case !answer.Background:
		return errors.New("the desktop doesn't let MixMusic run in the background: closing its window may stop it")
	}
	return nil
}

// runInBackground asks, beside the main thread, to let MixMusic play on in
// the tray, where close to the tray is on.
func runInBackground() {
	go func() {
		if err := keepInBackground(); err != nil {
			log.Printf("mixmusic: %v", err)
		}
	}()
}
