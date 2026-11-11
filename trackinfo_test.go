package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// fakeTrackInfo answers the dialog of Track info, in place of the app:
// with the answers it holds, in turn, the last one again once they ran
// out, each as it's asked, of the source the dialog says the track plays
// from.
type fakeTrackInfo struct {
	mu      sync.Mutex
	answers []app.TrackInfo
	asked   []*player.Source
}

// withTrackInfo has the dialog of Track info answered with answers, until
// the test ends.
func withTrackInfo(t *testing.T, answers ...app.TrackInfo) *fakeTrackInfo {
	t.Helper()
	f := &fakeTrackInfo{answers: answers}
	was := trackInfoOf
	trackInfoOf = func(_ *shell, _ context.Context, _ media.Track, now *player.Source) (app.TrackInfo, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		info := f.answers[min(len(f.asked), len(f.answers)-1)]
		f.asked = append(f.asked, now)
		info.Playing = now != nil
		return info, nil
	}
	t.Cleanup(func() { trackInfoOf = was })
	return f
}

// asks returns how the dialog said the track plays, at each ask.
func (f *fakeTrackInfo) asks() []*player.Source {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*player.Source{}, f.asked...)
}

// hasAll reports whether tt shows every text of want, which it tells of
// when it doesn't.
func hasAll(t *testing.T, tt interface{ HasText(string) bool }, want ...string) bool {
	t.Helper()
	ok := true
	for _, w := range want {
		if !tt.HasText(w) {
			t.Errorf("no %q", w)
			ok = false
		}
	}
	return ok
}

// TestTrackInfoFromTheMenu shows a track's Track info from its menu: its
// stream, as MixMusic found it, and how loud it plays; Close closes it.
func TestTrackInfoFromTheMenu(t *testing.T) {
	s := newTestShell(t)
	withTrackInfo(t, app.TrackInfo{Plays: app.PlaysEngine, EvenLoudness: true, Measured: true, LKFS: -9.2, Trim: -4.8,
		Stream: &app.TrackStream{Codec: "Opus", Container: "WebM", Bitrate: 139.3, Size: 3_400_000, Duration: 207 * time.Second}})
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Source: media.SourceYouTube, Tracks: keptTracks}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	if err := tt.RightClick("Teardrop"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Track info"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	want := []string{"track info", "Teardrop", "YouTube Music", "Opus in WebM", "139 kbit/s", "3:27", "3.2 MB",
		"YouTube measured it at −9.2 LKFS. Even loudness turns it down by 4.8 dB."}
	until(t, s, tt, "the dialog", func() bool { return tt.HasText("Opus in WebM") })
	if !hasAll(t, tt, want...) {
		t.Fatalf("the dialog shows %q", tt.Texts())
	}
	if tt.HasText(strings.ToLower(showInFolderLabel())) || tt.HasText("audio engine") {
		t.Errorf("a stream, not playing: %q", tt.Texts())
	}
	click(t, s, tt, "close")
	if tt.HasText("Opus in WebM") || s.trackInfo.open {
		t.Errorf("closed, the window shows %q", tt.Texts())
	}
}

