package native

import (
	"context"
	"errors"
	"image"
	"math/rand/v2"
	"net/url"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"

	"github.com/fzn0x/mixmusic/internal/artwork"
)

const (
	// artBudget bounds the pixels of the artwork kept, in bytes: what shows
	// fits, and some of what showed.
	artBudget = 48 << 20
	// artFirstRetry is how long an image that couldn't load waits at
	// first, doubling with each failure up to artMaxRetry; an address that
	// gives no image waits artMissingRetry.
	artFirstRetry   = 2 * time.Second
	artMaxRetry     = 2 * time.Minute
	artMissingRetry = 30 * time.Minute
)

// artKey names an image of artwork at a size, tinted as Terminal's
// phosphor when phosphor is set.
type artKey struct {
	url      string
	px       int
	phosphor bool
}

type artEntry struct {
	bmp  *ui.Bitmap
	size int
	used uint64
	// host is the address's host.
	host    string
	loading bool
	// fails counts the failures in a row, and retryAt is when the image
	// tries again; temporary is set when the last failure may pass, as
	// the network's does.
	fails     int
	retryAt   time.Time
	temporary bool
}

// artSource downloads the images, and keeps them: the app's
// artwork.Store, which both windows share.
type artSource interface {
	Image(ctx context.Context, rawURL string) (image.Image, error)
}

// artworks fetches the artwork the interface shows, scaled down to the
// sizes it shows it at, and keeps the most recently shown. An image that
// couldn't load tries again after a few seconds, then less often, and at
// once when another of its host loads (#73). Its maps belong to the main
// thread; images download and scale beside it.
type artworks struct {
	// update runs a function on the main thread, then redraws.
	update func(func())
	src    artSource
	m      map[artKey]*artEntry
	bytes  int
	clock  uint64
	// phosphor tints the artwork as the theme asks.
	phosphor bool
	// failing are the hosts whose images failed, until one loads.
	failing map[string]bool
	// now is time.Now, and jitter picks a wait from half of d to d: tests
	// replace them.
	now    func() time.Time
	jitter func(d time.Duration) time.Duration
}

func newArtworks(update func(func()), src artSource) *artworks {
	return &artworks{
		update:  update,
		src:     src,
		m:       map[artKey]*artEntry{},
		failing: map[string]bool{},
		now:     time.Now,
		jitter:  func(d time.Duration) time.Duration { return d/2 + rand.N(d/2+1) },
	}
}

// get returns the image at rawURL, scaled to about px pixels across, or nil
// while it loads, or when it can't. One that couldn't load asks c for a
// frame when it tries again.
func (a *artworks) get(c *ui.Context, rawURL string, px int) *ui.Bitmap {
	if rawURL == "" {
		return nil
	}
	k := artKey{rawURL, bucket(px), a.phosphor}
	a.clock++
	e, ok := a.m[k]
	if !ok {
		e = &artEntry{}
		if u, err := url.Parse(rawURL); err == nil {
			e.host = u.Hostname()
		}
		a.m[k] = e
	}
	e.used = a.clock
	switch now := a.now(); {
	case e.bmp != nil, e.loading:
	case now.Before(e.retryAt):
		if c != nil {
			c.After(e.retryAt.Sub(now))
		}
	default:
		e.loading = true
		go a.fetch(k, e)
	}
	return e.bmp
}

// bucket rounds a size up to one of the few sizes images are kept at.
func bucket(px int) int {
	for _, b := range []int{64, 96, 128, 192, 256, 384, 512, 768} {
		if px <= b {
			return b
		}
	}
	return 1024
}

func (a *artworks) fetch(k artKey, e *artEntry) {
	img, err := a.load(k)
	if err != nil {
		a.update(func() {
			e.loading = false
			e.fails++
			e.retryAt = a.now().Add(a.retryIn(e.fails, err))
			var ae *artwork.Error
			e.temporary = !errors.As(err, &ae) || ae.Temporary
			a.failing[e.host] = true
		})
		return
	}
	bmp := ui.NewBitmap(img)
	w, h := bmp.Size()
	a.update(func() {
		e.loading = false
		if a.m[k] != e {
			return // dropped meanwhile
		}
		e.bmp, e.size, e.fails, e.retryAt = bmp, w*h*4, 0, time.Time{}
		a.bytes += e.size
		a.evict()
		if a.failing[e.host] {
			// The host answers again: the images that failed as the
			// network did try again at once.
			delete(a.failing, e.host)
			for _, o := range a.m {
				if o.host == e.host && o.bmp == nil && !o.loading && o.temporary {
					o.retryAt = time.Time{}
				}
			}
		}
	})
}

// retryIn is how long an image waits to load again after fails failures in
// a row, the last err: 1 to 2 seconds at first, doubling up to a minute or
// two, at least what the host asked for, and half an hour for an address
// that gives no image.
func (a *artworks) retryIn(fails int, err error) time.Duration {
	var ae *artwork.Error
	if errors.As(err, &ae) && !ae.Temporary {
		return artMissingRetry
	}
	d := artMaxRetry
	if fails < 8 {
		d = min(artFirstRetry<<(fails-1), artMaxRetry)
	}
	d = a.jitter(d)
	if ae != nil {
		d = max(d, ae.RetryAfter)
	}
	return d
}

// artColorWait is how long the accent waits to ask again for the colour
// of artwork whose image couldn't load with err: artColorRetry, at least
// what the host asked for, and half an hour, as a cover, for an address
// that gives no image.
func artColorWait(err error) time.Duration {
	var ae *artwork.Error
	switch {
	case !errors.As(err, &ae):
		return artColorRetry
	case !ae.Temporary:
		return artMissingRetry
	}
	return max(artColorRetry, ae.RetryAfter)
}

// load gets an image and scales it down to k's size.
func (a *artworks) load(k artKey) (image.Image, error) {
	src, err := a.src.Image(context.Background(), k.url)
	if err != nil {
		return nil, err
	}
	img := scaleDown(src, k.px)
	if k.phosphor {
		img = phosphorize(img)
	}
	return img, nil
}

// scaleDown scales img so that its longer side is at most px.
func scaleDown(img image.Image, px int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= px && h <= px {
		return img
	}
	if w >= h {
		w, h = px, max(1, h*px/w)
	} else {
		w, h = max(1, w*px/h), px
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// evict drops the images shown least recently, past the budget.
func (a *artworks) evict() {
	for a.bytes > artBudget {
		var oldest artKey
		var found bool
		var min uint64
		for k, e := range a.m {
			if e.bmp != nil && (!found || e.used < min) {
				oldest, min, found = k, e.used, true
			}
		}
		if !found {
			return
		}
		a.bytes -= a.m[oldest].size
		delete(a.m, oldest)
	}
}
