package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
)

// coverPNG is a PNG w by h, red at its sides and blue in its middle.
func coverPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 220, A: 255}
			if x >= (w-h)/2 && x < (w+h)/2 {
				c = color.RGBA{B: 220, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// chooseMixCover opens Change cover… in the menu of the More button.
func chooseMixCover(t *testing.T, s *shell, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Change cover…"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	tt.Frame()
	if !tt.HasText("change cover") {
		t.Fatalf("no dialog shows: %q", tt.Texts())
	}
}

// shownCover is the file of the cover chosen for the mix id, as the
// windows show it, once the mix shows one.
func shownCover(t *testing.T, s *shell, tt *ui.Tester, id string) string {
	t.Helper()
	var url string
	until(t, s, tt, "the mix's own cover", func() bool {
		for _, p := range s.svc.Collection.Playlists() {
			if p.ID == id && len(p.Artwork) == 1 {
				if _, ok := library.CoverName(p.Artwork[0].URL); ok {
					url = p.Artwork[0].URL
					return true
				}
			}
		}
		return false
	})
	path, ok := s.localArt(url)
	if !ok {
		t.Fatalf("the windows find no file of %s", url)
	}
	// The page shows it once it read the mix again, and the windows draw
	// it from its file.
	until(t, s, tt, "the page's cover", func() bool {
		l, _ := s.loads["playlist:"+id].(*loaded[media.Playlist])
		return l != nil && len(l.value.Artwork) == 1 && l.value.Artwork[0].URL == url && s.art.get(nil, url, 64) != nil
	})
	return path
}

// TestMixCoverFromFile changes a mix's cover to an image file the system's
// dialog opens, from the More menu of its page, and back to its songs'
// covers from the menu of its tile in Library › Mixes.
func TestMixCoverFromFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(file, coverPNG(t, 800, 600), 0o644); err != nil {
		t.Fatal(err)
	}
	chose := make(chan mygo.OpenDialogOptions, 1)
	dialog := openFileDialog
	openFileDialog = func(opts mygo.OpenDialogOptions) ([]string, error) {
		chose <- opts
		return []string{file}, nil
	}
	t.Cleanup(func() { openFileDialog = dialog })
	s := newTestShell(t)
	mix, err := s.svc.Collection.CreatePlaylist("Night drive", "", testTracks[:2])
	if err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	show(s, tt, playlistPath(mix.ID))
	until(t, s, tt, "the mix's page", func() bool { return tt.HasText("your mix") })
	chooseMixCover(t, s, tt)
	if tt.HasText("use the songs' covers") {
		t.Error("a mix showing its songs' covers offers them")
	}
	click(t, s, tt, "choose an image…")
	select {
	case opts := <-chose:
		if len(opts.Filters) != 1 || !slices.Equal(opts.Filters[0].Extensions, []string{"png", "jpg", "jpeg", "webp"}) {
			t.Errorf("the dialog asks for %+v", opts.Filters)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no file dialog opened")
	}
	until(t, s, tt, "the toast", func() bool { return tt.HasText("Changed the cover of Night drive") })
	path := shownCover(t, s, tt, mix.ID)
	if dir := filepath.Dir(path); filepath.Base(dir) != "mix-covers" || filepath.Base(filepath.Dir(dir)) != "data" {
		t.Errorf("the cover is kept in %s", path)
	}

	// The sidebar names the mix too: the tile's line under it is its own.
	show(s, tt, "/library/mixes")
	if err := tt.RightClick("You · 2 tracks"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Change cover…"); err != nil {
		t.Fatalf("%v: the tile's menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	click(t, s, tt, "use the songs' covers")
	if !tt.HasText("Night drive shows its songs' covers again") {
		t.Errorf("no toast tells it: %q", tt.Texts())
	}
	if got := s.svc.Collection.Playlists()[0].Artwork; len(got) != 0 {
		t.Errorf("back to its songs' covers, the mix shows %v", got)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the cover taken back stays: %v", err)
	}
}

// TestMixCoverOfSong changes a mix's cover to one of its songs' covers,
// each offered once.
func TestMixCoverOfSong(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(coverPNG(t, 64, 64))
	}))
	t.Cleanup(srv.Close)
	tracks := slices.Clone(testTracks[:3])
	tracks[0].Artwork = []media.Image{{URL: srv.URL + "/a.png"}}
	tracks[1].Artwork = []media.Image{{URL: srv.URL + "/a.png"}}
	tracks[2].Artwork = []media.Image{{URL: srv.URL + "/c.png"}}
	s := newTestShell(t)
	mix, err := s.svc.Collection.CreatePlaylist("Night drive", "", tracks)
	if err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	show(s, tt, playlistPath(mix.ID))
	until(t, s, tt, "the mix's page", func() bool { return tt.HasText("your mix") })
	chooseMixCover(t, s, tt)
	for i, offered := range []bool{true, false, true} {
		if _, ok := tt.Access("Use the cover of " + tracks[i].Title); ok != offered {
			t.Errorf("the cover of %s offered: %v", tracks[i].Title, ok)
		}
	}
	click(t, s, tt, "Use the cover of "+tracks[2].Title)
	until(t, s, tt, "the toast", func() bool { return tt.HasText("Changed the cover of Night drive") })
	shownCover(t, s, tt, mix.ID)
}
