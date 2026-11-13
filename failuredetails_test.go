package main

import (
	"context"
	"iter"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// bothFailed is the error of a stream neither YouTube's player API nor
// yt-dlp found, as the resolver's tells its steps.
type bothFailed struct{}

func (bothFailed) Error() string { return "this track is private" }

func (bothFailed) Steps() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		_ = yield("YouTube's player API", "YouTube's player API answered LOGIN_REQUIRED: Sign in to confirm your age") &&
			yield("yt-dlp", "this track is private")
	}
}

// failingPlayer is the test player, whose tracks' streams aren't found.
type failingPlayer struct{ testPlayer }

func (failingPlayer) Prepare(context.Context, media.Track) (player.Source, error) {
	return player.Source{}, bothFailed{}
}

// brokenTrack is a track that can't play.
var brokenTrack = media.Track{ID: "yt:broken", Source: media.SourceYouTube, Title: "Broken Song",
	Artists: []media.Ref{{Name: "The Band"}}, DurationMs: 200000}

// failedShell is the main window of a player whose tracks can't play,
// once brokenTrack failed, with its notice.
func failedShell(t *testing.T) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShellWith(t, nil, func(a *app.App) player.Services { return failingPlayer{testPlayer{a: a}} }, player.Saved{})
	s.main = true
	t.Cleanup(s.p.Notices(func(n player.Notice) { s.update(func() { s.notes = append(s.notes, n) }) }))
	tt := tester(t, s)
	s.p.PlayTracks([]media.Track{brokenTrack}, 0)
	for deadline := time.Now().Add(5 * time.Second); len(s.p.Failures()) == 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the track didn't fail")
		}
	}
	s.settle(t, func(st player.State) bool { return st.Error != "" })
	tt.Frame()
	return s, tt
}

