package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// webmFetcher downloads the audio of every video as a short WebM file.
type webmFetcher struct{ audio []byte }

func (f webmFetcher) Download(ctx context.Context, video string, resume func(stream.Stream) (int64, error), w io.Writer, progress func(done, size int64)) (stream.Stream, error) {
	s := stream.Stream{URL: "https://example.com/a?itag=251&lmt=1", MimeType: "audio/webm", Size: int64(len(f.audio))}
	from, err := resume(s)
	if err != nil {
		return s, err
	}
	_, err = w.Write(f.audio[from:])
	return s, err
}

// matches matches Spotify tracks as the map says.
type matches map[string]media.Track

func (m matches) Match(_ context.Context, t media.Track) (media.Track, error) {
	if found, ok := m[t.ID]; ok {
		return found, nil
	}
	return media.Track{}, match.ErrNoMatch
}

// withDownloads gives n's app downloads of tracks, done, with the Spotify
// tracks matched as recordings says.
func withDownloads(t *testing.T, a *App, recordings matches, tracks ...media.Track) {
	t.Helper()
	audio, err := os.ReadFile(filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	a.downloads = downloads.Open(filepath.Join(t.TempDir(), "downloads"), downloads.Options{Fetcher: webmFetcher{audio}, Matcher: recordings})
	t.Cleanup(a.downloads.Close)
	a.downloads.Add(tracks, "best")
	a.downloads.Start()
	for deadline := time.Now().Add(5 * time.Second); a.downloads.Status().Tracks < len(tracks); time.Sleep(2 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the downloads didn't finish: %+v", a.downloads.Status())
		}
	}
}

// TestPrepareStreamPlaysDownload plays a track downloaded from its
// download, without yt-dlp, and a Spotify track as its recording,
// without a search.
func TestPrepareStreamPlaysDownload(t *testing.T) {
	n := newNetTest(t)
	n.net.Store(true)
	a := n.p.a
	a.matcher = match.New(nil, nil) // a search would panic
	rec := ytTrack("eeeeeeeeeee")
	withDownloads(t, a, matches{"sp:kept": rec}, ytTrack("fffffffffff"), media.Track{ID: "sp:kept", Source: media.SourceSpotify, Title: "Kept"})
	runs := n.runs.Load()
	src, err := n.p.PrepareStream(context.Background(), ytTrack("fffffffffff"))
	if err != nil || !src.Cached || !strings.HasSuffix(src.URL, "/yt/fffffffffff") || src.Track.ID != "yt:fffffffffff" {
		t.Errorf("a download: %+v, %v", src, err)
	}
	src, err = n.p.PrepareStream(context.Background(), media.Track{ID: "sp:kept", Source: media.SourceSpotify})
	if err != nil || !src.Cached || !strings.HasSuffix(src.URL, "/yt/eeeeeeeeeee") || src.Track.ID != rec.ID {
		t.Errorf("a Spotify download: %+v, %v", src, err)
	}
	if n.runs.Load() != runs {
		t.Errorf("yt-dlp ran %d times", n.runs.Load()-runs)
	}
	// The proxy serves it from the download.
	a.proxy = stream.NewProxy(a.resolver)
	a.proxy.Kept = a.downloads
	s, err := a.proxy.Open(context.Background(), "fffffffffff", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(s, head); err != nil || string(head) != "\x1a\x45\xdf\xa3" {
		t.Errorf("served %q, %v", head, err)
	}
}

// TestPrepareOfflineSpotify plays a Spotify track downloaded from its
// download while the network is gone, whatever the settings say of Spotify
// tracks, and has the others wait for it rather than a Spotify device.
func TestPrepareOfflineSpotify(t *testing.T) {
	n := newNetTest(t)
	a := n.p.a
	s := settings.Default()
	s.SpotifyPlayback = settings.SpotifyPlaybackConnect
	f, err := store.Open(filepath.Join(n.dir, "settings.json"), s)
	if err != nil {
		t.Fatal(err)
	}
	a.settings = f
	rec := ytTrack("ggggggggggg")
	kept := media.Track{ID: "sp:kept", Source: media.SourceSpotify, Title: "Kept"}
	other := media.Track{ID: "sp:other", Source: media.SourceSpotify, Title: "Other"}
	withDownloads(t, a, matches{kept.ID: rec}, kept)
	ctx := context.Background()
	n.net.Store(true)
	if src, err := n.p.Prepare(ctx, kept); err != nil || src.Mode != PlaybackConnect {
		t.Errorf("online, a Spotify track downloaded: %+v, %v", src, err)
	}
	n.net.Store(false)
	a.online.Kick()
	a.online.Check(ctx)
	src, err := n.p.Prepare(ctx, kept)
	if err != nil || src.Mode != PlaybackStream || src.Track.ID != rec.ID {
		t.Errorf("offline, a Spotify track downloaded: %+v, %v", src, err)
	}
	if _, err := n.p.Prepare(ctx, other); !errors.Is(err, player.ErrOffline) {
		t.Errorf("offline, a Spotify track not downloaded: %v", err)
	}
	// A Spotify track whose match isn't known isn't searched for offline.
	a.settings.Update(func(v *settings.Settings) error { v.SpotifyPlayback = settings.SpotifyPlaybackMatch; return nil })
	a.matcher = match.New(nil, nil) // a search would panic
	if _, err := n.p.PrepareStream(ctx, other); !errors.Is(err, player.ErrOffline) {
		t.Errorf("offline, an unknown match: %v", err)
	}
}

// TestPlaysOffline tells the tracks that play without the network: those
// downloaded, and those the offline cache holds.
func TestPlaysOffline(t *testing.T) {
	n := newNetTest(t)
	a := n.p.a
	withDownloads(t, a, nil, ytTrack("hhhhhhhhhhh"))
	if err := os.MkdirAll(filepath.Join(n.dir, "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(n.dir, "audio", "aaaaaaaaaaa.webm"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		track media.Track
		plays bool
	}{
		{ytTrack("hhhhhhhhhhh"), true},
		{ytTrack("aaaaaaaaaaa"), true},
		{media.Track{ID: "sp:known", Source: media.SourceSpotify}, true}, // matched to aaaaaaaaaaa, cached
		{ytTrack("iiiiiiiiiii"), false},
		{media.Track{ID: "sp:unknown", Source: media.SourceSpotify}, false},
	} {
		if got := a.playsOffline(c.track); got != c.plays {
			t.Errorf("%s plays offline: %v", c.track.ID, got)
		}
	}
}

// TestDownloadsMoveToDefault moves the downloads to a folder and back to
// the default one, which the settings keep as "".
func TestDownloadsMoveToDefault(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), DownloadsDir: filepath.Join(dir, "downloads"), Builtin: builtin.FS})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.downloads.Close)
	d := &Downloads{a}
	if d.Folder() != filepath.Join(dir, "downloads") || d.Default() != d.Folder() {
		t.Fatalf("folder %s, default %s", d.Folder(), d.Default())
	}
	mine := filepath.Join(dir, "mine")
	got, err := d.Move(context.Background(), mine, nil)
	if err != nil || got != mine || d.Folder() != mine {
		t.Fatalf("Move = %q, %v; in %s", got, err, d.Folder())
	}
	got, err = d.Move(context.Background(), d.Default(), nil)
	if err != nil || got != "" || d.Folder() != filepath.Join(dir, "downloads") {
		t.Errorf("back: %q, %v; in %s", got, err, d.Folder())
	}
	// The cache's folders can't hold them.
	if _, err := d.Move(context.Background(), filepath.Join(dir, "cache", "audio"), nil); err == nil {
		t.Error("moved into the cache")
	}
}
