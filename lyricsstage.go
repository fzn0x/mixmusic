package main

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The full-screen lyrics, the stage, cover the window: the cover blurred
// and darkened for a ground; at the left, the buttons of the player, the
// track's cover, title and artist; in the middle its lyrics, in one of the
// styles of lyricsstyles.go; and at the right the covers of the tracks next
// in the queue (lyricsstageside.go). The lines are those of the panel's
// lyrics (lyricsWith): the same lyrics, the same translation and
// romanization the user chose there, and a click on a line plays the song
// from there. The stage has no controls for its style, the size of its text
// or its ball: those are Settings', with a preview.
//
// The stage sets its own ink, white over the dark ground, whatever the
// theme: a light theme's black text would not read on the cover.

var (
	// stageInk is the ink of the stage, and stageDim its quieter one, for
	// what is small: both read at 4.5:1 on the lightest the ground gets,
	// that of a white cover under the style's scrim, which a test checks.
	stageInk = ui.Color{R: 255, G: 255, B: 255, A: 255}
	stageDim = stageInk.Alpha(0.8)
)

var (
	// stageGlide is how the lyrics glide to the line sung, slowly, as the
	// song goes on to the next, instead of jumping there. While the computer
	// asks for less motion they do jump.
	stageGlide = ui.Animation{Duration: 650 * time.Millisecond, Ease: ui.EaseInOut}
)

// stageFade is how long a line takes to fade and blur to where its distance
// from the line sung puts it, as the lyrics glide.
const stageFade = 600 * time.Millisecond

// stageState is the stage's own.
type stageState struct {
	// open is set while the stage shows.
	open bool
	// look is what the stage was last built in. When it changes, the lines
	// change height, and the line sung is centered again at once, not glided
	// to.
	look stageLook
	// glided is set in a frame in which the line sung asked the lyrics to
	// glide to it, so that the list doesn't jump there too.
	glided bool
	// ball is what the line sung tells the ball of itself, which the overlay
	// over the lyrics paints it from (lyricsballrig.go).
	ball ballRig
}

// stageLook is what sets the height of the stage's lines: the style, the
// size of the text, the room the ball keeps above each line, and the size of
// the window, which wraps them.
type stageLook struct {
	style settings.LyricStyle
	size  float64
	ball  bool
	w, h  float32
}

// toggleStage shows the stage, or hides it. Nothing playing, and a podcast's
// episode, have no lyrics to show.
func (s *shell) toggleStage() {
	if s.stage.open {
		s.stage.open = false
		return
	}
	cur, ok := s.st.Current()
	switch {
	case !ok:
		s.notes = append(s.notes, notice("Play something first"))
	case cur.Track.IsEpisode():
		s.notes = append(s.notes, notice("Podcast episodes have no lyrics"))
	default:
		s.stage.open = true
	}
}

// stageKeys run the window's shortcuts, which wait behind a dialog, over the
// stage: the arrows seek, Space plays and pauses and the rest do what they
// do elsewhere. The stage has no keys of its own: its style, the size of its
// text and its ball are Settings'.
func (s *shell) stageKeys(c *ui.Context) { s.runShortcuts(c) }

// lyricStage shows the stage over the window, while it's open.
func (s *shell) lyricStage(c *ui.Context) {
	if !s.stage.open {
		// Shown again, it centers the line sung at once.
		s.stage.look = stageLook{}
		return
	}
	cur, ok := s.st.Current()
	if !ok || cur.Track.IsEpisode() {
		// Nothing to show: an episode has its details, not lyrics.
		s.stage.open = false
		return
	}
	t, k := cur.Track, &s.lk
	style := lyricStyleOf(s.set.LyricsStyle)
	w, h := c.Size()
	// A style, a size, the ball or a window sets the height of the lines, so
	// the line sung is no longer in the middle: center it again, at once, and
	// let the user's scrolling go, which would hold it back.
	if look := (stageLook{style.id, s.set.LyricsSize, s.set.LyricsBall, w, h}); look != s.stage.look {
		s.stage.look = look
		view := &s.lyricsView.stage
		view.scrolled, view.userAt = -1, time.Time{}
	}
	// Its lyrics refresh with the page, as the side panel's do.
	s.collecting = true
	defer func() { s.collecting = false }()
	ui.DialogBase(c, &s.stage.open, func(_, panel ui.Element) {
		// The panel fills the window, so that no click falls on the backdrop,
		// which would close the stage.
		panel.Size(w, h).Label("Full-screen lyrics").Clip()
		// The focus starts on a point that is nothing to press, without a
		// ring, so that Enter, pressed by a user who opened the stage with
		// its keys, doesn't press the close button, which would have the
		// focus. Not the panel: DialogBase makes it clickable, and a click
		// takes Space, which plays and pauses.
		ui.Box(c.Key("stage-focus")).Absolute().Top(0).Left(0).Size(1, 1).Label("Full-screen lyrics").Focusable().AutoFocus().FocusRing(false)
		s.stageGround(c, panel, t, style)
		s.stageKeys(c)
		margin := max(24, 0.03*w)
		left := min(max(0.26*w, 190), 380)
		cover := min(left, 0.42*h)
		// A narrow window has room for the lyrics before the buttons and the
		// covers of the queue.
		controls, queue := w >= 600, w >= 760
		ui.Row(c).Size(w, h).AlignItems(ui.Center).Padding(0, margin).Gap(max(20, 0.02*w)).Children(func() {
			ui.Row(c).Shrink(0).AlignItems(ui.Start).Gap(16).Children(func() {
				if controls {
					s.stageControls(c, cover)
				}
				ui.Column(c).Width(left).Shrink(0).AlignItems(ui.Center).Gap(14).Children(func() {
					s.stageCover(c, t, cover, style)
					ui.Text(c, t.Title).FontSize(k.size(1)).FontWeight(700).TextColor(stageInk).TextAlign(ui.Center).MaxLines(2)
					ui.Text(c, names(t.Artists)).FontSize(k.smallSize()).TextColor(stageDim).TextAlign(ui.Center).MaxLines(1)
				})
			})
			ui.Column(c).Grow(1).MinWidth(0).Height(h).Children(func() {
				s.lyricsWith(c, lyricsOptions{stage: &style})
				// The ball flies over the lyrics, and past the list's edges.
				ballOverlay(c, &s.stage.ball)
			})
			if queue {
				s.stageQueue(c, h)
			}
		})
		s.stageClose(c)
	})
}

