// Package native draws MixMusic's native interface, a beta beside the web
// one: windows MyGo draws itself with its ui package, without a webview.
// They show what the web interface shows, from the same services, and play
// through the queue in Go (internal/player).
package native

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/plugins"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// Open opens the native interface's main window, hidden when asked.
func Open(a *app.App, hidden bool) *mygo.Window {
	registerFonts()
	s := newShell(a, a.GoPlayer())
	s.attach(mygo.NewWindow(mygo.WindowOptions{
		Title:           "MixMusic",
		Width:           1280,
		Height:          820,
		MinWidth:        940,
		MinHeight:       600,
		BackgroundColor: hexColor(s.lk.bg),
		StateKey:        "main",
		TitleBarStyle:   mygo.TitleBarHidden,
		TitleBarHeight:  48,
		Hidden:          hidden,
		Content:         ui.View(s.view),
	}))
	s.start()
	s.usePlugins()
	s.offerCrashReport()
	s.captureFromEnv()
	return s.win
}

// shell is the main window: its state, which belongs to the main thread
// but where its methods say otherwise.
type shell struct {
	a   *app.App
	svc app.Services
	p   *player.Player
	win *mygo.Window

	router       *ui.Router
	set          settings.Settings
	inv          addons.Inventory
	lk           look
	art          *artworks
	loads        map[string]any
	notes        []player.Notice
	actionToasts []actionToast
	stop         []func()
	// pending holds what update ran without a window, which pendingMu
	// guards with win while it's set.
	pendingMu sync.Mutex
	pending   []func()

	// st is what the player does, as the frame shows it.
	st player.State
	// artURL is the artwork the accent follows, and artColor its colour,
	// once known.
	artURL, artColor string
	// asking is the question a dialog asks before an action, if any.
	asking *question
	// panel is the side panel shown: "queue", "lyrics" or "".
	panel string
	// query is what the search box holds.
	query     string
	searchBox ui.Handle
	queueList ui.ListState
	// seeking and seekTo are the seek bar while the user drags it.
	seeking bool
	seekTo  float64
	volume  float64

	// The pages' own state.
	homeFilter  int
	searchPivot int
	// shelfScroll is where the shelves scrolled, by page and shelf.
	shelfScroll map[string]*ui.ScrollState
	// artistPivot is the pivot chosen on the page of the artist artistOf.
	artistPivot int
	artistOf    string
	searchScope int
	libraryTab  int
	settingsTab int
	statsRange  int
	// The command palette, what it searches, and its row chosen.
	paletteOpen  bool
	paletteQuery string
	paletteAt    int
	lyricsView   lyricsState
	// pagePath is the path of the page being built.
	pagePath string
	// capScroll scrolls the pages in capture mode, nil otherwise, and
	// capBound tells whether a page took it in the build.
	capScroll *ui.ScrollState
	capBound  bool
	// Dialogs, and their subjects; naming asks for a name, as a new mix's.
	naming                              *namePrompt
	renaming, deleting, clearingHistory bool
	renameTo, renameID                  string
	deleteID, deleteTitle               string
	// spotifyMore holds the liked songs on Spotify loaded past the first
	// page.
	spotifyMore []media.Track
	loadingMore bool
	// drafts are the texts of the settings' inputs, by key; the Last.fm
	// sign-in's fields; and the service whose sign-in waits.
	drafts                  map[string]*draft
	lastfmKey, lastfmSecret string
	signingIn               string
	// wells are the colour wells' colours, by key; tools are how the user
	// sorted and searched the lists of tracks, by key.
	wells map[string]*colorWell
	tools map[string]*trackTool

	// plugins runs the app's plugins, for the main window; nil in others.
	plugins *plugins.Host
	// mountedViews are the plugins' views the window mounted, seenViews
	// those the frame showed; pluginInputs the texts of their inputs,
	// canvasSizes the sizes their canvases drew at, cssColors the colours
	// they drew in, and svgs their icons.
	mountedViews, seenViews map[*plugins.View]bool
	pluginInputs            map[string]*string
	canvasSizes             map[*plugins.Canvas][2]float32
	cssColors               map[string]ui.Color
	svgs                    map[string]*ui.SVG
	// licenseOpen is the license shown.
	licenseOpen int
	// installing is set while yt-dlp installs, which installDone of
	// installTotal bytes came of; skippedYTDLP is the release the user put
	// off.
	installing                bool
	installDone, installTotal int64
	skippedYTDLP              string
	// resetting asks to reset the settings; editingShortcut is the command
	// whose shortcut the user writes, shortcutDraft what they wrote.
	resetting       bool
	editingShortcut string
	shortcutDraft   string
	// offer is the new version the window offers, until the user answers;
	// updating is set while it downloads, updated once it is installed.
	offer                   *app.Release
	updating                bool
	updateDone, updateTotal int64
	updated                 string
	// The add-on installing, and the plugin whose install the user
	// confirms.
	installingAddon  string
	pluginToInstall  addons.Entry
	confirmingPlugin bool
}

