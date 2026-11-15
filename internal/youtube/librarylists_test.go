package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// fakeLibraryList is a list of the user's library on YouTube Music, a grid
// of pages pages of perPage cards, card(page, i) each: the browse ID asks
// for the first, and "P1", "P2" and on for the others. fails tells how
// many times the page fails before it's answered, with a 503; next, when
// set, is the token of the page after page, "" after the last; visitor,
// when set, is a page answered as to a visitor. It keeps the pages asked
// for, in order.
type fakeLibraryList struct {
	pages, perPage int
	card           func(page, i int) any
	fails          map[int]int
	next           func(page int) string
	visitor        int
	// asking, when set, is told each page asked for.
	asking func(page int)

	mu    sync.Mutex
	asked []int
}

func (f *fakeLibraryList) client(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Continuation string `json:"continuation"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		page := 0
		if body.Continuation != "" {
			page, _ = strconv.Atoi(strings.TrimPrefix(body.Continuation, "P"))
		}
		if f.asking != nil {
			f.asking(page)
		}
		f.mu.Lock()
		f.asked = append(f.asked, page)
		fail := f.fails[page] > 0
		if fail {
			f.fails[page]--
		}
		f.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": {"code": 503, "message": "The service is currently unavailable."}}`))
			return
		}
		json.NewEncoder(w).Encode(f.page(page))
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	c.SetCookies(session)
	return c
}

// page is the answer of the page.
func (f *fakeLibraryList) page(page int) map[string]any {
	items := []any{}
	for i := range f.perPage {
		items = append(items, f.card(page, i))
	}
	grid := map[string]any{"items": items}
	token := ""
	switch {
	case f.next != nil:
		token = f.next(page)
	case page+1 < f.pages:
		token = "P" + strconv.Itoa(page+1)
	}
	if token != "" {
		grid["continuations"] = next(token)
	}
	var answer map[string]any
	if page == 0 {
		answer = libraryPage(map[string]any{"gridRenderer": grid}).v.(map[string]any)
	} else {
		answer = map[string]any{"continuationContents": map[string]any{"gridContinuation": grid}}
	}
	if f.visitor > 0 && page == f.visitor {
		answer["responseContext"] = loggedIn("0")["responseContext"]
	}
	return answer
}

// askedFor returns the pages asked for, in order.
func (f *fakeLibraryList) askedFor() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.asked)
}

// browseCard is a card of the library that opens a page of pageType,
// browseID's.
func browseCard(pageType, browseID, title string) any {
	return map[string]any{"musicTwoRowItemRenderer": map[string]any{
		"title": map[string]any{"runs": []any{map[string]any{"text": title}}},
		"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": browseID,
			"browseEndpointContextSupportedConfigs": map[string]any{"browseEndpointContextMusicConfig": map[string]any{"pageType": pageType}}}},
	}}
}

// playlistCardAt is the card of the playlist "PL<page>-<i>".
func playlistCardAt(page, i int) any {
	id := "PL" + strconv.Itoa(page) + "-" + strconv.Itoa(i)
	return browseCard(pageTypePlaylist, "VL"+id, "Playlist "+id)
}

// playlistsOf are the IDs of the playlists of pages pages of perPage
// cards, in their order.
func playlistsOf(pages, perPage int) []string {
	var ids []string
	for page := range pages {
		for i := range perPage {
			ids = append(ids, "yt:PL"+strconv.Itoa(page)+"-"+strconv.Itoa(i))
		}
	}
	return ids
}

func playlistIDs(ps []media.Playlist) []string {
	var ids []string
	for _, p := range ps {
		ids = append(ids, p.ID)
	}
	return ids
}

// quickPages shortens the waits between the tries of a page, for a test.
func quickPages(t *testing.T) {
	t.Helper()
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
}

// TestLibraryListReadsEveryPage reads all 60 pages of a list of the
// library, which stopped at 40, each once, with no error.
func TestLibraryListReadsEveryPage(t *testing.T) {
	f := &fakeLibraryList{pages: 60, perPage: 2, card: playlistCardAt}
	got, err := f.client(t).LibraryPlaylists(context.Background())
	if err != nil || !slices.Equal(playlistIDs(got), playlistsOf(60, 2)) {
		t.Fatalf("LibraryPlaylists = %d playlists, %v", len(got), err)
	}
	if asked := f.askedFor(); len(asked) != 60 {
		t.Errorf("asked for %d pages", len(asked))
	}
}

