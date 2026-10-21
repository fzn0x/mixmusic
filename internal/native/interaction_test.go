package native

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/lyrics"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// tester is a window of the shell, without one.
func tester(t *testing.T, s *shell) *ui.Tester { return testerOf(t, s, 1280, 820) }

// testerOf is a window of the shell of a size.
func testerOf(t *testing.T, s *shell, w, h int) *ui.Tester {
	t.Helper()
	for _, key := range []string{"home", "explore", "charts"} {
		s.loads[key] = &loaded[[]media.Shelf]{done: true}
	}
	tt := ui.NewTester(s.view, w, h)
	tt.Frame()
	return tt
}

// show shows the page at path, once its transition ends.
func show(s *shell, tt *ui.Tester, path string) {
	s.navigate(path)
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
}

func click(t *testing.T, s *shell, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("clicking %q: %v; the window shows %q", label, err, tt.Texts())
	}
	s.drain()
	tt.Frame()
}

func TestNavigation(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	for _, n := range []struct{ label, path string }{
		{"library", "/library"}, {"stats", "/stats"}, {"settings", "/settings"}, {"explore", "/explore"}, {"home", "/"},
	} {
		click(t, s, tt, n.label)
		if got := s.router.Path(); got != n.path {
			t.Errorf("clicking %s went to %q", n.label, got)
		}
	}
	click(t, s, tt, "Back")
	if got := s.router.Path(); got != "/explore" {
		t.Errorf("back went to %q", got)
	}
}

func TestTransportClicks(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	click(t, s, tt, "Pause")
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	tt.Key(0, ui.KeySpace)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	click(t, s, tt, "Next")
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	tt.Frame()
	click(t, s, tt, "Shuffle")
	s.settle(t, func(st player.State) bool { return st.Shuffle })
	tt.Frame()
	click(t, s, tt, "Repeat: off")
	s.settle(t, func(st player.State) bool { return st.Repeat == player.RepeatAll })
	tt.Frame()
	if !tt.HasText(testTracks[1].Title) {
		t.Errorf("the window doesn't show what plays: %q", tt.Texts())
	}
}

func TestQueuePanel(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	// The queue lists what plays next.
	for _, tr := range testTracks[1:] {
		if !tt.HasText(tr.Title) {
			t.Fatalf("the queue doesn't list %s: %q", tr.Title, tt.Texts())
		}
	}
	click(t, s, tt, "Play "+testTracks[3].Title)
	s.settle(t, func(st player.State) bool { return st.Index == 3 })
	tt.Frame()
	click(t, s, tt, "Clear the queue")
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 4 })
	tt.Frame()
	click(t, s, tt, "Close up next")
	if s.panel != "" {
		t.Error("the queue didn't close")
	}
}

// TestUpdateOffer offers what this copy can do with a new version, install
// it or download what replaces it, and what's new.
func TestUpdateOffer(t *testing.T) {
	s := newTestShell(t)
	s.offer = &app.Release{Version: "9.9.9", Newer: true, Page: "https://github.com/fzn0x/mixmusic/releases/tag/v9.9.9",
		Download: "https://github.com/fzn0x/mixmusic/releases/download/v9.9.9/mixmusic-9.9.9-linux-amd64.deb",
		File:     "mixmusic-9.9.9-linux-amd64.deb"}
	tt := tester(t, s)
	u := s.svc.System.Updates()
	act := downloadLabel(u.Kind)
	if u.Enabled {
		act = "install 9.9.9"
	}
	for _, text := range []string{"mixmusic 9.9.9 is out", "what's new", "later", "skip this version", act} {
		if !tt.HasText(text) {
			t.Errorf("the offer doesn't show %q: %q", text, tt.Texts())
		}
	}
	if !u.Enabled && tt.HasText("install 9.9.9") {
		t.Error("a copy that can't replace itself offers to install")
	}
}