// TestFailureDetails tells of a track that couldn't play in a toast whose
// Details the keyboard reaches, which shows the track, when it failed,
// what was tried and why, and Copy, which has the focus and puts them on
// the clipboard as text for an issue (#351).
func TestFailureDetails(t *testing.T) {
	var copied []string
	was := copyText
	copyText = func(text string) { copied = append(copied, text) }
	t.Cleanup(func() { copyText = was })
	s, tt := failedShell(t)
	if !tt.HasText("Couldn't play Broken Song: this track is private") {
		t.Fatalf("no toast: %q", tt.Texts())
	}
	for i := 0; i < 300 && !tt.Focused("details"); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !tt.Focused("details") {
		t.Fatalf("Tab doesn't reach Details: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	for _, text := range []string{
		"Why it couldn't play",
		"Broken Song, by The Band",
		"YouTube Music, yt:broken · ",
		"What was tried",
		"1. YouTube's player API: YouTube's player API answered LOGIN_REQUIRED: Sign in to confirm your age",
		"2. yt-dlp: this track is private",
	} {
		if !tt.HasText(text) {
			t.Errorf("the details don't show %q: %q", text, tt.Texts())
		}
	}
	if !tt.Focused("copy the details") {
		t.Fatalf("Copy doesn't have the focus: %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	s.drain()
	tt.Frame()
	if len(copied) != 1 {
		t.Fatalf("copied %q", copied)
	}
	for _, line := range []string{
		"MixMusic couldn't play a track\n\n",
		"- Track: Broken Song, by The Band\n",
		"- From: YouTube Music, yt:broken\n",
		"- What was tried:\n  1. YouTube's player API: YouTube's player API answered LOGIN_REQUIRED: Sign in to confirm your age\n  2. yt-dlp: this track is private\n",
		"- MixMusic 0.4.0 on ",
	} {
		if !strings.Contains(copied[0], line) {
			t.Errorf("the text copied has no %q:\n%s", line, copied[0])
		}
	}
	if !tt.HasText("Copied the details") {
		t.Errorf("no toast tells it copied: %q", tt.Texts())
	}
	click(t, s, tt, "close")
	tt.Advance(time.Second)
	if tt.HasText("Why it couldn't play") {
		t.Error("the dialog didn't close")
	}
}

// TestRecentPlaybackErrors lists the tracks that couldn't play in Settings
// › About, between Updates and Licenses, with their details, and says
// none did before; a search for "playback errors" finds them.
func TestRecentPlaybackErrors(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/about")
	if !tt.HasText(locales.PlaybackErrorsNone()) {
		t.Errorf("no word of no failures: %q", tt.Texts())
	}

	s, tt = failedShell(t)
	tt.SetSize(1280, 2400)
	show(s, tt, "/settings/about")
	// Updates show only in builds that update themselves.
	var above float32
	for _, text := range []string{"open logs folder", s.lk.words("Recent playback errors"), "Broken Song, by The Band", s.lk.words("Licenses")} {
		r, ok := tt.Find(text)
		if !ok || r.Y <= above {
			t.Fatalf("Settings › About shows %q out of place: %q", text, tt.Texts())
		}
		above = r.Y
	}
	if tt.HasText(locales.PlaybackErrorsNone()) || !tt.HasText(" · this track is private") {
		t.Errorf("the failure's row: %q", tt.Texts())
	}
	click(t, s, tt, "details of Broken Song, by The Band")
	tt.Advance(time.Second)
	if !tt.HasText("2. yt-dlp: this track is private") {
		t.Errorf("no details: %q", tt.Texts())
	}
	s.failure.open = false
	for _, q := range []string{"playback errors", "couldn't play"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText("Broken Song, by The Band") {
			t.Errorf("searching %q doesn't find the failure: %q", q, tt.Texts())
		}
	}
}

// TestFailureReport writes the details of a failure as text for an issue,
// without the names of the accounts signed in, the home folder, URLs'
// queries, cookies, tokens or addresses.
func TestFailureReport(t *testing.T) {
	home := "/home/Robin Example"
	if runtime.GOOS == "windows" {
		home = `C:\Users\Robin Example`
	}
	path := filepath.Join(home, "Music", "Song.mp3")
	inHome := strings.Replace(path, home, "~", 1)
	pr := privacyOf(app.AccountsStatus{
		Spotify: app.SpotifyAccount{Profile: &spotify.Profile{ID: "robin.example", Name: "Robin Example"}},
		YouTube: app.YouTubeAccount{Account: &youtube.Account{Name: "Robin E.", Handle: "@robinmusic"}},
		Lastfm:  app.LastfmAccount{Name: "robinlistens"},
	}, home)
	at := time.Date(2026, 11, 8, 14, 3, 12, 0, time.FixedZone("", 7*3600))
	f := player.Failure{
		At:        at,
		Track:     media.Track{ID: "spotify:4uLU6hMCjMI75M1A2tKUQC", Source: media.SourceSpotify, Title: "Song", Artists: []media.Ref{{Name: "Artist"}}, Album: &media.Ref{Name: "Album"}},
		Recording: media.Track{ID: "yt:dQw4w9WgXcQ", Source: media.SourceYouTube, Title: "Song (Official Audio)", Artists: []media.Ref{{Name: "Artist"}}},
		Steps: []player.Step{
			{What: "Spotify Connect", Why: "Spotify refused robin.example's request: Authorization: Bearer abc.def", Times: 1},
			{What: "Spotify in MixMusic", Why: "the player sent Bearer abc.def, api_key=\"k1\"", Times: 1},
			{What: "the stream", Why: "GET https://rr1.googlevideo.com/videoplayback?ip=203.0.113.9&sig=xyz: dial tcp 203.0.113.9:443 and [2001:db8::1]:443 refused, token=abc Cookie: SAPISID=x; SID=y", Times: 1},
			{What: "the stream, tried again", Why: "the file " + path + " of @robinmusic is gone", Times: 3},
		},
		Stopped: true,
	}
	got := failureReport(f, app.Info{Version: "0.5.11", Platform: "windows", Arch: "amd64"}, pr)
	want := "MixMusic couldn't play a track\n\n" +
		"- Track: Song, by Artist, on Album\n" +
		"- From: Spotify, spotify:4uLU6hMCjMI75M1A2tKUQC\n" +
		"- Played as: Song (Official Audio), by Artist, from YouTube Music, yt:dQw4w9WgXcQ\n" +
		"- When: 2026-11-08 14:03:12 +07:00\n" +
		"- What was tried:\n" +
		"  1. Spotify Connect: Spotify refused (account)'s request: Authorization: (hidden)\n" +
		"  2. Spotify in MixMusic: the player sent Bearer (hidden), api_key=(hidden)\n" +
		"  3. The stream: GET https://rr1.googlevideo.com/videoplayback: dial tcp (address) and (address) refused, token=(hidden) Cookie: (hidden)\n" +
		"  4. The stream, tried again, 3 times: the file " + inHome + " of (account) is gone\n" +
		"- " + failureStopped + "\n" +
		"- MixMusic 0.5.11 on windows amd64\n"
	if got != want {
		t.Errorf("the report is\n%s\nwant\n%s", got, want)
	}
	// A track on this computer tells its file, not its ID, which holds the
	// path.
	file := player.Failure{At: at, Track: media.Track{ID: localfiles.TrackID(path), Source: media.SourceFile, Title: "Song"}}
	if got := failureReport(file, app.Info{}, pr); !strings.Contains(got, "- From: a file on this computer, "+inHome+"\n") || strings.Contains(got, "file:") {
		t.Errorf("a file's report:\n%s", got)
	}
	if got := failureWhen(at, at.Add(time.Hour)); got != "14:03:12" {
		t.Errorf("today: %q", got)
	}
	if got := failureWhen(at, at.Add(24*time.Hour)); got != "8 Nov 2026, 14:03:12" {
		t.Errorf("yesterday: %q", got)
	}
	if names := pr.names; !slices.Contains(names, "robinmusic") || slices.Contains(names, "") {
		t.Errorf("hidden names %q", names)
	}
}
