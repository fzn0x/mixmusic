package native

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // artwork is mostly JPEG
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // and sometimes WebP
)

const (
	// artBudget bounds the pixels of the artwork kept, in bytes: what shows
	// fits, and some of what showed.
	artBudget = 48 << 20
	// artMaxDownload bounds an image downloaded.
	artMaxDownload = 8 << 20
	// artDownloads is how many images download at once.
	artDownloads = 6
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
	// failed is when the image couldn't load, which it tries again a
	// while after.
	failed time.Time
}

// artRetry is how long an image that couldn't load waits to load again.
const artRetry = time.Minute

// artworks fetches the artwork the interface shows, scaled down to the
// sizes it shows it at, and keeps the most recently shown. Its maps belong
// to the main thread; images download and scale beside it.
type artworks struct {
	// update runs a function on the main thread, then redraws.
	update func(func())
	client *http.Client
	slots  chan struct{}
	m      map[artKey]*artEntry
	bytes  int
	clock  uint64
	// phosphor tints the artwork as the theme asks.
	phosphor bool
}

func newArtworks(update func(func())) *artworks {
	return &artworks{
		update: update,
		client: &http.Client{Timeout: 20 * time.Second},
		slots:  make(chan struct{}, artDownloads),
		m:      map[artKey]*artEntry{},
	}
}

// get returns the image at rawURL, scaled to about px pixels across, or nil
// while it loads, or when it can't.
func (a *artworks) get(rawURL string, px int) *ui.Bitmap {
	if rawURL == "" {
		return nil
	}
	k := artKey{rawURL, bucket(px), a.phosphor}
	a.clock++
	if e, ok := a.m[k]; ok && (e.failed.IsZero() || time.Since(e.failed) < artRetry) {
		e.used = a.clock
		return e.bmp
	}
	e := &artEntry{used: a.clock}
	a.m[k] = e
	go a.fetch(k, e)
	return nil
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
	a.slots <- struct{}{}
	img, err := a.load(k)
	<-a.slots
	if err != nil {
		a.update(func() { e.failed = time.Now() })
		return
	}
	bmp := ui.NewBitmap(img)
	w, h := bmp.Size()
	a.update(func() {
		if a.m[k] != e {
			return // dropped meanwhile
		}
		e.bmp, e.size = bmp, w*h*4
		a.bytes += e.size
		a.evict()
	})
}

// load downloads an image and scales it down to k's size.
func (a *artworks) load(k artKey) (image.Image, error) {
	u, err := url.Parse(k.url)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("artwork: not a web address")
	}
	res, err := a.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artwork: HTTP %d", res.StatusCode)
	}
	src, _, err := image.Decode(io.LimitReader(res.Body, artMaxDownload))
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
