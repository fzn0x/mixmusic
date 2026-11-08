package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

const (
	// miniArt is the size the mini player's artwork loads at: the tallest
	// the window grows, so that resizing it loads no other image.
	miniArt = 180
	// miniRoom is the width the mini player keeps beside its artwork: the
	// window's buttons, and the transport with the like, dislike and
	// volume buttons, which comes as near the window's buttons as its own
	// gap. At the size it opens at, the artwork still takes the window's
	// height, on Windows too, where the frameless window's borders take 8
	// DIPs of it at the sides and the foot.
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
		Title:       "MixMusic mini player",
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
	s.follow()
	s.captureMiniFromEnv()
	return s.win
}

// miniView builds the mini player: the artwork the window's height, what
// plays and the transport beside it, buttons that open MixMusic or close
// the window, and the progress in a line along the foot. Dragging it
// anywhere but its buttons moves the window.
func (s *shell) miniView(c *ui.Context) {
	s.st = s.p.State()
	s.applyMiniZoom(c)
	// The mini player runs no shortcuts (#38) but the zoom's: pointerKeys
	// leaves its controls no keys.
	s.plainKeys = s.plainKeys[:0]
	s.runZoomShortcuts(c)
	s.followArtwork()
	c.SetTheme(&s.lk.theme)
	s.toasts(c)
	k := &s.lk
	w, h := c.Size()
	// The artwork is square, but narrower in a narrow window, which it
	// leaves the transport.
	side := min(h, max(w-miniRoom, 0))
	cur, ok := s.st.Current()
	// A click in it leaves the focus where it was: the mini player runs no
	// shortcuts (#38), and Space would press the button clicked last again.
	root := ui.Row(c).Fill().Background(k.bg).TextColor(k.text).Font(k.body).DragWindow().KeepFocus()
	// Its menu, on a right click, sets the speed of what plays.
	root.ContextMenu(s.speedItems)
	s.ground(c, root)
	root.Children(func() {
		art := ui.Box(c.Key("mini-art")).Size(side, h).Shrink(0).Clip()
		var b *ui.Bitmap
		if ok {
			b = s.art.get(c, artworkURL(cur.Track.Artwork, int(miniArt*s.zoomOf())), s.artPixels(miniArt))
		}
		if b == nil {
			art.Background(k.surface3)
		}
		art.Children(func() {
			if b != nil {
				ui.Image(c, b).Size(side, h).Fit(ui.Cover)
			}
		})
		// The transport reaches nearer the window's buttons than what plays
		// above it, as near as its own gap.
		ui.Column(c).Grow(1).MinWidth(0).PaddingStart(12).PaddingEnd(4).Gap(8).Justify(ui.Center).Children(func() {
			title, line := k.words("Nothing playing"), "play something in MixMusic"
			if ok {
				title, line = cur.Track.Title, names(cur.Track.Artists)
			}
			if ok && s.st.Waiting {
				line += " · waiting for the network"
			}
			ui.Column(c).MinWidth(0).PaddingEnd(8).Children(func() {
				ui.Text(c, title).FontSize(k.size(1.25)).FontWeight(k.titleWeight()).Font(k.display).
					LetterSpacing(k.tracking(-0.02, k.size(1.25))).SingleLine().MinWidth(0)
				ui.Row(c).Gap(6).MinWidth(0).Children(func() {
					if ok {
						s.sourceMark(c, cur.Track.Source, false)
					}
					ui.Text(c, line).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().Grow(1).MinWidth(0)
					switch {
					case ok && s.st.Live:
						ui.Text(c, k.sentence("live")).FontSize(k.captionSize()).TextColor(k.accent).Shrink(0).SingleLine()
					case ok:
						ui.Text(c, clock(s.st.Position)+" / "+clock(s.miniDuration())).FontSize(k.captionSize()).
							TextColor(k.muted).FontFeatures("tnum").Shrink(0).SingleLine()
					}
				})
			})
			s.miniControls(c)
		})
		ui.Column(c).Shrink(0).AlignSelf(ui.Start).Padding(4).Children(func() {
			// Both close this window once this frame is drawn: on Linux the
			// frame draws in the window's GL area, and GTK crashed finishing
			// it once the window was gone (#37).
			if s.iconButton(c, "mini-open", "mini", "Open MixMusic", false, 16).Clicked() {
				go s.svc.System.ShowMainWindow()
			}
			if s.iconButton(c, "mini-close", "close", "Close the mini player", false, 16).Clicked() {
				go s.win.Close()
			}
		})
	})
	if ok && !s.st.Live {
		s.miniSeek(c, w)
	}
	s.overlay(c)
}

// miniControls are the mini player's transport, its like and dislike
// buttons, and its volume.
func (s *shell) miniControls(c *ui.Context) {
	cur, ok := s.st.Current()
	ui.Row(c).Gap(8).Children(func() {
		if s.iconButton(c, "mini-previous", "previous", "Previous", false, 18).Clicked() {
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
		if s.iconButton(c, "mini-next", "next", "Next", false, 18).Clicked() {
			s.p.Next()
		}
		ui.Row(c).Children(func() {
			if ok {
				s.likeButton(c, "mini-like", cur.Track, 16)
				s.dislikeButton(c, "mini-dislike", cur.Track, 16)
			}
			s.miniVolume(c)
		})
	})
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
		c.Announce("Zoom " + percent(z))
	}
	if z == was || s.win == nil {
		return
	}
	// After the frame, as closing it is (#37).
	go fitMini(s.win, was, z)
}

// fitMini sizes the mini player's window for the zoom z, from the zoom
// was, 0 when it just opened: at the size its last run left it.
func fitMini(win *mygo.Window, was, z float64) {
	scale := func(n int) int { return int(float64(n)*z + 0.5) }
	win.SetMinimumSize(scale(miniMinWidth), scale(miniMinHeight))
	win.SetMaximumSize(0, scale(miniArt))
	w, h := win.ContentSize()
	if was > 0 {
		w, h = int(float64(w)*z/was+0.5), int(float64(h)*z/was+0.5)
	}
	w = max(w, scale(miniMinWidth))
	h = min(max(h, scale(miniMinHeight)), scale(miniArt))
	if cw, ch := win.ContentSize(); cw != w || ch != h {
		win.SetContentSize(w, h)
	}
}
