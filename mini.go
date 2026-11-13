package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

const (
	// miniArt is the size the mini player's artwork loads at: the tallest
	// the window grows, so that resizing it loads no other image.
	miniArt = 180
	// miniRoom is the width the mini player keeps beside its artwork: the
	// title with the window's buttons, the artists with the panels'
	// buttons, and the transport with the like, dislike and volume
	// buttons, which miniRowOf fits in it. At the size it opens at, the
	// artwork still takes the window's height, on Windows too, where the
	// frameless window's borders take 8 DIPs of it at the sides and the
	// foot.
	miniRoom = 280
	// The mini player's window at 100%: the size it opens at, and the
	// least it takes.
	miniWidth, miniHeight       = 420, 132
	miniMinWidth, miniMinHeight = 340, 112
)

// OpenMini opens the native interface's mini player: a small window over
// the others that shows what plays and controls it, as the web
// interface's does.
func OpenMini(a *app.App) *mygo.Window {
	registerFonts()
	s := newShell(a, a.GoPlayer())
	s.attach(mygo.NewWindow(mygo.WindowOptions{
		Title:       locales.MiniWindowTitle(),
		Width:       miniWidth,
		Height:      miniHeight,
		MinWidth:    miniMinWidth,
		MinHeight:   miniMinHeight,
		MaxHeight:   miniArt,
		Frameless:   true,
		AlwaysOnTop: true,
		// The whole window drags it: a double click on it doesn't spread
		// it across the screen.
		DisableMaximize: true,
		BackgroundColor: hexColor(s.lk.bg),
		StateKey:        "mini",
		Content:         ui.View(s.miniView),
	}))
	s.mini.win = nativeMini{s.win}
	// Closed with its panel open, the window keeps the player's size for
	// the next time, which opens the panel again.
	s.follow(append(s.followMiniPanel(), mygo.App.OnLocaleChanged(func(string) { s.win.SetTitle(locales.MiniWindowTitle()) }), s.win.OnClose(func(*mygo.CloseEvent) { s.compactMini() }))...)
	s.captureMiniFromEnv()
	return s.win
}

// miniView builds the mini player: the artwork the height of the player,
// what plays with the buttons of the panels and the transport beside it,
// buttons that open MixMusic or close the window, and the progress along
// its foot, which seeks. The panel shows above or below it, where the
// window grows for it (minipanel.go). Dragging it anywhere but its
// controls moves the window.
func (s *shell) miniView(c *ui.Context) {
	s.st = s.p.State()
	s.applyMiniZoom(c)
	// The mini player runs no shortcuts (#38) but the zoom's, and Escape,
	// which closes its panel: pointerKeys leaves its controls no keys.
	s.plainKeys = s.plainKeys[:0]
	s.runZoomShortcuts(c)
	s.followArtwork()
	c.SetTheme(&s.lk.theme)
	s.toasts(c)
	s.beginMini(c)
	k := &s.lk
	w, h := c.Size()
	part, room := s.miniHeights(h)
	// The artwork is square, but narrower in a narrow window, which it
	// leaves the transport.
	side := min(part, max(w-miniRoom, 0))
	_, ok := s.st.Current()
	// A click in it leaves the focus where it was: the mini player runs no
	// shortcuts (#38), and Space would press the button clicked last again.
	root := ui.Column(c).Fill().Background(k.bg).TextColor(k.text).Font(k.body).DragWindow().KeepFocus()
	// The panel above the player comes after it, for Tab, as below it.
	if !s.mini.below {
		root.Reverse()
	}
	// Its menu, on a right click, holds the buttons the window has no
	// room for, and sets the speed of what plays.
	root.ContextMenu(s.miniMenu)
	s.ground(c, root)
	root.Children(func() {
		ui.Row(c.Key("mini-player")).Height(part).Shrink(0).Children(func() {
			s.miniArtwork(c, side, part)
			s.miniBody(c, w-side)
			if ok && !s.st.Live {
				s.miniSeek(c, w)
			}
		})
		if room > 0 {
			s.miniPanel(c, w, room)
		}
	})
	s.miniEscape(c)
	s.moveMini(c)
	s.miniPages()
	if s.mini.win != nil {
		// What the menus of the queue's entries show and ask, as a
		// track's Track info and a new mix's name.
		s.questionDialog(c)
		s.trackInfoDialog(c)
		s.promptDialog(c)
	}
	s.overlay(c)
}

