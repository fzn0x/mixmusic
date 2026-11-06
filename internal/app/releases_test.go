package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// releaseDay is the day the release tests run on.
var releaseDay = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

// fakeReleases answers for the releases of the artists, by ID, and counts
// the asks; an artist it has no releases of fails.
type fakeReleases struct {
	mu   sync.Mutex
	of   map[string][]media.Release
	asks map[string]int
}

func (f *fakeReleases) read(_ context.Context, id string) ([]media.Release, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asks[id]++
	r, ok := f.of[id]
	if !ok {
		return nil, errors.New("no such artist")
	}
	return r, nil
}

// asked returns how many asks there were, and forgets them.
func (f *fakeReleases) asked() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.asks {
		n += c
	}
	clear(f.asks)
	return n
}

// newReleaseApp is an app signed out of both services whose library and
// cache are in dir, and whose releases f answers for at the time *now.
func newReleaseApp(t *testing.T, dir string, f *fakeReleases, now *time.Time) *App {
	t.Helper()
	lib, err := library.Open(filepath.Join(dir, "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{opts: Options{CacheDir: dir}, library: lib, youtube: youtube.New(), spotify: spotify.NewClient(spotify.NewAuth("client"))}
	a.releases.read = f.read
	a.releases.now = func() time.Time { return *now }
	return a
}

func release(id, title, released string, kind media.AlbumKind) media.Release {
	source, _, _ := media.ParseID(id)
	return media.Release{Album: media.Album{ID: id, Source: source, Title: title, Kind: kind, Tracks: []media.Track{}}, Released: released}
}

func releaseIDs(rs []media.Release) []string {
	out := []string{}
	for _, r := range rs {
		out = append(out, r.Album.ID)
	}
	return out
}

func follow(t *testing.T, a *App, ids ...string) {
	t.Helper()
	for _, id := range ids {
		source, _, _ := media.ParseID(id)
		if err := (&Collection{a: a}).SetArtistFollowed(media.Artist{ID: id, Source: source, Name: "Artist " + id}, true); err != nil {
			t.Fatal(err)
		}
	}
}

// TestNewReleases shows the releases of the artists followed in the last
// eight weeks, the newest first, and those known by their year alone of
// this year, after them, by their places in their artists' lists. It asks
// for an artist's releases again only once they're old, an artist that
// fails an hour later, and keeps what was read for the next start.
func TestNewReleases(t *testing.T) {
	dir, now := t.TempDir(), releaseDay
	f := &fakeReleases{asks: map[string]int{}, of: map[string][]media.Release{
		"yt:UCharbour": {
			release("yt:MPREsingle", "Pier", "2026", media.AlbumKindSingle),
			release("yt:MPREold", "First Light", "2025", media.AlbumKindAlbum),
			release("yt:MPREalbum", "Second Light", "2026", media.AlbumKindAlbum),
		},
		"sp:dunes": {
			release("sp:week", "Low Tide", "2026-11-01", media.AlbumKindSingle),
			release("sp:summer", "Heatwave", "2026-08-01", media.AlbumKindAlbum),
			release("sp:month", "Shore", "2026-10", media.AlbumKindAlbum),
			release("sp:soon", "Next Year", "2026-12-04", media.AlbumKindAlbum),
		},
	}}
	a := newReleaseApp(t, dir, f, &now)
	follow(t, a, "sp:dunes", "yt:UCharbour")
	c := &Catalog{a: a}
	got, err := c.NewReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sp:week", "sp:month", "yt:MPREsingle", "yt:MPREalbum"}
	if !slices.Equal(releaseIDs(got), want) {
		t.Errorf("new releases %q, want %q", releaseIDs(got), want)
	}
	if got[0].Of.ID != "sp:dunes" || got[0].Of.Name != "Artist sp:dunes" || got[2].Of.ID != "yt:UCharbour" {
		t.Errorf("their artists: %+v, %+v", got[0].Of, got[2].Of)
	}
	if n := f.asked(); n != 2 {
		t.Errorf("asked %d times for two artists", n)
	}

	now = now.Add(time.Hour)
	if again, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(again), want) || f.asked() != 0 {
		t.Errorf("an hour later: %q, asked again", releaseIDs(again))
	}
	// The next start knows them before it asks.
	restarted := &Catalog{a: newReleaseApp(t, dir, f, &now)}
	if known := restarted.KnownReleases(); !slices.Equal(releaseIDs(known), want) {
		t.Errorf("after a restart, it knows %q", releaseIDs(known))
	}

	// Half a day on, they're asked again: Harbour fails, and keeps what
	// was read before, until an hour on.
	now = now.Add(releasesFresh)
	delete(f.of, "yt:UCharbour")
	if again, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(again), want) || f.asked() != 2 {
		t.Errorf("half a day on, with one failing: %q", releaseIDs(again))
	}
	now = now.Add(30 * time.Minute)
	if c.NewReleases(context.Background()); f.asked() != 0 {
		t.Error("the artist that failed was asked again within the hour")
	}
	now = now.Add(31 * time.Minute)
	if c.NewReleases(context.Background()); f.asks["yt:UCharbour"] != 1 || f.asked() != 1 {
		t.Error("the artist that failed wasn't asked again after the hour")
	}

	// Releases of eight weeks ago and a day are no longer new; a year's
	// stay new eight weeks into the next.
	now = time.Date(2027, 2, 20, 10, 0, 0, 0, time.UTC)
	f.of["yt:UCharbour"] = []media.Release{release("yt:MPREsingle", "Pier", "2026", media.AlbumKindSingle)}
	if late, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(late), []string{"yt:MPREsingle"}) {
		t.Errorf("in February: %q", releaseIDs(late))
	}
	now = time.Date(2027, 2, 27, 10, 0, 0, 0, time.UTC)
	if later := newReleases(a.releases.file.Get(), []string{"yt:UCharbour"}, now); len(later) != 0 {
		t.Errorf("past eight weeks into the next year: %q", releaseIDs(later))
	}
}

