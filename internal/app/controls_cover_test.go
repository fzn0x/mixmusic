package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/artwork"
	"github.com/fzn0x/mixmusic/internal/media"
)

// pngCover is a cover, a PNG of w×w pixels.
func pngCover(w int) []byte {
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, w)))
	return b.Bytes()
}

// testCovers returns covers that load with load and keep their files in
// dir, and a channel told whenever they call changed.
func testCovers(load func(context.Context, string) ([]byte, error), dir string) (*controlsCovers, chan struct{}) {
	changed := make(chan struct{}, 16)
	return &controlsCovers{load: load, dir: dir, changed: func() { changed <- struct{}{} }, now: time.Now}, changed
}

// await waits for changed.
func await(t *testing.T, changed <-chan struct{}) {
	t.Helper()
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("no cover came")
	}
}

// coverServer serves cover, after release is closed when it's set, and
// counts the requests.
type coverServer struct {
	*httptest.Server
	gets    atomic.Int32
	release chan struct{}
}

func newCoverServer(t *testing.T, cover []byte, release chan struct{}) *coverServer {
	t.Helper()
	s := &coverServer{release: release}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.gets.Add(1)
		if s.release != nil {
			<-s.release
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(cover)
	}))
	t.Cleanup(s.Close)
	return s
}

// coverStore is an artwork store that downloads from srv.
func coverStore(t *testing.T, srv *coverServer) *artwork.Store {
	t.Helper()
	st := artwork.NewStore(t.TempDir(), 1<<20)
	st.HTTP = srv.Client()
	t.Cleanup(st.Close)
	return st
}

func TestControlsCoverFromTheStore(t *testing.T) {
	cover := pngCover(16)
	srv := newCoverServer(t, cover, nil)
	st := coverStore(t, srv)
	url := srv.URL + "/cover.png"
	// A window shows the cover, which the store keeps.
	if _, err := st.Image(context.Background(), url); err != nil {
		t.Fatal(err)
	}
	// Then the network goes: the controls show the cover kept.
	srv.Close()
	cc, changed := testCovers(st.Bytes, "")
	if data, _ := cc.get(url); data != nil {
		t.Error("a cover before the store read it")
	}
	await(t, changed)
	if data, file := cc.get(url); !bytes.Equal(data, cover) || file != "" {
		t.Errorf("the controls got %d bytes, in %q; want the %d of the cover kept, in no file", len(data), file, len(cover))
	}
	if n := srv.gets.Load(); n != 1 {
		t.Errorf("the cover downloaded %d times", n)
	}
}

func TestControlsCoverDownloadsOnce(t *testing.T) {
	cover := pngCover(16)
	release := make(chan struct{})
	srv := newCoverServer(t, cover, release)
	st := coverStore(t, srv)
	url := srv.URL + "/cover.png"
	cc, changed := testCovers(st.Bytes, "")
	// The controls ask first, then a window, while it downloads.
	cc.get(url)
	eventually(t, "the download", func() bool { return srv.gets.Load() == 1 })
	page := make(chan []byte, 1)
	go func() {
		data, _ := st.Bytes(context.Background(), url)
		page <- data
	}()
	close(release)
	await(t, changed)
	if data, _ := cc.get(url); !bytes.Equal(data, cover) {
		t.Errorf("the controls got %d bytes, want %d", len(data), len(cover))
	}
	if data := <-page; !bytes.Equal(data, cover) {
		t.Errorf("the window got %d bytes, want %d", len(data), len(cover))
	}
	if n := srv.gets.Load(); n != 1 {
		t.Errorf("the cover downloaded %d times, want once for both", n)
	}
}

// fakeCovers loads the covers of a map, and counts the loads.
type fakeCovers struct {
	mu     sync.Mutex
	covers map[string][]byte
	loads  map[string]int
	// hold, when set, holds the loads of an address until it's closed.
	hold map[string]chan struct{}
}

