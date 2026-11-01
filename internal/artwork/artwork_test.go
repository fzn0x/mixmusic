package artwork

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

// split returns an image whose left part is a, the rest b.
func split(a, b color.RGBA, share float64) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			if float64(x) < share*100 {
				img.Set(x, y, a)
			} else {
				img.Set(x, y, b)
			}
		}
	}
	return img
}

func TestPickPrefersVividOverGrey(t *testing.T) {
	grey := color.RGBA{0x40, 0x40, 0x40, 0xff}
	red := color.RGBA{0xe0, 0x30, 0x30, 0xff}
	if got := Pick(split(red, grey, 0.2)); got != red {
		t.Errorf("Pick = %v, want the red", got)
	}
}

func TestPickFallsBackToAverage(t *testing.T) {
	black := color.RGBA{0, 0, 0, 0xff}
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	got := Pick(split(black, white, 0.5))
	if got.R < 0x60 || got.R > 0xa0 || got.R != got.G || got.G != got.B {
		t.Errorf("Pick of black and white = %v, want a grey", got)
	}
}

func TestColorFetches(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, split(color.RGBA{0x20, 0x60, 0xe0, 0xff}, color.RGBA{0x10, 0x10, 0x10, 0xff}, 0.5))
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(buf.Bytes())
	}))
	defer srv.Close()
	c := New()
	c.HTTP = srv.Client()
	got, err := c.Color(context.Background(), srv.URL+"/cover.png")
	if err != nil || got != "#2060e0" {
		t.Errorf("Color = %q, %v", got, err)
	}
	if _, err := c.Color(context.Background(), "http://example.com/a.png"); err == nil {
		t.Error("read artwork over plain HTTP")
	}
}

// TestColorReadsTheStore takes the color of the artwork the store keeps,
// downloaded once for the window and the accent alike.
func TestColorReadsTheStore(t *testing.T) {
	h := newHost(t)
	s, _, _ := testStore(t, t.TempDir(), 1<<20, h.srv)
	c := New()
	c.Store = s
	mustBytes(t, s, h.srv.URL+"/a.png")
	got, err := c.Color(context.Background(), h.srv.URL+"/a.png")
	if err != nil || got != "#2060e0" {
		t.Errorf("Color = %q, %v", got, err)
	}
	if n := h.hits.Load(); n != 1 {
		t.Errorf("the host was asked %d times", n)
	}
}
