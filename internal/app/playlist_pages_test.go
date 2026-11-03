package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// longPlaylist answers YouTube Music's requests for a playlist with the
// recorded one, cut into pages of two tracks: the first page, then those
// of the tokens P1 and P2, the last. fail says how many times the page of
// a token fails before it's answered.
type longPlaylist struct {
	mu    sync.Mutex
	pages map[string]any
	fail  map[string]int
	asked []string
}

func newLongPlaylist(t *testing.T) *longPlaylist {
	t.Helper()
	data, err := os.ReadFile("../youtube/testdata/playlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var first map[string]any
	if err := json.Unmarshal(data, &first); err != nil {
		t.Fatal(err)
	}
	shelf := dig(first, "contents", "twoColumnBrowseResultsRenderer", "secondaryContents", "sectionListRenderer", "contents", 0,
		"musicPlaylistShelfRenderer").(map[string]any)
	items := shelf["contents"].([]any)
	if len(items) < 6 {
		t.Fatalf("the recorded playlist has %d tracks", len(items))
	}
	more := func(token string) any {
		return map[string]any{"continuationItemRenderer": map[string]any{"continuationEndpoint": map[string]any{
			"continuationCommand": map[string]any{"token": token}}}}
	}
	page := func(tracks []any, next string) any {
		if next != "" {
			tracks = append(slices.Clone(tracks), more(next))
		}
		return map[string]any{"onResponseReceivedActions": []any{map[string]any{"appendContinuationItemsAction": map[string]any{
			"continuationItems": tracks}}}}
	}
	shelf["contents"] = append(slices.Clone(items[:2]), more("P1"))
	return &longPlaylist{pages: map[string]any{"": first, "P1": page(items[2:4], "P2"), "P2": page(items[4:6], "")}, fail: map[string]int{}}
}

// dig is the value at path in v, of maps by key and lists by index.
func dig(v any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			v = v.(map[string]any)[k]
		case int:
			v = v.([]any)[k]
		}
	}
	return v
}

func (l *longPlaylist) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	var body struct {
		Continuation string `json:"continuation"`
	}
	json.Unmarshal(data, &body)
	l.mu.Lock()
	l.asked = append(l.asked, body.Continuation)
	page, ok := l.pages[body.Continuation]
	failing := l.fail[body.Continuation] > 0
	if failing {
		l.fail[body.Continuation]--
	}
	l.mu.Unlock()
	switch {
	case !ok:
		w.WriteHeader(http.StatusNotFound)
	case failing:
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		json.NewEncoder(w).Encode(page)
	}
}

func (l *longPlaylist) catalog(t *testing.T) *Catalog {
	t.Helper()
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
	srv := httptest.NewServer(l)
	t.Cleanup(srv.Close)
	a := &App{youtube: youtube.New()}
	a.youtube.BaseURL = srv.URL
	return &Catalog{a: a}
}

const longID = "yt:PLfP6i5T0-DkIMLNRwmJpRBs4PJvxfgwBg"

// TestPlaylistWhole reads a playlist whole for what takes all of it, as
// a download, asking again for a page that fails: one that fails each
// time fails it, saying how many tracks it read.
func TestPlaylistWhole(t *testing.T) {
	l := newLongPlaylist(t)
	c := l.catalog(t)
	l.fail["P1"] = 2
	p, err := c.Playlist(context.Background(), longID)
	if err != nil || len(p.Tracks) != 6 || p.More != "" || p.Cut {
		t.Fatalf("the playlist: %d tracks, more %q, cut %v, %v", len(p.Tracks), p.More, p.Cut, err)
	}
	if want := []string{"", "P1", "P1", "P1", "P2"}; !slices.Equal(l.asked, want) {
		t.Errorf("asked for the pages %q, want %q", l.asked, want)
	}

	l.fail["P2"] = 4
	_, err = c.Playlist(context.Background(), longID)
	var api *youtube.APIError
	if !errors.As(err, &api) || !strings.Contains(err.Error(), "only 4 of the tracks") {
		t.Errorf("a page that failed four times: %v", err)
	}
}

// TestPlaylistReadOn gives a playlist's first page at once, and its pages
// after it as they come; after a page that fails each time, it holds what
// came before, and reads on from it.
func TestPlaylistReadOn(t *testing.T) {
	l := newLongPlaylist(t)
	c := l.catalog(t)
	ctx := context.Background()
	p, err := c.PlaylistStart(ctx, longID)
	if err != nil || len(p.Tracks) != 2 || p.More != "P1" || len(l.asked) != 1 {
		t.Fatalf("the first page: %d tracks, more %q, %d asks, %v", len(p.Tracks), p.More, len(l.asked), err)
	}

	l.fail["P2"] = 4
	var told []int
	p, err = c.ReadOn(ctx, p, func(now media.Playlist) { told = append(told, len(now.Tracks)) })
	if err == nil || len(p.Tracks) != 4 || p.More != "P2" || !slices.Equal(told, []int{4}) {
		t.Fatalf("a page that failed four times: %d tracks, more %q, told %v, %v", len(p.Tracks), p.More, told, err)
	}
	p, err = c.ReadOn(ctx, p, func(now media.Playlist) { told = append(told, len(now.Tracks)) })
	if err != nil || len(p.Tracks) != 6 || p.More != "" || !slices.Equal(told, []int{4, 6}) {
		t.Errorf("read on: %d tracks, more %q, told %v, %v", len(p.Tracks), p.More, told, err)
	}
	if p.TrackCount != 56 {
		t.Errorf("the playlist's count is %d, not its header's", p.TrackCount)
	}

	// A mix of MixMusic's own has no pages.
	local := media.Playlist{ID: "mx:1", Source: media.SourceLocal, Tracks: []media.Track{{ID: "yt:1"}}}
	if got, err := c.ReadOn(ctx, local, nil); err != nil || len(got.Tracks) != 1 {
		t.Errorf("a mix: %+v, %v", got, err)
	}
}
