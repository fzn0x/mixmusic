package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
)

// coverMix is an app whose artwork store downloads from a server of
// thumbnail(1280, 720)'s, with a mix of a track whose cover is there.
func coverMix(t *testing.T) (*Collection, media.Playlist, *coverServer) {
	t.Helper()
	srv := newCoverServer(t, thumbnail(t, 1280, 720), nil)
	a := noteApp(t, srv)
	c := &Collection{a}
	p, err := c.CreatePlaylist("Late night", "", []media.Track{noteTrack("yt:v1", srv.URL+"/cover.jpg")})
	if err != nil {
		t.Fatal(err)
	}
	return c, p, srv
}

// mixCover returns the file of the cover a mix shows, and its picture.
func mixCover(t *testing.T, c *Collection, id string) (string, image.Image) {
	t.Helper()
	for _, p := range c.Playlists() {
		if p.ID != id {
			continue
		}
		if len(p.Artwork) != 1 {
			t.Fatalf("the mix shows %v", p.Artwork)
		}
		path, ok := c.MixCoverFile(p.Artwork[0].URL)
		if !ok {
			t.Fatalf("the mix shows %v, no cover of its own", p.Artwork)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return path, img
	}
	t.Fatalf("no mix %s", id)
	return "", nil
}

// blue reports whether a colour is the blue of thumbnail's centre.
func blue(c color.Color) bool {
	r, _, b, _ := c.RGBA()
	return b>>8 > 180 && r>>8 < 60
}

// writeImage writes data to a file named name, in a folder of the test's,
// and returns its path.
func writeImage(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestMixCoverFile gives a mix the centre square of an image file, 600
// pixels across, kept in the data folder; a new one takes its place, and
// the songs' covers come back, each leaving no file behind. Deleting the
// mix removes its cover.
func TestMixCoverFile(t *testing.T) {
	c, p, _ := coverMix(t)
	if err := c.SetMixCover(p.ID, writeImage(t, "wide.jpg", thumbnail(t, 1200, 900))); err != nil {
		t.Fatal(err)
	}
	first, img := mixCover(t, c, p.ID)
	if filepath.Dir(first) != filepath.Join(c.a.opts.DataDir, "mix-covers") {
		t.Errorf("the cover is kept in %s", first)
	}
	if b := img.Bounds(); b.Dx() != 600 || b.Dy() != 600 {
		t.Errorf("the cover is %v", b)
	}
	for _, at := range []image.Point{{2, 2}, {597, 300}, {300, 597}} {
		if px := img.At(at.X, at.Y); !blue(px) {
			t.Errorf("at %v the cover is %v, not the image's centre", at, px)
		}
	}
	// A small one stays its size.
	if err := c.SetMixCover(p.ID, writeImage(t, "small.jpg", thumbnail(t, 320, 200))); err != nil {
		t.Fatal(err)
	}
	second, img := mixCover(t, c, p.ID)
	if b := img.Bounds(); second == first || b.Dx() != 200 || b.Dy() != 200 {
		t.Errorf("the new cover is %s, %v", second, b)
	}
	if _, err := os.Stat(first); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the cover replaced stays: %v", err)
	}
	if err := c.UseSongsCovers(p.ID); err != nil {
		t.Fatal(err)
	}
	if got := c.Playlists()[0].Artwork; len(got) != 1 || got[0].URL != p.Artwork[0].URL {
		t.Errorf("back to the songs' covers, the mix shows %v", got)
	}
	if _, err := os.Stat(second); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the cover taken back stays: %v", err)
	}
	if err := c.SetMixCover(p.ID, writeImage(t, "again.jpg", thumbnail(t, 400, 400))); err != nil {
		t.Fatal(err)
	}
	third, _ := mixCover(t, c, p.ID)
	if err := c.DeletePlaylist(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(third); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the cover of a mix deleted stays: %v", err)
	}
}

// tinyWebP is a WebP image of one pixel.
const tinyWebP = "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"

// TestMixCoverWebP makes a cover of a WebP image.
func TestMixCoverWebP(t *testing.T) {
	c, p, _ := coverMix(t)
	data, err := base64.StdEncoding.DecodeString(tinyWebP)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetMixCover(p.ID, writeImage(t, "one.webp", data)); err != nil {
		t.Fatal(err)
	}
	if path, img := mixCover(t, c, p.ID); filepath.Ext(path) != ".jpg" || img.Bounds().Dx() != 1 {
		t.Errorf("the cover is %s, %v", path, img.Bounds())
	}
}

// TestMixCoverRefused makes no cover of what isn't a PNG, a JPEG or a
// WebP, nor for what isn't a mix, and keeps no file of them.
func TestMixCoverRefused(t *testing.T) {
	c, p, _ := coverMix(t)
	var g bytes.Buffer
	if err := gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black}), nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"notes.txt": []byte("not an image"), "moving.gif": g.Bytes()} {
		if err := c.SetMixCover(p.ID, writeImage(t, name, data)); !errors.Is(err, errNotCover) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := c.SetMixCover(p.ID, filepath.Join(t.TempDir(), "gone.png")); err == nil {
		t.Error("a file that isn't there made a cover")
	}
	fine := writeImage(t, "fine.jpg", thumbnail(t, 64, 64))
	for _, id := range []string{"local:nope", "yt:PLx"} {
		if err := c.SetMixCover(id, fine); err == nil {
			t.Errorf("%s took a cover", id)
		}
	}
	if err := c.SetMixCover("local:nope", fine); !errors.Is(err, library.ErrNoPlaylist) {
		t.Errorf("a mix that isn't there: %v", err)
	}
	if entries, _ := os.ReadDir(c.a.mixCovers()); len(entries) != 0 {
		t.Errorf("the covers' folder holds %d files", len(entries))
	}
}

// TestMixCoverOfTrack gives a mix the cover of one of its tracks, which
// the artwork store downloads.
func TestMixCoverOfTrack(t *testing.T) {
	c, p, srv := coverMix(t)
	tr := noteTrack("yt:v1", srv.URL+"/cover.jpg")
	if err := c.SetMixCoverOf(context.Background(), p.ID, tr); err != nil {
		t.Fatal(err)
	}
	if _, img := mixCover(t, c, p.ID); img.Bounds().Dx() != 600 || !blue(img.At(5, 5)) {
		t.Errorf("the cover is %v, %v at its corner", img.Bounds(), img.At(5, 5))
	}
	if err := c.SetMixCoverOf(context.Background(), p.ID, noteTrack("yt:v2", "")); err == nil {
		t.Error("a track without a cover made one")
	}
}