// TestLibraryListTriesAPageAgain tries a page that fails again, and of a
// list whose fourth page of ten always fails, gives the three before it,
// with the error that tells it came in part, and why.
func TestLibraryListTriesAPageAgain(t *testing.T) {
	quickPages(t)
	f := &fakeLibraryList{pages: 10, perPage: 2, card: playlistCardAt, fails: map[int]int{3: 2}}
	got, err := f.client(t).LibraryPlaylists(context.Background())
	if err != nil || !slices.Equal(playlistIDs(got), playlistsOf(10, 2)) {
		t.Fatalf("a page that failed twice: %d playlists, %v", len(got), err)
	}

	f = &fakeLibraryList{pages: 10, perPage: 2, card: playlistCardAt, fails: map[int]int{3: 100}}
	got, err = f.client(t).LibraryPlaylists(context.Background())
	short, ok := media.Short(err)
	if !ok || short.Got != 6 || short.Total != 0 || short.Cut || !slices.Equal(playlistIDs(got), playlistsOf(3, 2)) {
		t.Fatalf("a page that always fails: %d playlists, %v", len(got), err)
	}
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusServiceUnavailable {
		t.Errorf("the error doesn't tell why: %v", err)
	}
	if asked := f.askedFor(); !slices.Equal(asked, []int{0, 1, 2, 3, 3, 3, 3}) {
		t.Errorf("asked for pages %v", asked)
	}
}

// TestLibraryListStopsAtARepeatedToken stops a list whose page names a
// page read before, as the next, and says it came in part; a page that
// names itself stops it too.
func TestLibraryListStopsAtARepeatedToken(t *testing.T) {
	for _, back := range []int{2, 4} {
		f := &fakeLibraryList{perPage: 2, card: playlistCardAt, next: func(page int) string {
			if page == 4 {
				return "P" + strconv.Itoa(back)
			}
			return "P" + strconv.Itoa(page+1)
		}}
		got, err := f.client(t).LibraryPlaylists(context.Background())
		short, ok := media.Short(err)
		if !ok || short.Cut || !errors.Is(err, errPagesGoRound) || !slices.Equal(playlistIDs(got), playlistsOf(5, 2)) {
			t.Errorf("back to page %d: %d playlists, %v", back, len(got), err)
		}
		if asked := f.askedFor(); !slices.Equal(asked, []int{0, 1, 2, 3, 4}) {
			t.Errorf("back to page %d: asked for pages %v", back, asked)
		}
	}
}

// TestLibraryListCut stops a list at maxLibraryPages, and says it was cut
// there; a list of as many pages, read to its end, isn't.
func TestLibraryListCut(t *testing.T) {
	bound := maxLibraryPages
	maxLibraryPages = 5
	t.Cleanup(func() { maxLibraryPages = bound })
	f := &fakeLibraryList{pages: 10, perPage: 2, card: playlistCardAt}
	got, err := f.client(t).LibraryPlaylists(context.Background())
	short, ok := media.Short(err)
	if !ok || !short.Cut || short.Got != 10 || !slices.Equal(playlistIDs(got), playlistsOf(5, 2)) {
		t.Fatalf("10 pages: %d playlists, %v", len(got), err)
	}
	if asked := f.askedFor(); len(asked) != 5 {
		t.Errorf("asked for pages %v", asked)
	}
	f = &fakeLibraryList{pages: 5, perPage: 2, card: playlistCardAt}
	if got, err := f.client(t).LibraryPlaylists(context.Background()); err != nil || len(got) != 10 {
		t.Errorf("5 pages: %d playlists, %v", len(got), err)
	}
}

// TestLibraryListOnce gives an item that comes again on a later page, as
// the list moves while it's read, once.
func TestLibraryListOnce(t *testing.T) {
	// Each page begins with the last card of the page before.
	moved := func(page, i int) any { return playlistCardAt(0, page*2+i) }
	f := &fakeLibraryList{pages: 3, perPage: 3, card: moved}
	got, err := f.client(t).LibraryPlaylists(context.Background())
	if want := playlistsOf(1, 7); err != nil || !slices.Equal(playlistIDs(got), want) {
		t.Errorf("LibraryPlaylists = %v, %v; want %v", playlistIDs(got), err, want)
	}
}

