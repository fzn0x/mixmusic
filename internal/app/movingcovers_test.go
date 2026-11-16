package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// fakeClips is a source of clips that gives the renditions of its own,
// or err, and counts what it was asked.
type fakeClips struct {
	mu    sync.Mutex
	asked []ClipQuery
	rs    []ClipRendition
	err   error
}

func (f *fakeClips) Clips(_ context.Context, q ClipQuery) ([]ClipRendition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, q)
	return f.rs, f.err
}

func (f *fakeClips) asks() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

// clipApp is an app in a folder of its own whose clips come from src, or
// from none for nil, and a server of clips, which serves each path's own
// bytes.
func clipApp(t *testing.T, src *fakeClips) (*App, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "gone") {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, "clip of "+r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	o := Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"}
	if src != nil {
		o.Clips = src
	}
	a, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a, srv
}

// clipTrack is a track of an album.
func clipTrack() media.Track {
	return media.Track{ID: "yt:clip0000001", Title: "Kites", Artists: []media.Ref{{Name: "Ada  Field"}}, Album: &media.Ref{Name: "Paper"},
		DurationMs: 211000}
}

// TestPickRendition takes H.264, the narrowest rendition as wide as the
// sleeve of at most 3 Mbps, else the widest of those, HEVC when there's
// no H.264, and none tried before.
func TestPickRendition(t *testing.T) {
	rs := []ClipRendition{
		{URL: "avc-2160", Codec: "avc1", Width: 2160, Bitrate: 9_000_000},
		{URL: "avc-1080", Codec: "avc1", Width: 1080, Bitrate: 2_800_000},
		{URL: "avc-720", Codec: "avc1", Width: 720, Bitrate: 1_500_000},
		{URL: "avc-480", Codec: "avc1", Width: 480, Bitrate: 800_000},
		{URL: "hevc-720", Codec: "hvc1", Width: 720, Bitrate: 900_000},
	}
	for _, c := range []struct {
		name  string
		rs    []ClipRendition
		px    int
		tried []string
		want  string
	}{
		{"as wide as the sleeve", rs, 600, nil, "avc-720"},
		{"narrower", rs, 400, nil, "avc-480"},
		{"wider than any within the bitrate", rs, 1600, nil, "avc-1080"},
		{"the one tried left out", rs, 600, []string{"avc-720"}, "avc-1080"},
		{"HEVC without H.264", rs[4:], 600, nil, "hevc-720"},
		{"H.264 all tried", rs[:4], 600, []string{"avc-1080", "avc-720", "avc-480"}, ""},
		{"none", nil, 600, nil, ""},
	} {
		got, ok := pickRendition(c.rs, c.px, c.tried)
		if got.URL != c.want || ok != (c.want != "") {
			t.Errorf("%s: %q, want %q", c.name, got.URL, c.want)
		}
	}
}

// TestClipKey names an album in any script, as services write it.
func TestClipKey(t *testing.T) {
	a, b := clipTrack(), clipTrack()
	b.Artists = []media.Ref{{Name: "ada field"}}
	b.Album = &media.Ref{Name: " PAPER "}
	if clipKey(a) != clipKey(b) || clipKey(a) == "" {
		t.Errorf("%q and %q", clipKey(a), clipKey(b))
	}
	jp := media.Track{Title: "花束", Artists: []media.Ref{{Name: "宇多田ヒカル"}}}
	if got := clipKey(jp); got != "宇多田ヒカル|single:花束" {
		t.Errorf("a single in Japanese is %q", got)
	}
	if got := clipKey(media.Track{Title: "Nobody's"}); got != "" {
		t.Errorf("a track of no artist is %q", got)
	}
}

