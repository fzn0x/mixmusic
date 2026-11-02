package app

import (
	"context"
	"log"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// URLScheme is the scheme of MixMusic's deep links, such as
// mixmusic://search?q=daft+punk.
const URLScheme = "mixmusic"

// mediaKeys are the global shortcuts of the keyboard's media keys.
var mediaKeys = map[string]Command{
	"MediaPlayPause":     CommandToggle,
	"MediaNextTrack":     CommandNext,
	"MediaPreviousTrack": CommandPrevious,
	"MediaStop":          CommandPause,
}

// Start opens the main window and puts MixMusic on the desktop: the tray
// icon, the media keys, the deep links of portable copies. It runs once the
// app is ready.
func (a *App) Start() {
	// Quitting closes every window; the main window must close then, not
	// hide in the tray, or the quit stops there.
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting.Store(true) })
	// What waits a moment to be saved is saved as MixMusic quits: once its
	// windows closed, before the single instance lock is released and an
	// update's restart starts the new copy, and again once the loop
	// stopped, which is all that runs when Windows ends the session. The
	// second writes the queue again only if the player changed since.
	// Nothing after mygo.App.Run in main would do: Windows ends the
	// process before it returns.
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.flush() })
	mygo.App.OnQuit(a.flush)
	if runtime.GOOS != "darwin" && !mygo.App.IsPackaged() {
		if err := mygo.App.RegisterURLScheme(URLScheme); err != nil {
			log.Printf("mixmusic: registering %s links: %v", URLScheme, err)
		}
	}
	if a.openMain(mygo.App.WasOpenedAtLogin()) == nil {
		return
	}
	// Edits of settings.json apply while MixMusic runs. The watch starts
	// beside the main thread, as WatchFiles waits for it to enroll.
	ctx, stop := context.WithCancel(context.Background())
	a.mu.Lock()
	a.stopWatch = stop
	a.mu.Unlock()
	go a.watchSettings(ctx)
	a.tellStart()
	a.setupTray()
	if a.settings.Get().CloseToTray {
		runInBackground()
	}
	a.applyMediaKeys(a.settings.Get().MediaKeys)
	a.watchUpdates()
	a.watchNetwork()
	go a.keepYTDLPCurrent()
}

// keepYTDLPCurrent updates the yt-dlp MixMusic installed a minute after
// start, then once a day: YouTube changes often, and an old yt-dlp stops
// finding its streams.
func (a *App) keepYTDLPCurrent() {
	wait := time.Minute
	for {
		time.Sleep(wait)
		wait = 24 * time.Hour
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		updated, err := a.tools.UpdateYTDLP(ctx)
		cancel()
		switch {
		case err != nil:
			log.Printf("mixmusic: updating yt-dlp: %v", err)
		case updated:
			log.Printf("mixmusic: updated yt-dlp")
		}
	}
}

// openMain opens the main window, with its player, hidden when MixMusic
// starts at login. Where MixMusic can't play, it says why and quits, and
// returns nil. With close to the tray on, closing it hides it, on macOS
// out of full screen first.
func (a *App) openMain(hidden bool) *mygo.Window {
	if _, err := a.startGoPlayer(); err != nil {
		log.Printf("mixmusic: can't play: %v", err)
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{Type: mygo.MessageError, Title: "MixMusic",
			Message: "MixMusic can't play on this computer", Detail: err.Error()})
		mygo.App.Quit()
		return nil
	}
	win := a.opts.Interface(a, hidden)
	a.attachTaskbar(win)
	win.OnClose(func(e *mygo.CloseEvent) {
		a.mu.Lock()
		tray := a.tray
		a.mu.Unlock()
		if tray != nil && a.settings.Get().CloseToTray && !a.quitting.Load() {
			e.PreventDefault()
			a.trayHide.hide(win) // keep playing in the tray
		}
	})
	win.OnEnterFullScreen(func() { a.trayHide.entered(win) })
	win.OnLeaveFullScreen(func() { a.trayHide.left(win) })
	win.OnClosed(func() {
		a.mu.Lock()
		if a.main == win {
			// The thumbnail's buttons go with their window.
			a.main, a.taskbar = nil, nil
			a.trayHide.forget()
		}
		mini := a.mini
		a.mu.Unlock()
		if mini != nil {
			mini.Close()
		}
		// Spotify's hidden page goes with the main window: open, it would
		// keep MixMusic running, and playing, with no window to show it.
		a.here.close()
	})
	a.mu.Lock()
	a.main = win
	a.mu.Unlock()
	// The player may have told what plays before the window opened.
	a.retitle()
	return win
}

// titleWait is how long the main window's title waits to change. As one
// track ends and the next loads, the player tells of a pause a moment
// before it tells of the next track: the title goes from one track to the
// other.
var titleWait = 200 * time.Millisecond

// retitle names what plays in the main window's title, a moment later,
// with what changes meanwhile (#105). It changes while the window is
// minimized too, when it draws no frames, for the taskbar and the window
// switcher show the title then.
func (a *App) retitle() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.main == nil || a.retitling {
		return
	}
	a.retitling = true
	time.AfterFunc(titleWait, func() {
		mygo.App.WhenReady(func() {
			a.mu.Lock()
			a.retitling = false
			win, np := a.main, a.nowPlaying
			a.mu.Unlock()
			// On the main thread, where SetTitle doesn't wait, and never
			// with a.mu held, which the main thread takes too.
			if t := windowTitle(np, a.settings.Get().ShowInTitle); win != nil && win.Title() != t {
				win.SetTitle(t)
			}
		})
	})
}

