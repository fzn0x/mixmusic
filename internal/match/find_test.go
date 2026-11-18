package match

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// song is a song an import names: a title, its artists, its length.
func song(title string, ms int, artists ...string) media.Track {
	t := media.Track{Title: title, DurationMs: ms}
	for _, a := range artists {
		t.Artists = append(t.Artists, media.Ref{Name: a})
	}
	return t
}

// TestJudge is sure of a candidate whose title, artists, version and
// length agree, and has the others checked, or found not.
func TestJudge(t *testing.T) {
	lights := song("Harbour Lights", 215000, "Ola Brine")
	for _, c := range []struct {
		name    string
		t, cand media.Track
		want    Verdict
	}{
		{"the same", lights, track("Harbour Lights", "Ola Brine", 216000, false), Sure},
		{"a remaster", lights, track("Harbour Lights (Remastered 2011)", "Ola Brine", 215500, false), Sure},
		{"3 seconds off", lights, track("Harbour Lights", "Ola Brine", 218000, false), Sure},
		{"4 seconds off a short one", song("Low Tide", 150000, "Ola Brine"), track("Low Tide", "Ola Brine", 154100, false), ToCheck},
		{"2% of a long one", song("Suite", 600000, "Ola Brine"), track("Suite", "Ola Brine", 611000, false), Sure},
		{"no length", song("Harbour Lights", 0, "Ola Brine"), track("Harbour Lights", "Ola Brine", 216000, false), Sure},
		{"live", lights, track("Harbour Lights (Live)", "Ola Brine", 215000, false), ToCheck},
		{"both live", song("Harbour Lights - Live", 215000, "Ola Brine"), track("Harbour Lights (Live)", "Ola Brine", 215000, false), Sure},
		{"a remix", lights, track("Harbour Lights (Tide Remix)", "Ola Brine", 215000, false), ToCheck},
		{"acoustic", lights, track("Harbour Lights - Acoustic", "Ola Brine", 215000, false), ToCheck},
		{"another artist", lights, track("Harbour Lights", "Tam Reed", 215000, false), ToCheck},
		{"featured", song("Harbour Lights", 215000, "Ola Brine", "Tam Reed"),
			track("Harbour Lights (feat. Tam Reed)", "Ola Brine", 215000, false), Sure},
		{"no artist", song("Harbour Lights", 215000), track("Harbour Lights", "Ola Brine", 215000, false), ToCheck},
		{"another song", lights, track("Gulls at Dawn", "Ola Brine", 215000, false), NotFound},
	} {
		if got := Judge(c.t, c.cand); got != c.want {
			t.Errorf("%s: %d, want %d (score %.2f)", c.name, got, c.want, Score(c.t, c.cand))
		}
	}
	if len(versionWords) < 14 {
		t.Errorf("only %d version words", len(versionWords))
	}
}

// querySearch answers each query with what songs and videos map it to,
// and keeps the queries.
type querySearch struct {
	mu            sync.Mutex
	songs, videos map[string][]media.Item
	queries       []string
	fail          error
}

func (q *querySearch) Search(_ context.Context, query string, filter youtube.Filter) ([]media.Item, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queries = append(q.queries, query+"/"+string(filter))
	if q.fail != nil {
		return nil, q.fail
	}
	if filter == youtube.FilterSongs {
		return q.songs[query], nil
	}
	return q.videos[query], nil
}

// TestFindLibraryFirst takes a sure match of the library without a
// search.
func TestFindLibraryFirst(t *testing.T) {
	yt := &querySearch{}
	m := New(yt, nil)
	mine := track("Harbour Lights", "Ola Brine", 215000, false)
	got, err := m.Find(context.Background(), song("Harbour Lights", 215000, "Ola Brine"), FindOptions{Library: []media.Track{mine}})
	if err != nil || got.Verdict != Sure || !got.Library || got.Track.ID != mine.ID {
		t.Errorf("found %+v, %v", got, err)
	}
	if len(yt.queries) != 0 {
		t.Errorf("searched %q", yt.queries)
	}
}

// TestFindSongsThenVideos looks among the songs, then the videos, then
// by the first artist alone, and keeps the closest to check when none is
// sure.
func TestFindSongsThenVideos(t *testing.T) {
	ctx := context.Background()
	gulls := song("Gulls at Dawn", 241000, "Ola Brine & Tam Reed")
	live := track("Gulls at Dawn (Live)", "Ola Brine", 241000, false)
	right := track("Gulls at Dawn", "Ola Brine & Tam Reed", 241000, true)
	yt := &querySearch{
		songs:  map[string][]media.Item{"Gulls at Dawn Ola Brine & Tam Reed": items(live)},
		videos: map[string][]media.Item{"Gulls at Dawn Ola Brine": items(right)},
	}
	got, err := New(yt, nil).Find(ctx, gulls, FindOptions{})
	if err != nil || got.Verdict != Sure || got.Track.ID != right.ID {
		t.Fatalf("found %+v, %v", got, err)
	}
	want := []string{"Gulls at Dawn Ola Brine & Tam Reed/songs", "Gulls at Dawn Ola Brine & Tam Reed/videos",
		"Gulls at Dawn Ola Brine/songs", "Gulls at Dawn Ola Brine/videos"}
	if !slices.Equal(yt.queries, want) {
		t.Errorf("searched %q, want %q", yt.queries, want)
	}

	yt = &querySearch{songs: map[string][]media.Item{"Gulls at Dawn Ola Brine & Tam Reed": items(live)}}
	if got, _ := New(yt, nil).Find(ctx, gulls, FindOptions{}); got.Verdict != ToCheck || got.Track.ID != live.ID {
		t.Errorf("found %+v, want the live take to check", got)
	}
	yt = &querySearch{}
	if got, err := New(yt, nil).Find(ctx, gulls, FindOptions{}); err != nil || got.Verdict != NotFound {
		t.Errorf("found %+v, %v of nothing", got, err)
	}
}

// TestFindFails stops at a search that fails, with its error.
func TestFindFails(t *testing.T) {
	refused := errors.New("HTTP 429")
	yt := &querySearch{fail: refused}
	if _, err := New(yt, nil).Find(context.Background(), song("Low Tide", 0, "Ola Brine"), FindOptions{}); !errors.Is(err, refused) {
		t.Errorf("Find answered %v", err)
	}
	if len(yt.queries) != 1 {
		t.Errorf("searched %q after a failure", yt.queries)
	}
}

// TestTitleKey keys the titles of a song alike, whatever names their
// release, version or featured artists.
func TestTitleKey(t *testing.T) {
	want := TitleKey("Harbour Lights")
	for _, title := range []string{"Harbour Lights (Remastered 2011)", "harbour lights", "Harbour Lights - Live",
		"Harbour Lights (feat. Tam Reed)", "Harbour Lights [Official Video]", "HARBOUR LIGHTS ft. Tam"} {
		if got := TitleKey(title); got != want {
			t.Errorf("TitleKey(%q) = %q, want %q", title, got, want)
		}
	}
	if TitleKey("Stay With Me") == TitleKey("Stay") {
		t.Error("Stay With Me keys as Stay")
	}
	if got := FirstArtist("Ola Brine feat. Tam Reed"); got != "Ola Brine" {
		t.Errorf("FirstArtist = %q", got)
	}
}
