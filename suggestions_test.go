package main

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestSuggestedItems shows, below the searches the box suggests, the
// songs, albums, artists and playlists YouTube Music suggests, saying
// what each is and by whom, in the words of the layout (Tiles's are
// lowercase). The arrows reach them past the searches: a
// song chosen plays at once, and the others open their page.
func TestSuggestedItems(t *testing.T) {
	s := newTestShell(t)
	daft := []media.Ref{{ID: "yt:UCdaft", Name: "Daft Punk"}}
	song := media.Track{ID: "yt:4D7u5KF7SP8", Source: media.SourceYouTube, Title: "Get Lucky", Artists: daft, DurationMs: 248000}
	s.loads["suggest:daft"] = &loaded[media.Suggestions]{done: true, value: media.Suggestions{
		Searches: []string{"daft punk"},
		Items: []media.Item{
			{Kind: media.ItemArtist, Artist: &media.Artist{ID: "yt:UCdaft", Source: media.SourceYouTube, Name: "Daft Punk"}},
			{Kind: media.ItemTrack, Track: &song},
			{Kind: media.ItemAlbum, Album: &media.Album{ID: "yt:MPREb_ram", Source: media.SourceYouTube, Title: "Random Access Memories", Artists: daft, Kind: media.AlbumKindAlbum}},
			{Kind: media.ItemPlaylist, Playlist: &media.Playlist{ID: "yt:PLchill", Source: media.SourceYouTube, Title: "Chill Bangers", Owner: "YouTube Music"}},
		},
	}}
	s.loads["album:yt:MPREb_ram"] = &loaded[media.Album]{done: true, value: media.Album{ID: "yt:MPREb_ram", Source: media.SourceYouTube, Title: "Random Access Memories"}}
	s.loads["playlist:yt:PLchill"] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: "yt:PLchill", Source: media.SourceYouTube, Title: "Chill Bangers"}}
	s.loads["artist:yt:UCdaft"] = &loaded[media.Artist]{done: true, value: media.Artist{ID: "yt:UCdaft", Source: media.SourceYouTube, Name: "Daft Punk"}}
	tt := tester(t, s)
	open := func() []string {
		t.Helper()
		s.query = ""
		tt.Frame()
		if err := tt.Click("Search"); err != nil {
			t.Fatal(err)
		}
		tt.Type("daft")
		tt.Advance(time.Second)
		return tt.Texts()
	}

	texts := open()
	for _, want := range []string{"daft punk", "artist", "Get Lucky", "song · Daft Punk", "Random Access Memories", "album · Daft Punk", "Chill Bangers", "playlist · YouTube Music"} {
		if !slices.Contains(texts, want) {
			t.Fatalf("the suggestions lack %q: %q", want, texts)
		}
	}
	if slices.Index(texts, "Get Lucky") < slices.Index(texts, "daft punk") {
		t.Errorf("the items come before the searches: %q", texts)
	}

	// From daft, highlighted as typed: daft punk, the artist, the song.
	for range 3 {
		tt.Key(0, ui.KeyDown)
	}
	tt.Key(0, ui.KeyEnter)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == song.ID
	})
	if got := s.router.Location(); got == searchPath("daft") {
		t.Errorf("choosing the song searched as well")
	}

	for label, want := range map[string]string{
		"Random Access Memories, album · Daft Punk": albumPath("yt:MPREb_ram"),
		"Chill Bangers, playlist · YouTube Music":   playlistPath("yt:PLchill"),
		"Daft Punk, artist":                         artistPath("yt:UCdaft"),
	} {
		open()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		if got := s.router.Location(); got != want {
			t.Errorf("a click on %s went to %q", label, got)
		}
	}
}

// TestSuggestedItemsFit has the suggestions scroll in a window too short
// for them all, rather than run past its bottom, and a long title end in
// an ellipsis, rather than widen them past the search box.
func TestSuggestedItemsFit(t *testing.T) {
	s := newTestShell(t)
	var items []media.Item
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		items = append(items, media.Item{Kind: media.ItemTrack, Track: &media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: "Song " + id}})
	}
	long := "Get Lucky (feat. Pharrell Williams and Nile Rodgers) [Radio Edit, Remastered]"
	items[0].Track.Title = long
	s.loads["suggest:daft"] = &loaded[media.Suggestions]{done: true, value: media.Suggestions{
		Searches: []string{"daft 1", "daft 2", "daft 3", "daft 4", "daft 5", "daft 6", "daft 7"}, Items: items,
	}}
	tt := testerOf(t, s, 920, 480)
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	tt.Type("daft")
	tt.Advance(time.Second)
	// The box holds the input 38 from its left (padding, icon and gap) and
	// 12 from its right; the popup is as wide, or 360 in a narrower box.
	in, _ := tt.Find("Search")
	if row, ok := tt.Find(long); !ok || row.X+row.W > in.X-38+max(in.W+50, 360) {
		t.Errorf("the long title shows at %v, the box's input at %v", row, in)
	}
	last, ok := tt.Find("Song e")
	if !ok {
		t.Fatalf("the suggestions are %q", tt.Texts())
	}
	const h = 480
	if last.Y+last.H <= h {
		t.Fatalf("the last song shows at %v, so the window had room for all", last)
	}
	for range 13 {
		tt.Key(0, ui.KeyDown)
	}
	if last, _ := tt.Find("Song e"); last.Y+last.H > h {
		t.Errorf("the last song, reached by the arrows, stays below the window, at %v", last)
	}
}