// TestLibraryListsInPart gives, of each list of the library whose second
// page always fails, what its first page holds of its kind, with a
// ShortList that counts them: liked songs aren't among the playlists.
func TestLibraryListsInPart(t *testing.T) {
	quickPages(t)
	kinds := map[string]struct {
		card func(page, i int) any
		read func(c *Client) (int, error)
	}{
		"playlists": {
			card: func(page, i int) any {
				if page == 0 && i == 0 {
					return browseCard(pageTypePlaylist, "VL"+likedSongs, "Liked Music")
				}
				return playlistCardAt(page, i)
			},
			read: func(c *Client) (int, error) {
				got, err := c.LibraryPlaylists(context.Background())
				return len(got), err
			},
		},
		"albums": {
			card: func(page, i int) any {
				return browseCard(pageTypeAlbum, "MPREb_"+strconv.Itoa(page)+"-"+strconv.Itoa(i), "Album")
			},
			read: func(c *Client) (int, error) {
				got, err := c.LibraryAlbums(context.Background())
				return len(got), err
			},
		},
		"artists": {
			card: func(page, i int) any {
				return browseCard(pageTypeArtist, "UC"+strconv.Itoa(page)+"-"+strconv.Itoa(i), "Artist")
			},
			read: func(c *Client) (int, error) {
				got, err := c.LibraryArtists(context.Background())
				return len(got), err
			},
		},
		"songs": {
			card: func(page, i int) any {
				return map[string]any{"musicTwoRowItemRenderer": map[string]any{
					"title":              map[string]any{"runs": []any{map[string]any{"text": "Song"}}},
					"navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{"videoId": "v" + strconv.Itoa(page) + "-" + strconv.Itoa(i)}},
				}}
			},
			read: func(c *Client) (int, error) {
				got, err := c.LibrarySongs(context.Background())
				return len(got), err
			},
		},
		"podcasts": {
			card: func(page, i int) any {
				return browseCard(pageTypePodcast, podcastPrefix+strconv.Itoa(page)+"-"+strconv.Itoa(i), "Show")
			},
			read: func(c *Client) (int, error) {
				got, err := c.LibraryPodcasts(context.Background())
				return len(got), err
			},
		},
	}
	for kind, k := range kinds {
		f := &fakeLibraryList{pages: 3, perPage: 3, card: k.card, fails: map[int]int{1: 100}}
		n, err := k.read(f.client(t))
		want := 3
		if kind == "playlists" {
			want = 2
		}
		if short, ok := media.Short(err); !ok || n != want || short.Got != want || short.Cut {
			t.Errorf("%s: %d came, %v", kind, n, err)
		}
	}
}

// TestLibraryListEndsWithItsRead stops reading the pages of a list once
// its read is dropped, without waiting to try a page again, with what
// came.
func TestLibraryListEndsWithItsRead(t *testing.T) {
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Hour}
	t.Cleanup(func() { media.PageWaits = waits })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeLibraryList{pages: 10, perPage: 2, card: playlistCardAt, fails: map[int]int{1: 1}, asking: func(page int) {
		if page == 1 {
			cancel()
		}
	}}
	start := time.Now()
	got, err := f.client(t).LibraryPlaylists(ctx)
	if s, ok := media.Short(err); !ok || s.Got != 2 || len(got) != 2 || time.Since(start) > time.Minute {
		t.Errorf("a read dropped: %d playlists, %v, after %v", len(got), err, time.Since(start))
	}
}

// TestLibraryListOfASessionEnded stops a list at a page YouTube answers
// as to a visitor, which ends the session, and leaves out what it holds.
func TestLibraryListOfASessionEnded(t *testing.T) {
	f := &fakeLibraryList{pages: 10, perPage: 2, card: playlistCardAt, visitor: 2}
	got, err := f.client(t).LibraryPlaylists(context.Background())
	if s, ok := media.Short(err); !ok || !errors.Is(err, ErrSessionRejected) || s.Got != 4 || !slices.Equal(playlistIDs(got), playlistsOf(2, 2)) {
		t.Errorf("a session ended: %d playlists, %v", len(got), err)
	}
	if asked := f.askedFor(); !slices.Equal(asked, []int{0, 1, 2}) {
		t.Errorf("asked for pages %v", asked)
	}
}
