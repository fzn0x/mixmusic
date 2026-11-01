package main

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

func TestMain(m *testing.M) {
	isDark = func() bool { return true }
	setAppearance = func(mygo.ThemeSource) {}
	copyText = func(string) {}
	// The tests show what they give the pages: a request that reaches for
	// the network goes to a closed port, and fails.
	for _, v := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"} {
		os.Setenv(v, "http://127.0.0.1:9")
	}
	os.Unsetenv("NO_PROXY")
	os.Unsetenv("no_proxy")
	os.Exit(m.Run())
}

// testPlayer is the app as a player's services, playing nothing.
type testPlayer struct{}

func (testPlayer) Prepare(_ context.Context, t media.Track) (player.Source, error) {
	return player.Source{Mode: player.ModeStream, URL: "url:" + t.ID, Track: t}, nil
}

func (p testPlayer) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return p.Prepare(ctx, t)
}
func (testPlayer) StreamFailure(media.Track) string { return "" }
func (testPlayer) RecordPlay(media.Track)           {}
func (testPlayer) Radio(context.Context, media.Track) ([]media.Track, error) {
	return nil, nil
}
func (testPlayer) Settings() settings.Settings { return settings.Default() }
func (testPlayer) SaveVolume(float64)          {}
func (testPlayer) Report(player.Now)           {}
func (testPlayer) SaveQueue(player.Saved)      {}

// testStream is an engine that plays at once, silently.
type testStream struct {
	mu sync.Mutex
	p  *player.Player
}

func (s *testStream) Load(_ string, _ time.Duration, autoplay bool) error {
	if autoplay {
		s.player().Playing(true)
	}
	return nil
}
func (s *testStream) player() *player.Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.p
}
func (s *testStream) Preload(string)                        {}
func (s *testStream) Crossfade(string, time.Duration) error { return nil }
func (s *testStream) Play() error                           { s.player().Playing(true); return nil }
func (s *testStream) Pause()                                { s.player().Playing(false) }
func (s *testStream) Stop()                                 {}
func (s *testStream) Unload()                               {}
func (s *testStream) Seek(time.Duration)                    {}
func (s *testStream) SetVolume(float64)                     {}

type testDevice struct{}

func (testDevice) Play(context.Context, media.Track, time.Duration) error { return nil }
func (testDevice) Pause(context.Context) error                            { return nil }
func (testDevice) Resume(context.Context) error                           { return nil }
func (testDevice) Seek(context.Context, time.Duration) error              { return nil }
func (testDevice) SetVolume(context.Context, float64) error               { return nil }
func (testDevice) State(context.Context) (*player.DeviceState, error)     { return nil, nil }

func testTrack(id, title, artist, album string, source media.Source) media.Track {
	return media.Track{ID: id, Source: source, Title: title, Artists: []media.Ref{{ID: "ar-" + artist, Name: artist}},
		Album: &media.Ref{ID: "al-" + album, Name: album}, DurationMs: 211000}
}

var testTracks = []media.Track{
	testTrack("yt:1", "Midnight City", "M83", "Hurry Up, We're Dreaming", media.SourceYouTube),
	testTrack("spotify:2", "Teardrop", "Massive Attack", "Mezzanine", media.SourceSpotify),
	testTrack("yt:3", "Strobe", "deadmau5", "For Lack of a Better Name", media.SourceYouTube),
	testTrack("yt:4", "Windowlicker", "Aphex Twin", "Windowlicker", media.SourceYouTube),
	testTrack("spotify:5", "Archangel", "Burial", "Untrue", media.SourceSpotify),
	testTrack("yt:6", "Glósóli", "Sigur Rós", "Takk...", media.SourceYouTube),
}

// newTestShell is the main window of an app in a folder of its own, whose
// player plays nothing, and whose pages show what the test gives them.
func newTestShell(t testing.TB) *shell { return newTestShellOf(t, nil) }

// newTestShellOf is newTestShell of an app whose options set changes.
func newTestShellOf(t testing.TB, set func(o *app.Options)) *shell {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"data", "cache", "logs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	opts := app.Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"),
		LogsDir: filepath.Join(dir, "logs"), Builtin: builtin.FS, Version: "0.4.0"}
	if set != nil {
		set(&opts)
	}
	a, err := app.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	// The covers' downloads end before the folder goes.
	t.Cleanup(a.Services().System.Artwork().Close)
	stream := &testStream{}
	p := player.New(testPlayer{}, stream, testDevice{}, player.Saved{})
	stream.mu.Lock()
	stream.p = p
	stream.mu.Unlock()
	t.Cleanup(p.Close)
	registerFonts()
	s := newShell(a, p)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true}
	s.loads["tools"] = &loaded[tools.Status]{done: true}
	for _, tr := range testTracks {
		s.loads["related-"+tr.ID] = &loaded[[]media.Shelf]{done: true}
	}
	return s
}