// TestMovingCover fetches the clip of an album once, whole, into the
// store, which the stream scheme serves to the page that loops it, and
// asks its source once of an album that has none.
func TestMovingCover(t *testing.T) {
	src := &fakeClips{}
	a, srv := clipApp(t, src)
	src.rs = []ClipRendition{{URL: srv.URL + "/kites-720.mp4", Mime: "video/mp4", Codec: "avc1", Width: 720, Height: 720, Bitrate: 1_000_000}}
	p := &Player{a}
	if !p.HasClips() {
		t.Fatal("no clips with a source of them")
	}
	clip, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600})
	if err != nil {
		t.Fatal(err)
	}
	if !clip.Loop || clip.MimeType != "video/mp4" || clip.Width != 720 || !strings.HasPrefix(clip.Src, schemeURL(streamScheme)+"moving-cover/") {
		t.Errorf("the clip is %+v", clip)
	}
	if q := src.asked[0]; q.Title != "Kites" || q.Album != "Paper" || q.Artists[0] != "Ada  Field" || q.Length.Seconds() != 211 {
		t.Errorf("the source was asked %+v", q)
	}
	if again, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600}); err != nil || again.Src != clip.Src || src.asks() != 1 {
		t.Errorf("again: %+v, %v, %d asks", again, err, src.asks())
	}
	if u := (&System{a}).MovingCoverUsage(); u.Clips != 1 || u.Bytes != int64(len("clip of /kites-720.mp4")) {
		t.Errorf("the store holds %+v", u)
	}

	// The page plays it from the stream scheme.
	page := new(mygo.Page)
	saved := callerPage
	callerPage = func(context.Context) *mygo.Page { return page }
	t.Cleanup(func() { callerPage = saved })
	a.SetVideoPage(page)
	path := strings.TrimPrefix(clip.Src, strings.TrimSuffix(schemeURL(streamScheme), "/"))
	for _, c := range []struct {
		path string
		want int
		body string
	}{
		{path, http.StatusOK, "clip of /kites-720.mp4"},
		{"/moving-cover/nothing.mp4", http.StatusNotFound, ""},
		{"/moving-cover/../index.json", http.StatusNotFound, ""},
	} {
		rec := httptest.NewRecorder()
		a.videoHandler().ServeHTTP(rec, httptest.NewRequest("GET", "http://stream.localhost"+c.path, nil))
		if rec.Code != c.want || c.body != "" && rec.Body.String() != c.body {
			t.Errorf("%s: %d %q", c.path, rec.Code, rec.Body.String())
		}
	}

	// An album that has none is asked of once.
	none := clipTrack()
	none.Album = &media.Ref{Name: "Glass"}
	src.err = ErrNoClip
	for range 2 {
		if _, err := p.MovingCover(context.Background(), none, CoverClipOptions{Px: 600}); !errors.Is(err, ErrNoClip) {
			t.Errorf("an album of no clip: %v", err)
		}
	}
	if n := src.asks(); n != 2 {
		t.Errorf("the source was asked %d times", n)
	}
	// Find the moving cover again asks again.
	p.ForgetMovingCover(none)
	p.MovingCover(context.Background(), none, CoverClipOptions{Px: 600})
	if n := src.asks(); n != 3 {
		t.Errorf("found again, the source was asked %d times", n)
	}
	if err := (&System{a}).ClearMovingCovers(); err != nil {
		t.Fatal(err)
	}
	if u := (&System{a}).MovingCoverUsage(); u.Clips != 0 {
		t.Errorf("cleared, the store holds %+v", u)
	}
}

// TestMovingCoverFailed drops a kept clip that didn't play, and tries
// another rendition once; with none left, the album keeps its still.
func TestMovingCoverFailed(t *testing.T) {
	src := &fakeClips{}
	a, srv := clipApp(t, src)
	src.rs = []ClipRendition{
		{URL: srv.URL + "/kites-720.mp4", Codec: "avc1", Width: 720, Bitrate: 1_000_000},
		{URL: srv.URL + "/kites-1080.mp4", Codec: "avc1", Width: 1080, Bitrate: 2_000_000},
	}
	p := &Player{a}
	first, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600})
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600, Failed: first.Src})
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := a.clips.Match(clipKey(clipTrack())); m.Source != srv.URL+"/kites-1080.mp4" || second.Src == "" {
		t.Errorf("tried again with %+v", m)
	}
	src.rs = src.rs[1:]
	if _, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600, Failed: second.Src}); !errors.Is(err, ErrNoClip) {
		t.Errorf("with no other rendition: %v", err)
	}
	if _, ok := a.clips.Match(clipKey(clipTrack())); ok {
		t.Error("the clip that failed is kept")
	}
	// A host that doesn't give it keeps nothing.
	src.rs = []ClipRendition{{URL: srv.URL + "/gone.mp4", Codec: "avc1", Width: 720}}
	if _, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600}); err == nil || errors.Is(err, ErrNoClip) {
		t.Errorf("a clip not found: %v", err)
	}
}

// TestMovingCoverSaver plays only the clips kept with data saver on, and
// takes no album for one without a clip then; with no source, there is
// none.
func TestMovingCoverSaver(t *testing.T) {
	src := &fakeClips{}
	a, srv := clipApp(t, src)
	src.rs = []ClipRendition{{URL: srv.URL + "/kites.mp4", Codec: "avc1", Width: 720}}
	p := &Player{a}
	if _, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600}); err != nil {
		t.Fatal(err)
	}
	set := a.settings.Get()
	set.Quality = settings.QualitySaver
	if _, err := (&Preferences{a}).Set(set); err != nil {
		t.Fatal(err)
	}
	if _, err := p.MovingCover(context.Background(), clipTrack(), CoverClipOptions{Px: 600}); err != nil {
		t.Errorf("a kept clip with data saver: %v", err)
	}
	other := clipTrack()
	other.Album = &media.Ref{Name: "Glass"}
	if _, err := p.MovingCover(context.Background(), other, CoverClipOptions{Px: 600}); !errors.Is(err, ErrNoClip) || src.asks() != 1 {
		t.Errorf("another with data saver: %v, %d asks", err, src.asks())
	}
	if _, ok := a.clips.Match(clipKey(other)); ok {
		t.Error("data saver kept a miss")
	}

	none, _ := clipApp(t, nil)
	if (&Player{none}).HasClips() {
		t.Error("clips with no source")
	}
}