func TestReplacing(t *testing.T) {
	if got := replacing("deb", "m.deb"); !strings.Contains(got, "sudo apt install ./m.deb") {
		t.Errorf("the .deb says %q", got)
	}
	if got := downloadLabel("appimage"); got != "download the AppImage" {
		t.Errorf("the AppImage's download is %q", got)
	}
	if got := downloadLabel("unknown"); got != "download it" {
		t.Errorf("an unknown kind's download is %q", got)
	}
}

// TestQueueMenu plays an entry of the queue from its menu, keeping the
// queue, as a double click does, and takes one out.
func TestQueueMenu(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	choose := func(title string, item ...string) {
		t.Helper()
		if err := tt.RightClick(title); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(item...); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		tt.Frame()
	}
	choose(testTracks[3].Title, "Play")
	s.settle(t, func(st player.State) bool { return st.Index == 3 })
	if n := len(s.p.State().Queue); n != len(testTracks) {
		t.Errorf("playing from the menu left %d tracks in the queue, of %d", n, len(testTracks))
	}
	tt.Frame()
	choose(testTracks[5].Title, "Remove from the queue")
	s.settle(t, func(st player.State) bool { return len(st.Queue) == len(testTracks)-1 })
}

// TestNowPlayingRelated shows what YouTube Music relates to the track
// under the player, and nothing for a track it relates nothing to.
func TestNowPlayingRelated(t *testing.T) {
	s := newTestShell(t)
	lua := testTrack("yt:9", "Lua", "Bright Eyes", "I'm Wide Awake, It's Morning", media.SourceYouTube)
	s.loads["related-"+testTracks[0].ID] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{
		{Title: "you might also like", Items: []media.Item{{Kind: media.ItemTrack, Track: &lua}}},
	}}
	s.panel = ""
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	// The shelf is below the fold of a window of the usual size.
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/now-playing")
	if !tt.HasText("you might also like") || !tt.HasText("Lua") {
		t.Fatalf("no related shelf: %q", tt.Texts())
	}
	s.p.Next()
	s.settle(t, func(st player.State) bool { return st.Index == 1 })
	tt.Frame()
	tt.Advance(time.Second)
	if tt.HasText("you might also like") {
		t.Error("the shelf of the track before stays")
	}
}

// TestLyricsTwice shows the lyrics on the now playing page and in the side
// panel at once, each in a list of its own.
func TestLyricsTwice(t *testing.T) {
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "Waiting in a car"}, {TimeMs: 60000, Text: "Waiting for a ride in the dark"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/now-playing")
	s.open("/lyrics")
	tt.Frame()
	tt.Advance(time.Second)
	n := 0
	for _, text := range tt.Texts() {
		if text == "Waiting in a car" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("the lyrics show %d times, not on the page and in the panel", n)
	}
}

// TestAlbumArtistLinks opens the page of an album's artist from its head.
func TestAlbumArtistLinks(t *testing.T) {
	s := newTestShell(t)
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine",
		Artists: []media.Ref{{ID: "yt:UCneil", Name: "Neil Davidge"}}, Tracks: testTracks[:2]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	click(t, s, tt, "Neil Davidge")
	if got := s.router.Path(); got != artistPath("yt:UCneil") {
		t.Errorf("the artist's link went to %q", got)
	}
}

// TestTrackMoreButton opens a track's menu from its row's button.
func TestTrackMoreButton(t *testing.T) {
	s := newTestShell(t)
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Tracks: testTracks[:2]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	click(t, s, tt, "More for "+testTracks[1].Title)
	if !slices.Contains(tt.Menu(), "Play next") {
		t.Errorf("the button opened %q", tt.Menu())
	}
}

