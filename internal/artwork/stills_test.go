package artwork

import (
	"bytes"
	"context"
	"crypto/tls"
	"image"
	"image/color"
	"net"
	"net/http"
	"net/http/httptest"
	"path"
	"sync/atomic"
	"testing"
)

func TestStills(t *testing.T) {
	for _, c := range []struct {
		name string
		url  string
		want []string
	}{
		{
			name: "a listed still asks the larger ones first",
			url:  "https://i.ytimg.com/vi/abc/hqdefault.jpg",
			want: []string{
				"https://i.ytimg.com/vi/abc/maxresdefault.jpg",
				"https://i.ytimg.com/vi/abc/hq720.jpg",
				"https://i.ytimg.com/vi/abc/sddefault.jpg",
				"https://i.ytimg.com/vi/abc/hqdefault.jpg",
			},
		},
		{
			name: "the largest still asks nothing more",
			url:  "https://i.ytimg.com/vi/abc/maxresdefault.jpg",
			want: []string{"https://i.ytimg.com/vi/abc/maxresdefault.jpg"},
		},
		{
			name: "a webp still stays webp",
			url:  "https://i.ytimg.com/vi_webp/abc/mqdefault.webp",
			want: []string{
				"https://i.ytimg.com/vi_webp/abc/maxresdefault.webp",
				"https://i.ytimg.com/vi_webp/abc/hq720.webp",
				"https://i.ytimg.com/vi_webp/abc/sddefault.webp",
				"https://i.ytimg.com/vi_webp/abc/hqdefault.webp",
				"https://i.ytimg.com/vi_webp/abc/mqdefault.webp",
			},
		},
		{
			name: "a song's cover can be asked at any size, so it isn't a still",
			url:  "https://lh3.googleusercontent.com/cover=w544-h544",
			want: nil,
		},
		{
			name: "another host's still isn't YouTube's",
			url:  "https://example.com/vi/abc/hqdefault.jpg",
			want: nil,
		},
	} {
		got := Stills(c.url)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d stills, want %d: %v", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: still %d is %s, want %s", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// stillHost serves YouTube's stills, of which only those in have answer,
// at the size each has, and counts what it is asked.
type stillHost struct {
	srv  *httptest.Server
	hits atomic.Int32
	have map[string]bool
}

// stillSizesOf are the pictures the host answers with, by name: the
// placeholder YouTube gives for a still it doesn't keep is 120 by 90.
var stillSizesOf = map[string][2]int{
	"maxresdefault.jpg": {120, 90},
	"hq720.jpg":         {1280, 720},
	"sddefault.jpg":     {640, 480},
	"hqdefault.jpg":     {480, 360},
	"mqdefault.jpg":     {320, 180},
	"default.jpg":       {120, 90},
	"cover.png":         {8, 8},
}

func newStillHost(t *testing.T, have ...string) *stillHost {
	t.Helper()
	h := &stillHost{have: map[string]bool{}}
	for _, name := range have {
		h.have[name] = true
	}
	h.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.hits.Add(1)
		name := path.Base(r.URL.Path)
		if !h.have[name] {
			http.NotFound(w, r)
			return
		}
		wh, ok := stillSizesOf[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(cover(wh[0], wh[1], color.RGBA{0x20, 0x60, 0xe0, 0xff}))
	}))
	t.Cleanup(h.srv.Close)
	return h
}

// stillURL is the address of a still of the video abc on YouTube's host,
// which the store asks by name: its client sends every request to the test
// server.
func (h *stillHost) stillURL(name string) string {
	return "https://i.ytimg.com/vi/abc/" + name + ".jpg"
}

// stillStore is a store that downloads from h, whatever the address's host
// says.
func stillStore(t *testing.T, h *stillHost) *Store {
	t.Helper()
	s, _, _ := testStore(t, t.TempDir(), 1<<20, nil)
	s.HTTP = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, h.srv.Listener.Addr().String())
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	return s
}

// widthOf is the width of an encoded image.
func widthOf(t *testing.T, data []byte) int {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	return cfg.Width
}

