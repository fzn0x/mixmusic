package downloads

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tools"
)

var (
	// webmAudio and mp4Audio are short tracks of Opus in WebM, and AAC-LC
	// in MP4, as YouTube serves them.
	webmAudio, mp4Audio []byte
)

func TestMain(m *testing.M) {
	var err error
	if webmAudio, err = os.ReadFile(filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm")); err != nil {
		panic(err)
	}
	if mp4Audio, err = os.ReadFile(filepath.Join("..", "..", "mp4", "testdata", "faststart.m4a")); err != nil {
		panic(err)
	}
	retryWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	startGap, saveAfter, tellEvery, busyWait = 0, 5*time.Millisecond, time.Millisecond, 5*time.Millisecond
	os.Exit(m.Run())
}

// call is a download the fake fetcher was asked for: of video, from.
type call struct {
	video string
	from  int64
}

// fetcher is YouTube, as the downloads fetch from it: it sends audio of
// webmAudio, unless told otherwise.
type fetcher struct {
	mu    sync.Mutex
	audio map[string][]byte
	mime  map[string]string
	lmt   string
	calls []call
	// fail, if set, gives the error of the nth call of a video, before a
	// byte comes.
	fail func(video string, n int) error
	// cut stops a download from before byte cut there, with cutErr; hold
	// holds one from before byte hold there, until it's canceled, telling
	// holding.
	cut     int64
	cutErr  error
	hold    int64
	holding chan string
	// step stops each download step bytes after where it went on from,
	// with cutErr, as a slow, flaky network does.
	step int64
}

func (f *fetcher) Download(ctx context.Context, video string, resume func(s stream.Stream) (int64, error), w io.Writer, progress func(done, size int64)) (stream.Stream, error) {
	f.mu.Lock()
	data, ok := f.audio[video]
	if !ok {
		data = webmAudio
	}
	mime := "audio/webm"
	if m, ok := f.mime[video]; ok {
		mime = m
	}
	n := 0
	for _, c := range f.calls {
		if c.video == video {
			n++
		}
	}
	lmt := f.lmt
	f.mu.Unlock()
	s := stream.Stream{URL: fmt.Sprintf("https://example.com/videoplayback?itag=251&lmt=%s&clen=%d", lmt, len(data)), MimeType: mime, Size: int64(len(data))}
	if f.fail != nil {
		if err := f.fail(video, n); err != nil {
			f.record(video, -1)
			return s, err
		}
	}
	from, err := resume(s)
	if err != nil {
		return s, err
	}
	f.record(video, from)
	end := int64(len(data))
	at := f.cut
	if f.step > 0 && from+f.step < end {
		at = from + f.step
	}
	cut, hold := at > from, f.hold > from
	switch {
	case cut:
		end = at
	case hold:
		end = f.hold
	}
	k, err := w.Write(data[from:end])
	if progress != nil {
		progress(from+int64(k), s.Size)
	}
	switch {
	case err != nil:
		return s, err
	case cut:
		return s, f.cutErr
	case hold:
		if f.holding != nil {
			f.holding <- video
		}
		<-ctx.Done()
		return s, ctx.Err()
	}
	return s, nil
}

func (f *fetcher) record(video string, from int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call{video, from})
}

func (f *fetcher) asked() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// matcher matches Spotify tracks as its map says, counting its searches.
type matcher struct {
	mu      sync.Mutex
	matches map[string]media.Track
	n       int
}

func (m *matcher) Match(_ context.Context, t media.Track) (media.Track, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.n++
	if found, ok := m.matches[t.ID]; ok {
		return found, nil
	}
	return media.Track{}, match.ErrNoMatch
}

func yt(video, title string) media.Track {
	return media.Track{ID: "yt:" + video, Source: media.SourceYouTube, Title: title, DurationMs: 200000,
		Artwork: []media.Image{{URL: "https://img.test/" + video + "-60", Width: 60}, {URL: "https://img.test/" + video + "-800", Width: 800}}}
}

func sp(id, title string) media.Track {
	return media.Track{ID: "sp:" + id, Source: media.SourceSpotify, Title: title, DurationMs: 200000}
}

