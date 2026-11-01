package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/plugins"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The tests refresh pages whose asks stay on the computer, or fail before
// they leave it: a mix of the user's, an album of a malformed ID. Home,
// Search, Explore and Settings › About and Playback ask the services, run
// yt-dlp or ask GitHub.

// waitRefreshed draws frames until the refresh ended, and what it asked
// came.
func waitRefreshed(t *testing.T, s *shell, tt *ui.Tester) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		s.drain()
		tt.Frame()
		if !s.refreshing() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the refresh didn't end; the window shows %q", tt.Texts())
		}
	}
}

// refreshToasts are the toasts of refreshes that failed the window shows.
func refreshToasts(tt *ui.Tester) []string {
	var out []string
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "Couldn't refresh: ") {
			out = append(out, text)
		}
	}
	return out
}

// badAlbum is the page of an album that showed, whose ID names no service:
// asking for it again fails at once, offline.
func badAlbum(s *shell) *loaded[media.Album] {
	l := &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Tracks: testTracks[:2]}}
	s.loads["album:al-x"] = l
	return l
}

// TestRefreshMix shows a mix changed behind its page, by a plugin or the
// player's menu, once the page is refreshed.
func TestRefreshMix(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	mix, err := s.svc.Collection.CreatePlaylist("Night drive", "", testTracks[:1])
	if err != nil {
		t.Fatal(err)
	}
	show(s, tt, playlistPath(mix.ID))
	waitRefreshed(t, s, tt)
	for deadline := time.Now().Add(5 * time.Second); !tt.HasText(testTracks[0].Title); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the mix didn't show: %q", tt.Texts())
		}
		s.drain()
		tt.Frame()
	}
	if err := s.svc.Collection.AddToPlaylist(context.Background(), mix.ID, testTracks[2:3]); err != nil {
		t.Fatal(err)
	}
	s.drain()
	tt.Frame()
	if tt.HasText(testTracks[2].Title) {
		t.Fatal("the page followed the mix by itself: nothing to refresh")
	}
	tt.Announcements()
	tt.Key(0, ui.KeyF5)
	waitRefreshed(t, s, tt)
	if !tt.HasText(testTracks[2].Title) {
		t.Errorf("F5 didn't show the mix's new track: %q", tt.Texts())
	}
	if !slices.Contains(tt.Announcements(), "Page refreshed") {
		t.Error("screen readers didn't hear the refresh end")
	}
}

// TestRefreshKeepsWhatFails keeps what the page showed when its refresh
// fails, which one toast tells, with the button a spinner meanwhile.
func TestRefreshKeepsWhatFails(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	l := badAlbum(s)
	show(s, tt, "/album/al-x")
	if !s.shown["album:al-x"] {
		t.Fatalf("the album isn't what the page shows: %v", s.shown)
	}
	tt.Key(0, ui.KeyF5)
	if !l.refreshing || l.asks != 1 {
		t.Fatalf("F5 didn't ask again: refreshing %v, asks %d", l.refreshing, l.asks)
	}
	if _, ok := tt.Find("Refreshing"); !ok {
		t.Error("the button isn't a spinner while the answer comes")
	}
	if !tt.HasText("mezzanine") {
		t.Errorf("the page didn't keep the album while it refreshed: %q", tt.Texts())
	}
	tt.Announcements()
	waitRefreshed(t, s, tt)
	if !tt.HasText("mezzanine") || l.err != nil || !l.done {
		t.Errorf("the refresh that failed dropped the album: err %v, the window shows %q", l.err, tt.Texts())
	}
	if got := refreshToasts(tt); !slices.Equal(got, []string{"Couldn't refresh: media: malformed id"}) {
		t.Errorf("the toasts are %q", got)
	}
	if _, ok := tt.Find("Refreshing"); ok {
		t.Error("the spinner stays")
	}
	if slices.Contains(tt.Announcements(), "Page refreshed") {
		t.Error("screen readers heard the refresh that failed end well")
	}
}

