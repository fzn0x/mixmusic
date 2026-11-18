package main

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// The song the links of these tests name, and its radio.
var (
	linkedSong  = testTrack("yt:dQw4w9WgXcQ", "Glass Harbor", "North Pier", "Paper Lanterns", media.SourceYouTube)
	linkedRadio = []media.Track{
		testTrack("yt:radioTrack1", "Tidewater", "North Pier", "Paper Lanterns", media.SourceYouTube),
		testTrack("yt:radioTrack2", "Breakwater", "Harbour Lights", "Breakwater", media.SourceYouTube),
	}
)

// readsLink has the catalog read link as v, or fail with err, as the
// tests' window would ask it.
func readsLink(t *testing.T, s *shell, link string, v app.Linked, err error) media.Link {
	t.Helper()
	l, ok := media.ParseLink(link)
	if !ok {
		t.Fatalf("%q is no link", link)
	}
	s.loads["link:"+l.URL()] = &loaded[app.Linked]{done: true, value: v, err: err}
	return l
}

// pasteInSearch pastes text in the search box, and waits as the box waits
// for the typing to pause.
func pasteInSearch(t *testing.T, s *shell, tt *ui.Tester, text string) {
	t.Helper()
	s.query = ""
	tt.Frame()
	if err := tt.Click(locales.TopbarSearch()); err != nil {
		t.Fatal(err)
	}
	tt.SetClipboard(text)
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Advance(time.Second)
	if s.query != text {
		t.Fatalf("the search box holds %q", s.query)
	}
}

// TestPasteSongLink pastes a song's link in the search box: the song is
// the only suggestion, as YouTube Music's suggestions show it, and Enter
// plays it at once, as a suggested song plays. Nothing is searched, nor
// kept among the recent searches.
func TestPasteSongLink(t *testing.T) {
	s := newTestShell(t)
	song := linkedSong
	const link = "https://youtu.be/dQw4w9WgXcQ?si=AbCdEfGh"
	readsLink(t, s, link, app.Linked{Item: media.Item{Kind: media.ItemTrack, Track: &song}}, nil)
	tt := tester(t, s)
	pasteInSearch(t, s, tt, link)

	texts := tt.Texts()
	if !slices.Contains(texts, "Glass Harbor") || !slices.Contains(texts, "song · North Pier") {
		t.Fatalf("the suggestions lack the song: %q", texts)
	}
	// The input holds the link; no row offers to search it.
	if n := len(slices.DeleteFunc(slices.Clone(texts), func(txt string) bool { return txt != link })); n > 1 {
		t.Errorf("the link shows %d times, as a search too: %q", n, texts)
	}
	if _, ok := tt.Find("Glass Harbor, song · North Pier"); !ok {
		t.Errorf("the song's row isn't named for what it is: %q", texts)
	}
	if _, asked := s.loads["suggest:"+link]; asked {
		t.Error("YouTube Music was asked for suggestions of the link")
	}

	tt.Key(0, ui.KeyEnter)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == song.ID
	})
	if got := s.router.Location(); strings.HasPrefix(got, "/search") {
		t.Errorf("the link was searched: %q", got)
	}
	if got := s.svc.Collection.Searches(); len(got) > 0 {
		t.Errorf("the recent searches keep %q", got)
	}
}

// TestPasteRadioLink pastes the link of a YouTube radio: its row says it is
// one, and choosing it plays the song with that radio after it, as the
// radio's entries.
func TestPasteRadioLink(t *testing.T) {
	s := newTestShell(t)
	song := linkedSong
	const link = "https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=RDEMhe2sl9h4F3oBTwCDoU8Q3w&si=x"
	readsLink(t, s, link, app.Linked{Item: media.Item{Kind: media.ItemTrack, Track: &song}, Radio: linkedRadio}, nil)
	tt := tester(t, s)
	pasteInSearch(t, s, tt, link)
	if err := tt.Click("Glass Harbor, radio · North Pier"); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 3 })
	st := s.p.State()
	if cur, ok := st.Current(); !ok || cur.Track.ID != song.ID || cur.Radio {
		t.Fatalf("plays %+v", cur)
	}
	for i, it := range st.Queue[1:] {
		if it.Track.ID != linkedRadio[i].ID || !it.Radio {
			t.Errorf("entry %d is %s, radio %v", i+1, it.Track.ID, it.Radio)
		}
	}
}