// open opens the downloads of dir, and closes them as the test ends.
func open(t *testing.T, dir string, o Options) *Manager {
	t.Helper()
	m := Open(dir, o)
	t.Cleanup(m.Close)
	return m
}

// until waits for ok, failing the test after a few seconds.
func until(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen", what)
		}
	}
}

func kind(m *Manager, id string) Kind { return m.State(id).Kind }

func TestAddDownloads(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{}
	m := open(t, dir, Options{Fetcher: f})
	a, b := yt("aaaaaaaaaaa", "One"), yt("bbbbbbbbbbb", "Two")
	if n := m.Add([]media.Track{a, b}, "best"); n != 2 {
		t.Errorf("Add added %d", n)
	}
	if kind(m, a.ID) != Waiting {
		t.Errorf("before the workers start, a track is %v", kind(m, a.ID))
	}
	m.Start()
	until(t, "the downloads", func() bool { return m.Status().Tracks == 2 })
	k, ok := m.Lookup(a.ID)
	if !ok || k.Video != "aaaaaaaaaaa" || k.Path != filepath.Join(dir, "audio", "aaaaaaaaaaa.webm") || k.Track.ID != a.ID {
		t.Fatalf("Lookup = %+v, %v", k, ok)
	}
	if got, err := os.ReadFile(k.Path); err != nil || !bytes.Equal(got, webmAudio) {
		t.Errorf("the file holds %d bytes, %v", len(got), err)
	}
	m.Flush()
	var idx index
	data, _ := os.ReadFile(filepath.Join(dir, "index.json"))
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatal(err)
	}
	e, f2 := idx.Tracks[a.ID], idx.Files["aaaaaaaaaaa"]
	if e == nil || !slices.Equal(e.Pins, []string{"track"}) || e.Quality != "best" || f2 == nil ||
		f2.Path != "audio/aaaaaaaaaaa.webm" || f2.Bytes != int64(len(webmAudio)) || f2.Format != "webm" || f2.Codec != "opus" || f2.Itag != 251 {
		t.Errorf("the index keeps %+v and %+v", e, f2)
	}
	if st := m.Status(); st.Bytes != 2*int64(len(webmAudio)) || st.Waiting != 0 || len(m.Snapshot().Tracks()) != 2 {
		t.Errorf("status %+v", st)
	}
	if got := m.Snapshot().Tracks(); got[0].ID != b.ID {
		t.Errorf("the tracks downloaded last come first: %v", got)
	}
}

// TestAACDownload keeps the AAC of a track YouTube has only in AAC, as it
// plays.
func TestAACDownload(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{audio: map[string][]byte{"ccccccccccc": mp4Audio}, mime: map[string]string{"ccccccccccc": "audio/mp4"}}
	m := open(t, dir, Options{Fetcher: f})
	m.Add([]media.Track{yt("ccccccccccc", "AAC")}, "best")
	m.Start()
	until(t, "the download", func() bool { return kind(m, "yt:ccccccccccc") == Done })
	k, _ := m.Lookup("yt:ccccccccccc")
	if filepath.Base(k.Path) != "ccccccccccc.m4a" {
		t.Errorf("kept at %s", k.Path)
	}
	rec := httptest.NewRecorder()
	if !m.Serve(rec, httptest.NewRequest("GET", "/yt/ccccccccccc", nil), "ccccccccccc") || rec.Header().Get("Content-Type") != "audio/mp4" {
		t.Errorf("served as %q", rec.Header().Get("Content-Type"))
	}
}

func TestSpotifyTrackDownloadsItsMatch(t *testing.T) {
	f := &fetcher{}
	rec := yt("ddddddddddd", "Teardrop")
	mt := &matcher{matches: map[string]media.Track{"sp:tear": rec}}
	m := open(t, t.TempDir(), Options{Fetcher: f, Matcher: mt})
	m.Add([]media.Track{sp("tear", "Teardrop")}, "best")
	m.Start()
	until(t, "the download", func() bool { return kind(m, "sp:tear") == Done })
	k, _ := m.Lookup("sp:tear")
	if k.Video != "ddddddddddd" || k.Track.ID != rec.ID || mt.n != 1 {
		t.Errorf("Lookup = %+v, after %d searches", k, mt.n)
	}
	if got := f.asked(); len(got) != 1 || got[0].video != "ddddddddddd" {
		t.Errorf("downloads %+v", got)
	}
}