// settle lets the player take in what the test asked, and the pages what
// they fetched.
func (s *shell) settle(t *testing.T, ok func(player.State) bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; {
		s.drain()
		if ok == nil || ok(s.p.State()) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the player didn't settle: %+v", s.p.State())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// renderTo draws the window as tt shows it, once what it does settled.
func renderTo(t *testing.T, s *shell, tt *ui.Tester, dir, name string) {
	t.Helper()
	s.drain()
	tt.Frame()
	tt.Advance(2 * time.Second)
	img := tt.Image()
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestRender draws the main window's pages to MIXMUSIC_RENDER, for a look.
func TestRender(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s := newTestShell(t)
	album := media.Album{ID: "al-1", Source: media.SourceYouTube, Title: "Hurry Up, We're Dreaming", Artists: []media.Ref{{ID: "ar-m83", Name: "M83"}},
		Year: "2011", Tracks: testTracks}
	var items []media.Item
	for i := range 6 {
		a := album
		a.ID = "al-" + count(i)
		items = append(items, media.Item{Kind: media.ItemAlbum, Album: &a})
	}
	var trackItems []media.Item
	for i := range testTracks {
		trackItems = append(trackItems, media.Item{Kind: media.ItemTrack, Track: &testTracks[i]})
	}
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{
		{Title: "Listen again", Source: media.SourceYouTube, Items: trackItems},
		{Title: "Albums for you", Source: media.SourceYouTube, Items: items},
	}}
	s.loads["album:al-1"] = &loaded[media.Album]{done: true, value: album}
	s.loads["lyrics:spotify:2"] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB", Lines: []lyrics.Line{
		{TimeMs: 0, Text: "Love, love is a verb"}, {TimeMs: 4000, Text: "Love is a doing word"},
		{TimeMs: 9000, Text: "Fearless on my breath"}, {TimeMs: 14000, Text: "Gentle impulsion"},
		{TimeMs: 19000, Text: "Shakes me, makes me lighter"}, {TimeMs: 24000, Text: "Fearless on my breath"}}}}

	// Home's jump back in tile offers what played last.
	if err := s.svc.Collection.RecordPlay(testTracks[0]); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(s.view, 1280, 820)
	renderTo(t, s, tt, dir, "home-idle")

	s.p.PlayTracks(testTracks, 1)
	s.settle(t, func(st player.State) bool { return st.Playing })
	renderTo(t, s, tt, dir, "home-playing")

	s.navigate("/album/al-1")
	renderTo(t, s, tt, dir, "album")

	s.navigate("/now-playing")
	s.panel = ""
	renderTo(t, s, tt, dir, "now-playing")

	// The pages of what the user did: plays and likes, in the app's library.
	for i, tr := range testTracks {
		for range len(testTracks) - i {
			if err := s.svc.Collection.RecordPlay(tr); err != nil {
				t.Fatal(err)
			}
		}
		if tr.Source == media.SourceYouTube {
			if err := s.svc.Collection.SetLiked(context.Background(), tr, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	s.navigate("/stats")
	renderTo(t, s, tt, dir, "stats")
	s.navigate("/library/liked")
	renderTo(t, s, tt, dir, "library-liked")

	s.loads["search:midnight"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{
		YouTube: media.SearchResults{Tracks: testTracks[:3], Albums: []media.Album{album}, Artists: []media.Artist{{ID: "ar-m83", Name: "M83", Source: media.SourceYouTube}}},
		Spotify: media.SearchResults{Tracks: testTracks[3:]}, SpotifySignedIn: true}}
	s.navigate(searchPath("midnight"))
	renderTo(t, s, tt, dir, "search")

	s.loads["moods"] = &loaded[[]youtube.MoodGroup]{done: true, value: []youtube.MoodGroup{
		{Title: "Moods & moments", Moods: []youtube.Mood{{Title: "Chill", Params: "a"}, {Title: "Commute", Params: "b"}, {Title: "Energy boosters", Params: "c"}, {Title: "Feel good", Params: "d"}, {Title: "Focus", Params: "e"}, {Title: "Party", Params: "f"}}},
		{Title: "Genres", Moods: []youtube.Mood{{Title: "Ambient", Params: "g"}, {Title: "Electronic", Params: "h"}, {Title: "Hip-hop", Params: "i"}}},
	}}
	s.navigate("/explore/moods")
	renderTo(t, s, tt, dir, "explore-moods")

	s.navigate("/settings/playback")
	renderTo(t, s, tt, dir, "settings-playback")
	s.navigate("/settings/appearance")
	renderTo(t, s, tt, dir, "settings-appearance")
	s.navigate("/settings/accounts")
	renderTo(t, s, tt, dir, "settings-accounts")
	s.navigate("/settings/plugins")
	renderTo(t, s, tt, dir, "settings-plugins")
	tall := ui.NewTester(s.view, 1280, 1500)
	s.navigate("/settings/appearance")
	renderTo(t, s, tall, dir, "settings-appearance-tall")

	renderTo(t, s, ui.NewTester(s.miniView, 420, 132), dir, "mini")
	renderTo(t, s, ui.NewTester(s.miniView, 340, 180), dir, "mini-narrow")
}

// TestRenderThemes draws the home page and the mini player in each
// built-in theme to MIXMUSIC_RENDER, for a look at their effects.
func TestRenderThemes(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s := newTestShell(t)
	var items []media.Item
	for i := range testTracks {
		items = append(items, media.Item{Kind: media.ItemTrack, Track: &testTracks[i]})
	}
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{{Title: "Listen again", Source: media.SourceYouTube, Items: items}}}
	s.p.PlayTracks(testTracks, 1)
	s.settle(t, func(st player.State) bool { return st.Playing })
	for _, theme := range []struct {
		id      string
		options map[string]string
	}{{"mix", nil}, {"mix", map[string]string{"texture": "slashes", "white-tiles": "true"}}, {"aurora", nil}, {"sakura", nil}, {"terminal", nil}, {"daylight", nil},
		{"greenroom", nil}, {"glasshouse", nil}, {"anime", nil}, {"mochi", nil}} {
		st := s.svc.Preferences.Get()
		st.FollowSystem, st.Theme = false, theme.id
		st.ThemeOptions = map[string]map[string]string{theme.id: theme.options}
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		name := theme.id
		if theme.options != nil {
			name += "-options"
		}
		s.navigate("/")
		renderTo(t, s, ui.NewTester(s.view, 1280, 820), dir, "theme-"+name)
		renderTo(t, s, ui.NewTester(s.miniView, 420, 132), dir, "theme-"+name+"-mini")
		if theme.id == "aurora" {
			s.navigate("/settings/appearance")
			renderTo(t, s, ui.NewTester(s.view, 1280, 1700), dir, "theme-aurora-settings")
		}
	}
}

// TestRenderDialogs draws the dialogs to MIXMUSIC_RENDER, in a window as
// tall as usual and in one too short for them, at the largest text size,
// and the update offer in the themes whose panels differ.
func TestRenderDialogs(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	shell := func(theme string, scale float64) *shell {
		s := newTestShell(t)
		st := s.svc.Preferences.Get()
		st.FollowSystem, st.Theme, st.FontScale = false, theme, scale
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		return s
	}
	offer := testOffer
	for _, size := range []struct {
		name  string
		w, h  int
		scale float64
	}{{"", 1280, 820, 1}, {"-short", 911, 425, 1.3}, {"-shorter", 640, 360, 1.3}} {
		s := shell("mix", size.scale)
		s.offer = &offer
		renderTo(t, s, testerOf(t, s, size.w, size.h), dir, "dialog-offer"+size.name)

		s = shell("mix", size.scale)
		s.ask("enable Hello", enableMessage, "enable", func() {})
		renderTo(t, s, testerOf(t, s, size.w, size.h), dir, "dialog-question"+size.name)

		s = shell("mix", size.scale)
		s.promptName("New mix", "Name", "late night drive", "create", func(string) {})
		renderTo(t, s, testerOf(t, s, size.w, size.h), dir, "dialog-prompt"+size.name)

		s = shell("mix", size.scale)
		tt := testerOf(t, s, size.w, size.h)
		s.paletteOpen = true
		renderTo(t, s, tt, dir, "dialog-palette"+size.name)
	}
	for _, theme := range []string{"aurora", "glasshouse", "mochi"} {
		s := shell(theme, 1)
		s.offer = &offer
		renderTo(t, s, testerOf(t, s, 1280, 820), dir, "dialog-offer-"+theme)
	}
}
