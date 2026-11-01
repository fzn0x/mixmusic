package native

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
	// window's buttons, and the transport with the like button.
	miniRoom = 232
)

// OpenMini opens the native interface's mini player: a small window over
// the others that shows what plays and controls it, as the web
// interface's does.
func OpenMini(a *app.App) *mygo.Window {
	registerFonts()
	s := newShell(a, a.GoPlayer())
	s.attach(mygo.NewWindow(mygo.WindowOptions{
		Title:       "MixMusic mini player",
		Width:       420,
		Height:      132,
		MinWidth:    340,
		MinHeight:   112,
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
	s.followArtwork()
	c.SetTheme(&s.lk.theme)
	s.toasts(c)
	k := &s.lk
	w, h := c.Size()
	// The artwork is square, but narrower in a narrow window, which it
	// leaves the transport.
	side := min(h, max(w-miniRoom, 0))
	cur, ok := s.st.Current()
	root := ui.Row(c).Fill().Background(k.bg).TextColor(k.text).Font(k.body).DragWindow()
	s.ground(c, root)
	root.Children(func() {
		art := ui.Box(c.Key("mini-art")).Size(side, h).Shrink(0).Clip()
		var b *ui.Bitmap
		if ok {
			b = s.art.get(c, artworkURL(cur.Track.Artwork, miniArt), artPixels(miniArt))
		}
		if b == nil {
			art.Background(k.surface3)
		}
		art.Children(func() {
			if b != nil {
				ui.Image(c, b).Size(side, h).Fit(ui.Cover)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Padding(0, 12).Gap(8).Justify(ui.Center).Children(func() {
			title, line := "nothing playing", "play something in MixMusic"
			if ok {
				title, line = cur.Track.Title, names(cur.Track.Artists)
			}
			ui.Column(c).MinWidth(0).Children(func() {
				ui.Text(c, title).FontSize(k.size(1.25)).FontWeight(300).Font(k.display).
					LetterSpacing(-0.02 * k.size(1.25)).SingleLine().MinWidth(0)
				ui.Row(c).Gap(6).MinWidth(0).Children(func() {
					if ok {
						s.sourceMark(c, cur.Track.Source, false)
					}
					ui.Text(c, line).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().Grow(1).MinWidth(0)
					if ok {
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
	if ok {
		s.miniProgress(c)
	}
	s.overlay(c)
}

// miniControls are the mini player's transport, and its like button.
func (s *shell) miniControls(c *ui.Context) {
	cur, ok := s.st.Current()
	ui.Row(c).Gap(8).Children(func() {
		if s.iconButton(c, "mini-previous", "previous", "Previous", false, 18).Clicked() {
			s.p.Previous()
		}
		name, label := "play", "Play"
		switch {
		case s.st.Loading:
			label = "Loading"
		case s.st.Playing:
			name, label = "pause", "Pause"
		}
		toggle := s.round(c, "mini-toggle", name, label, true, s.st.Loading || !ok && len(s.st.Queue) == 0, 40, 18)
		if s.st.Buffering {
			toggle.Opacity(0.55 + 0.45*toggle.Loop("busy", 900*time.Millisecond, ui.EaseInOut))
		}
		if toggle.Clicked() {
			s.p.Toggle()
		}
		if s.iconButton(c, "mini-next", "next", "Next", false, 18).Clicked() {
			s.p.Next()
		}
		if ok {
			s.likeButton(c, "mini-like", cur.Track, 16)
		}
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

// miniProgress draws how far the track has played, in a 2px line of the
// accent along the foot of the window.
func (s *shell) miniProgress(c *ui.Context) {
	k := &s.lk
	done := float32(0)
	if d := s.miniDuration(); d > 0 {
		done = min(float32(s.st.Position)/float32(d), 1)
	}
	ui.Box(c).Absolute().Left(0).Bottom(0).FillWidth().Height(2).PassThrough().Label("Progress").
		Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * done, H: r.H}, k.accent, 0)
		})
}