func TestNoMatchFails(t *testing.T) {
	f := &fetcher{}
	m := open(t, t.TempDir(), Options{Fetcher: f, Matcher: &matcher{}})
	m.Add([]media.Track{sp("none", "Nothing")}, "best")
	m.Start()
	until(t, "the failure", func() bool { return kind(m, "sp:none") == Failed })
	if why := m.State("sp:none").Why; why != match.ErrNoMatch.Error() {
		t.Errorf("why: %q", why)
	}
	if len(f.asked()) != 0 {
		t.Error("a track without a match downloaded")
	}
	if st := m.Status(); st.Failed != 1 || len(st.FailedJobs) != 1 {
		t.Errorf("status %+v", st)
	}
}

// TestSharedVideo keeps one file for a track and another of its recording,
// until neither is kept.
func TestSharedVideo(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{}
	video := yt("eeeeeeeeeee", "Strobe")
	m := open(t, dir, Options{Fetcher: f, Matcher: &matcher{matches: map[string]media.Track{"sp:strobe": video}}})
	m.Start()
	m.Add([]media.Track{video}, "best")
	until(t, "the first", func() bool { return kind(m, video.ID) == Done })
	m.Add([]media.Track{sp("strobe", "Strobe")}, "best")
	until(t, "the second", func() bool { return kind(m, "sp:strobe") == Done })
	if n := len(f.asked()); n != 1 {
		t.Errorf("%d downloads of one recording", n)
	}
	path := filepath.Join(dir, "audio", "eeeeeeeeeee.webm")
	m.Remove(video.ID)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the file went with the other track still kept: %v", err)
	}
	m.Remove("sp:strobe")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the file stays: %v", err)
	}
}

// TestCollectionPins removes an album's tracks with it, but those
// downloaded on their own too.
func TestCollectionPins(t *testing.T) {
	m := open(t, t.TempDir(), Options{Fetcher: &fetcher{}})
	a, b := yt("fffffffffff", "A"), yt("ggggggggggg", "B")
	m.Add([]media.Track{a}, "best")
	if n := m.AddCollection(Collection{ID: "album:x", Kind: "album", Title: "X"}, []media.Track{a, b}, "best"); n != 1 {
		t.Errorf("the album added %d tracks", n)
	}
	m.Start()
	until(t, "the downloads", func() bool { return m.Status().Tracks == 2 })
	c, ok := m.Snapshot().Collection("album:x")
	if !ok || c.Kept != 2 || c.Title != "X" {
		t.Errorf("the album: %+v, %v", c, ok)
	}
	if n := m.Snapshot().Missing("album:x", []media.Track{a, b, yt("hhhhhhhhhhh", "C")}); n != 1 {
		t.Errorf("%d missing of the album", n)
	}
	m.RemoveCollection("album:x")
	if kind(m, a.ID) != Done || kind(m, b.ID) != None {
		t.Errorf("after the album went: a %v, b %v", kind(m, a.ID), kind(m, b.ID))
	}
	if _, ok := m.Snapshot().Collection("album:x"); ok {
		t.Error("the album stays")
	}
}

// fakeCache is the offline cache, holding the audio of some videos.
type fakeCache map[string]string

func (c fakeCache) Find(video string) (string, string, bool) {
	p, ok := c[video]
	return p, "audio/webm", ok
}

func (c fakeCache) Take(video, dst string) (bool, error) {
	p, ok := c[video]
	if !ok {
		return false, nil
	}
	os.MkdirAll(filepath.Dir(dst), 0o755)
	return true, os.Rename(p, dst)
}