// miniArtwork is the artwork of what plays, side wide and as tall as the
// player.
func (s *shell) miniArtwork(c *ui.Context, side, h float32) {
	art := ui.Box(c.Key("mini-art")).Size(side, h).Shrink(0).Clip()
	var b *ui.Bitmap
	if cur, ok := s.st.Current(); ok {
		b = s.art.get(c, artworkURL(cur.Track.Artwork, int(miniArt*s.zoomOf())), s.artPixels(miniArt))
	}
	if b == nil {
		art.Background(s.lk.surface3)
	}
	art.Children(func() {
		if b != nil {
			ui.Image(c, b).Size(side, h).Fit(ui.Cover)
		}
	})
}

// miniBody is what the mini player shows beside its artwork, w wide: the
// title with the window's buttons, the artists and the time with the
// panels' buttons, and the transport.
func (s *shell) miniBody(c *ui.Context, w float32) {
	k := &s.lk
	cur, ok := s.st.Current()
	ui.Column(c).Grow(1).MinWidth(0).Padding(2, 0, miniSeekRoom-4).PaddingStart(12).PaddingEnd(4).
		Gap(4).Justify(ui.Center).Children(func() {
		title, line := k.words(locales.MiniRowNothingPlaying()), locales.MiniRowPlaySomething()
		if ok {
			title, line = cur.Track.Title, names(cur.Track.Artists)
		}
		if ok && s.st.Waiting {
			line = locales.PlayerWaiting(line)
		}
		ui.Column(c).MinWidth(0).Children(func() {
			ui.Row(c).Gap(2).AlignItems(ui.Center).MinWidth(0).Children(func() {
				ui.Text(c, title).FontSize(k.size(1.25)).FontWeight(k.titleWeight()).Font(k.display).
					LetterSpacing(k.tracking(-0.02, k.size(1.25))).SingleLine().Grow(1).MinWidth(0).PaddingEnd(6)
				// Both close this window once this frame is drawn: on Linux
				// the frame draws in the window's GL area, and GTK crashed
				// finishing it once the window was gone (#37).
				if s.miniButton(c, "mini-open", "mini", locales.MiniRowOpen(), false, 26, 15).Clicked() {
					go s.svc.System.ShowMainWindow()
				}
				if s.miniButton(c, "mini-close", "close", locales.MiniRowClose(), false, 26, 15).Clicked() {
					go s.win.Close()
				}
			})
			ui.Row(c).Gap(6).AlignItems(ui.Center).MinWidth(0).Children(func() {
				if ok {
					s.sourceMark(c, cur.Track.Source, false)
				}
				ui.Text(c, line).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().Grow(1).MinWidth(0)
				switch {
				case ok && s.st.Live:
					ui.Text(c, k.sentence(locales.PlayerLive())).FontSize(k.captionSize()).TextColor(k.accent).Shrink(0).SingleLine()
				case ok:
					ui.Text(c, clock(s.st.Position)+" / "+clock(s.miniDuration())).FontSize(k.captionSize()).
						TextColor(k.muted).FontFeatures("tnum").Shrink(0).SingleLine()
				}
				s.miniPanelButtons(c)
			})
		})
		s.miniControls(c, w-16)
	})
}