// TestRefreshOneToast tells of a refresh whose asks failed in one toast:
// the page's and the side panel's.
func TestRefreshOneToast(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.panel = "lyrics"
	lyricsKey := "lyrics:" + testTracks[0].ID
	s.loads[lyricsKey] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Source: "LRCLIB", Lines: []lyrics.Line{{Text: "Waiting in a car"}}}}
	tt := tester(t, s)
	badAlbum(s)
	show(s, tt, "/album/al-x")
	if !s.shown["album:al-x"] || !s.shown[lyricsKey] {
		t.Fatalf("the page and the panel show %v", s.shown)
	}
	tt.Key(0, ui.KeyF5)
	if !s.refreshingKey(lyricsKey) {
		t.Fatal("the panel's lyrics don't refresh with the page")
	}
	waitRefreshed(t, s, tt)
	if got := refreshToasts(tt); len(got) != 1 {
		t.Errorf("two asks that failed showed the toasts %q", got)
	}
	if !tt.HasText("Waiting in a car") {
		t.Errorf("the panel dropped its lyrics: %q", tt.Texts())
	}
}

// TestRefreshOnce asks once while a refresh is out, however often the user
// presses its key or its button.
func TestRefreshOnce(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	l := badAlbum(s)
	show(s, tt, "/album/al-x")
	tt.Key(0, ui.KeyF5)
	tt.Key(0, ui.KeyF5)
	tt.Click("Refresh this page")
	tt.Frame()
	if l.asks != 1 {
		t.Errorf("three presses while the answer comes asked %d times", l.asks)
	}
	waitRefreshed(t, s, tt)
	// Once it came, the button refreshes again.
	tt.Click("Refresh this page")
	tt.Frame()
	if l.asks != 2 {
		t.Errorf("the button after the refresh asked %d times in all", l.asks)
	}
	waitRefreshed(t, s, tt)
}

// TestRefreshKeys keeps Mod+R for Repeat: F5 refreshes, and lists in the
// palette and Settings › Shortcuts, where the user may give it Mod+R.
func TestRefreshKeys(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/stats")
	tt.Key(0, ui.KeyF5)
	s.drain()
	if st := s.p.State(); st.Repeat != player.RepeatOff {
		t.Errorf("F5 changed repeat to %v", st.Repeat)
	}
	tt.Key(ui.Cmd, ui.KeyR)
	s.settle(t, func(st player.State) bool { return st.Repeat == player.RepeatAll })

	s.paletteOpen = true
	tt.Frame()
	if !tt.HasText("Refresh this page") || !tt.HasText(shortcutLabel(refreshKeys)) {
		t.Errorf("the palette doesn't list Refresh this page with its key: %q", tt.Texts())
	}
	s.paletteOpen = false
	show(s, tt, "/settings/shortcuts")
	click(t, s, tt, "change the shortcut of Refresh this page")
	tt.Key(ui.Cmd, ui.KeyR)
	s.drain()
	tt.Frame()
	if s.set.Shortcuts["view.refresh"] != "Mod+R" || s.set.Shortcuts["player.repeat"] != "" || !tt.HasText("Repeat has no shortcut now") {
		t.Errorf("Mod+R given to Refresh: %v", s.set.Shortcuts)
	}
}

