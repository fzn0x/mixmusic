package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// withEngine has TrackInfo see the engine play what l tells, until the
// test ends.
func withEngine(t *testing.T, l audio.Loaded) {
	t.Helper()
	was := engineLoaded
	engineLoaded = func(*App) (audio.Loaded, bool) { return l, true }
	t.Cleanup(func() { engineLoaded = was })
}

// infoOf is what TrackInfo tells of t, which fails the test when it fails.
func infoOf(t *testing.T, a *App, tr media.Track, now *player.Source) TrackInfo {
	t.Helper()
	info, err := (&Player{a}).TrackInfo(context.Background(), tr, now)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// TestTrackInfoOfAFile tells what a file of the music on this computer
// holds, and where it is, and what's wrong once it's gone.
func TestTrackInfoOfAFile(t *testing.T) {
	a, music := localApp(t)
	scanned(t, a)
	tracks := (&LocalFiles{a}).Tracks()
	info := infoOf(t, a, tracks[0], nil)
	f := info.File
	want := audio.Format{Container: "FLAC", Codec: "FLAC", SampleRate: 44100, BitDepth: 16, Channels: 2, Duration: 200 * time.Millisecond}
	path := filepath.Join(music, "Shore", "01 Slow Tide.flac")
	if info.Plays != PlaysEngine || info.Playing || f == nil || f.Kind != FileMusic || f.Path != path || f.Format != want || f.Problem != "" {
		t.Fatalf("info %+v, file %+v", info, f)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Size != fi.Size() || f.Bitrate() < 100 || f.Tags == nil || info.Measured || info.Engine != nil {
		t.Errorf("file %+v, %v kbit/s, %+v", f, f.Bitrate(), info)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if info := infoOf(t, a, tracks[0], nil); info.File == nil || info.File.Problem != "File not found" {
		t.Errorf("a file gone: %+v", info.File)
	}
}

// TestTrackInfoOfATrackPlaying tells what the engine plays of the track
// that plays, and how loud: only when it's what the engine plays.
func TestTrackInfoOfATrackPlaying(t *testing.T) {
	a := newQuietApp(t, nil)
	tr := ytTrack("aaaaaaaaaaa")
	now := &player.Source{Mode: player.ModeStream, URL: streamURL("aaaaaaaaaaa"), Track: tr}
	format := audio.Format{Container: "WebM", Codec: "Opus", SampleRate: 48000, Channels: 2, Duration: 3 * time.Second}
	withEngine(t, audio.Loaded{URL: now.URL, Format: format, Ready: true, LKFS: -9, Measured: true, EvenLoudness: true, Trim: -5})
	info := infoOf(t, a, tr, now)
	if !info.Playing || info.Engine == nil || info.Engine.Format != format || !info.Measured || info.LKFS != -9 || info.Trim != -5 || info.Surround {
		t.Errorf("playing: %+v, %+v", info, info.Engine)
	}
	if info.Recording != nil || info.File != nil {
		t.Errorf("a track of YouTube Music: %+v", info)
	}
	// The engine plays another track: a crossfade's, or the next one.
	withEngine(t, audio.Loaded{URL: streamURL("bbbbbbbbbbb"), Format: format, Ready: true})
	if info := infoOf(t, a, tr, now); info.Engine != nil {
		t.Errorf("what the engine plays of another track: %+v", info.Engine)
	}
}

// TestTrackInfoOfSurround tells of a track that plays its 5.1 surround
// audio, from the cache, as the settings ask.
func TestTrackInfoOfSurround(t *testing.T) {
	a := newQuietApp(t, nil)
	st := a.settings.Get()
	st.Surround = true
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	video := "ccccccccccc"
	b, err := os.ReadFile(filepath.Join("..", "..", "mp4", "testdata", "surround-ac3.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.cache.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.cache.Dir, stream.SurroundKey(video)+".m4a"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	surround := streamURL(video) + stream.SurroundSuffix
	now := &player.Source{Mode: player.ModeStream, URL: streamURL(video), SurroundURL: surround, Track: ytTrack(video)}
	withEngine(t, audio.Loaded{URL: surround, Ready: true, Format: audio.Format{Container: "MP4", Codec: "AC-3", SampleRate: 48000, Channels: 6}})
	info := infoOf(t, a, ytTrack(video), now)
	if !info.Surround || info.File == nil || info.File.Kind != FileCache || info.File.Format.Codec != "AC-3" || info.File.Format.Channels != 6 {
		t.Errorf("surround: %+v, %+v", info, info.File)
	}
}

// TestTrackInfoOfADownload tells of the download a Spotify track plays
// from: its recording's audio.
func TestTrackInfoOfADownload(t *testing.T) {
	a := newQuietApp(t, nil)
	st := a.settings.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackMatch
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	rec := ytTrack("eeeeeeeeeee")
	kept := media.Track{ID: "sp:kept", Source: media.SourceSpotify, Title: "Kept"}
	withDownloads(t, a, matches{"sp:kept": rec}, kept)
	a.loudness.keep("eeeeeeeeeee", -20)
	info := infoOf(t, a, kept, nil)
	if info.Plays != PlaysEngine || info.Recording == nil || info.Recording.ID != rec.ID || info.File == nil || info.File.Kind != FileDownload {
		t.Fatalf("a download: %+v, %+v", info, info.File)
	}
	if f := info.File.Format; f.Codec != "Opus" || f.Container != "WebM" || info.File.Tags != nil {
		t.Errorf("its file: %+v", info.File)
	}
	// A quiet recording plays at its own level.
	if !info.Measured || info.LKFS != -20 || info.Trim != 0 || !info.EvenLoudness {
		t.Errorf("its loudness: %+v", info)
	}
}

// TestTrackInfoOfAStream tells of the stream MixMusic found of a track,
// which it holds, and nothing of a track it found none of.
func TestTrackInfoOfAStream(t *testing.T) {
	a := newQuietApp(t, nil)
	a.resolver.Direct = nil
	expires := time.Now().Add(6 * time.Hour).Unix()
	a.resolver.Run = func(context.Context, ...string) ([]byte, error) {
		return fmt.Appendf(nil, `{"url": "https://example.com/a?expire=%d", "ext": "m4a", "acodec": "mp4a.40.2", "abr": 129.5,
			"filesize": 2000, "duration": 200}`, expires), nil
	}
	if info := infoOf(t, a, ytTrack("ddddddddddd"), nil); info.Stream != nil || info.File != nil || info.Engine != nil {
		t.Errorf("a track not found yet: %+v", info)
	}
	if _, err := a.resolver.Resolve(context.Background(), "ddddddddddd"); err != nil {
		t.Fatal(err)
	}
	info := infoOf(t, a, ytTrack("ddddddddddd"), nil)
	url := fmt.Sprintf("https://example.com/a?expire=%d", expires)
	want := TrackStream{Codec: "AAC", Container: "MP4", Bitrate: 129.5, Size: 2000, Duration: 200 * time.Second,
		Route: stream.RouteQuick, URL: url}
	got := info.Stream
	if got == nil {
		t.Fatal("no stream found")
	}
	command := got.Command
	got.Command = ""
	if *got != want {
		t.Errorf("a stream found: %+v, want %+v", got, want)
	}
	for _, part := range []string{"yt-dlp -J --no-playlist -f \"" + ytFormatOf(t) + "\"", `--extractor-args "youtube:player_client=visionos;skip=hls,dash"`,
		"-- https://music.youtube.com/watch?v=ddddddddddd"} {
		if !strings.Contains(command, part) {
			t.Errorf("the arguments shown lack %q: %s", part, command)
		}
	}
	if strings.Contains(command, url) {
		t.Errorf("the arguments shown hold the stream's address: %s", command)
	}
}

// ytFormatOf is the format selector yt-dlp asks for the best quality.
func ytFormatOf(t *testing.T) string {
	t.Helper()
	var got string
	r := stream.NewResolver(nil, "")
	r.Run = func(_ context.Context, args ...string) ([]byte, error) {
		got = args[slices.Index(args, "-f")+1]
		return nil, errors.New("stop")
	}
	r.Resolve(context.Background(), "ddddddddddd")
	return got
}

// TestTrackInfoOfAStreamAsTheAccount tells that a stream was asked for as
// the user's YouTube Music account, and hides the file of their session
// in the arguments it shows.
func TestTrackInfoOfAStreamAsTheAccount(t *testing.T) {
	a := newQuietApp(t, nil)
	a.resolver.Direct = nil
	// Without tools the resolver asks as the account, as it does in its tests.
	a.resolver.Tools = nil
	a.resolver.Session = func() []*http.Cookie {
		return []*http.Cookie{{Name: "LOGIN_INFO", Value: "made-up-login"}, {Name: "SAPISID", Value: "made-up-sapisid"}}
	}
	a.resolver.SetAsAccount(true)
	var cookies string
	a.resolver.Run = func(_ context.Context, args ...string) ([]byte, error) {
		cookies = args[slices.Index(args, "--cookies")+1]
		return fmt.Appendf(nil, `{"url": "https://example.com/a?expire=%d", "ext": "webm", "acodec": "opus", "abr": 256, "duration": 200}`,
			time.Now().Add(6*time.Hour).Unix()), nil
	}
	if _, err := a.resolver.Resolve(context.Background(), "ddddddddddd"); err != nil {
		t.Fatal(err)
	}
	st := infoOf(t, a, ytTrack("ddddddddddd"), nil).Stream
	if st == nil || !st.AsAccount || st.Route != stream.RouteAccount || cookies == "" {
		t.Fatalf("a stream as the account: %+v", st)
	}
	if !strings.Contains(st.Command, "--cookies (hidden)") || strings.Contains(st.Command, cookies) || strings.Contains(st.Command, "made-up") {
		t.Errorf("the arguments shown: %s", st.Command)
	}
}

// TestTrackInfoOfSpotify tells of a track Spotify's own player plays,
// which MixMusic hears nothing of, as the settings ask, and as it plays.
func TestTrackInfoOfSpotify(t *testing.T) {
	a := newQuietApp(t, nil)
	st := a.settings.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackConnect
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	tr := media.Track{ID: "sp:teardrop", Source: media.SourceSpotify, Title: "Teardrop"}
	if info := infoOf(t, a, tr, nil); info.Plays != PlaysConnect || info.File != nil || info.Stream != nil {
		t.Errorf("on a device: %+v", info)
	}
	if info := infoOf(t, a, tr, &player.Source{Mode: player.ModeSpotify, Track: tr}); info.Plays != PlaysSpotify || !info.Playing {
		t.Errorf("in MixMusic: %+v", info)
	}
	// Played from YouTube Music, as when no device answers.
	rec := ytTrack("fffffffffff")
	now := &player.Source{Mode: player.ModeStream, URL: streamURL("fffffffffff"), Track: rec}
	if info := infoOf(t, a, tr, now); info.Plays != PlaysEngine || info.Recording == nil || info.Recording.ID != rec.ID {
		t.Errorf("from its recording: %+v", info)
	}
}

// TestStreamNames names codecs and files as people know them.
func TestStreamNames(t *testing.T) {
	for _, c := range []struct{ codec, mime, wantCodec, wantContainer string }{
		{"opus", `audio/webm; codecs="opus"`, "Opus", "WebM"},
		{"mp4a.40.2", "audio/mp4", "AAC", "MP4"},
		{"ac-3", "audio/mp4", "AC-3", "MP4"},
		{"ec-3", "audio/mp4", "E-AC-3", "MP4"},
	} {
		if got, gotC := codecName(c.codec), containerName(c.mime); got != c.wantCodec || gotC != c.wantContainer {
			t.Errorf("%s in %s: %s in %s", c.codec, c.mime, got, gotC)
		}
	}
	if s := trackStream(stream.Stream{Live: true}); s.Codec != "AAC" || s.Container != "ADTS" {
		t.Errorf("a live stream: %+v", s)
	}
}