// TestPaletteOpensLinks offers Open link in place of Search for in the
// command palette, for a link it opens, and opens the page of what each
// link names: an album, a playlist, Spotify's own playlist too, an artist,
// and the podcast of a show and of an episode.
func TestPaletteOpensLinks(t *testing.T) {
	s := newTestShell(t)
	album := media.Album{ID: "yt:MPREb_lanterns", Source: media.SourceYouTube, Title: "Paper Lanterns", Tracks: testTracks[:2]}
	list := media.Playlist{ID: "yt:PLrainy", Source: media.SourceYouTube, Title: "Rainy Day Mix", Tracks: testTracks[:2]}
	own := media.Playlist{ID: "sp:37i9dQZF1DXcBWIGoYBM5M", Source: media.SourceSpotify}
	artist := media.Artist{ID: "sp:4tZwfgrHOc3mvqYlEYSvVi", Source: media.SourceSpotify, Name: "North Pier"}
	show := media.Podcast{ID: "sp:2MAi0BvDc6GTFvKFPXnkCL", Source: media.SourceSpotify, Title: "The Daily Thing"}
	s.loads["album:"+album.ID] = &loaded[media.Album]{done: true, value: album}
	s.loads["playlist:"+list.ID] = &loaded[media.Playlist]{done: true, value: list}
	s.loads["artist:"+artist.ID] = &loaded[media.Artist]{done: true, value: artist}
	s.loads["podcast:"+show.ID] = &loaded[media.Podcast]{done: true, value: show}
	for link, want := range map[string]struct {
		it   media.Item
		path string
	}{
		"https://music.youtube.com/browse/MPREb_lanterns?si=x":         {media.Item{Kind: media.ItemAlbum, Album: &album}, albumPath(album.ID)},
		"https://www.youtube.com/playlist?list=PLrainy":                {media.Item{Kind: media.ItemPlaylist, Playlist: &list}, playlistPath(list.ID)},
		"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M":     {media.Item{Kind: media.ItemPlaylist, Playlist: &own}, playlistPath(own.ID)},
		"spotify:artist:4tZwfgrHOc3mvqYlEYSvVi":                        {media.Item{Kind: media.ItemArtist, Artist: &artist}, artistPath(artist.ID)},
		"https://open.spotify.com/intl-de/show/2MAi0BvDc6GTFvKFPXnkCL": {media.Item{Kind: media.ItemPodcast, Podcast: &show}, podcastPath(show.ID)},
		"https://open.spotify.com/episode/512ojhOuo1ktJprKbVcKyQ?si=x": {media.Item{Kind: media.ItemPodcast, Podcast: &show}, podcastPath(show.ID)},
	} {
		readsLink(t, s, link, app.Linked{Item: want.it}, nil)
		s.navigate("/")
		tt := tester(t, s)
		tt.Key(ui.Cmd, ui.KeyK)
		tt.Frame()
		tt.SetClipboard(link)
		tt.Key(ui.Cmd, ui.KeyV)
		tt.Frame()
		if !tt.HasText(locales.CmdOpenLink()) || tt.HasText(locales.CmdSearchFor(link)) {
			t.Fatalf("for %s, the palette shows %q", link, tt.Texts())
		}
		tt.Key(0, ui.KeyEnter)
		s.drain()
		tt.Frame()
		if got := s.router.Location(); got != want.path {
			t.Errorf("%s went to %q, want %q", link, got, want.path)
		}
	}

	// What isn't a link is searched, as before.
	s.navigate("/")
	tt := tester(t, s)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	tt.Type("north pier")
	tt.Frame()
	if !tt.HasText(locales.CmdSearchFor("north pier")) || tt.HasText(locales.CmdOpenLink()) {
		t.Errorf("for a search, the palette shows %q", tt.Texts())
	}
}