// TestNewMix asks for a new mix's name, and makes it; from a track's menu,
// the track's title is the name to start from.
func TestNewMix(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	click(t, s, tt, "New mix")
	if s.naming == nil {
		t.Fatal("New mix asked for no name")
	}
	tt.Type("late night drive")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	mixes := s.svc.Collection.Playlists()
	if len(mixes) != 1 || mixes[0].Title != "late night drive" {
		t.Fatalf("the mixes are %+v", mixes)
	}
	if got := s.router.Path(); got != playlistPath(mixes[0].ID) {
		t.Errorf("the new mix didn't open: %q", got)
	}
	s.newMixOf(testTracks[:1])
	tt.Frame()
	if s.naming == nil || s.naming.value != testTracks[0].Title {
		t.Fatalf("a track's new mix starts from %+v", s.naming)
	}
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if mixes := s.svc.Collection.Playlists(); len(mixes) != 2 {
		t.Errorf("the track's mix wasn't made: %+v", mixes)
	}
}

// TestNewYouTubePlaylist asks for a new YouTube Music playlist's name,
// from a track's menu, its title to start from, and from the library.
func TestNewYouTubePlaylist(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true}}}
	s.loads["youtube:playlists"] = &loaded[[]media.Playlist]{done: true}
	s.loads["youtube:albums"] = &loaded[[]media.Album]{done: true}
	s.loads["youtube:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["youtube:liked"] = &loaded[[]media.Track]{done: true}
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Tracks: testTracks[:2]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	if err := tt.RightClick(testTracks[0].Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Add to YouTube Music playlist", "New playlist…"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	tt.Frame()
	if s.naming == nil || s.naming.value != testTracks[0].Title || !strings.Contains(s.naming.title, "YouTube Music") {
		t.Fatalf("the new playlist asks %+v", s.naming)
	}
	s.naming = nil
	show(s, tt, "/library/youtube")
	click(t, s, tt, "new playlist")
	if s.naming == nil || s.naming.value != "" {
		t.Errorf("the library's new playlist asks %+v", s.naming)
	}
}

// TestSaveAsMix saves an album's tracks as a new mix, named after it to
// start with.
func TestSaveAsMix(t *testing.T) {
	s := newTestShell(t)
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Tracks: testTracks[:2]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	menu := s.collectionMenu("Mezzanine", testTracks[:2], "", media.SourceYouTube, "")
	tt2 := ui.NewTester(func(c *ui.Context) {
		ui.Button(c, "More").Menu(menu)
	}, 200, 100)
	if err := tt2.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt2.ChooseMenuItem("Save as a new mix"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt2.Menu())
	}
	tt2.Frame()
	if s.naming == nil || s.naming.value != "Mezzanine" {
		t.Fatalf("saving as a mix asks %+v", s.naming)
	}
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	mixes := s.svc.Collection.Playlists()
	if len(mixes) != 1 || mixes[0].Title != "Mezzanine" || max(mixes[0].TrackCount, len(mixes[0].Tracks)) != 2 {
		t.Errorf("the mixes are %+v", mixes)
	}
}

// TestCopyIntoMix offers to copy a service's playlist into a mix, and
// only that.
func TestCopyIntoMix(t *testing.T) {
	s := newTestShell(t)
	items := func(importID string) []string {
		menu := s.collectionMenu("Chill", testTracks[:2], "", media.SourceYouTube, importID)
		tt := ui.NewTester(func(c *ui.Context) { ui.Button(c, "More").Menu(menu) }, 200, 100)
		if err := tt.Click("More"); err != nil {
			t.Fatal(err)
		}
		return tt.Menu()
	}
	if got := items("yt:PLchill"); !slices.Contains(got, "Copy into a mix you can extend") {
		t.Errorf("a playlist's menu holds %q", got)
	}
	if got := items(""); slices.Contains(got, "Copy into a mix you can extend") {
		t.Errorf("an album's menu holds %q", got)
	}
}