// TestTakesFromCache downloads a track the offline cache holds without the
// network.
func TestTakesFromCache(t *testing.T) {
	cached := filepath.Join(t.TempDir(), "iiiiiiiiiii.webm")
	if err := os.WriteFile(cached, webmAudio, 0o644); err != nil {
		t.Fatal(err)
	}
	f := &fetcher{}
	m := open(t, t.TempDir(), Options{Fetcher: f, Cache: fakeCache{"iiiiiiiiiii": cached}})
	m.Add([]media.Track{yt("iiiiiiiiiii", "Cached")}, "saver")
	m.Start()
	until(t, "the download", func() bool { return kind(m, "yt:iiiiiiiiiii") == Done })
	if len(f.asked()) != 0 {
		t.Error("a track the cache held downloaded")
	}
	if _, err := os.Stat(cached); !errors.Is(err, os.ErrNotExist) {
		t.Error("the cache keeps what it gave")
	}
}

// TestCancelRemovesPart stops a download under way, and drops what came.
func TestCancelRemovesPart(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{hold: 1000, holding: make(chan string, 1)}
	m := open(t, dir, Options{Fetcher: f})
	m.Add([]media.Track{yt("jjjjjjjjjjj", "Long")}, "best")
	m.Start()
	<-f.holding
	part := filepath.Join(dir, "audio", ".jjjjjjjjjjj.part")
	if info, err := os.Stat(part); err != nil || info.Size() != 1000 {
		t.Fatalf("the part: %v, %v", info, err)
	}
	if kind(m, "yt:jjjjjjjjjjj") != Downloading {
		t.Errorf("under way, it is %v", kind(m, "yt:jjjjjjjjjjj"))
	}
	m.Cancel("yt:jjjjjjjjjjj")
	until(t, "the part to go", func() bool {
		_, err := os.Stat(part)
		return errors.Is(err, os.ErrNotExist)
	})
	if kind(m, "yt:jjjjjjjjjjj") != None {
		t.Errorf("canceled, it is %v", kind(m, "yt:jjjjjjjjjjj"))
	}
}

// TestResumeAfterRestart goes on from what a download kept before
// MixMusic quit, when YouTube sends the same file, and starts over when it
// sends another.
func TestResumeAfterRestart(t *testing.T) {
	for _, c := range []struct {
		name string
		lmt  string
		from int64
	}{{"same file", "1", 1000}, {"another file", "2", 0}} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			first := &fetcher{lmt: "1", hold: 1000, holding: make(chan string, 1)}
			m := Open(dir, Options{Fetcher: first})
			m.Add([]media.Track{yt("kkkkkkkkkkk", "Long")}, "best")
			m.Start()
			<-first.holding
			m.Close()

			again := &fetcher{lmt: c.lmt}
			m = open(t, dir, Options{Fetcher: again})
			if kind(m, "yt:kkkkkkkkkkk") != Waiting {
				t.Fatalf("after a restart, it is %v", kind(m, "yt:kkkkkkkkkkk"))
			}
			m.Start()
			until(t, "the download", func() bool { return kind(m, "yt:kkkkkkkkkkk") == Done })
			if got := again.asked(); len(got) != 1 || got[0].from != c.from {
				t.Errorf("downloads %+v, want from %d", got, c.from)
			}
			k, _ := m.Lookup("yt:kkkkkkkkkkk")
			if got, _ := os.ReadFile(k.Path); !bytes.Equal(got, webmAudio) {
				t.Errorf("the file holds %d bytes, not the %d of the audio", len(got), len(webmAudio))
			}
		})
	}
}

// TestRetriesThenFails tries a download that fails again a few times, then
// gives up with why, until asked again.
func TestRetriesThenFails(t *testing.T) {
	f := &fetcher{fail: func(string, int) error { return errors.New("stream: YouTube answered HTTP 500") }}
	m := open(t, t.TempDir(), Options{Fetcher: f})
	m.Add([]media.Track{yt("lllllllllll", "Flaky")}, "best")
	m.Start()
	until(t, "the failure", func() bool { return kind(m, "yt:lllllllllll") == Failed })
	if n := len(f.asked()); n != len(retryWaits)+1 {
		t.Errorf("%d tries", n)
	}
	if why := m.State("yt:lllllllllll").Why; why != "YouTube answered HTTP 500" {
		t.Errorf("why %q", why)
	}
	f.mu.Lock()
	f.fail = nil
	f.mu.Unlock()
	m.RetryFailed()
	until(t, "the download", func() bool { return kind(m, "yt:lllllllllll") == Done })
}