// stageGround paints the ground of the stage on its panel, from the track's
// cover, small, which the GPU blends between the pixels of as it stretches
// it (as the grounds of Aurora and Glasshouse are: groundLights).
func (s *shell) stageGround(c *ui.Context, panel ui.Element, t media.Track, st lyricStyle) {
	var b *ui.Bitmap
	if len(t.Artwork) > 0 {
		b = s.art.get(c, artworkURL(t.Artwork, 64), 64)
	}
	accent := s.lk.accent
	panel.Background(ui.Color{A: 255}).Draw(func(p *ui.Painter, r ui.Rect) { paintStageGround(p, r, b, accent, st.scrim) })
}

// paintStageGround paints the ground of the stage on r: the cover b,
// stretched over it, then a dark layer of the scrim's strength, and a
// gradient that darkens its foot. Without a cover, the accent's light stands
// in for it.
func paintStageGround(p *ui.Painter, r ui.Rect, b *ui.Bitmap, accent ui.Color, scrim float32) {
	if b != nil {
		p.Image(b, r, ui.Cover)
	} else {
		p.Fill(r, accent.Alpha(0.4), 0)
	}
	p.Fill(r, ui.Color{A: uint8(255 * scrim)}, 0)
	p.FillGradient(r, ui.LinearGradient{From: ui.Color{}, To: ui.Color{A: 120}, Angle: 180}, 0)
}

// stageCover shows the track's cover, size across, in the style's corners.
func (s *shell) stageCover(c *ui.Context, t media.Track, size float32, st lyricStyle) {
	frame := ui.Box(c.Key("stage-cover")).Size(size, size).Shrink(0).Radius(st.cover).Clip().Shadow(0, 14, 44, 0, ui.Color{A: 140})
	var b *ui.Bitmap
	if len(t.Artwork) > 0 {
		b = s.art.get(c, artworkURL(t.Artwork, int(size*s.zoomOf())), s.artPixels(size))
	}
	if b == nil {
		frame.Background(stageInk.Alpha(0.12))
	}
	frame.Children(func() {
		if b != nil {
			ui.Image(c, b).Size(size, size).Fit(ui.Cover)
		}
	})
}

// stageClose is the button that closes the stage, at its top right.
func (s *shell) stageClose(c *ui.Context) {
	b := s.buttonBase(c.Key("stage-close")).Absolute().Top(20).Right(20).Size(40, 40).Radius(20).Label("Close full-screen lyrics").Cursor(ui.CursorPointer).Tooltip("Close")
	if b.Hovered() {
		b.Background(stageInk.Alpha(0.16))
	}
	b.Children(func() { ui.Icon(c, icon("close")).FontSize(18).TextColor(stageInk) })
	if b.Clicked() {
		s.stage.open = false
	}
}

// stageCredit names where the lyrics come from, as lyricsCredit does, in
// the stage's ink: their licence asks for it wherever they show.
func (s *shell) stageCredit(c *ui.Context, ly lyrics.Lyrics) {
	ui.RichText(c).FontSize(s.lk.captionSize()).TextColor(stageDim).Padding(16, 28).Children(func() {
		s.creditParts(c, ly)
	})
}

// lyricsEmpty says why there are no lyrics to show, in the panel's ink, or
// the stage's when o is for the stage.
func (s *shell) lyricsEmpty(c *ui.Context, o lyricsOptions, text string) {
	if o.stage == nil {
		s.emptyIn(c, text)
		return
	}
	ui.Text(c, text).FontSize(s.lk.size(1.25)).TextColor(stageDim).Padding(8, 28)
}

// lyricStyleCommands choose the style of the lyrics from the command
// palette, as the theme's commands choose a theme.
func (s *shell) lyricStyleCommands() []command {
	cmds := make([]command, 0, len(lyricStyles))
	for _, st := range lyricStyles {
		cmds = append(cmds, command{"lyrics.style." + string(st.id), "Lyrics style: " + st.name, "", "lyrics", func() {
			s.set.LyricsStyle = st.id
			s.save()
		}})
	}
	return cmds
}