// TestLinkThatFails says why a link didn't open, and searches nothing; the
// next try reads it again. Signed out of Spotify, a Spotify link asks the
// user to sign in, with a way to Settings › Accounts.
func TestLinkThatFails(t *testing.T) {
	s := newTestShell(t)
	const link = "https://music.youtube.com/watch?v=dQw4w9WgXcQ"
	l := readsLink(t, s, link, app.Linked{}, youtube.ErrNotFound)
	tt := tester(t, s)
	pasteInSearch(t, s, tt, link)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if want := locales.TopbarLinkCantOpen(app.ErrorText(youtube.ErrNotFound)); !tt.HasText(want) {
		t.Errorf("no toast says %q: %q", want, tt.Texts())
	}
	if got := s.router.Location(); got != "/" {
		t.Errorf("the link went to %q", got)
	}
	if _, kept := s.loads["link:"+l.URL()]; kept {
		t.Error("the link isn't read again")
	}
	if _, cur := s.p.State().Current(); cur {
		t.Error("something plays")
	}

	// The app reads it: the tests' app isn't signed in to Spotify. Once
	// read, the toast asks to sign in, with a way to Settings › Accounts.
	spotifyLink := "https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC?si=x"
	pasteInSearch(t, s, tt, spotifyLink)
	tt.Key(0, ui.KeyEnter)
	waitLink(t, s, spotifyLink)
	s.followLink()
	if n := len(s.actionToasts); n != 1 {
		t.Fatalf("%d toasts with a button", n)
	}
	toast := s.actionToasts[0]
	if toast.message != locales.TopbarLinkSpotifySignIn() || toast.action != locales.ShellToastSettings() {
		t.Errorf("the toast says %q, its button %q", toast.message, toast.action)
	}
	tt.Frame()
	if !tt.HasText(toast.message) {
		t.Errorf("no toast shows: %q", tt.Texts())
	}
	toast.run()
	if got := s.router.Location(); got != "/settings/accounts" {
		t.Errorf("the toast's button went to %q", got)
	}
}

// waitLink waits until the catalog read link, without a frame, which
// would act on it.
func waitLink(t *testing.T, s *shell, link string) {
	t.Helper()
	l, _ := media.ParseLink(link)
	for deadline := time.Now().Add(5 * time.Second); ; {
		s.drain()
		if res, ok := s.loads["link:"+l.URL()].(*loaded[app.Linked]); ok && res.done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s isn't read", link)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestPseudoLocaleLinks shows the words of a pasted link in en-XA: a
// radio's row, Open link and the toasts of a link that didn't open.
func TestPseudoLocaleLinks(t *testing.T) {
	usePseudoLocale(t)
	s := newTestShell(t)
	song := linkedSong
	const link = "https://youtu.be/dQw4w9WgXcQ?list=RDEMhe2sl9h4F3oBTwCDoU8Q3w"
	readsLink(t, s, link, app.Linked{Item: media.Item{Kind: media.ItemTrack, Track: &song}, Radio: linkedRadio}, nil)
	const failing = "https://music.youtube.com/channel/UCZh6nYJx1Tq_0yW7RWo8BVg"
	readsLink(t, s, failing, app.Linked{}, errors.New("dial tcp: refused"))
	tt := tester(t, s)
	tt.SetLocale("en-XA")
	pasteInSearch(t, s, tt, link)
	xa := locales.In("en-XA")
	if !slices.ContainsFunc(tt.Texts(), func(txt string) bool { return strings.HasPrefix(txt, s.lk.words(xa.TopbarKindRadio())+" · ") }) {
		t.Errorf("the radio's row doesn't say what it is: %q", tt.Texts())
	}
	checkPseudo(t, "A radio link's suggestion", tt.Texts(), "Glass Harbor", "North Pier", link)

	s.paletteOpen = true
	s.paletteQuery = link
	tt.Frame()
	if !tt.HasText(xa.CmdOpenLink()) {
		t.Errorf("the palette doesn't offer to open the link: %q", tt.Texts())
	}
	s.paletteOpen = false
	tt.Frame()

	pasteInSearch(t, s, tt, failing)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !tt.HasText(xa.TopbarLinkCantOpen("dial tcp: refused")) {
		t.Errorf("no toast says why the link didn't open: %q", tt.Texts())
	}
	pasteInSearch(t, s, tt, "spotify:album:4m2880jivSbbyEGAKfITCa")
	tt.Key(0, ui.KeyEnter)
	waitLink(t, s, "spotify:album:4m2880jivSbbyEGAKfITCa")
	tt.Frame()
	if !tt.HasText(xa.TopbarLinkSpotifySignIn()) {
		t.Errorf("no toast asks to sign in: %q", tt.Texts())
	}
	checkPseudo(t, "The toasts of links", tt.Texts(), "Glass Harbor", "North Pier", "dial tcp: refused", failing, "spotify:album:4m2880jivSbbyEGAKfITCa")
}
