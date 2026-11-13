package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// versionAlbum is an album of Ada Field's whose page holds one track,
// trackTitle, explicit or not; explicit is what its list says of it.
func versionAlbum(s *shell, source media.Source, id, title, year string, explicit bool, trackTitle string, trackExplicit bool) media.Album {
	a := media.Album{ID: id, Source: source, Title: title, Year: year, Kind: media.AlbumKindAlbum, TrackCount: 1,
		Artists: []media.Ref{{ID: "ar-ada", Name: "Ada Field"}}, Explicit: explicit}
	page := a
	page.Tracks = []media.Track{{ID: id + "-1", Source: source, Title: trackTitle, Explicit: trackExplicit,
		Artists: a.Artists, Album: &media.Ref{ID: id, Name: title}, DurationMs: 200000}}
	s.loads["album:"+id] = &loaded[media.Album]{done: true, value: page}
	return a
}

// kitesShell is a shell with an artist, Ada Field, whose album Kites
// YouTube Music lists explicit, clean and as its deluxe edition, beside
// another album.
func kitesShell(t *testing.T) *shell {
	s := newTestShell(t)
	yt := media.SourceYouTube
	albums := []media.Album{
		versionAlbum(s, yt, "yt:kites-c", "Kites", "2019", false, "Paper Moon (radio)", false),
		versionAlbum(s, yt, "yt:kites-e", "Kites", "2019", true, "Paper Moon", true),
		versionAlbum(s, yt, "yt:kites-d", "Kites (Deluxe Edition)", "2020", true, "Paper Moon (demo)", true),
		versionAlbum(s, yt, "yt:tide", "Tidewater", "2016", false, "Low Water", false),
	}
	s.loads["artist:ar-ada"] = &loaded[media.Artist]{done: true, value: media.Artist{ID: "ar-ada", Source: yt, Name: "Ada Field", Albums: albums}}
	return s
}

// tilesOf counts the tiles titled title: a tile shows its title twice, as
// its label and its text.
func tilesOf(tt *ui.Tester, title string) int { return textCount(tt, title) / 2 }

// shownAlbum waits for the album page to show its track, title.
func shownAlbum(t *testing.T, s *shell, tt *ui.Tester, title string) {
	t.Helper()
	until(t, s, tt, "the version with "+title, func() bool { return showsExactly(tt, title) })
}

// TestAlbumVersionsTile shows one tile for an album listed explicit, clean
// and as its deluxe edition, which opens on the explicit version, whose
// page switches to the others.
func TestAlbumVersionsTile(t *testing.T) {
	s := kitesShell(t)
	tt := tester(t, s)
	show(s, tt, artistPath("ar-ada"))
	if n := tilesOf(tt, "Kites"); n != 1 || !tt.HasText("2019 · 3 versions · Ada Field") || !tt.HasText("2016 · Ada Field") ||
		tt.HasText("2019 · Ada Field") || tt.HasText("2020 · Ada Field") {
		t.Fatalf("%d tiles of Kites; the page shows %q", n, tt.Texts())
	}
	click(t, s, tt, "Kites")
	shownAlbum(t, s, tt, "Paper Moon")
	if got := s.router.Path(); got != albumPath("yt:kites-e") {
		t.Errorf("the tile opened %s, not the explicit version", got)
	}
	for _, label := range []string{"explicit", "clean", "deluxe edition"} {
		if !tt.HasText(label) {
			t.Errorf("the switch has no %q: %q", label, tt.Texts())
		}
	}
	click(t, s, tt, "clean")
	shownAlbum(t, s, tt, "Paper Moon (radio)")
	click(t, s, tt, "deluxe edition")
	shownAlbum(t, s, tt, "Paper Moon (demo)")
	if got := s.router.Path(); got != albumPath("yt:kites-e") {
		t.Errorf("switching went to %s", got)
	}
	// Opened again from its tile, the album shows the explicit version.
	show(s, tt, artistPath("ar-ada"))
	click(t, s, tt, "Kites")
	shownAlbum(t, s, tt, "Paper Moon")
}