// TestThrottleHoldsQueue holds every download once YouTube turns one down.
func TestThrottleHoldsQueue(t *testing.T) {
	var m *Manager
	throttled := func() bool { return strings.Contains(m.Status().Hold, "YouTube is turning down") }
	f := &fetcher{fail: func(video string, n int) error {
		switch {
		case video == "mmmmmmmmmmm" && n == 0:
			until(t, "the second to start", func() bool { return kind(m, "yt:nnnnnnnnnnn") == Downloading })
			return fmt.Errorf("refused: %w", stream.ErrThrottled)
		case video == "nnnnnnnnnnn":
			// The other worker's download, under way as YouTube turns the
			// first down, ends after.
			until(t, "the hold", throttled)
		}
		return nil
	}}
	m = open(t, t.TempDir(), Options{Fetcher: f})
	m.Add([]media.Track{yt("mmmmmmmmmmm", "First"), yt("nnnnnnnnnnn", "Second"), yt("nnnnnnnnnno", "Third")}, "best")
	m.Start()
	until(t, "the hold", throttled)
	until(t, "the second", func() bool { return kind(m, "yt:nnnnnnnnnnn") == Done })
	time.Sleep(20 * time.Millisecond)
	if got := f.asked(); len(got) != 2 {
		t.Errorf("downloads went on: %+v", got)
	}
	if kind(m, "yt:mmmmmmmmmmm") != Waiting || kind(m, "yt:nnnnnnnnnno") != Waiting {
		t.Errorf("the tracks held are %v and %v", kind(m, "yt:mmmmmmmmmmm"), kind(m, "yt:nnnnnnnnnno"))
	}
	m.RetryFailed()
	until(t, "the rest", func() bool { return m.Status().Tracks == 3 })
}

// network is the network, gone until up closes.
type network struct{ up chan struct{} }

func (n network) Check(context.Context) bool {
	select {
	case <-n.up:
		return true
	default:
		return false
	}
}

func (n network) Wait(ctx context.Context) error {
	select {
	case <-n.up:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TestHoldOffline waits for the network while it's gone, without spending
// the downloads' tries, and goes on once it's back.
func TestHoldOffline(t *testing.T) {
	net := network{up: make(chan struct{})}
	f := &fetcher{fail: func(string, int) error {
		if net.Check(context.Background()) {
			return nil
		}
		return fmt.Errorf("%w: dial tcp: no route", stream.ErrNetwork)
	}}
	m := open(t, t.TempDir(), Options{Fetcher: f, Network: net})
	m.Add([]media.Track{yt("ooooooooooo", "Offline")}, "best")
	m.Start()
	until(t, "the hold", func() bool { return m.Status().Hold == "Waiting for the network" })
	if kind(m, "yt:ooooooooooo") != Waiting {
		t.Errorf("offline, it is %v", kind(m, "yt:ooooooooooo"))
	}
	close(net.up)
	until(t, "the download", func() bool { return kind(m, "yt:ooooooooooo") == Done })
}

func TestDiskFull(t *testing.T) {
	var full error = syscall.ENOSPC
	if runtime.GOOS == "windows" {
		full = syscall.Errno(112)
	}
	f := &fetcher{cut: 100, cutErr: &os.PathError{Op: "write", Path: "x", Err: full}}
	m := open(t, t.TempDir(), Options{Fetcher: f})
	m.Add([]media.Track{yt("ppppppppppp", "Big")}, "best")
	m.Start()
	until(t, "the hold", func() bool { return strings.Contains(m.Status().Hold, "full") })
	if kind(m, "yt:ppppppppppp") != Waiting {
		t.Errorf("on a full disk, it is %v", kind(m, "yt:ppppppppppp"))
	}
}

func TestNoYTDLP(t *testing.T) {
	m := open(t, t.TempDir(), Options{Fetcher: &fetcher{fail: func(string, int) error { return tools.ErrNoYTDLP }}})
	m.Add([]media.Track{yt("qqqqqqqqqqq", "Kids")}, "best")
	m.Start()
	until(t, "the failure", func() bool { return kind(m, "yt:qqqqqqqqqqq") == Failed })
	if why := m.State("yt:qqqqqqqqqqq").Why; !strings.Contains(why, "yt-dlp") {
		t.Errorf("why %q", why)
	}
}

// TestVerifyRejectsGarbage keeps no file that doesn't play.
func TestVerifyRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{audio: map[string][]byte{"rrrrrrrrrrr": bytes.Repeat([]byte("not audio "), 500)}}
	m := open(t, dir, Options{Fetcher: f})
	m.Add([]media.Track{yt("rrrrrrrrrrr", "Garbage")}, "best")
	m.Start()
	until(t, "the failure", func() bool { return kind(m, "yt:rrrrrrrrrrr") == Failed })
	if why := m.State("yt:rrrrrrrrrrr").Why; why != errBadAudio.Error() {
		t.Errorf("why %q", why)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "audio"))
	if len(entries) != 0 {
		t.Errorf("the folder keeps %v", entries)
	}
}