// TestTrackInfoOfAFile shows what a file on this computer holds, where,
// its size and tags, and Show in Explorer or Finder, which shows it; of a
// file gone, why.
func TestTrackInfoOfAFile(t *testing.T) {
	s := newTestShell(t)
	path := filepath.Join(t.TempDir(), "Shore", "01 Slow Tide.flac")
	file := &app.TrackFile{Kind: app.FileMusic, Path: path, Size: 23_000_000,
		Format: audio.Format{Container: "FLAC", Codec: "FLAC", SampleRate: 44100, BitDepth: 16, Channels: 2, Duration: 211 * time.Second},
		Tags:   &tags.Tags{Title: "Slow Tide", Artists: []string{"The Shore"}, Album: "Low Water", Track: 1, Year: 2011}}
	withTrackInfo(t, app.TrackInfo{Plays: app.PlaysEngine, EvenLoudness: true, File: file})
	var shown []string
	was := showInFolder
	showInFolder = func(p string) { shown = append(shown, p) }
	t.Cleanup(func() { showInFolder = was })
	tt := tester(t, s)
	track := media.Track{ID: "file:x", Source: media.SourceFile, Title: "Slow Tide", Artists: []media.Ref{{Name: "The Shore"}}, DurationMs: 211000}
	s.showTrackInfo(track)
	until(t, s, tt, "the dialog", func() bool { return tt.HasText("FLAC") })
	if !hasAll(t, tt, "A file on this computer", "44.1 kHz", "16-bit", "Stereo", "3:31", path, "22 MB", "About 872 kbit/s, on average",
		"its tags", "Low Water", "2011", "Even loudness turns tracks down from YouTube's measure of each") {
		t.Fatalf("the dialog shows %q", tt.Texts())
	}
	click(t, s, tt, strings.ToLower(showInFolderLabel()))
	if len(shown) != 1 || shown[0] != path {
		t.Errorf("shown %q", shown)
	}

	// The file is gone.
	gone := *file
	gone.Problem, gone.Format, gone.Tags = "File not found", audio.Format{}, nil
	withTrackInfo(t, app.TrackInfo{Plays: app.PlaysEngine, File: &gone})
	s.showTrackInfo(track)
	until(t, s, tt, "the file gone", func() bool { return tt.HasText(path + " · File not found") })
	if tt.HasText(strings.ToLower(showInFolderLabel())) || tt.HasText("44.1 kHz") {
		t.Errorf("a file gone: %q", tt.Texts())
	}
}

// TestTrackInfoOfSpotify tells of a track Spotify's own player plays what
// is known, and no figures.
func TestTrackInfoOfSpotify(t *testing.T) {
	s := newTestShell(t)
	withTrackInfo(t, app.TrackInfo{Plays: app.PlaysSpotify, EvenLoudness: true})
	tt := tester(t, s)
	s.showTrackInfo(testTracks[1])
	until(t, s, tt, "the dialog", func() bool { return tt.HasText("In Spotify's own player, in MixMusic") })
	if !hasAll(t, tt, "Spotify", "As your Spotify account and Spotify's settings choose: Spotify tells MixMusic nothing of its audio.",
		"3:31", "Spotify plays it at its own loudness.") {
		t.Fatalf("the dialog shows %q", tt.Texts())
	}
	for _, figure := range []string{"kbit/s", "kHz", "Stereo", "LKFS"} {
		if tt.HasText(figure) {
			t.Errorf("a figure of Spotify's audio, %q: %q", figure, tt.Texts())
		}
	}
}

// TestTrackInfoOfTheTrackPlaying tells what the engine plays of the track
// playing, once it opened it: the dialog asks again until it has.
func TestTrackInfoOfTheTrackPlaying(t *testing.T) {
	s := newTestShell(t)
	stream := &app.TrackStream{Codec: "AAC", Container: "MP4", Bitrate: 129.5, Size: 3_300_000, Duration: 211 * time.Second}
	opening := app.TrackInfo{Plays: app.PlaysEngine, EvenLoudness: true, Stream: stream}
	opened := opening
	opened.Engine = &audio.Loaded{URL: "url:" + testTracks[0].ID, Ready: true, EvenLoudness: true,
		Format: audio.Format{Container: "MP4", Codec: "AAC", SampleRate: 44100, Channels: 2, Duration: 211 * time.Second}}
	f := withTrackInfo(t, opening, opened)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Source != nil })
	tt := tester(t, s)
	s.showTrackInfo(testTracks[0])
	until(t, s, tt, "the dialog", func() bool { return tt.HasText("MixMusic's audio engine is opening it…") })
	tt.Advance(1100 * time.Millisecond)
	until(t, s, tt, "what the engine plays", func() bool {
		tt.Advance(100 * time.Millisecond)
		return tt.HasText("In MixMusic's audio engine, streamed from YouTube, converted to 48 kHz")
	})
	if !hasAll(t, tt, "AAC in MP4", "130 kbit/s", "44.1 kHz", "Stereo", "MixMusic has no measure of it yet: it plays at its own level.") {
		t.Errorf("the dialog shows %q", tt.Texts())
	}
	asks := f.asks()
	if len(asks) < 2 || asks[0] == nil || asks[0].URL != "url:"+testTracks[0].ID || asks[0].Mode != player.ModeStream {
		t.Errorf("asked of %+v", asks)
	}
	// Once it has, it doesn't ask again.
	n := len(asks)
	tt.Advance(3 * time.Second)
	s.drain()
	tt.Frame()
	if got := len(f.asks()); got != n {
		t.Errorf("asked %d times, then %d", n, got)
	}
}