// TestNewReleasesBounded asks for the releases of thirty artists in a
// look, those never asked first, three at a time, and shows four of an
// artist's at most, each release once.
func TestNewReleasesBounded(t *testing.T) {
	now := releaseDay
	f := &fakeReleases{asks: map[string]int{}, of: map[string][]media.Release{}}
	var ids []string
	for i := range releasesPerLook + 5 {
		id := "yt:UC" + string(rune('a'+i/26)) + string(rune('a'+i%26))
		ids = append(ids, id)
		f.of[id] = []media.Release{}
	}
	for _, d := range []string{"2026-11-06", "2026-11-05", "2026-11-04", "2026-11-03", "2026-11-02"} {
		f.of[ids[0]] = append(f.of[ids[0]], release("sp:"+d, d, d, media.AlbumKindSingle))
	}
	// Two artists of one album.
	f.of[ids[1]] = []media.Release{release("sp:2026-11-06", "2026-11-06", "2026-11-06", media.AlbumKindSingle)}
	a := newReleaseApp(t, t.TempDir(), f, &now)
	var busy, most int
	var mu sync.Mutex
	read := a.releases.read
	a.releases.read = func(ctx context.Context, id string) ([]media.Release, error) {
		mu.Lock()
		busy++
		most = max(most, busy)
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		busy--
		mu.Unlock()
		return read(ctx, id)
	}
	// The library lists the last followed first: a look asks for those,
	// and leaves the first five followed to the next.
	follow(t, a, ids...)
	c := &Catalog{a: a}
	got, _ := c.NewReleases(context.Background())
	if n := f.asked(); n != releasesPerLook || most > releasesAtOnce || len(got) != 0 {
		t.Errorf("a look asked %d times, %d at once, and found %q", n, most, releaseIDs(got))
	}
	got, _ = c.NewReleases(context.Background())
	if n := f.asked(); n != 5 {
		t.Errorf("the second look asked %d times", n)
	}
	if want := []string{"sp:2026-11-06", "sp:2026-11-05", "sp:2026-11-04", "sp:2026-11-03"}; !slices.Equal(releaseIDs(got), want) {
		t.Errorf("new releases %q, want %q", releaseIDs(got), want)
	}
}