// TestStoreLargeTakesTheBiggestStill shows the larger stills asked in
// order, the first that answers kept, and asked once.
func TestStoreLargeTakesTheBiggestStill(t *testing.T) {
	h := newStillHost(t, "sddefault.jpg", "hqdefault.jpg")
	s := stillStore(t, h)
	listed := h.stillURL("hqdefault")
	for range 2 {
		got, err := s.Large(context.Background(), listed)
		if err != nil {
			t.Fatalf("Large: %v", err)
		}
		if w := widthOf(t, got); w != 640 {
			t.Fatalf("got a %d pixel still, want sddefault's 640", w)
		}
	}
	// maxresdefault and hq720 don't answer, sddefault does: three requests,
	// then none, as the still that answered is kept for the video.
	if n := h.hits.Load(); n != 3 {
		t.Errorf("the host was asked %d times, want 3", n)
	}
}

// TestStoreLargeFallsBackToTheListedStill shows a video whose larger
// stills YouTube doesn't keep showing the one the answer listed.
func TestStoreLargeFallsBackToTheListedStill(t *testing.T) {
	h := newStillHost(t, "hqdefault.jpg")
	s := stillStore(t, h)
	got, err := s.Large(context.Background(), h.stillURL("hqdefault"))
	if err != nil {
		t.Fatalf("Large: %v", err)
	}
	if w := widthOf(t, got); w != 480 {
		t.Errorf("got a %d pixel still, want the listed 480", w)
	}
	if n := h.hits.Load(); n != 4 {
		t.Errorf("the host was asked %d times, want 4", n)
	}
}

// TestStoreLargeKeepsOtherCovers shows an address that isn't a still loaded
// as it is, with no still asked for.
func TestStoreLargeKeepsOtherCovers(t *testing.T) {
	h := newStillHost(t, "cover.png")
	s := stillStore(t, h)
	if _, err := s.Large(context.Background(), h.srv.URL+"/cover.png"); err != nil {
		t.Fatalf("Large: %v", err)
	}
	if n := h.hits.Load(); n != 1 {
		t.Errorf("the host was asked %d times, want 1", n)
	}
}

// TestStoreLargeRefusesAPlaceholder shows a still that answers with a
// picture smaller than the one listed left out, as YouTube does for a
// maxresdefault it doesn't keep.
func TestStoreLargeRefusesAPlaceholder(t *testing.T) {
	h := newStillHost(t, "maxresdefault.jpg", "sddefault.jpg", "hqdefault.jpg")
	s := stillStore(t, h)
	got, err := s.Large(context.Background(), h.stillURL("hqdefault"))
	if err != nil {
		t.Fatalf("Large: %v", err)
	}
	if w := widthOf(t, got); w != 640 {
		t.Errorf("got a %d pixel still, want sddefault's 640, not the placeholder", w)
	}
	// maxresdefault answers 200 with a 120 by 90 placeholder and hq720 is
	// a 404, so sddefault is asked, the first real still: three requests.
	if n := h.hits.Load(); n != 3 {
		t.Errorf("the host was asked %d times, want 3", n)
	}
}

// TestSquareKeepsADarkEdge shows a cover whose dark edge is on one side
// only, or of uneven depth, kept as it is: a dark frame is no letterbox.
func TestSquareKeepsADarkEdge(t *testing.T) {
	const w, h = 400, 400
	for _, c := range []struct {
		name string
		bar  func(x, y int) bool
	}{
		{"a dark left edge", func(x, _ int) bool { return x < 40 }},
		{"an uneven letterbox", func(_, y int) bool { return y < 20 || y > h-3 }},
		{"a black cover", func(_, _ int) bool { return true }},
	} {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				c0 := color.NRGBA{R: 200, G: 100, B: 40, A: 255}
				if c.bar(x, y) {
					c0 = color.NRGBA{A: 255}
				}
				img.SetNRGBA(x, y, c0)
			}
		}
		kept := trimBars(img)
		if b := kept.Bounds(); b.Dx() != w || b.Dy() != h {
			t.Errorf("%s: trimmed to %dx%d, want it kept whole", c.name, b.Dx(), b.Dy())
		}
	}
}