// TestTrackFacts words the facts of a few tracks, and their copies' place.
func TestTrackFacts(t *testing.T) {
	values := func(rows []infoRow) map[string]string {
		m := map[string]string{}
		for _, r := range rows {
			m[r.label] = r.value
		}
		return m
	}
	spotify := testTracks[1]
	rec := media.Track{ID: "yt:aaaaaaaaaaa", Title: "Teardrop", Artists: []media.Ref{{Name: "Massive Attack"}}}
	got := values(trackFacts(spotify, app.TrackInfo{Plays: app.PlaysEngine, Recording: &rec, Levelling: true,
		File: &app.TrackFile{Kind: app.FileDownload, Path: "/d/a.webm", Size: 4_000_000,
			Format: audio.Format{Container: "WebM", Codec: "Opus", SampleRate: 48000, Channels: 2, Duration: 200 * time.Second}}}))
	for label, want := range map[string]string{
		"Source":   "Spotify, downloaded",
		"Plays as": "Teardrop · Massive Attack, on YouTube Music",
		"Format":   "Opus in WebM",
		"Bitrate":  "About 160 kbit/s, on average",
		"Download": "/d/a.webm",
		"Loudness": "MixMusic has no measure of it yet: it plays at its own level. Volume levelling squeezes its loud and quiet moments closer together.",
	} {
		if got[label] != want {
			t.Errorf("%s: %q, want %q", label, got[label], want)
		}
	}
	// Not found yet.
	if got := values(trackFacts(spotify, app.TrackInfo{Plays: app.PlaysEngine})); got["Format"] != "MixMusic finds its recording on YouTube Music as it plays, and tells its format then." {
		t.Errorf("a Spotify track not found: %q", got)
	}
	if got := values(trackFacts(testTracks[0], app.TrackInfo{Plays: app.PlaysEngine})); got["Format"] != "MixMusic finds its stream as it plays, and tells its format then." {
		t.Errorf("a track not played: %q", got)
	}
	// 5.1 surround, from the cache, and a quiet track.
	got = values(trackFacts(testTracks[0], app.TrackInfo{Plays: app.PlaysEngine, Playing: true, Surround: true, EvenLoudness: true, Measured: true, LKFS: -16,
		File:   &app.TrackFile{Kind: app.FileCache, Path: "/c/a.5.1.m4a"},
		Engine: &audio.Loaded{Ready: true, Format: audio.Format{Container: "MP4", Codec: "E-AC-3", SampleRate: 48000, Channels: 6}}}))
	for label, want := range map[string]string{
		"Format":       "E-AC-3 in MP4",
		"Channels":     "5.1 surround",
		"Offline copy": "/c/a.5.1.m4a",
		"Playing":      "In MixMusic's audio engine, its 5.1 surround audio, from the offline cache",
		"Loudness":     "YouTube measured it at −16.0 LKFS. Even loudness leaves it as it is: it isn't louder than −14.0 LKFS.",
	} {
		if got[label] != want {
			t.Errorf("%s: %q, want %q", label, got[label], want)
		}
	}
	// A live stream, with Even loudness off.
	got = values(trackFacts(testTracks[0], app.TrackInfo{Plays: app.PlaysEngine, Measured: true, LKFS: -8,
		Stream: &app.TrackStream{Codec: "AAC", Container: "ADTS", Bitrate: 128, Live: true}}))
	if got["Format"] != "AAC in HLS packed audio" || got["Length"] != "Live" || got["Loudness"] != "YouTube measured it at −8.0 LKFS. Even loudness is off: it plays at its own level." {
		t.Errorf("a live stream: %q", got)
	}
	// WAV of floating-point samples, in mono.
	got = values(trackFacts(media.Track{Source: media.SourceFile}, app.TrackInfo{Plays: app.PlaysEngine,
		File: &app.TrackFile{Kind: app.FileMusic, Format: audio.Format{Container: "WAV", Codec: "PCM", SampleRate: 96000, BitDepth: 32, Float: true, Channels: 1}}}))
	if got["Format"] != "PCM in WAV" || got["Bit depth"] != "32-bit floating point" || got["Channels"] != "Mono" || got["Sample rate"] != "96 kHz" {
		t.Errorf("a WAV file: %q", got)
	}
}