func TestServeRanges(t *testing.T) {
	m := open(t, t.TempDir(), Options{Fetcher: &fetcher{}})
	m.Add([]media.Track{yt("sssssssssss", "Ranges")}, "best")
	m.Start()
	until(t, "the download", func() bool { return kind(m, "yt:sssssssssss") == Done })
	req := httptest.NewRequest("GET", "/yt/sssssssssss", nil)
	req.Header.Set("Range", "bytes=100-199")
	rec := httptest.NewRecorder()
	if !m.Serve(rec, req, "sssssssssss") || rec.Code != http.StatusPartialContent || !bytes.Equal(rec.Body.Bytes(), webmAudio[100:200]) {
		t.Errorf("%d, %d bytes", rec.Code, rec.Body.Len())
	}
	if m.Serve(httptest.NewRecorder(), req, "ttttttttttt") {
		t.Error("served a track it doesn't hold")
	}
}

// TestPauseAndMetered hold the downloads as the user and the connection
// ask, and keep what came.
func TestPauseAndMetered(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{hold: 1000, holding: make(chan string, 2)}
	m := open(t, dir, Options{Fetcher: f})
	m.Add([]media.Track{yt("uuuuuuuuuuu", "Paused")}, "best")
	m.Start()
	<-f.holding
	m.SetPaused(true)
	until(t, "the pause", func() bool { return kind(m, "yt:uuuuuuuuuuu") == Waiting })
	if st := m.Status(); !st.Paused || st.Hold != "Paused" {
		t.Errorf("paused: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(dir, "audio", ".uuuuuuuuuuu.part")); err != nil {
		t.Errorf("the part went: %v", err)
	}
	m.SetMetered(true, false)
	m.SetPaused(false)
	if h := m.Status().Hold; !strings.Contains(h, "metered") {
		t.Errorf("on a metered connection: %q", h)
	}
	f.mu.Lock()
	f.hold = 0
	f.mu.Unlock()
	m.SetMetered(true, true)
	until(t, "the download", func() bool { return kind(m, "yt:uuuuuuuuuuu") == Done })
	if got := f.asked(); got[len(got)-1].from != 1000 {
		t.Errorf("downloads %+v", got)
	}
}

// TestMoveFolder moves the downloads to another folder, renaming their
// files, or copying them where they don't rename, and joins those the
// folder held.
func TestMoveFolder(t *testing.T) {
	for _, copying := range []bool{false, true} {
		t.Run(fmt.Sprintf("copying %v", copying), func(t *testing.T) {
			if copying {
				was := rename
				rename = func(string, string) error { return errors.New("another drive") }
				t.Cleanup(func() { rename = was })
			}
			from, to := t.TempDir(), t.TempDir()
			// The folder holds the downloads of another time.
			other := open(t, to, Options{Fetcher: &fetcher{}})
			other.Add([]media.Track{yt("vvvvvvvvvvv", "Before")}, "best")
			other.Start()
			until(t, "the download there", func() bool { return kind(other, "yt:vvvvvvvvvvv") == Done })
			other.Close()

			m := open(t, from, Options{Fetcher: &fetcher{}})
			m.Add([]media.Track{yt("wwwwwwwwwww", "Moved")}, "best")
			m.AddCollection(Collection{ID: "album:y", Title: "Y"}, []media.Track{yt("wwwwwwwwwww", "Moved")}, "best")
			m.Start()
			until(t, "the download", func() bool { return kind(m, "yt:wwwwwwwwwww") == Done })
			var last int64
			got, err := m.Move(context.Background(), to, false, func(done, total int64) { last = total })
			if err != nil || got != to {
				t.Fatalf("Move = %q, %v", got, err)
			}
			if last != int64(len(webmAudio)) {
				t.Errorf("progress told %d bytes", last)
			}
			for _, id := range []string{"yt:vvvvvvvvvvv", "yt:wwwwwwwwwww"} {
				k, ok := m.Lookup(id)
				if !ok || filepath.Dir(filepath.Dir(k.Path)) != to {
					t.Errorf("%s: %+v, %v", id, k, ok)
				}
				if _, err := os.Stat(k.Path); err != nil {
					t.Error(err)
				}
			}
			if _, ok := m.Snapshot().Collection("album:y"); !ok {
				t.Error("the album didn't move")
			}
			if entries, _ := os.ReadDir(from); len(entries) != 0 {
				t.Errorf("the old folder keeps %v", entries)
			}
			if m.Folder() != to {
				t.Errorf("the folder is %s", m.Folder())
			}
		})
	}
}

func TestMoveRefuses(t *testing.T) {
	from := t.TempDir()
	cache := t.TempDir()
	m := open(t, from, Options{Fetcher: &fetcher{}, Avoid: []string{cache}})
	for _, dir := range []string{from, filepath.Join(from, "audio"), cache, filepath.Join(cache, "x")} {
		if _, err := m.Move(context.Background(), dir, true, nil); err == nil {
			t.Errorf("moved to %s", dir)
		}
	}
	// A folder of other files gets a folder of the downloads in it.
	busy := t.TempDir()
	os.WriteFile(filepath.Join(busy, "notes.txt"), []byte("mine"), 0o644)
	got, err := m.Move(context.Background(), busy, false, nil)
	if err != nil || got != filepath.Join(busy, moveFolder) {
		t.Errorf("Move = %q, %v", got, err)
	}
}

func TestCorruptIndex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.json"), []byte("{not json"), 0o644)
	m := open(t, dir, Options{Fetcher: &fetcher{}})
	if m.Status().Problem != "" {
		t.Errorf("problem %q", m.Status().Problem)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.json.broken")); err != nil {
		t.Error(err)
	}
	m.Add([]media.Track{yt("xxxxxxxxxxx", "After")}, "best")
	m.Start()
	until(t, "the download", func() bool { return kind(m, "yt:xxxxxxxxxxx") == Done })
}

// TestFolderUnreachable tells why the downloads can't be used, as on a
// drive unplugged, and streams instead.
func TestFolderUnreachable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, []byte("x"), 0o644)
	m := open(t, filepath.Join(file, "downloads"), Options{Fetcher: &fetcher{}})
	if st := m.Status(); !strings.Contains(st.Problem, "can't be reached") || st.Hold == "" {
		t.Errorf("status %+v", st)
	}
	if m.Serve(httptest.NewRecorder(), httptest.NewRequest("GET", "/yt/a", nil), "aaaaaaaaaaa") {
		t.Error("served from a folder that isn't there")
	}
	// A folder the user chose, gone, as with its drive, isn't made again.
	gone := filepath.Join(t.TempDir(), "unplugged", "MixMusic downloads")
	m = open(t, gone, Options{Fetcher: &fetcher{}, Exists: true})
	if st := m.Status(); !strings.Contains(st.Problem, "can't be reached") {
		t.Errorf("status %+v", st)
	}
	if _, err := os.Stat(gone); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder was made: %v", err)
	}
}

