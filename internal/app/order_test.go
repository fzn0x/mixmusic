package app

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// mixTitles returns the titles of playlists, in their order.
func mixTitles(ps []media.Playlist) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Title)
	}
	return out
}

// TestMixesListAsSorted lists the mixes everywhere as the Library sorts
// them, the pinned ones first: to the window, to Home's shelf of them,
// and to the assistants.
func TestMixesListAsSorted(t *testing.T) {
	a := newQuietApp(t, nil)
	c := &Collection{a}
	for _, title := range []string{"Banjo", "apples", "Cello"} {
		if _, err := c.CreatePlaylist(title, "", nil); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := mixTitles(c.Playlists()), []string{"Cello", "apples", "Banjo"}; !slices.Equal(got, want) {
		t.Fatalf("in their own order: %q, want %q", got, want)
	}
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.MixesSort = settings.MixesByTitle }); err != nil {
		t.Fatal(err)
	}
	if got, want := mixTitles(c.Playlists()), []string{"apples", "Banjo", "Cello"}; !slices.Equal(got, want) {
		t.Errorf("by title: %q, want %q", got, want)
	}
	cello := c.Playlists()[2]
	if err := c.SetPinned(library.Mark{ID: cello.ID, Kind: library.MarkPlaylist, Title: cello.Title}, true); err != nil {
		t.Fatal(err)
	}
	want := []string{"Cello", "apples", "Banjo"}
	if got := mixTitles(c.Playlists()); !slices.Equal(got, want) {
		t.Errorf("Cello pinned: %q, want %q", got, want)
	}
	shelves := (&Catalog{a}).LocalShelves()
	if len(shelves) == 0 {
		t.Fatal("Home has no shelf of the mixes")
	}
	var shelf []string
	for _, it := range shelves[len(shelves)-1].Items {
		shelf = append(shelf, it.Playlist.Title)
	}
	if !slices.Equal(shelf, want) {
		t.Errorf("Home's shelf: %q, want %q", shelf, want)
	}
	got, _ := appCatalog{a}.Playlists(context.Background())
	if titles := mixTitles(got); !slices.Equal(titles, want) {
		t.Errorf("the assistants' playlists: %q, want %q", titles, want)
	}
}

// TestMoveMixes moves a mix in the user's own order, by the IDs the
// window has, among those a search shows.
func TestMoveMixes(t *testing.T) {
	a := newQuietApp(t, nil)
	c := &Collection{a}
	ids := map[string]string{}
	for _, title := range []string{"C", "B", "A"} {
		p, err := c.CreatePlaylist(title, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		ids[title] = p.ID
	}
	if err := c.MovePlaylist(ids["A"], ids["C"], nil); err != nil {
		t.Fatal(err)
	}
	if got, want := mixTitles(c.Playlists()), []string{"B", "C", "A"}; !slices.Equal(got, want) {
		t.Errorf("A onto C: %q, want %q", got, want)
	}
	if err := c.MovePlaylist(ids["A"], ids["B"], []string{ids["A"], ids["B"]}); err != nil {
		t.Fatal(err)
	}
	if got, want := mixTitles(c.Playlists()), []string{"A", "C", "B"}; !slices.Equal(got, want) {
		t.Errorf("A onto B, among them: %q, want %q", got, want)
	}
	if err := c.MovePlaylist("yt:PL1", ids["B"], nil); err == nil {
		t.Error("a playlist of YouTube Music moved among the mixes")
	}
}

// TestPinsFollowWholeLists drops the pin of a playlist of YouTube Music
// once a whole list of its account lacks it, and only then: not while the
// list comes in part, nor while another channel is chosen.
func TestPinsFollowWholeLists(t *testing.T) {
	quickPageWaits(t)
	var partial atomic.Bool
	kept := youtubeCard("MUSIC_PAGE_TYPE_PLAYLIST", "VLPLkept", "Kept")
	yt := partialYouTube(t, &partial, []any{kept}, []any{kept})
	a := newQuietApp(t, nil)
	a.youtube.BaseURL = yt.URL
	a.youtube.SetCookies(testCookies)
	a.mu.Lock()
	a.youtubeMe = &youtube.Account{Name: "Ana", Handle: "@ana"}
	a.mu.Unlock()
	c := &Collection{a}
	for _, id := range []string{"yt:PLkept", "yt:PLgone"} {
		if err := c.SetPinned(library.Mark{ID: id, Kind: library.MarkPlaylist, Title: id}, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.SetHidden(library.Mark{ID: "yt:PLhidden", Kind: library.MarkPlaylist}, true); err != nil {
		t.Fatal(err)
	}
	pinned := func() []string {
		var out []string
		for _, m := range c.Pins() {
			out = append(out, m.ID)
		}
		return out
	}

	partial.Store(true)
	if _, err := c.YouTubePlaylists(context.Background()); err == nil {
		t.Fatal("the partial list came whole")
	}
	if got := pinned(); len(got) != 2 || !c.IsHidden("yt:PLhidden") {
		t.Errorf("after a partial list, the pins are %q", got)
	}

	// Another channel's whole list drops none of this one's.
	a.youtube.SetChannel("UCbrand")
	partial.Store(false)
	if _, err := c.YouTubePlaylists(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := pinned(); len(got) != 2 {
		t.Errorf("after another channel's list, the pins are %q", got)
	}

	a.youtube.SetChannel("")
	if _, err := c.YouTubePlaylists(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := pinned(); !slices.Equal(got, []string{"yt:PLkept"}) || c.IsHidden("yt:PLhidden") {
		t.Errorf("after a whole list, the pins are %q, hidden %+v", got, c.Hidden())
	}
	if m := c.Pins()[0]; m.Title != "Kept" {
		t.Errorf("the pin is titled %q, not as the list has it", m.Title)
	}
}

// TestUnfollowedArtistLosesItsPin unpins an artist the user stops
// following in MixMusic while signed out of their service, and keeps the
// pin of one whose service may follow them still.
func TestUnfollowedArtistLosesItsPin(t *testing.T) {
	a := newQuietApp(t, nil)
	c := &Collection{a}
	sp := media.Artist{ID: "sp:artist1", Source: media.SourceSpotify, Name: "Tide"}
	yt := media.Artist{ID: "yt:UCharbour", Source: media.SourceYouTube, Name: "Harbour"}
	a.youtube.SetCookies(testCookies)
	for _, ar := range []media.Artist{sp, yt} {
		if err := c.SetArtistFollowed(ar, true); err != nil {
			t.Fatal(err)
		}
		if err := c.SetPinned(library.Mark{ID: ar.ID, Kind: library.MarkArtist, Title: ar.Name}, true); err != nil {
			t.Fatal(err)
		}
		if err := c.SetArtistFollowed(ar, false); err != nil {
			t.Fatal(err)
		}
	}
	if c.PinAt(sp.ID) >= 0 {
		t.Error("an artist of Spotify, signed out, kept their pin")
	}
	if c.PinAt(yt.ID) < 0 {
		t.Error("an artist of YouTube Music, signed in, lost their pin before its list")
	}
}
