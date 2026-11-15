package spotify

import (
	"context"
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

// fakeList is a list of the user's library on the Web API, of total items,
// whose JSON item writes, paged by offset or, with cursor, by the ID of
// the item before, as /me/following is. fails says how many times the
// page from an offset fails before it's answered; it counts the pages
// asked for and the most answered at once.
type fakeList struct {
	total  int
	item   func(i int) string
	cursor bool
	fails  map[int]int
	// ends, when set, is where the items end, though the count and the
	// pages' next go on, as of a service that goes wrong.
	ends int
	// asking, when set, is told the offset of each page asked for.
	asking func(offset int)

	mu        sync.Mutex
	asked     []string
	now, most int
}

func (f *fakeList) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	if f.cursor && q.Get("after") != "" {
		offset, _ = strconv.Atoi(strings.TrimPrefix(q.Get("after"), "a"))
		offset++
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if f.asking != nil {
		f.asking(offset)
	}
	f.mu.Lock()
	f.asked = append(f.asked, r.URL.RawQuery)
	f.now++
	f.most = max(f.most, f.now)
	fail := f.fails[offset] > 0
	if fail {
		f.fails[offset]--
	}
	f.mu.Unlock()
	// Pages overlap, as they do over the network.
	time.Sleep(2 * time.Millisecond)
	f.mu.Lock()
	f.now--
	f.mu.Unlock()
	if fail {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": {"status": 503, "message": "Service unavailable"}}`))
		return
	}
	var items []string
	for i := offset; i < min(offset+limit, f.total); i++ {
		if f.ends == 0 || i < f.ends {
			items = append(items, f.item(i))
		}
	}
	next, after := "null", ""
	if offset+limit < f.total {
		next = `"http://` + r.Host + r.URL.Path + "?offset=" + strconv.Itoa(offset+limit) + "&limit=" + strconv.Itoa(limit) + `"`
		after = "a" + strconv.Itoa(offset+limit-1)
	}
	page := `{"items": [` + strings.Join(items, ", ") + `], "total": ` + strconv.Itoa(f.total) + `, "next": ` + next +
		`, "cursors": {"after": "` + after + `"}}`
	if f.cursor {
		page = `{"artists": ` + page + `}`
	}
	w.Write([]byte(page))
}

// pages returns how many pages were asked for.
func (f *fakeList) pages() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

// atOnce returns the most pages answered at once.
func (f *fakeList) atOnce() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.most
}

func playlistAt(i int) string {
	return `{"id": "p` + strconv.Itoa(i) + `", "name": "Playlist ` + strconv.Itoa(i) + `", "owner": {"id": "me", "display_name": "Me"}, "items": {"total": 1}}`
}

func savedAlbumAt(i int) string {
	return `{"album": {"id": "al` + strconv.Itoa(i) + `", "name": "Album ` + strconv.Itoa(i) + `"}}`
}

func savedShowAt(i int) string {
	return `{"show": {"id": "s` + strconv.Itoa(i) + `", "name": "Show ` + strconv.Itoa(i) + `"}}`
}

func artistAt(i int) string {
	return `{"id": "a` + strconv.Itoa(i) + `", "name": "Artist ` + strconv.Itoa(i) + `"}`
}

// libraryServer serves the lists of lists by their paths.
func libraryServer(t *testing.T, lists map[string]*fakeList) *Client {
	t.Helper()
	mux := http.NewServeMux()
	for path, l := range lists {
		mux.Handle("GET "+path, l)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return testClient(t, srv)
}

// quickPages shortens the waits between the tries of a page, for a test.
func quickPages(t *testing.T) {
	t.Helper()
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
}

// TestHomeReadsOnePage shows on Home the first page of the user's 250
// playlists and saved albums, without waiting for the rest.
func TestHomeReadsOnePage(t *testing.T) {
	playlists := &fakeList{total: 250, item: playlistAt}
	albums := &fakeList{total: 250, item: savedAlbumAt}
	c := libraryServer(t, map[string]*fakeList{"/me/playlists": playlists, "/me/albums": albums})
	shelves, err := c.Home(context.Background(), "me")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, sh := range shelves {
		got[sh.Key] = len(sh.Items)
	}
	if got["playlists"] != pageLimit || got["albums"] != pageLimit || playlists.pages() != 1 || albums.pages() != 1 {
		t.Errorf("Home shows %v, of %d and %d pages asked for", got, playlists.pages(), albums.pages())
	}
}

// playlistIDs are the IDs of the playlists in their order.
func playlistIDs(ps []media.Playlist) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out
}

// inOrder reports whether ids are those of the first n items of a list,
// of prefix, each once.
func inOrder(ids []string, prefix string, n int) bool {
	if len(ids) != n {
		return false
	}
	for i, id := range ids {
		if id != prefix+strconv.Itoa(i) {
			return false
		}
	}
	return true
}