// TestPivotCounts counts what the pivots of a search show.
func TestPivotCounts(t *testing.T) {
	s := newTestShell(t)
	s.loads["search:lua"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{YouTube: media.SearchResults{
		Tracks: testTracks[:4], Albums: []media.Album{{ID: "al-1", Title: "Lifted"}, {ID: "al-2", Title: "Fevers"}}}}}
	tt := tester(t, s)
	show(s, tt, searchPath("lua"))
	texts := tt.Texts()
	counted := func(pivot, n string) bool {
		for i := range len(texts) - 1 {
			if texts[i] == pivot && texts[i+1] == n {
				return true
			}
		}
		return false
	}
	if !counted("songs", "4") || !counted("albums", "2") {
		t.Errorf("the pivots don't count the results: %q", texts)
	}
}

// TestArtistPivots has an artist's page pivot to the sections it shows.
func TestArtistPivots(t *testing.T) {
	s := newTestShell(t)
	s.loads["artist:ar-x"] = &loaded[media.Artist]{done: true, value: media.Artist{ID: "ar-x", Name: "Bonobo",
		TopTracks: testTracks[:3], Albums: []media.Album{{ID: "al-1", Title: "Migration"}},
		Related: []media.Ref{{ID: "ar-y", Name: "Tycho"}}}}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/artist/ar-x")
	for _, p := range []string{"songs", "albums", "related"} {
		if !tt.HasText(p) {
			t.Errorf("no %s pivot: %q", p, tt.Texts())
		}
	}
	if tt.HasText("singles") {
		t.Error("a singles pivot for no singles")
	}
	click(t, s, tt, "related")
	if s.artistPivot != 2 {
		t.Errorf("the related pivot is %d", s.artistPivot)
	}
}

// TestShelfArrows scrolls a shelf on with its arrow.
func TestShelfArrows(t *testing.T) {
	s := newTestShell(t)
	var items []media.Item
	for i := range 20 {
		items = append(items, media.Item{Kind: media.ItemAlbum, Album: &media.Album{ID: "al-" + count(i), Title: "Album " + count(i)}})
	}
	// The shelf is below the fold of a window of the usual size.
	tt := testerOf(t, s, 1280, 1600)
	s.loads["home"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{{Title: "Albums for you", Source: media.SourceYouTube, Items: items}}}
	show(s, tt, "/")
	click(t, s, tt, "Scroll Albums for you on")
	tt.Advance(time.Second)
	tt.Frame()
	if st := s.shelfScroll["/ home-0"]; st == nil || st.X <= 0 {
		t.Errorf("the shelf didn't scroll on: %+v", st)
	}
}

// TestActionToasts shows a toast with an action, which its button runs.
func TestActionToasts(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	ran := false
	s.actionToasts = append(s.actionToasts, actionToast{message: "Installed Neon Nights", action: "use it", run: func() { ran = true }})
	tt.Frame()
	if !tt.HasText("Installed Neon Nights") {
		t.Fatalf("no toast: %q", tt.Texts())
	}
	click(t, s, tt, "use it")
	if !ran {
		t.Error("the toast's action didn't run")
	}
}

// TestPanelKept opens the side panel the user left, at the next start.
func TestPanelKept(t *testing.T) {
	s := newTestShell(t)
	if s.panel != "queue" {
		t.Fatalf("the first panel is %q", s.panel)
	}
	s.togglePanel("lyrics")
	if again := newShell(s.a, s.p); again.panel != "lyrics" {
		t.Errorf("the next start shows %q", again.panel)
	}
	s.togglePanel("lyrics")
	if again := newShell(s.a, s.p); again.panel != "" {
		t.Errorf("the next start shows %q, not none", again.panel)
	}
}

// TestLyricsPlainKept shows lyrics as plain text at the next start, once
// the user chose it.
func TestLyricsPlainKept(t *testing.T) {
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "Waiting in a car"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.showPanel("lyrics")
	tt := tester(t, s)
	click(t, s, tt, "plain text")
	if !s.svc.Preferences.Get().LyricsPlain {
		t.Fatal("plain text wasn't kept")
	}
	if again := newShell(s.a, s.p); !again.set.LyricsPlain {
		t.Error("the next start shows the lyrics synced")
	}
}