// TestRefreshYieldsKeys leaves F5 to the command the user gave it before
// Refresh this page had it: that command keeps it, and Refresh has none.
func TestRefreshYieldsKeys(t *testing.T) {
	s := newTestShell(t)
	s.set.Shortcuts = map[string]string{"player.mute": "F5"}
	tt := tester(t, s)
	l := badAlbum(s)
	show(s, tt, "/album/al-x")
	tt.Key(0, ui.KeyF5)
	s.settle(t, func(st player.State) bool { return st.Muted })
	if l.refreshing || l.asks != 0 {
		t.Error("F5, the user's for Mute, refreshed the page")
	}
	if keys := s.shortcutOf(command{id: "view.refresh", shortcut: refreshKeys}); keys != "" {
		t.Errorf("Refresh this page keeps %q", keys)
	}
	if refreshKeysOn("darwin") != "Mod+Shift+R" || refreshKeysOn("windows") != "F5" || refreshKeysOn("linux") != "F5" {
		t.Errorf("Refresh's keys: macOS %q, Windows %q", refreshKeysOn("darwin"), refreshKeysOn("windows"))
	}
}

// TestRefreshShown tells what each page refreshes: nothing where what it
// shows is always current, where the button is dimmed.
func TestRefreshShown(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	for _, path := range []string{"/library/liked", "/library/mixes", "/library/history", "/stats", "/settings/appearance", "/settings/shortcuts"} {
		show(s, tt, path)
		tt.Frame()
		if len(s.shown) != 0 {
			t.Errorf("%s refreshes %v", path, s.shown)
		}
		s.refreshPage()
		if s.refreshAt != "" {
			t.Errorf("a refresh of %s asked for something", path)
		}
	}
	for path, want := range map[string][]string{
		"/":               {"home:spotify", "home:youtube", "tools"},
		"/settings/about": {"release"},
		"/explore/charts": {"charts"},
	} {
		show(s, tt, path)
		tt.Frame()
		var got []string
		for key := range s.shown {
			got = append(got, key)
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s refreshes %v, want %v", path, got, want)
		}
	}
}

// TestRefreshShownPageOnly refreshes the page shown, not the one that
// slides away as it comes.
func TestRefreshShownPageOnly(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	s.loads["moods"] = &loaded[[]youtube.MoodGroup]{done: true}
	show(s, tt, "/explore")
	s.navigate("/explore/moods")
	s.drain()
	tt.Frame() // the new releases slide away, the moods come over them
	if !s.shown["moods"] || s.shown["explore"] {
		t.Errorf("the moods, as the new releases slide away, refresh %v", s.shown)
	}
}

// TestRefreshPluginPage renders a plugin's page again.
func TestRefreshPluginPage(t *testing.T) {
	s := newTestShell(t)
	dir := filepath.Join(s.svc.System.Info().DataDir, "addons", "plugins", "again")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"plugin.json": `{"id": "again", "name": "Again", "version": "1.0.0", "author": "A", "main": "index.js", "api": 2}`,
		"index.js": `
export default function activate(api) {
  let renders = 0;
  api.ui.addPage({ name: "home", title: "again", render(view) { view.set(api.ui.text("rendered " + ++renders)) } });
}`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := s.svc.Preferences.Get()
	st.EnabledPlugins = []string{"again"}
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.svc.Addons.Reload()
	s.refresh()
	s.plugins = plugins.New(s.a, s.p)
	s.plugins.SetUI(pluginUI{s})
	t.Cleanup(s.plugins.Close)
	s.syncPlugins()
	tt := tester(t, s)
	until(t, s, tt, "the plugin's page", func() bool { return s.plugins.Page("/plugin/again/home") != nil })
	show(s, tt, "/plugin/again/home")
	until(t, s, tt, "the first render", func() bool { return tt.HasText("rendered 1") })
	tt.Key(0, ui.KeyF5)
	until(t, s, tt, "the render of the refresh", func() bool { return tt.HasText("rendered 2") })
}

