package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/internal/store"
)

// fakeSponsorBlock is SponsorBlock telling the parts of videos: those of
// parts, failing those of errs, every lookup waiting for release, when
// set. It counts the lookups.
type fakeSponsorBlock struct {
	mu      sync.Mutex
	parts   map[string][]sponsorblock.Segment
	errs    map[string]error
	release chan struct{}
	asked   map[string]int
	servers []string
}

func (f *fakeSponsorBlock) lookup(ctx context.Context, server, videoID string) ([]sponsorblock.Segment, error) {
	f.mu.Lock()
	f.asked[videoID]++
	f.servers = append(f.servers, server)
	release := f.release
	f.mu.Unlock()
	if release != nil {
		<-release
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.parts[videoID], f.errs[videoID]
}

func (f *fakeSponsorBlock) count(videoID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[videoID]
}

// intro is the parts of a video with an intro.
var intro = []sponsorblock.Segment{{Kind: sponsorblock.Intro, From: 0, To: 12 * time.Second, Of: 200 * time.Second}}

func openParts(t *testing.T, path string, f *fakeSponsorBlock, c *clock) *partCache {
	t.Helper()
	file, err := store.Open(path, map[string]keptParts{})
	if err != nil {
		t.Fatal(err)
	}
	cache := newPartCache(f.lookup, file, c.now)
	t.Cleanup(cache.save)
	return cache
}

func newFakeSponsorBlock() *fakeSponsorBlock {
	return &fakeSponsorBlock{parts: map[string][]sponsorblock.Segment{"withAnIntro": intro}, errs: map[string]error{}, asked: map[string]int{}}
}

// TestPartCacheAsksOnce asks SponsorBlock once of a video, however many
// ask at once, and not again while it knows the answer.
func TestPartCacheAsksOnce(t *testing.T) {
	f := newFakeSponsorBlock()
	f.release = make(chan struct{})
	c := openParts(t, filepath.Join(t.TempDir(), "sponsorblock.json"), f, &clock{t: time.Unix(1_800_000_000, 0)})
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if got, err := c.Get(context.Background(), "", "withAnIntro"); err != nil || !slices.Equal(got, intro) {
				t.Errorf("Get = %v, %v", got, err)
			}
		})
	}
	time.Sleep(10 * time.Millisecond)
	close(f.release)
	wg.Wait()
	if got, ok := c.Known("withAnIntro", false); !ok || !slices.Equal(got, intro) {
		t.Errorf("Known = %v, %v", got, ok)
	}
	c.Get(context.Background(), "", "withAnIntro")
	if n := f.count("withAnIntro"); n != 1 {
		t.Errorf("asked %d times", n)
	}
}

// TestPartCacheKeeps keeps the parts found in the file, for the next run,
// and a video found without any in memory, each for a while: the parts
// found stay known offline however old.
func TestPartCacheKeeps(t *testing.T) {
	dir := t.TempDir()
	f := newFakeSponsorBlock()
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	c := openParts(t, filepath.Join(dir, "sponsorblock.json"), f, clk)
	ctx := context.Background()
	c.Get(ctx, "", "withAnIntro")
	c.Get(ctx, "", "noParts")
	if _, ok := c.Known("noParts", false); !ok {
		t.Error("a video without parts isn't known")
	}
	c.save()
	again := openParts(t, filepath.Join(dir, "sponsorblock.json"), f, clk)
	if got, ok := again.Known("withAnIntro", false); !ok || !slices.Equal(got, intro) {
		t.Errorf("after a restart: %v, %v", got, ok)
	}
	if _, ok := again.Known("noParts", false); ok {
		t.Error("a video without parts was kept in the file")
	}
	clk.move(partsFor)
	if _, ok := again.Known("withAnIntro", false); ok {
		t.Error("parts found long ago are taken as they are")
	}
	if got, ok := again.Known("withAnIntro", true); !ok || !slices.Equal(got, intro) {
		t.Errorf("stale, offline: %v, %v", got, ok)
	}
}

// TestPartCacheFails keeps no failed lookup: the next asks again, and a
// video found with parts before keeps them.
func TestPartCacheFails(t *testing.T) {
	f := newFakeSponsorBlock()
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	c := openParts(t, filepath.Join(t.TempDir(), "sponsorblock.json"), f, clk)
	ctx := context.Background()
	f.errs["broken"] = errors.New("HTTP 502")
	if _, err := c.Get(ctx, "", "broken"); err == nil {
		t.Error("no error")
	}
	if _, ok := c.Known("broken", false); ok {
		t.Error("a failed lookup is known")
	}
	c.Get(ctx, "", "broken")
	if n := f.count("broken"); n != 2 {
		t.Errorf("asked %d times", n)
	}
	c.Get(ctx, "", "withAnIntro")
	clk.move(partsFor)
	f.errs["withAnIntro"] = errors.New("HTTP 502")
	if got, err := c.Get(ctx, "", "withAnIntro"); err != nil || !slices.Equal(got, intro) {
		t.Errorf("a failure after it was found: %v, %v", got, err)
	}
}

// TestPartCacheNil knows nothing and asks nothing.
func TestPartCacheNil(t *testing.T) {
	var c *partCache
	if _, ok := c.Known("x", true); ok {
		t.Error("a nil cache knows parts")
	}
	if got, err := c.Get(context.Background(), "", "x"); got != nil || err != nil {
		t.Errorf("Get = %v, %v", got, err)
	}
	c.save()
}