func newShell(a *app.App, p *player.Player) *shell {
	s := &shell{a: a, svc: a.Services(), p: p, router: ui.NewRouter("/"), loads: map[string]any{}, drafts: map[string]*draft{}, shelfScroll: map[string]*ui.ScrollState{},
		wells: map[string]*colorWell{}, tools: map[string]*trackTool{}, panel: "queue", licenseOpen: -1,
		mountedViews: map[*plugins.View]bool{}, seenViews: map[*plugins.View]bool{}, pluginInputs: map[string]*string{},
		canvasSizes: map[*plugins.Canvas][2]float32{}, cssColors: map[string]ui.Color{}, svgs: map[string]*ui.SVG{}}
	s.queueList.Selected = new(int)
	*s.queueList.Selected = -1
	s.art = newArtworks(s.update)
	s.refresh()
	return s
}

// start follows the player and the app, until the window closes, and
// shows the player's notices and the pages that deep links ask for.
func (s *shell) start() {
	s.follow(
		s.p.Notices(func(n player.Notice) { s.update(func() { s.notes = append(s.notes, n) }) }),
		s.a.OnNavigate(func(target string) { s.update(func() { s.open(target) }) }),
	)
}

// follow redraws the window as the player, the app and the computer's
// appearance change, until it closes, and runs stop then too.
func (s *shell) follow(stop ...func()) {
	s.stop = append(s.stop,
		s.p.Subscribe(func(player.Event) { s.win.Invalidate() }),
		s.a.Watch(func() { s.update(s.refresh) }),
		mygo.Theme.OnUpdated(func() { s.update(s.restyle) }),
	)
	s.stop = append(s.stop, stop...)
	s.win.OnClosed(func() {
		for _, fn := range s.stop {
			fn()
		}
	})
}

// update runs fn on the main thread, then redraws. Until the shell has its
// window, which its first frame comes before, fn waits for it; without
// one, as in tests, for drain.
func (s *shell) update(fn func()) {
	s.pendingMu.Lock()
	win := s.win
	if win == nil {
		s.pending = append(s.pending, fn)
	}
	s.pendingMu.Unlock()
	if win != nil {
		win.Update(fn)
	}
}

// attach gives the shell its window, which runs what update kept for it.
func (s *shell) attach(win *mygo.Window) {
	s.pendingMu.Lock()
	s.win = win
	fns := s.pending
	s.pending = nil
	s.pendingMu.Unlock()
	for _, fn := range fns {
		win.Update(fn)
	}
}

// drain runs what update kept without a window.
func (s *shell) drain() {
	s.pendingMu.Lock()
	fns := s.pending
	s.pending = nil
	s.pendingMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// refresh reads the settings and the add-ons again, and the look they make.
func (s *shell) refresh() {
	s.set = s.svc.Preferences.Get()
	s.inv = s.svc.Addons.List()
	s.forget("accounts")
	if s.offer == nil {
		s.offer = s.svc.System.UpdateOffer()
	}
	s.restyle()
	s.syncPlugins()
}

// restyle makes the look again, of the settings, the computer's appearance
// and the artwork playing.
func (s *shell) restyle() {
	s.lk = newLook(s.set, s.inv.Themes, isDark(), s.artColor)
	s.art.phosphor = s.lk.fx.phosphor
	s.plugins.SetTheme(pluginTheme(&s.lk))
}

// followArtwork takes the accent from the artwork of what plays, as the
// settings ask: once a track with other artwork starts, it asks for the
// artwork's colour, beside the main thread.
func (s *shell) followArtwork() {
	url := ""
	if cur, ok := s.st.Current(); ok {
		url = artworkURL(cur.Track.Artwork, 120)
	}
	switch {
	case url == "":
		s.artURL = ""
		if s.artColor != "" {
			s.artColor = ""
			s.restyle()
		}
		return
	case !s.set.DynamicAccent || url == s.artURL:
		return
	}
	s.artURL = url
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		color, err := s.svc.System.ArtworkColor(ctx, url)
		s.update(func() {
			if s.artURL != url {
				return // another track's meanwhile
			}
			if err != nil {
				color = ""
			}
			if color != s.artColor {
				s.artColor = color
				s.restyle()
			}
		})
	}()
}