// TestYTDLPNotNow puts off an outdated yt-dlp's release, past restarts.
func TestYTDLPNotNow(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	s.loads["tools"] = &loaded[tools.Status]{done: true, value: tools.Status{
		YTDLP: tools.Tool{Found: true, Version: "2025.01.01", Latest: "2026.10.01"}}}
	show(s, tt, "/")
	if !tt.HasText("Your yt-dlp is out of date") {
		t.Fatalf("no offer: %q", tt.Texts())
	}
	click(t, s, tt, "not now")
	if got := s.svc.Preferences.Get().SkippedYTDLP; got != "2026.10.01" {
		t.Errorf("put off %q", got)
	}
	if tt.HasText("Your yt-dlp is out of date") {
		t.Error("the offer stays")
	}
}

// TestLyricsScrolledStay keeps lyrics the user scrolls from following the
// song for a while.
func TestLyricsScrolledStay(t *testing.T) {
	s := newTestShell(t)
	var lines []lyrics.Line
	for i := range 40 {
		lines = append(lines, lyrics.Line{TimeMs: i * 1000, Text: "line " + count(i)})
	}
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB", Lines: lines}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.showPanel("lyrics")
	tt := tester(t, s)
	r, ok := tt.Find("line 2")
	if !ok {
		t.Fatalf("no lyrics: %q", tt.Texts())
	}
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 120)
	tt.Frame()
	if time.Since(s.lyricsView.panel.userAt) > time.Second {
		t.Error("scrolling the lyrics wasn't noticed")
	}
}

// TestPaletteThemes wears a theme from the palette.
func TestPaletteThemes(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	tt.Type("theme: aurora")
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().ActiveTheme(s.lk.systemDark); got != "aurora" {
		t.Errorf("the theme is %q, not aurora", got)
	}
}

func TestPaletteRuns(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	if !s.paletteOpen {
		t.Fatal("Mod+K didn't open the palette")
	}
	tt.Type("stats")
	tt.Frame()
	// Enter searches what was typed; Down reaches the command.
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if s.paletteOpen || s.router.Path() != "/stats" {
		t.Errorf("the palette is open: %v; the page is %q", s.paletteOpen, s.router.Path())
	}
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	tt.Type("lua")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.router.Location(); got != searchPath("lua") {
		t.Errorf("Enter in the palette went to %q, not the search", got)
	}
	tt.Key(ui.Alt, ui.Key3)
	tt.Frame()
	if got := s.router.Path(); got != "/library/liked" {
		t.Errorf("Alt+3 went to %q", got)
	}
}

func TestSearchBox(t *testing.T) {
	s := newTestShell(t)
	s.loads["search:daft punk"] = &loaded[app.SearchResponse]{done: true}
	tt := tester(t, s)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("daft punk")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if got := s.router.Location(); got != "/search?q=daft+punk" {
		t.Errorf("searching went to %q", got)
	}
}

// TestSettingsKeep shows the settings without changing them: Spotify
// tracks played inside MixMusic, which the native interface's choices
// don't name, stay so.
func TestSettingsKeep(t *testing.T) {
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackHere
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt := tester(t, s)
	for _, path := range []string{"/settings/playback", "/settings/appearance", "/settings/desktop", "/settings/about"} {
		s.navigate(path)
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
	}
	if got := s.svc.Preferences.Get().SpotifyPlayback; got != settings.SpotifyPlaybackHere {
		t.Errorf("showing the settings made Spotify playback %q", got)
	}
	click(t, s, tt, "playback")
	click(t, s, tt, "in mixmusic, from youtube music")
	if got := s.svc.Preferences.Get().SpotifyPlayback; got != settings.SpotifyPlaybackMatch {
		t.Errorf("choosing YouTube Music made Spotify playback %q", got)
	}
}