// showMain shows the main window, opening it again if it was closed.
func (a *App) showMain() {
	a.mu.Lock()
	win := a.main
	a.mu.Unlock()
	if win == nil {
		if win = a.openMain(false); win == nil {
			return
		}
	}
	if win.IsMinimized() {
		win.Restore()
	}
	// Hidden in the tray out of full screen, it goes back in. A hide on its
	// way out of full screen is called off.
	fullScreen := a.trayHide.shown(win.IsVisible())
	win.Show()
	win.Focus()
	if fullScreen {
		win.SetFullScreen(true)
	}
}

// showMini shows the mini player.
func (a *App) showMini() {
	a.mu.Lock()
	mini := a.mini
	a.mu.Unlock()
	if mini != nil {
		mini.Show()
		mini.Focus()
		return
	}
	if a.goPlayer.Load() == nil {
		return
	}
	mini = a.opts.Mini(a)
	mini.OnClosed(func() {
		a.mu.Lock()
		if a.mini == mini {
			a.mini = nil
		}
		a.mu.Unlock()
	})
	a.mu.Lock()
	a.mini = mini
	a.mu.Unlock()
}

// setupTray adds the tray icon, whose menu controls the player.
func (a *App) setupTray() {
	if len(a.opts.TrayIcon) == 0 {
		return
	}
	cmd := func(c Command) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { a.command(PlayerCommand{Command: c}) }
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Play / Pause", Click: cmd(CommandToggle)},
		{Label: "Next", Click: cmd(CommandNext)},
		{Label: "Previous", Click: cmd(CommandPrevious)},
		mygo.Separator(),
		{Label: "Show MixMusic", Click: func(*mygo.MenuItem, *mygo.Window) { a.showMain() }},
		{Label: "Mini player", Click: func(*mygo.MenuItem, *mygo.Window) { a.showMini() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
	opts := mygo.TrayOptions{Icon: a.opts.TrayIcon, ToolTip: "MixMusic"}
	if runtime.GOOS == "linux" {
		opts.Menu = menu // AppIndicator reports no clicks: it shows the menu
	}
	tray, err := mygo.NewTray(opts)
	if err != nil {
		log.Printf("mixmusic: tray: %v", err)
		return
	}
	if runtime.GOOS != "linux" {
		// A click shows the window; a right click, the menu.
		tray.OnClick(a.showMain)
		tray.OnRightClick(func() {
			tray.SetMenu(menu)
			tray.PopUpMenu()
			tray.SetMenu(nil)
		})
	}
	a.mu.Lock()
	a.tray = tray
	a.mu.Unlock()
}

// applyMediaKeys registers the media keys as global shortcuts, or
// unregisters them.
func (a *App) applyMediaKeys(on bool) {
	a.mu.Lock()
	changed := a.mediaKeys != on
	a.mediaKeys = on
	a.mu.Unlock()
	if !changed {
		return
	}
	for key, c := range mediaKeys {
		if !on {
			mygo.GlobalShortcut.Unregister(key)
			continue
		}
		if err := mygo.GlobalShortcut.Register(key, func() { a.command(PlayerCommand{Command: c}) }); err != nil {
			log.Printf("mixmusic: media key %s: %v", key, err)
		}
	}
}

// deepLinkPathRe matches the paths deep links may open.
var deepLinkPathRe = regexp.MustCompile(`^/(search|album|artist|playlist|library|settings|marketplace|lyrics|stats|explore|mood)(/[A-Za-z0-9:_%=-]*)?$`)

// HandleURL opens a deep link: mixmusic://search?q=…, mixmusic://album/<id>,
// mixmusic://play/<track id> and the like. Register it with
// mygo.App.OnOpenURL before the app runs.
func (a *App) HandleURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != URLScheme {
		return
	}
	path := "/" + u.Host + strings.TrimSuffix(u.Path, "/")
	mygo.App.WhenReady(func() {
		if id, ok := strings.CutPrefix(path, "/play/"); ok {
			a.command(PlayerCommand{Command: CommandPlayTrack, TrackID: id})
			return
		}
		if !deepLinkPathRe.MatchString(path) {
			return
		}
		a.showMain()
		target := path
		if q := u.Query().Encode(); q != "" {
			target += "?" + q
		}
		a.navigateTo(target)
	})
}

// navigateTo shows a page of the main window.
func (a *App) navigateTo(target string) {
	a.mu.Lock()
	fn := a.onNavigate
	a.mu.Unlock()
	if fn != nil {
		(*fn)(target)
	}
}

// OnNavigate has fn show the pages that deep links ask for: the main
// window's, until cancel.
func (a *App) OnNavigate(fn func(target string)) (cancel func()) {
	p := &fn
	a.mu.Lock()
	a.onNavigate = p
	a.mu.Unlock()
	return func() {
		a.mu.Lock()
		if a.onNavigate == p {
			a.onNavigate = nil
		}
		a.mu.Unlock()
	}
}

// SecondInstance shows the main window when the user starts MixMusic again.
func (a *App) SecondInstance([]string, string) { a.showMain() }

// Activate shows the main window when the app is activated without it in
// view, as by a click on its Dock icon after it was closed to the tray, or
// while it leaves full screen to hide there.
func (a *App) Activate(bool) {
	a.mu.Lock()
	win := a.main
	a.mu.Unlock()
	if win == nil || !win.IsVisible() || win.IsMinimized() || a.trayHide.pending() {
		a.showMain()
	}
}