// question is what a dialog asks before an action: run does it, and
// cancel, if set, runs when the user turns it down.
type question struct {
	title, message, ok string
	run, cancel        func()
}

// ask has a dialog ask the user before run does what it says.
func (s *shell) ask(title, message, ok string, run func()) {
	s.asking = &question{title: title, message: message, ok: ok, run: run}
}

// questionDialog shows the question asked, if any.
func (s *shell) questionDialog(c *ui.Context) {
	q := s.asking
	if q == nil {
		return
	}
	open := true
	if ui.AlertDialog(c, &open, q.title, q.message, "Cancel", q.ok) == 1 {
		s.asking = nil
		q.run()
		return
	}
	if !open {
		s.asking = nil
		if q.cancel != nil {
			q.cancel()
		}
	}
}

// namePrompt asks for a name, as a new mix's: what the dialog says, the
// name so far, and what is done with it once confirmed.
type namePrompt struct {
	title, label, value, confirm string
	done                         func(name string)
}

// promptName asks for a name, starting from value, and runs done with it
// once the user confirms it.
func (s *shell) promptName(title, label, value, confirm string, done func(name string)) {
	s.naming = &namePrompt{title: title, label: label, value: value, confirm: confirm, done: done}
}

// promptDialog shows the name asked for, if any.
func (s *shell) promptDialog(c *ui.Context) {
	p := s.naming
	if p == nil {
		return
	}
	k := &s.lk
	open := true
	panel := ui.Modal(c, &open, func() {
		ui.Text(c, strings.ToLower(p.title)).FontSize(k.titleSize()).FontWeight(300).Font(k.display)
		in := ui.TextInput(c.Key("prompt"), &p.value).AutoFocus().Label(p.label).Width(360)
		name := strings.TrimSpace(p.value)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if s.button(c, "prompt-cancel", "cancel", false).Clicked() {
				s.naming = nil
			}
			if (s.button(c, "prompt-ok", p.confirm, true).Disabled(name == "").Clicked() || in.Submitted()) && name != "" {
				s.naming = nil
				p.done(name)
			}
		})
	})
	s.frosted(panel).Padding(32).Gap(16).Border(1, k.border)
	if !open {
		s.naming = nil
	}
}

// offerCrashReport asks, when MixMusic crashed the time before, to report
// it on GitHub; either way, it doesn't ask again.
func (s *shell) offerCrashReport() {
	report := s.svc.System.LastCrash()
	if report == "" {
		return
	}
	s.asking = &question{
		title:   "MixMusic quit unexpectedly",
		message: "It kept a report of what happened, " + report + ", in its logs folder. Reporting it on GitHub, with that file attached, helps fix it.",
		ok:      "Report it",
		run: func() {
			s.svc.System.DismissCrash()
			go func() {
				_ = s.svc.System.OpenExternal(s.svc.System.ReportURL(context.Background()))
				_ = s.svc.System.OpenLogsFolder()
			}()
		},
		cancel: s.svc.System.DismissCrash,
	}
}

// isDark reports whether the computer is in dark mode. It asks MyGo's main
// loop, which tests don't run.
var isDark = func() bool { return mygo.Theme.IsDark() }

// navigate shows the page at path.
func (s *shell) navigate(path string) { s.router.Push(path) }

// open shows what a deep link asks for: a page, or the lyrics, which show
// in the side panel. A search shows in the search box too.
func (s *shell) open(target string) {
	if target == "/lyrics" {
		s.panel = "lyrics"
		return
	}
	if u, err := url.Parse(target); err == nil && u.Path == "/search" {
		s.query = u.Query().Get("q")
	}
	s.navigate(target)
}

// loaded is what a page shows that comes from the network.
type loaded[T any] struct {
	value T
	err   error
	done  bool
}

// fetch returns what get gives for key, which it asks for once, beside the
// main thread: until it comes, the result isn't done.
func fetch[T any](s *shell, key string, get func(ctx context.Context) (T, error)) *loaded[T] {
	if l, ok := s.loads[key].(*loaded[T]); ok {
		return l
	}
	l := &loaded[T]{}
	s.loads[key] = l
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		v, err := get(ctx)
		s.update(func() { l.value, l.err, l.done = v, err, true })
	}()
	return l
}

// forget drops what fetch got for key, so that the page asks again.
func (s *shell) forget(key string) { delete(s.loads, key) }

func hexColor(c ui.Color) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
