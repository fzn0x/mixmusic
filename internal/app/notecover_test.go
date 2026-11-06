package app

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/media"
)

// thumbnail is a music video's 16:9 thumbnail, w by h, as a JPEG: red at
// its sides, blue in its centre square.
func thumbnail(t *testing.T, w, h int) []byte {
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
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// noteApp is an app in a folder of its own, whose artwork store downloads
// from srv. It has no player, whose reports would tell what started.
func noteApp(t *testing.T, srv *coverServer) *App {
	t.Helper()
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	a.artStore.HTTP = srv.Client()
	return a
}

// noteTrack is a track of its own with artwork at url.
func noteTrack(id, url string) media.Track {
	tr := media.Track{ID: id, Title: "Made-up song", Artists: []media.Ref{{Name: "Nobody"}}, Album: &media.Ref{Name: "Made-up album"}}
	if url != "" {
		tr.Artwork = []media.Image{{URL: url, Width: 1280, Height: 720}}
	}
	return tr
}

// notePlays tells a that tr plays, as the player's reports do: started, or
// loading.
func notePlays(a *App, tr media.Track, loading bool) media.Track {
	a.mu.Lock()
	a.nowPlaying = NowPlaying{Track: &tr, Playing: !loading, Loading: loading}
	if !loading {
		a.started = tr.ID
	}
	a.mu.Unlock()
	return tr
}

// shortWait has the notifications wait d at most for a cover.
func shortWait(t *testing.T, d time.Duration) {
	was := noteCoverWait
	noteCoverWait = d
	t.Cleanup(func() { noteCoverWait = was })
}

// TestTrackNoteCover gives the notification of a track that starts its
// title, artists and album, with its cover: the centre square of the one
// the system's media controls show, 512 pixels across at most, so the
// middle of a music video's thumbnail (#102).
func TestTrackNoteCover(t *testing.T) {
	srv := newCoverServer(t, thumbnail(t, 1280, 720), nil)
	a := noteApp(t, srv)
	opts, ok := a.trackNote(notePlays(a, noteTrack("yt:aaaaaaaaaaa", srv.URL+"/maxresdefault.jpg"), false))
	if !ok {
		t.Fatal("the notification of the track that started was dropped")
	}
	if opts.ID != "now-playing" || opts.Title != "Made-up song" || opts.Body != "Nobody • Made-up album" || !opts.Silent {
		t.Errorf("the notification: %+v", opts)
	}
	img, format, err := image.Decode(bytes.NewReader(opts.Image))
	if err != nil {
		t.Fatalf("the notification's cover: %v", err)
	}
	if b := img.Bounds(); format != "jpeg" || b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("the notification's cover is a %s of %v, not a JPEG of 512x512", format, b)
	}
	for _, p := range []image.Point{{40, 40}, {471, 40}, {256, 256}, {40, 471}, {471, 471}} {
		if r, _, b, _ := img.At(p.X, p.Y).RGBA(); r>>8 > 40 || b>>8 < 180 {
			t.Errorf("the notification's cover is %v at %v, not the thumbnail's middle", img.At(p.X, p.Y), p)
		}
	}
}

// TestTrackNoteLocalCover gives the notification of a track on this
// computer the cover the index keeps of it, as the system's media
// controls show it.
func TestTrackNoteLocalCover(t *testing.T) {
	a, music := localApp(t)
	cover := pngCover(16)
	if err := os.WriteFile(filepath.Join(music, "Shore", "cover.png"), cover, 0o644); err != nil {
		t.Fatal(err)
	}
	scanned(t, a)
	tr := notePlays(a, (&LocalFiles{a}).Tracks()[0], false)
	opts, ok := a.trackNote(tr)
	if !ok || !bytes.Equal(opts.Image, cover) {
		t.Errorf("the notification of %s: %d bytes of cover, not the index's %d (shown: %v)", tr.ID, len(opts.Image), len(cover), ok)
	}
}

// TestTrackNoteWithoutCover shows the notification as before, without a
// picture, when the track has no cover, or it doesn't come in time.
func TestTrackNoteWithoutCover(t *testing.T) {
	release := make(chan struct{})
	srv := newCoverServer(t, thumbnail(t, 320, 180), release)
	a := noteApp(t, srv)
	// The download ends before the app closes, which waits for it.
	t.Cleanup(func() { close(release) })
	shortWait(t, 200*time.Millisecond)
	if opts, ok := a.trackNote(notePlays(a, noteTrack("yt:bbbbbbbbbbb", ""), false)); !ok || opts.Image != nil {
		t.Errorf("a track without artwork: %d bytes of cover (shown: %v)", len(opts.Image), ok)
	}
	start := time.Now()
	opts, ok := a.trackNote(notePlays(a, noteTrack("yt:ccccccccccc", srv.URL+"/slow.jpg"), false))
	if !ok || opts.Image != nil {
		t.Errorf("a cover that doesn't come: %d bytes of cover (shown: %v)", len(opts.Image), ok)
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Errorf("the notification waited %v for a cover, not 200ms", waited)
	}
}

// TestTrackNoteDropped drops the notification of a track whose cover
// loads after the next track started, which shows its own, or while it
// loads.
func TestTrackNoteDropped(t *testing.T) {
	for _, loading := range []bool{false, true} {
		release := make(chan struct{})
		srv := newCoverServer(t, thumbnail(t, 320, 180), release)
		a := noteApp(t, srv)
		shown := make(chan bool, 1)
		tr := notePlays(a, noteTrack("yt:ddddddddddd", srv.URL+"/hqdefault.jpg"), false)
		go func() {
			_, ok := a.trackNote(tr)
			shown <- ok
		}()
		for deadline := time.Now().Add(5 * time.Second); srv.gets.Load() == 0 && time.Now().Before(deadline); {
			time.Sleep(5 * time.Millisecond)
		}
		notePlays(a, noteTrack("yt:eeeeeeeeeee", ""), loading)
		close(release)
		if <-shown {
			t.Errorf("the notification of a track showed once the next one played (loading: %v)", loading)
		}
	}
}