// TestStreamVideo finds the video of a stream or a download, and none of
// a file on this computer or a live stream.
func TestStreamVideo(t *testing.T) {
	for _, c := range []struct {
		src  player.Source
		want string
	}{
		{player.Source{Mode: player.ModeStream, URL: streamURL("dQw4w9WgXcQ")}, "dQw4w9WgXcQ"},
		{player.Source{Mode: player.ModeStream, URL: streamURL("dQw4w9WgXcQ") + "/5.1?retry=1"}, "dQw4w9WgXcQ"},
		{player.Source{Mode: player.ModeStream, URL: schemeURL(streamScheme) + "file/C%3A%2Fsong.mp3"}, ""},
		{player.Source{Mode: player.ModeStream, URL: streamURL("dQw4w9WgXcQ"), Live: true}, ""},
		{player.Source{Mode: player.ModeConnect, URL: streamURL("dQw4w9WgXcQ")}, ""},
	} {
		if got, ok := streamVideo(c.src); ok != (c.want != "") || ok && got != c.want {
			t.Errorf("%s: %q, %v", c.src.URL, got, ok)
		}
	}
}

// TestMarksWhileOn asks SponsorBlock, at the server the settings name,
// only while they skip parts.
func TestMarksWhileOn(t *testing.T) {
	f := newFakeSponsorBlock()
	a := &App{settings: testSettings(t)}
	a.parts = openParts(t, filepath.Join(t.TempDir(), "sponsorblock.json"), f, &clock{t: time.Unix(1_800_000_000, 0)})
	s := playerServices{a}
	src := player.Source{Mode: player.ModeStream, URL: streamURL("withAnIntro"), Track: media.Track{ID: "yt:withAnIntro"}}
	if got, err := s.Marks(context.Background(), src); got != nil || err != nil || f.count("withAnIntro") != 0 {
		t.Errorf("off: %v, %v", got, err)
	}
	a.settings.Update(func(st *settings.Settings) error {
		st.SkipParts.On, st.SkipParts.Server = true, "https://sponsor.example"
		return nil
	})
	if got, err := s.Marks(context.Background(), src); err != nil || !slices.Equal(got, intro) {
		t.Errorf("on: %v, %v", got, err)
	}
	if f.servers[0] != "https://sponsor.example" {
		t.Errorf("asked %v", f.servers)
	}
	if got, ok := s.KnownMarks(src); !ok || !slices.Equal(got, intro) {
		t.Errorf("KnownMarks = %v, %v", got, ok)
	}
	file := player.Source{Mode: player.ModeStream, URL: schemeURL(streamScheme) + "file/C%3A%2Fsong.mp3"}
	if got, ok := s.KnownMarks(file); !ok || got != nil {
		t.Errorf("a file: %v, %v", got, ok)
	}
}

// TestScrobbleWithoutParts scrobbles a track by its length without the
// parts the player passes over.
func TestScrobbleWithoutParts(t *testing.T) {
	track := &media.Track{ID: "yt:a", Title: "Song", Artists: []media.Ref{{Name: "Ann"}}, DurationMs: 180000}
	_, got := lastfmTrack(NowPlaying{Track: track, DurationMs: 180000, SkippedMs: 40000})
	if want := (lastfm.Track{Title: "Song", Artist: "Ann", Duration: 140 * time.Second}); got != want {
		t.Errorf("%+v, want %+v", got, want)
	}
}

// TestPartCacheKeepsDownloadsFirst keeps partsKept videos' parts at most
// in the file: those of downloads, then the latest.
func TestPartCacheKeepsDownloadsFirst(t *testing.T) {
	dir := t.TempDir()
	f := newFakeSponsorBlock()
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	c := openParts(t, filepath.Join(dir, "sponsorblock.json"), f, clk)
	c.keep = func(id string) bool { return id == "downloaded" }
	c.mu.Lock()
	c.found["downloaded"] = keptOf(intro, clk.now().Add(-time.Hour))
	c.found["oldest"] = keptOf(intro, clk.now().Add(-time.Hour))
	for i := range partsKept {
		c.found[fmt.Sprint("v", i)] = keptOf(intro, clk.now())
	}
	c.dirty = true
	c.mu.Unlock()
	c.save()
	again := openParts(t, filepath.Join(dir, "sponsorblock.json"), f, clk)
	if _, ok := again.Known("downloaded", false); !ok {
		t.Error("the parts of a download weren't kept")
	}
	if _, ok := again.Known("oldest", false); ok {
		t.Error("more than partsKept were kept")
	}
}

// TestNoticesCreditSponsorBlock lists SponsorBlock's data among the
// licenses, under CC BY-NC-SA 4.0, as its attribution asks.
func TestNoticesCreditSponsorBlock(t *testing.T) {
	for _, n := range (&System{}).Notices() {
		if strings.HasPrefix(n.Name, "SponsorBlock") {
			if !strings.Contains(n.Text, "CC BY-NC-SA 4.0") || !strings.Contains(n.Text, "https://sponsor.ajay.app/") {
				t.Errorf("the notice doesn't credit SponsorBlock: %s", n.Text)
			}
			return
		}
	}
	t.Error("no notice of SponsorBlock's data")
}