func TestShortcutParsing(t *testing.T) {
	for in, ok := range map[string]bool{"Space": true, "Mod+Shift+L": true, "Alt+1": true, "Mod+,": true, "Shift+ArrowRight": true, "": false, "Hyper+K": false, "Mod+NoSuchKey": false} {
		if _, _, got := parseShortcut(in); got != ok {
			t.Errorf("parseShortcut(%q) = %v, want %v", in, got, ok)
		}
	}
	mods, key, _ := parseShortcut("Mod+Shift+L")
	if mods != ui.Cmd|ui.Shift || key != ui.KeyL {
		t.Errorf("Mod+Shift+L is %v %v", mods, key)
	}
}

// TestCrashReport turns down the offer to report a crash, which isn't made
// again.
func TestCrashReport(t *testing.T) {
	s := newTestShellOf(t, func(o *app.Options) { o.LastCrash = filepath.Join(o.LogsDir, "crash-20261021.log") })
	tt := tester(t, s)
	s.offerCrashReport()
	tt.Frame()
	if !tt.HasText("crash-20261021.log") {
		t.Fatalf("the offer doesn't name the report: %q", tt.Texts())
	}
	click(t, s, tt, "Cancel")
	if s.asking != nil || s.svc.System.LastCrash() != "" {
		t.Errorf("turning the offer down left it: asking %v, crash %q", s.asking != nil, s.svc.System.LastCrash())
	}
}

// TestQueueDrag drags an upcoming entry of the queue onto another, which
// takes it in its place.
func TestQueueDrag(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	from, ok := tt.Find(testTracks[4].Title)
	to, ok2 := tt.Find(testTracks[1].Title)
	if !ok || !ok2 {
		t.Fatalf("the queue doesn't list the tracks: %q", tt.Texts())
	}
	tt.Press(from.X+4, from.Y+4)
	for step := float32(1); step <= 8; step++ {
		tt.Move(from.X+4, from.Y+4+(to.Y-from.Y)*step/8)
		tt.Frame()
	}
	tt.Release(to.X+4, to.Y+4)
	tt.Frame()
	s.settle(t, func(st player.State) bool { return st.Queue[1].Track.ID == testTracks[4].ID })
	if got := s.p.State().Queue[2].Track.ID; got != testTracks[1].ID {
		t.Errorf("the entry it went before is now %s", got)
	}
}

// TestDayChart tells a day's minutes as the pointer comes on its column,
// and as the arrow keys go from day to day, and shows the days as a table,
// newest first.
func TestDayChart(t *testing.T) {
	s := newTestShell(t)
	for _, tr := range testTracks[:3] {
		if err := s.svc.Collection.RecordPlay(tr); err != nil {
			t.Fatal(err)
		}
	}
	days := s.svc.Collection.Stats(statsRanges[0].days).Daily
	n := len(days)
	tt := testerOf(t, s, 1280, 1600)
	show(s, tt, "/stats")
	plot, ok := tt.Find("Minutes listened each day")
	if !ok {
		t.Fatalf("no chart: %q", tt.Texts())
	}
	today := describeDay(days[n-1])
	tt.Move(plot.X+plot.W-4, plot.Y+plot.H/2)
	tt.Frame()
	if !tt.HasText(today) {
		t.Fatalf("the pointer on today's column doesn't tell it: %q", tt.Texts())
	}
	tt.Move(plot.X+plot.W/2, plot.Y-80)
	tt.Frame()
	if tt.HasText(today) {
		t.Error("the tip stays as the pointer leaves the chart")
	}
	tt.Click("Minutes listened each day")
	tt.Move(plot.X+plot.W/2, plot.Y-80)
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyLeft)
	yesterday := describeDay(days[n-2])
	if !tt.HasText(yesterday) || !slices.Contains(tt.Announcements(), yesterday) {
		t.Fatalf("Left after End doesn't tell yesterday %q: %q, read out %q", yesterday, tt.Texts(), tt.Announcements())
	}
	tt.Click("show as a table")
	tt.Advance(time.Second)
	texts := tt.Texts()
	first := slices.Index(texts, dayLabel(days[n-1].Date))
	second := slices.Index(texts, dayLabel(days[n-2].Date))
	if first < 0 || second < first {
		t.Errorf("the table doesn't list the days newest first: %q", texts)
	}
}