// TestMyPlaylistsReadsEveryPage reads all of the user's 250 playlists, in
// their order, each once, at most listPagesAtOnce pages at a time, and
// tries a page that fails again.
func TestMyPlaylistsReadsEveryPage(t *testing.T) {
	quickPages(t)
	list := &fakeList{total: 250, item: playlistAt, fails: map[int]int{100: 2}}
	c := libraryServer(t, map[string]*fakeList{"/me/playlists": list})
	got, err := c.MyPlaylists(context.Background(), "me")
	if err != nil || !inOrder(playlistIDs(got), "sp:p", 250) {
		t.Fatalf("MyPlaylists = %d playlists, %v", len(got), err)
	}
	if list.pages() != 5+2 || list.atOnce() > listPagesAtOnce {
		t.Errorf("%d pages asked for, at most %d at once", list.pages(), list.atOnce())
	}
}

// TestMyPlaylistsInPart gives the playlists that came of a list whose
// second page fails each time it's tried, with the error that tells how
// many of how many came.
func TestMyPlaylistsInPart(t *testing.T) {
	quickPages(t)
	list := &fakeList{total: 120, item: playlistAt, fails: map[int]int{50: 100}}
	c := libraryServer(t, map[string]*fakeList{"/me/playlists": list})
	got, err := c.MyPlaylists(context.Background(), "me")
	short, ok := media.Short(err)
	if !ok || short.Got != 50 || short.Total != 120 || short.Cut || !inOrder(playlistIDs(got), "sp:p", 50) {
		t.Fatalf("MyPlaylists = %d playlists, %v", len(got), err)
	}
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusServiceUnavailable {
		t.Errorf("the error doesn't tell why: %v", err)
	}

	// Its first page failing, nothing came.
	list.mu.Lock()
	list.fails = map[int]int{0: 1}
	list.mu.Unlock()
	if got, err := c.MyPlaylists(context.Background(), "me"); got != nil || err == nil {
		t.Errorf("a first page that fails gives %d playlists, %v", len(got), err)
	} else if _, ok := media.Short(err); ok {
		t.Errorf("nothing came, but the list is short: %v", err)
	}
}

// TestListStopsShort stops a list at a page of nothing, or of nothing new,
// that names a page after it, as at Spotify's last offset, and says so;
// an item that comes again as the list moves comes once.
func TestListStopsShort(t *testing.T) {
	quickPages(t)
	read := func(list *fakeList) ([]string, error) {
		t.Helper()
		c := libraryServer(t, map[string]*fakeList{"/me/playlists": list})
		got, err := c.MyPlaylists(context.Background(), "me")
		return playlistIDs(got), err
	}
	short := func(err error, got, total int, cut bool) bool {
		s, ok := media.Short(err)
		return ok && s.Got == got && s.Total == total && s.Cut == cut
	}

	if ids, err := read(&fakeList{total: 120, item: playlistAt, ends: 50}); !short(err, 50, 120, false) || !inOrder(ids, "sp:p", 50) {
		t.Errorf("a page of nothing: %d playlists, %v", len(ids), err)
	}
	again := func(i int) string { return playlistAt(i % pageLimit) }
	if ids, err := read(&fakeList{total: 250, item: again}); !short(err, 50, 250, false) || !inOrder(ids, "sp:p", 50) {
		t.Errorf("a page of what came before: %d playlists, %v", len(ids), err)
	}
	// A playlist added at the top as the list is read moves the others
	// one down: the last of a page comes again as the first of the next.
	moved := func(i int) string { return playlistAt(max(i-i/pageLimit, 0)) }
	if ids, err := read(&fakeList{total: 250, item: moved}); err != nil || !inOrder(ids, "sp:p", 246) {
		t.Errorf("a list that moved: %d playlists, %v", len(ids), err)
	}

	bound := maxListOffset
	maxListOffset = 100
	t.Cleanup(func() { maxListOffset = bound })
	if ids, err := read(&fakeList{total: 250, item: playlistAt}); !short(err, 150, 250, true) || !inOrder(ids, "sp:p", 150) {
		t.Errorf("past Spotify's last offset: %d playlists, %v", len(ids), err)
	}
}

// TestListEndsWithItsRead stops reading the pages of a list once the read
// is dropped, without waiting to try a page again.
func TestListEndsWithItsRead(t *testing.T) {
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Hour}
	t.Cleanup(func() { media.PageWaits = waits })
	ctx, cancel := context.WithCancel(context.Background())
	list := &fakeList{total: 250, item: playlistAt, fails: map[int]int{50: 1}, asking: func(offset int) {
		if offset == 50 {
			cancel()
		}
	}}
	c := libraryServer(t, map[string]*fakeList{"/me/playlists": list})
	start := time.Now()
	got, err := c.MyPlaylists(ctx, "me")
	if s, ok := media.Short(err); !ok || s.Got != 50 || len(got) != 50 || time.Since(start) > time.Minute {
		t.Errorf("a read dropped: %d playlists, %v, after %v", len(got), err, time.Since(start))
	}
}

