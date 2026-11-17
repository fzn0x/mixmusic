package main

// Covers that move (#569). With Show moving covers on, the GIF that is a
// cover plays on the now playing page's sleeve, pausing with the music,
// and in the header of a mix's page: a mix's own (internal/app/mixcovers.go)
// or one of the music on this computer, inside its file or beside it as
// cover.gif. Elsewhere, under reduced motion and until it decodes, its
// still shows, the first frame.
//
// A GIF decodes once, beside the main thread, with MyGo's GIF decoder
// (ui.NewGIFDecoder), which keeps its frames' pixels a byte each and draws
// each frame as it comes due into one picture, which a bitmap shows. The
// frame is drawn in a Draw closure, which asks with After for the frame
// that shows the next: ui.Video would build the page again at every frame.
// What moves is computed in the closure from the painter's time, so a
// window that is hidden or minimized, or a page that doesn't show the
// cover, paints nothing and decodes nothing. A GIF that takes more than
// movingCoverMax decoded stays still, and one that no page showed for
// movingCoverKeep is let go.

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/artwork"
	"github.com/fzn0x/mixmusic/internal/media"
)

const (
	// movingCoverMax bounds what a GIF takes decoded, its frames and the
	// picture they draw into: one larger stays still.
	movingCoverMax = 24 << 20
	// movingCoverFile bounds the GIF read from its file.
	movingCoverFile = 40 << 20
	// movingCoverKeep is how long a GIF decoded stays once no page built
	// it, for a page that shows it again soon.
	movingCoverKeep = 30 * time.Second
	// movingCoverHeld is how many GIFs stay decoded at most: the sleeve's,
	// a mix's, and one to come back to.
	movingCoverHeld = 3
)

// movingCovers are the GIFs of the covers that move, by their covers'
// addresses: those decoded, those that decode, and those that can't move.
// They belong to the main thread.
type movingCovers struct {
	m map[string]*gifCover
	// sweep lets go of the GIFs no page showed, as no frame comes to.
	sweep *time.Timer
	// now is time.Now, which tests replace.
	now func() time.Time
	// clips are the albums' clips (movingclip.go).
	clips movingClips
}

// gifCover is the GIF of a cover, as it plays.
type gifCover struct {
	// loading is set while it decodes, and still once it can't move: no
	// GIF, one of a frame, too large or broken.
	loading, still bool
	dec            ui.VideoDecoder
	bmp            *ui.Bitmap
	// pos is how long the frame shown has shown, of end, how long it
	// shows; drawn is when it last painted while it played.
	pos, end time.Duration
	drawn    time.Time
	// built is when a frame last built it.
	built time.Time
}

// errStill is a cover that doesn't move.
var errStill = errors.New("it doesn't move")

// movingOn tells whether covers move: with the setting on, and while the
// desktop doesn't ask for less motion.
func (s *shell) movingOn(c *ui.Context) bool {
	return s.set.MovingCovers && !c.Preferences().ReduceMotion
}

// gifOf returns the GIF of the cover at url once it decoded, or nil while
// it decodes, or when it can't move. The first ask decodes it, beside the
// main thread.
func (s *shell) gifOf(url string) *gifCover {
	if url == "" {
		return nil
	}
	m := &s.moving
	if m.m == nil {
		m.m = map[string]*gifCover{}
	}
	now := m.clock()
	g, ok := m.m[url]
	if !ok {
		g = &gifCover{loading: true}
		m.m[url] = g
		go s.decodeGIF(url, g)
	}
	g.built = now
	if g.loading || g.still {
		return nil
	}
	return g
}

func (m *movingCovers) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

// decodeGIF decodes the GIF of the cover at url into g, from a goroutine.
func (s *shell) decodeGIF(url string, g *gifCover) {
	dec, first, err := openGIF(s.gifFile(url))
	var bmp *ui.Bitmap
	if err == nil {
		bmp = ui.NewBitmap(first.Image)
	}
	s.update(func() {
		g.loading = false
		if err != nil {
			g.still = true
			if !errors.Is(err, errStill) {
				log.Printf("mixmusic: the cover %s stays still: %v", url, err)
			}
			return
		}
		g.dec, g.bmp, g.end = dec, bmp, first.Duration
	})
}

// gifFile returns the file of the GIF of the cover at url: beside a mix's
// own still, or the cover of the music on this computer, when it's a GIF;
// "" for none.
func (s *shell) gifFile(url string) string {
	if path, ok := s.svc.Collection.MovingMixCover(url); ok {
		return path
	}
	if path, ok := s.localArt(url); ok && strings.EqualFold(filepath.Ext(path), ".gif") {
		return path
	}
	return ""
}