func (f *fakeCovers) load(ctx context.Context, rawURL string) ([]byte, error) {
	f.mu.Lock()
	f.loads[rawURL]++
	hold := f.hold[rawURL]
	f.mu.Unlock()
	if hold != nil {
		<-hold
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if data, ok := f.covers[rawURL]; ok {
		return data, nil
	}
	return nil, errors.New("offline")
}

func (f *fakeCovers) loaded(rawURL string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loads[rawURL]
}

func newFakeCovers(covers map[string][]byte) *fakeCovers {
	return &fakeCovers{covers: covers, loads: map[string]int{}, hold: map[string]chan struct{}{}}
}

// files returns the names of the files in dir.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestControlsCoverFiles(t *testing.T) {
	a, b := pngCover(8), pngCover(12)
	f := newFakeCovers(map[string][]byte{"https://i/a": a, "https://i/b": b})
	dir := filepath.Join(t.TempDir(), "now-playing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// One of a run before.
	if err := os.WriteFile(filepath.Join(dir, "old.jpg"), []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	cc, changed := testCovers(f.load, dir)
	cc.get("https://i/a")
	await(t, changed)
	data, fileA := cc.get("https://i/a")
	if !bytes.Equal(data, a) || filepath.Dir(fileA) != dir || filepath.Ext(fileA) != ".png" {
		t.Fatalf("the cover's %d bytes in %q", len(data), fileA)
	}
	if got, err := os.ReadFile(fileA); err != nil || !bytes.Equal(got, a) {
		t.Errorf("its file holds %d bytes (%v), want %d", len(got), err, len(a))
	}
	if names := files(t, dir); len(names) != 1 {
		t.Errorf("the folder holds %q, want the cover's file alone", names)
	}

	// The next cover has a file of its own, and the first goes.
	cc.get("https://i/b")
	await(t, changed)
	_, fileB := cc.get("https://i/b")
	if fileB == "" || fileB == fileA {
		t.Fatalf("the next cover's file %q, after %q", fileB, fileA)
	}
	if names := files(t, dir); len(names) != 1 || names[0] != filepath.Base(fileB) {
		t.Errorf("the folder holds %q, want %q", names, filepath.Base(fileB))
	}

	// A cover asked for, then not, leaves the file of the one that shows.
	hold := make(chan struct{})
	f.mu.Lock()
	f.hold["https://i/a"] = hold
	f.mu.Unlock()
	cc.get("https://i/a")
	eventually(t, "the load", func() bool { return f.loaded("https://i/a") == 2 })
	if _, file := cc.get("https://i/b"); file != fileB {
		t.Errorf("back to the cover kept, its file %q, want %q", file, fileB)
	}
	close(hold)
	eventually(t, "the load's end", func() bool {
		cc.mu.Lock()
		defer cc.mu.Unlock()
		return len(cc.loading) == 0
	})
	if names := files(t, dir); len(names) != 1 || names[0] != filepath.Base(fileB) {
		t.Errorf("the folder holds %q, want %q", names, filepath.Base(fileB))
	}
	select {
	case <-changed:
		t.Error("a cover no longer asked for came")
	default:
	}
}

func TestControlsCoverMissing(t *testing.T) {
	f := newFakeCovers(map[string][]byte{})
	c := &clock{t: time.Unix(1000, 0)}
	cc, changed := testCovers(f.load, "")
	cc.now = c.now
	cc.get("https://i/a")
	await(t, changed)
	// Until a while later, the controls go without it.
	if data, _ := cc.get("https://i/a"); data != nil {
		t.Error("a cover that didn't load")
	}
	c.move(coverRetry - time.Second)
	cc.get("https://i/a")
	if n := f.loaded("https://i/a"); n != 1 {
		t.Errorf("loaded %d times before the wait, want once", n)
	}
	f.mu.Lock()
	f.covers["https://i/a"] = pngCover(8)
	f.mu.Unlock()
	c.move(time.Second)
	cc.get("https://i/a")
	await(t, changed)
	if data, _ := cc.get("https://i/a"); data == nil {
		t.Error("no cover once it loads again")
	}
	if n := f.loaded("https://i/a"); n != 2 {
		t.Errorf("loaded %d times, want twice", n)
	}
}

func TestControlsShowTheCover(t *testing.T) {
	cover := pngCover(8)
	f := newFakeCovers(map[string][]byte{"https://i/544": cover})
	cc, changed := testCovers(f.load, "")
	c := &controls{native: true, covers: cc, np: NowPlaying{Track: &media.Track{
		ID:      "yt:1",
		Title:   "Song",
		Artwork: []media.Image{{URL: "https://i/60", Width: 60}, {URL: "https://i/544", Width: 544}},
	}}}
	if info := c.shown(); info.Artwork != "https://i/544" || info.Cover != nil {
		t.Errorf("at first, the cover of %q, %d bytes", info.Artwork, len(info.Cover))
	}
	await(t, changed)
	if info := c.shown(); !bytes.Equal(info.Cover, cover) {
		t.Errorf("once loaded, %d bytes of cover, want %d", len(info.Cover), len(cover))
	}
	c.native = false
	if info := c.shown(); info != nil {
		t.Errorf("without the engine, the controls show %+v", info)
	}
}