// TestShortcutRecorder records a command's shortcut from the keys pressed,
// which the command that had them gives up, removes one with Backspace,
// keeps one with Escape, and puts every command's own back.
func TestShortcutRecorder(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/shortcuts")
	tt.Click("change the shortcut of Mute")
	if s.recording != "player.mute" || !tt.Focused("cancel the shortcut of Mute") {
		t.Fatalf("change doesn't record the keys: recording %q", s.recording)
	}
	// The palette's keys name the shortcut, rather than open it.
	tt.Key(ui.Cmd, ui.KeyK)
	if s.paletteOpen || s.recording != "" || s.set.Shortcuts["player.mute"] != "Mod+K" {
		t.Fatalf("Mod+K while recording: palette %v, recording %q, shortcuts %v", s.paletteOpen, s.recording, s.set.Shortcuts)
	}
	if keys, ok := s.set.Shortcuts["view.palette"]; !ok || keys != "" || !tt.HasText("Command palette has no shortcut now") {
		t.Errorf("the palette keeps its keys: %q, %v", keys, ok)
	}
	tt.Click("change the shortcut of Next track")
	tt.Key(0, ui.KeyBackspace)
	if keys, ok := s.set.Shortcuts["player.next"]; !ok || keys != "" {
		t.Errorf("Backspace leaves the shortcut %q, %v", keys, ok)
	}
	tt.Click("change the shortcut of Volume up")
	tt.Key(0, ui.KeyEscape)
	if keys, ok := s.set.Shortcuts["player.volumeUp"]; ok || s.recording != "" {
		t.Errorf("Escape changes the shortcut to %q, or records still: %q", keys, s.recording)
	}
	tt.Click("change the shortcut of Mute")
	tt.Key(0, ui.KeyM)
	if keys, ok := s.set.Shortcuts["player.mute"]; ok {
		t.Errorf("the command's own keys stay the user's: %q", keys)
	}
	tt.Click("reset all")
	if len(s.set.Shortcuts) != 0 || s.shortcutOf(command{id: "view.palette", shortcut: "Mod+K"}) != "Mod+K" {
		t.Errorf("reset all leaves %v", s.set.Shortcuts)
	}
}

// TestShortcutName writes the keys pressed as shortcuts are written.
func TestShortcutName(t *testing.T) {
	for _, c := range []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
	}{
		{0, ui.KeySpace, "Space"},
		{ui.Cmd | ui.Shift, ui.KeyL, "Mod+Shift+L"},
		{ui.Alt, ui.Key1, "Alt+1"},
		{ui.Cmd, ui.KeyComma, "Mod+,"},
		{ui.Shift, ui.KeyRight, "Shift+ArrowRight"},
		{ui.Shift, ui.KeyUnknown, ""},
	} {
		if got := shortcutName(c.mods, c.key); got != c.want {
			t.Errorf("shortcutName(%v, %v) = %q, want %q", c.mods, c.key, got, c.want)
		}
		if c.want != "" && !sameShortcut(shortcutName(c.mods, c.key), c.want) {
			t.Errorf("%q isn't the same as itself", c.want)
		}
	}
	// Mod is Ctrl, or Cmd on macOS.
	written := "Ctrl+K"
	if runtime.GOOS == "darwin" {
		written = "Cmd+K"
	}
	if !sameShortcut("Mod+K", written) || sameShortcut("Mod+K", "Mod+J") {
		t.Errorf("sameShortcut tells Mod+K from %s, or not from Mod+J", written)
	}
}