// openGIF decodes the GIF of a file, and its first frame, unless it isn't
// one that moves (errStill), or takes more than movingCoverMax decoded.
func openGIF(path string) (ui.VideoDecoder, ui.VideoFrame, error) {
	if path == "" {
		return nil, ui.VideoFrame{}, errStill
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ui.VideoFrame{}, err
	}
	data, err := io.ReadAll(io.LimitReader(f, movingCoverFile+1))
	f.Close()
	switch {
	case err != nil:
		return nil, ui.VideoFrame{}, err
	case len(data) > movingCoverFile:
		return nil, ui.VideoFrame{}, fmt.Errorf("its file is over %d MB", movingCoverFile>>20)
	case !artwork.IsGIF(data):
		return nil, ui.VideoFrame{}, errStill
	}
	size, err := artwork.SizeOfGIF(data)
	switch {
	case err != nil:
		return nil, ui.VideoFrame{}, err
	case size.Frames < 2:
		return nil, ui.VideoFrame{}, errStill
	case size.Memory() > movingCoverMax:
		return nil, ui.VideoFrame{}, fmt.Errorf("it takes %d MB decoded, over %d MB", size.Memory()>>20, movingCoverMax>>20)
	}
	dec, err := ui.NewGIFDecoder(data)
	if err != nil {
		return nil, ui.VideoFrame{}, err
	}
	first, err := dec.Next()
	if err != nil {
		return nil, ui.VideoFrame{}, err
	}
	return dec, first, nil
}

// paint draws the frame of g due at the painter's time into r, cut to fill
// it, and asks for the frame that shows the next, while it plays. Frames
// keep to the GIF's time, those that came due meanwhile passed over, up to
// gifCatchUp of them; one painted long after it was due, as once the
// window shows again, shows the next frame from its start.
func (g *gifCover) paint(p *ui.Painter, r ui.Rect, playing bool) {
	if playing && !g.still {
		now := p.Now()
		if !g.drawn.IsZero() && now.After(g.drawn) {
			if since := now.Sub(g.drawn); since-(g.end-g.pos) > gifLate {
				g.pos = g.end
			} else {
				g.pos += since
			}
		}
		g.drawn = now
		for n := 0; g.pos+gifSlack >= g.end && !g.still; n++ {
			if n == gifCatchUp {
				g.pos = 0
				break
			}
			g.pos -= g.end
			g.next()
		}
		if !g.still {
			p.After(max(g.end-g.pos, 0))
		}
	} else {
		g.drawn = time.Time{}
	}
	p.Image(g.bmp, r, ui.Cover)
}

const (
	// gifSlack is how early a frame may show: the display's frames come
	// at times of their own.
	gifSlack = time.Millisecond
	// gifLate is how late a frame may be painted and still keep the GIF's
	// time, rather than go on from where it was.
	gifLate = 250 * time.Millisecond
	// gifCatchUp bounds the frames a paint draws to keep the GIF's time,
	// each over the one before.
	gifCatchUp = 4
)

// next shows the next frame, the first again after the last. A frame that
// doesn't decode leaves the one shown, still.
func (g *gifCover) next() {
	f, err := g.dec.Next()
	if err == io.EOF {
		if err = g.dec.Rewind(); err == nil {
			f, err = g.dec.Next()
		}
	}
	if err != nil {
		g.still = true
		return
	}
	g.bmp.Update(f.Image)
	g.end = f.Duration
}

// movingCover is the cover images at size, as cover draws it, which plays
// while playing when it is a GIF that moves: label names it then.
func (s *shell) movingCover(c *ui.Context, key string, images []media.Image, size float32, playing bool, label string) {
	var g *gifCover
	if s.movingOn(c) {
		g = s.gifOf(s.artAt(images, size))
	}
	if g == nil {
		s.cover(c, key, images, size, false)
		return
	}
	ui.Box(c.Key(key+"-moving")).Size(size, size).Radius(s.lk.radiusSm).Shrink(0).Clip().Children(func() {
		s.cover(c, key, images, size, false)
		ui.Box(c.Key(key+"-frames")).Absolute().Left(0).Top(0).Size(size, size).PassThrough().Label(label).
			Draw(func(p *ui.Painter, r ui.Rect) { g.paint(p, r, playing) })
	})
}

// sweepMovingCovers lets go of the GIFs no page built for movingCoverKeep,
// and of the oldest past movingCoverHeld, after each frame, and once no
// frame comes, as while the window is hidden.
func (s *shell) sweepMovingCovers() {
	m := &s.moving
	if len(m.m) == 0 {
		return
	}
	now := m.clock()
	held := 0
	for url, g := range m.m {
		switch {
		case g.loading:
		case now.Sub(g.built) >= movingCoverKeep:
			delete(m.m, url)
		case g.dec != nil:
			held++
		}
	}
	for ; held > movingCoverHeld; held-- {
		oldest := ""
		for url, g := range m.m {
			if g.dec != nil && (oldest == "" || g.built.Before(m.m[oldest].built)) {
				oldest = url
			}
		}
		delete(m.m, oldest)
	}
	if m.sweep == nil && len(m.m) > 0 {
		m.sweep = time.AfterFunc(movingCoverKeep, func() {
			s.update(func() {
				m.sweep = nil
				s.sweepMovingCovers()
			})
		})
	}
}