// miniControls are the mini player's transport, with shuffle and repeat,
// its like and dislike buttons, and its volume, in a row w wide, which
// leaves out the buttons it has no room for (miniRowOf): the mini player's
// menu holds them then.
func (s *shell) miniControls(c *ui.Context, w float32) {
	cur, ok := s.st.Current()
	r := miniRowOf(w, ok)
	s.mini.hidden = r.hidden
	ui.Row(c).Gap(r.groupGap).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(r.gap).AlignItems(ui.Center).Children(func() {
			if r.shows(miniShuffle) {
				shuffle := s.miniButton(c, "mini-shuffle", "shuffle", locales.PlayerShuffle(), s.st.Shuffle, 28, 16)
				if shuffle.Role(ui.RoleToggleButton).Checked(s.st.Shuffle).Size(28, 32).Clicked() {
					s.p.ToggleShuffle()
				}
			}
			if s.miniButton(c, "mini-previous", "previous", locales.PlayerPrevious(), false, 30, 18).Size(30, 32).Clicked() {
				s.p.Previous()
			}
			name, label := toggleOf(s.st)
			toggle := s.round(c, "mini-toggle", name, label, true, s.st.Loading || !ok && len(s.st.Queue) == 0, 40, 18)
			if pulses(s.st) {
				toggle.Opacity(0.55 + 0.45*toggle.Loop("busy", 900*time.Millisecond, ui.EaseInOut))
			}
			if toggle.Clicked() {
				s.p.Toggle()
			}
			if s.miniButton(c, "mini-next", "next", locales.PlayerNext(), false, 30, 18).Size(30, 32).Clicked() {
				s.p.Next()
			}
			if r.shows(miniRepeat) {
				name, label := repeatOf(s.st.Repeat)
				if s.miniButton(c, "mini-repeat", name, label, s.st.Repeat != player.RepeatOff, 28, 16).Size(28, 32).Clicked() {
					s.p.CycleRepeat()
				}
			}
		})
		ui.Row(c).Children(func() {
			if r.shows(miniLike) {
				s.likeButton(c, "mini-like", cur.Track, 16)
			}
			if r.shows(miniDislike) {
				s.dislikeButton(c, "mini-dislike", cur.Track, 16)
			}
			s.miniVolume(c)
		})
	})
}

// miniButton is an iconButton of the mini player, size wide and high, its
// icon glyph high.
func (s *shell) miniButton(c *ui.Context, key, name, label string, on bool, size, glyph float32) ui.Element {
	return s.iconButton(c, key, name, label, on, glyph).Size(size, size).Radius(s.lk.round(size))
}

// repeatOf is the icon and the label of the repeat button, which tell
// what it repeats.
func repeatOf(r player.Repeat) (name, label string) {
	switch r {
	case player.RepeatAll:
		return "repeat", locales.PlayerRepeatAll()
	case player.RepeatOne:
		return "repeatOne", locales.PlayerRepeatOne()
	}
	return "repeat", locales.PlayerRepeatOff()
}

// miniDuration is the length of the track playing: the engine's, or the
// catalog's until the engine knows.
func (s *shell) miniDuration() time.Duration {
	if s.st.Duration > 0 {
		return s.st.Duration
	}
	if cur, ok := s.st.Current(); ok {
		return ms(cur.Track.DurationMs)
	}
	return 0
}

// applyMiniZoom zooms the mini player by the zoom chosen, and its window
// with it, so that it looks as at 100%, only larger or smaller. A change
// made in the mini player is read out rather than told in a toast, which
// would cover its controls: the window growing or shrinking shows it. The
// main window, which hears of it, doesn't toast it either (miniChose).
func (s *shell) applyMiniZoom(c *ui.Context) {
	z, was := s.set.Zoom, s.zoom
	s.zoom = z
	c.SetZoom(float32(z))
	if s.zoomToast {
		s.zoomToast = false
		c.Announce(locales.MenuZoomTitle(percent(z)))
	}
	if z == was || s.win == nil {
		return
	}
	// After the frame, as closing it is (#37), and in turn with the moves
	// of the panel's window, which read its size.
	win := s.win
	mygo.App.WhenReady(func() { fitMini(win, was, z, s.miniLimits()) })
}

// fitMini sizes the mini player's window for the zoom z, from the zoom
// was, 0 when it just opened: at the size its last run left it. Its least
// and largest heights are lim's, at 100%.
func fitMini(win *mygo.Window, was, z float64, lim miniLimit) {
	scale := func(n float32) int { return int(float64(n)*z + 0.5) }
	win.SetMinimumSize(scale(miniMinWidth), scale(lim.min))
	win.SetMaximumSize(0, scale(lim.max))
	w, h := win.ContentSize()
	if was > 0 {
		w, h = int(float64(w)*z/was+0.5), int(float64(h)*z/was+0.5)
	}
	w = max(w, scale(miniMinWidth))
	h = max(h, scale(lim.min))
	if lim.max > 0 {
		h = min(h, scale(lim.max))
	}
	if cw, ch := win.ContentSize(); cw != w || ch != h {
		win.SetContentSize(w, h)
	}
}