// TestFileGoneMissing downloads a track again whose file went.
func TestFileGoneMissing(t *testing.T) {
	dir := t.TempDir()
	f := &fetcher{}
	m := Open(dir, Options{Fetcher: f})
	m.Add([]media.Track{yt("yyyyyyyyyyy", "Gone")}, "best")
	m.Start()
	until(t, "the download", func() bool { return kind(m, "yt:yyyyyyyyyyy") == Done })
	m.Close()
	os.Remove(filepath.Join(dir, "audio", "yyyyyyyyyyy.webm"))
	m = open(t, dir, Options{Fetcher: f})
	m.Start()
	until(t, "the download again", func() bool { return len(f.asked()) == 2 && kind(m, "yt:yyyyyyyyyyy") == Done })
}

func TestEstimate(t *testing.T) {
	tracks := []media.Track{{DurationMs: 213000}}
	if got := Estimate(tracks, "best"); got < 3_400_000 || got > 3_800_000 {
		t.Errorf("best: %d", got)
	}
	if got := Estimate(tracks, "saver"); got < 1_200_000 || got > 1_500_000 {
		t.Errorf("saver: %d", got)
	}
	if got := Estimate([]media.Track{{}}, "best"); got != 3_500_000 {
		t.Errorf("of no length: %d", got)
	}
}

