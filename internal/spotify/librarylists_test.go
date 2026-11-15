package spotify

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	// next, when set, writes the next page's address in place of the
	// real one, as a service that goes wrong might.
	next func(offset int) string

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
		items = append(items, f.item(i))
	}
	next, after := "null", ""
	if offset+limit < f.total {
		next = `"http://` + r.Host + r.URL.Path + "?offset=" + strconv.Itoa(offset+limit) + "&limit=" + strconv.Itoa(limit) + `"`
		after = "a" + strconv.Itoa(offset+limit-1)
	}
	if f.next != nil {
		next = f.next(offset)
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