// TestSavedAlbumsReadsEveryPage reads all of the user's 250 saved albums,
// and of 120 whose second page fails, the first 50, saying so.
func TestSavedAlbumsReadsEveryPage(t *testing.T) {
	quickPages(t)
	ids := func(albums []media.Album) []string {
		out := []string{}
		for _, al := range albums {
			out = append(out, al.ID)
		}
		return out
	}
	c := libraryServer(t, map[string]*fakeList{"/me/albums": {total: 250, item: savedAlbumAt}})
	got, err := c.SavedAlbums(context.Background())
	if err != nil || !inOrder(ids(got), "sp:al", 250) || got[0].Tracks == nil {
		t.Fatalf("SavedAlbums = %d albums, %v", len(got), err)
	}
	c = libraryServer(t, map[string]*fakeList{"/me/albums": {total: 120, item: savedAlbumAt, fails: map[int]int{50: 100}}})
	got, err = c.SavedAlbums(context.Background())
	if s, ok := media.Short(err); !ok || s.Got != 50 || s.Total != 120 || !inOrder(ids(got), "sp:al", 50) {
		t.Errorf("SavedAlbums of a page that fails = %d albums, %v", len(got), err)
	}
}

// TestSavedShowsReadsEveryPage reads all of the user's 250 saved shows,
// and of 120 whose second page fails, the first 50, saying so.
func TestSavedShowsReadsEveryPage(t *testing.T) {
	quickPages(t)
	ids := func(shows []media.Podcast) []string {
		out := []string{}
		for _, p := range shows {
			out = append(out, p.ID)
		}
		return out
	}
	c := libraryServer(t, map[string]*fakeList{"/me/shows": {total: 250, item: savedShowAt}})
	got, err := c.SavedShows(context.Background())
	if err != nil || !inOrder(ids(got), "sp:s", 250) {
		t.Fatalf("SavedShows = %d shows, %v", len(got), err)
	}
	c = libraryServer(t, map[string]*fakeList{"/me/shows": {total: 120, item: savedShowAt, fails: map[int]int{50: 100}}})
	got, err = c.SavedShows(context.Background())
	if s, ok := media.Short(err); !ok || s.Got != 50 || s.Total != 120 || !inOrder(ids(got), "sp:s", 50) {
		t.Errorf("SavedShows of a page that fails = %d shows, %v", len(got), err)
	}
}

// TestFollowedArtistsReadsEveryPage follows the cursor of each page of
// the user's 250 artists, a page at a time, trying a page that fails
// again, and of 120 whose second page fails, gives the first 50, saying
// so.
func TestFollowedArtistsReadsEveryPage(t *testing.T) {
	quickPages(t)
	ids := func(artists []media.Artist) []string {
		out := []string{}
		for _, a := range artists {
			out = append(out, a.ID)
		}
		return out
	}
	list := &fakeList{total: 250, item: artistAt, cursor: true, fails: map[int]int{100: 1}}
	c := libraryServer(t, map[string]*fakeList{"/me/following": list})
	got, err := c.FollowedArtists(context.Background())
	if err != nil || !inOrder(ids(got), "sp:a", 250) {
		t.Fatalf("FollowedArtists = %d artists, %v", len(got), err)
	}
	list.mu.Lock()
	asked := slices.Clone(list.asked)
	list.mu.Unlock()
	if len(asked) != 5+1 || list.atOnce() != 1 || !strings.Contains(asked[1], "after=a49") {
		t.Errorf("asked for %q, at most %d at once", asked, list.atOnce())
	}

	c = libraryServer(t, map[string]*fakeList{"/me/following": {total: 120, item: artistAt, cursor: true, fails: map[int]int{50: 100}}})
	got, err = c.FollowedArtists(context.Background())
	if s, ok := media.Short(err); !ok || s.Got != 50 || s.Total != 120 || !inOrder(ids(got), "sp:a", 50) {
		t.Errorf("FollowedArtists of a page that fails = %d artists, %v", len(got), err)
	}

	// A cursor that goes back to the first page ends the list there.
	again := func(i int) string { return artistAt(i % pageLimit) }
	c = libraryServer(t, map[string]*fakeList{"/me/following": {total: 120, item: again, cursor: true}})
	got, err = c.FollowedArtists(context.Background())
	if s, ok := media.Short(err); !ok || s.Got != 50 || !inOrder(ids(got), "sp:a", 50) {
		t.Errorf("FollowedArtists of a page of what came before = %d artists, %v", len(got), err)
	}
}
