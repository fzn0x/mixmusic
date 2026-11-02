package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/online"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// ytdlpUnreachable is how yt-dlp 2026.10.26 fails when it can't open a
// connection to YouTube, as without a network.
const ytdlpUnreachable = "yt-dlp: w3XzgjlKFjc: Unable to download API page: HTTPSConnection(host='www.youtube.com', port=443): " +
	"Failed to establish a new connection: [WinError 10051] A socket operation was attempted to an unreachable network " +
	"(caused by TransportError(\"HTTPSConnection(host='www.youtube.com', port=443): Failed to establish a new connection\"))"

// netTest is an app whose yt-dlp and network check answer while their
// switches are on, counting yt-dlp's runs; hang has yt-dlp wait for the
// test's end instead.
type netTest struct {
	p                *Player
	dir              string
	ytdlp, net, hang atomic.Bool
	runs             atomic.Int32
}

func newNetTest(t *testing.T) *netTest {
	t.Helper()
	n := &netTest{dir: t.TempDir()}
	ended := make(chan struct{})
	t.Cleanup(func() { close(ended) })
	r := stream.NewResolver(nil, "")
	r.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		n.runs.Add(1)
		switch {
		case n.hang.Load():
			<-ended
			return nil, errors.New(ytdlpUnreachable)
		case !n.ytdlp.Load():
			return nil, errors.New(ytdlpUnreachable)
		}
		return fmt.Appendf(nil, `{"url": "https://example.com/audio?expire=%d", "ext": "webm", "acodec": "opus", "filesize": 1000}`,
			time.Now().Add(6*time.Hour).Unix()), nil
	}
	matches, err := store.Open(filepath.Join(n.dir, "matches.json"), map[string]media.Track{
		"sp:known": {ID: "yt:aaaaaaaaaaa", Source: media.SourceYouTube, Title: "Known"},
	})
	if err != nil {
		t.Fatal(err)
	}
	a := &App{resolver: r, cache: stream.NewCache(filepath.Join(n.dir, "audio"), 1<<30), matcher: match.New(nil, matches), settings: testSettings(t)}
	a.online = online.New(func(context.Context) error {
		if n.net.Load() {
			return nil
		}
		return errors.New("no network")
	})
	n.p = &Player{a: a}
	return n
}

func ytTrack(id string) media.Track {
	return media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: "Track " + id}
}

func TestPrepareStreamOffline(t *testing.T) {
	n := newNetTest(t)
	ctx := context.Background()
	// Found while the network answered.
	n.ytdlp.Store(true)
	n.net.Store(true)
	if _, err := n.p.PrepareStream(ctx, ytTrack("bbbbbbbbbbb")); err != nil {
		t.Fatal(err)
	}
	n.ytdlp.Store(false)
	n.net.Store(false)
	_, err := n.p.PrepareStream(ctx, ytTrack("w3XzgjlKFjc"))
	if !errors.Is(err, player.ErrOffline) || !strings.Contains(err.Error(), "Unable to download API page") {
		t.Errorf("without the network: %v", err)
	}
	// Known offline now, yt-dlp doesn't run.
	runs := n.runs.Load()
	if _, err := n.p.PrepareStream(ctx, ytTrack("ccccccccccc")); !errors.Is(err, player.ErrOffline) || n.runs.Load() != runs {
		t.Errorf("offline: %v, yt-dlp ran %d times more", err, n.runs.Load()-runs)
	}
	// What plays without the network still does: a track found before, and
	// one the offline cache holds.
	if src, err := n.p.PrepareStream(ctx, ytTrack("bbbbbbbbbbb")); err != nil || src.Cached {
		t.Errorf("found before: %+v, %v", src, err)
	}
	if err := os.MkdirAll(filepath.Join(n.dir, "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(n.dir, "audio", "ddddddddddd.webm"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if src, err := n.p.PrepareStream(ctx, ytTrack("ddddddddddd")); err != nil || !src.Cached {
		t.Errorf("cached: %+v, %v", src, err)
	}
	if n.runs.Load() != runs {
		t.Errorf("yt-dlp ran %d times more", n.runs.Load()-runs)
	}
	// A Spotify track whose recording is known, and cached.
	if err := os.WriteFile(filepath.Join(n.dir, "audio", "aaaaaaaaaaa.webm"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	sp := media.Track{ID: "sp:known", Source: media.SourceSpotify, Title: "Known"}
	if src, err := n.p.PrepareStream(ctx, sp); err != nil || !src.Cached || src.Track.ID != "yt:aaaaaaaaaaa" {
		t.Errorf("a known Spotify track: %+v, %v", src, err)
	}
}

func TestPrepareStreamOnline(t *testing.T) {
	n := newNetTest(t)
	n.net.Store(true)
	// The network answers: yt-dlp's error is yt-dlp's own.
	_, err := n.p.PrepareStream(context.Background(), ytTrack("w3XzgjlKFjc"))
	if err == nil || errors.Is(err, player.ErrOffline) {
		t.Errorf("with the network: %v", err)
	}
}

func TestPrepareStreamChecksBesideYTDLP(t *testing.T) {
	check := resolveCheck
	resolveCheck = 20 * time.Millisecond
	t.Cleanup(func() { resolveCheck = check })
	n := newNetTest(t)
	n.hang.Store(true)
	started := time.Now()
	_, err := n.p.PrepareStream(context.Background(), ytTrack("w3XzgjlKFjc"))
	if !errors.Is(err, player.ErrOffline) {
		t.Errorf("yt-dlp still running without the network: %v", err)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("gave up after %s", took)
	}
}