// TestTrackInfoTellsHowAStreamWasFound shows the route that found a
// stream, the arguments of the run of yt-dlp, and whether it was asked
// for as the account; Copy stream URL copies the address the engine plays,
// and says it stops working.
func TestTrackInfoTellsHowAStreamWasFound(t *testing.T) {
	const address = "https://rr1.example.invalid/audio?expire=4102444800&ip=203.0.113.7"
	for _, c := range []struct {
		name    string
		stream  app.TrackStream
		want    []string
		unwant  []string
		command string
	}{
		{"quick", app.TrackStream{Route: stream.RouteQuick, Command: "yt-dlp -J --cookies (hidden)", URL: address},
			[]string{"With yt-dlp's quick run", "yt-dlp -J --cookies (hidden)", "No: asked for signed out"}, []string{"Yes: asked"}, ""},
		{"player API", app.TrackStream{Route: stream.RoutePlayerAPI, URL: address},
			[]string{"through YouTube's player API, with no yt-dlp", "No: asked for signed out"}, []string{"yt-dlp ran"}, ""},
		{"account", app.TrackStream{Route: stream.RouteAccount, AsAccount: true, Command: "yt-dlp -J", URL: address},
			[]string{"session of your YouTube Music account", "Yes: asked for as your YouTube Music account"}, []string{"No: asked"}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newTestShell(t)
			c.stream.Codec, c.stream.Container = "Opus", "WebM"
			withTrackInfo(t, app.TrackInfo{Plays: app.PlaysEngine, Stream: &c.stream})
			var copied string
			was := copyText
			copyText = func(text string) { copied = text }
			t.Cleanup(func() { copyText = was })
			tt := tester(t, s)
			s.showTrackInfo(testTracks[0])
			until(t, s, tt, "the dialog", func() bool { return tt.HasText("Opus in WebM") })
			if !hasAll(t, tt, c.want...) || !hasAll(t, tt, "stops working after a few hours", "tied to this network") {
				t.Fatalf("the dialog shows %q", tt.Texts())
			}
			for _, w := range c.unwant {
				if tt.HasText(w) {
					t.Errorf("the dialog shows %q: %q", w, tt.Texts())
				}
			}
			for _, text := range tt.Texts() {
				if strings.Contains(text, "203.0.113.7") {
					t.Errorf("the dialog shows the address: %q", text)
				}
			}
			click(t, s, tt, "copy stream URL")
			if copied != address {
				t.Errorf("copied %q, want the stream's address", copied)
			}
			until(t, s, tt, "the note", func() bool { return tt.HasText("Copied the stream's address: it stops working after a few hours") })
		})
	}
}

// A stream that plays from a file, or that MixMusic holds none of, has no
// address to copy and no route to tell.
func TestTrackInfoWithoutAStreamHasNoAddress(t *testing.T) {
	s := newTestShell(t)
	withTrackInfo(t, app.TrackInfo{Plays: app.PlaysEngine,
		File: &app.TrackFile{Kind: app.FileDownload, Path: "/d/a.webm", Format: audio.Format{Container: "WebM", Codec: "Opus"}}})
	tt := tester(t, s)
	s.showTrackInfo(testTracks[0])
	until(t, s, tt, "the dialog", func() bool { return tt.HasText("Opus in WebM") })
	if tt.HasText("copy stream URL") || tt.HasText("Signed in") || tt.HasText("Found") {
		t.Errorf("a download: %q", tt.Texts())
	}
}