// TestNewReleasesUnfollowed forgets the releases of an artist the user
// stops following, at once in MixMusic, and at the next look on a
// service.
func TestNewReleasesUnfollowed(t *testing.T) {
	now := releaseDay
	f := &fakeReleases{asks: map[string]int{}, of: map[string][]media.Release{
		"yt:UCa": {release("yt:MPREa", "A", "2026", media.AlbumKindSingle)},
		"yt:UCb": {release("yt:MPREb", "B", "2026", media.AlbumKindSingle)},
	}}
	dir := t.TempDir()
	a := newReleaseApp(t, dir, f, &now)
	follow(t, a, "yt:UCa", "yt:UCb")
	c := &Catalog{a: a}
	if got, _ := c.NewReleases(context.Background()); len(got) != 2 {
		t.Fatalf("new releases %q", releaseIDs(got))
	}
	col := &Collection{a: a}
	if err := col.SetArtistFollowed(media.Artist{ID: "yt:UCa"}, false); err != nil {
		t.Fatal(err)
	}
	if known := c.KnownReleases(); !slices.Equal(releaseIDs(known), []string{"yt:MPREb"}) {
		t.Errorf("after unfollowing A, it knows %q", releaseIDs(known))
	}
	// B, unfollowed elsewhere, goes at the next look.
	if err := a.library.SetArtistFollowed(media.Artist{ID: "yt:UCb"}, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.NewReleases(context.Background()); len(got) != 0 || len(c.KnownReleases()) != 0 {
		t.Errorf("after unfollowing B: %q", releaseIDs(got))
	}
	data, err := os.ReadFile(filepath.Join(dir, "releases.json"))
	if err != nil || strings.TrimSpace(string(data)) != "{}" {
		t.Errorf("releases.json holds %s, %v", data, err)
	}
}

// TestFollowedArtists gathers the artists followed in MixMusic, then on
// Spotify, each once, and keeps the releases of the artists of a service
// whose list doesn't read.
func TestFollowedArtists(t *testing.T) {
	var down bool
	sp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down || r.URL.Path != "/me/following" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte(`{"artists": {"items": [{"id": "dunes", "name": "Dunes"}, {"id": "tide", "name": "Tide"}], "cursors": {}}}`))
	}))
	defer sp.Close()
	now := releaseDay
	f := &fakeReleases{asks: map[string]int{}, of: map[string][]media.Release{
		"sp:dunes": {release("sp:week", "Low Tide", "2026-11-01", media.AlbumKindSingle)},
		"sp:tide":  {},
		"yt:UCa":   {},
	}}
	a := newReleaseApp(t, t.TempDir(), f, &now)
	a.spotify.BaseURL = sp.URL
	a.spotify.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	follow(t, a, "sp:tide", "yt:UCa")
	followed, lost := a.followedArtists(context.Background())
	var ids []string
	for _, ar := range followed {
		ids = append(ids, ar.ID)
	}
	if !slices.Equal(ids, []string{"yt:UCa", "sp:tide", "sp:dunes"}) || len(lost) != 0 {
		t.Errorf("followed %q, lost %v", ids, lost)
	}
	c := &Catalog{a: a}
	if got, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(got), []string{"sp:week"}) {
		t.Errorf("new releases %q", releaseIDs(got))
	}
	down = true
	if followed, lost := a.followedArtists(context.Background()); len(followed) != 2 || !lost[media.SourceSpotify] {
		t.Errorf("Spotify down: followed %+v, lost %v", followed, lost)
	}
	if got, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(got), []string{"sp:week"}) {
		t.Errorf("Spotify down, it forgot Dunes': %q", releaseIDs(got))
	}
}

// TestArtistReleasesOfYouTube reads an artist's albums and singles off
// their YouTube Music page, each with its year, and nothing of Spotify
// signed out.
func TestArtistReleasesOfYouTube(t *testing.T) {
	page, err := os.ReadFile(filepath.Join("..", "youtube", "testdata", "artist.json"))
	if err != nil {
		t.Fatal(err)
	}
	yt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(page) }))
	defer yt.Close()
	a := &App{youtube: youtube.New(), spotify: spotify.NewClient(spotify.NewAuth("client"))}
	a.youtube.BaseURL = yt.URL
	got, err := a.artistReleases(context.Background(), "yt:UCbM50mQtG58vhLZonicYUrQ")
	if err != nil || len(got) < 2 {
		t.Fatalf("releases %+v, %v", got, err)
	}
	if got[0].Album.Title != "Q-ENA II" || got[0].Released != "2021" || got[1].Album.Kind != media.AlbumKindSingle {
		t.Errorf("releases %+v", got)
	}
	for _, r := range got {
		if r.Released == "" || r.Released != r.Album.Year {
			t.Errorf("%s came out %q", r.Album.Title, r.Released)
		}
	}
	if _, err := a.artistReleases(context.Background(), "sp:dunes"); !errors.Is(err, errNotSignedIn) {
		t.Errorf("Spotify signed out: %v", err)
	}
}