// TestAlbumVersionsPreferClean shows and opens the clean version of an
// album, where the settings prefer clean versions, and the switch still
// has the explicit one.
func TestAlbumVersionsPreferClean(t *testing.T) {
	s := kitesShell(t)
	s.set.CleanVersions = true
	tt := tester(t, s)
	show(s, tt, artistPath("ar-ada"))
	click(t, s, tt, "Kites")
	shownAlbum(t, s, tt, "Paper Moon (radio)")
	if got := s.router.Path(); got != albumPath("yt:kites-c") {
		t.Errorf("the tile opened %s, not the clean version", got)
	}
	click(t, s, tt, "explicit")
	shownAlbum(t, s, tt, "Paper Moon")
}

// TestAlbumVersionsUntold opens an album that Spotify lists twice, alike,
// on its explicit version, which the versions' pages tell by their tracks;
// its tile's Play plays that version too.
func TestAlbumVersionsUntold(t *testing.T) {
	s := newTestShell(t)
	sp := media.SourceSpotify
	albums := []media.Album{
		versionAlbum(s, sp, "sp:kites-c", "Kites", "2019", false, "Paper Moon (radio)", false),
		versionAlbum(s, sp, "sp:kites-e", "Kites", "2019", false, "Paper Moon", true),
	}
	s.loads["artist:sp:ada"] = &loaded[media.Artist]{done: true, value: media.Artist{ID: "sp:ada", Source: sp, Name: "Ada Field", Albums: albums}}
	tt := tester(t, s)
	show(s, tt, artistPath("sp:ada"))
	if n := tilesOf(tt, "Kites"); n != 1 || !tt.HasText("2019 · 2 versions · Ada Field") {
		t.Fatalf("%d tiles of Kites; the page shows %q", n, tt.Texts())
	}
	click(t, s, tt, "Kites")
	shownAlbum(t, s, tt, "Paper Moon")
	if !tt.HasText("explicit") || !tt.HasText("clean") {
		t.Errorf("the switch: %q", tt.Texts())
	}
	show(s, tt, artistPath("sp:ada"))
	if err := tt.RightClick("Kites"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Play"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == "sp:kites-e-1"
	})
}

// TestAlbumVersionsSearchAndLibrary shows one tile for an album with
// versions among the albums a search found, whose Albums pivot counts it
// once, and in the library.
func TestAlbumVersionsSearchAndLibrary(t *testing.T) {
	s := kitesShell(t)
	artist := s.loads["artist:ar-ada"].(*loaded[media.Artist]).value
	other := media.Album{ID: "yt:lights-kites", Source: media.SourceYouTube, Title: "Kites", Year: "2021",
		Artists: []media.Ref{{Name: "Harbour Lights"}}}
	found := append(artist.Albums, other)
	s.loads["search:kites"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{NetEase: media.SearchResults{Albums: found}}}
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true}}}
	s.loads["youtube:playlists"] = &loaded[[]media.Playlist]{done: true}
	s.loads["youtube:albums"] = &loaded[[]media.Album]{done: true, value: artist.Albums}
	s.loads["youtube:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["youtube:liked"] = &loaded[media.Playlist]{done: true}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, searchPath("kites"))
	if n := tilesOf(tt, "Kites"); n != 2 || !tt.HasText("2019 · 3 versions · Ada Field") {
		t.Errorf("the search shows %d tiles of Kites: %q", n, tt.Texts())
	}
	if got := len(s.albumGroups("album", found)); got != 3 {
		t.Errorf("the Albums pivot counts %d albums", got)
	}
	click(t, s, tt, "albums")
	if n := tilesOf(tt, "Kites"); n != 2 {
		t.Errorf("the Albums pivot shows %d tiles of Kites: %q", n, tt.Texts())
	}
}

// TestCleanVersionsSetting turns on Prefer clean versions in Settings ›
// Playback.
func TestCleanVersionsSetting(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	const label = "Prefer clean versions"
	if s.svc.Preferences.Get().CleanVersions {
		t.Fatal("on at first")
	}
	const description = "An album that comes explicit and clean shows its clean version, and its page opens on it. Its version switch still has the explicit one."
	focused := func() bool { return tt.Focused(label) && !tt.Focused(description) }
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	until(t, s, tt, "Prefer clean versions saved", func() bool { return s.set.CleanVersions && s.svc.Preferences.Get().CleanVersions })
}