// TestRefreshRegistry asks the community registry itself on a refresh of
// the marketplace, past the answer the app keeps for ten minutes, which
// coming back to the page shows.
func TestRefreshRegistry(t *testing.T) {
	s := newTestShell(t)
	community := t.TempDir()
	s.a.ReadCommunityFrom(community)
	theme := func(id, name string) string {
		return `{"id": "` + id + `", "name": "` + name + `", "version": "1.0.0", "author": "A", "path": "themes/` + id + `", "files": ["theme.json"]}`
	}
	publish := func(themes ...string) {
		t.Helper()
		data := `{"version": 2, "baseUrl": "https://raw.githubusercontent.com/fzn0x/mixmusic/registry/", "themes": [` + strings.Join(themes, ", ") + `], "plugins": []}`
		if err := os.WriteFile(filepath.Join(community, "registry-2.json"), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	publish(theme("neon-nights", "Neon Nights"))
	tt := tester(t, s)
	show(s, tt, "/marketplace")
	came := func() bool { return s.loads["registry"].(*loaded[addons.Registry]).done }
	until(t, s, tt, "the registry", came)
	// What another test kept of the registry goes.
	s.refreshPage()
	waitRefreshed(t, s, tt)
	if !tt.HasText("neon nights") {
		t.Fatalf("the marketplace doesn't show the registry: %q", tt.Texts())
	}

	publish(theme("neon-nights", "Neon Nights"), theme("paper-mint", "Paper Mint"))
	s.forget("registry") // as the page's try again
	tt.Frame()
	until(t, s, tt, "the registry again", came)
	if tt.HasText("paper mint") {
		t.Fatal("the registry kept for ten minutes didn't keep")
	}
	tt.Key(0, ui.KeyF5)
	waitRefreshed(t, s, tt)
	if !tt.HasText("paper mint") {
		t.Errorf("the refresh didn't ask the registry: %q", tt.Texts())
	}
}

// TestSpotifyLikedStartsAgain starts the liked songs on Spotify again at
// the first page that came last, of a refresh, a try again or another
// account, rather than keep the pages load more brought after the one
// before; a page of load more that comes after it is dropped.
func TestSpotifyLikedStartsAgain(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{Spotify: app.SpotifyAccount{SignedIn: true}}}
	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true}
	s.loads["spotify:albums"] = &loaded[[]media.Album]{done: true}
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true}
	first := &loaded[app.TrackPage]{done: true, value: app.TrackPage{Tracks: testTracks[:2], Total: 4}}
	s.loads["spotify:liked"] = first
	tt := testerOf(t, s, 1280, 2000)
	show(s, tt, "/library/spotify")
	if !tt.HasText("load more · 2 of 4") {
		t.Fatalf("the liked songs on Spotify: %q", tt.Texts())
	}
	// The second page, as load more brings it.
	s.tookSpotifyLiked(first, first.answers, app.TrackPage{Tracks: testTracks[2:4], Total: 4}, nil)
	tt.Frame()
	if !tt.HasText(testTracks[3].Title) || tt.HasText("load more") {
		t.Fatalf("load more didn't add the second page: %q", tt.Texts())
	}

	// A refresh's first page.
	first.value, first.answers = app.TrackPage{Tracks: testTracks[4:5], Total: 4}, first.answers+1
	tt.Frame()
	if tt.HasText(testTracks[3].Title) || !tt.HasText(testTracks[4].Title) || !tt.HasText("load more · 1 of 4") {
		t.Errorf("the refresh's first page didn't start the list again: %q", tt.Texts())
	}
	s.tookSpotifyLiked(first, first.answers-1, app.TrackPage{Tracks: testTracks[2:4], Total: 4}, nil)
	tt.Frame()
	if tt.HasText(testTracks[3].Title) {
		t.Errorf("a page of load more of the first page before stayed: %q", tt.Texts())
	}

	// Another account's, after signing in, as clear(s.loads) leaves it.
	s.loads["spotify:liked"] = &loaded[app.TrackPage]{done: true, value: app.TrackPage{Tracks: testTracks[5:6], Total: 1}}
	tt.Frame()
	if tt.HasText(testTracks[4].Title) || !tt.HasText(testTracks[5].Title) {
		t.Errorf("another first page didn't start the list again: %q", tt.Texts())
	}
}