// TestArtworkKept keeps the cover of a track downloaded, of 768 pixels or
// more, for every address of its artwork.
func TestArtworkKept(t *testing.T) {
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	jpeg.Encode(&b, img, nil)
	var mu sync.Mutex
	var asked []string
	art := func(_ context.Context, url string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		asked = append(asked, url)
		return b.Bytes(), nil
	}
	dir := t.TempDir()
	m := open(t, dir, Options{Fetcher: &fetcher{}, Art: art})
	tr := yt("zzzzzzzzzzz", "Covered")
	m.Add([]media.Track{tr}, "best")
	m.Start()
	until(t, "the cover", func() bool {
		_, ok := m.ArtworkFile(tr.Artwork[0].URL)
		return ok
	})
	p, _ := m.ArtworkFile(tr.Artwork[0].URL)
	q, _ := m.ArtworkFile(tr.Artwork[1].URL)
	if p != q || filepath.Dir(p) != filepath.Join(dir, "art") || filepath.Ext(p) != ".jpg" {
		t.Errorf("covers at %s and %s", p, q)
	}
	mu.Lock()
	if len(asked) != 1 || asked[0] != tr.Artwork[1].URL {
		t.Errorf("asked for %q", asked)
	}
	mu.Unlock()
	m.Remove(tr.ID)
	if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the cover stays: %v", err)
	}
}

// TestNilManager holds nothing, as the app's downloads before they open.
func TestNilManager(t *testing.T) {
	var m *Manager
	if _, ok := m.Lookup("yt:a"); ok || m.State("yt:a").Kind != None || m.Snapshot().Any() || m.Serve(nil, nil, "a") {
		t.Error("a nil manager holds something")
	}
	m.Add(nil, "best")
	m.Remove("yt:a")
	m.Close()
}

// TestWatchAsksBack tells the watchers beside the manager's lock, so that
// they may ask it what changed.
func TestWatchAsksBack(t *testing.T) {
	m := open(t, t.TempDir(), Options{Fetcher: &fetcher{}})
	told := make(chan string, 100)
	defer m.Watch(func() { told <- m.Folder() })()
	m.Add([]media.Track{yt("abababababa", "Asked")}, "best")
	select {
	case <-told:
	case <-time.After(5 * time.Second):
		t.Fatal("the watcher wasn't told, or waits for the manager")
	}
}

// TestFlakyProgress goes on with a download that keeps breaking off on a
// slow network, as long as each try brings more of it: its tries start
// over each time.
func TestFlakyProgress(t *testing.T) {
	was := retryWaits
	retryWaits = []time.Duration{time.Millisecond}
	t.Cleanup(func() { retryWaits = was })
	f := &fetcher{step: 10000, cutErr: errors.New("stream: YouTube answered HTTP 500")}
	m := open(t, t.TempDir(), Options{Fetcher: f})
	m.Add([]media.Track{yt("acacacacaca", "Slow")}, "best")
	m.Start()
	until(t, "the download", func() bool { return kind(m, "yt:acacacacaca") != Waiting && kind(m, "yt:acacacacaca") != Downloading })
	if k := kind(m, "yt:acacacacaca"); k != Done {
		t.Fatalf("it is %v: %s", k, m.State("yt:acacacacaca").Why)
	}
	if got := f.asked(); len(got) != 4 || got[3].from != 30000 {
		t.Errorf("downloads %+v", got)
	}
}
