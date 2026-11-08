package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

const ownPlaylistID = "yt:PLmine"

// fakeSuggestions answers for YouTube Music the suggestions of the user's
// playlist by their token, and takes the songs added to it, each answer
// once the test lets it: what the page does with an answer then waits for
// the test's drain, and no frame takes it unseen.
type fakeSuggestions struct {
	mu    sync.Mutex
	pages map[string]app.PlaylistSuggestions
	fail  error
	asked []string
	added []string
	gate  chan struct{}
}

func (f *fakeSuggestions) tokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.asked)
}

func (f *fakeSuggestions) adds() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.added)
}

// let lets the ask or the add waiting answer.
func (f *fakeSuggestions) let(t *testing.T) {
	t.Helper()
	select {
	case f.gate <- struct{}{}:
	case <-time.After(5 * time.Second):
		t.Fatal("nothing asks YouTube Music")
	}
}

// suggestedTrack is a song YouTube Music suggests.
func suggestedTrack(id, title string) media.Track {
	return testTrack("yt:"+id, title, "Someone", "Something", media.SourceYouTube)
}

// suggestionsFor has YouTube Music answer the test with f.
func suggestionsFor(t *testing.T, f *fakeSuggestions) {
	t.Helper()
	f.gate = make(chan struct{})
	ended := make(chan struct{})
	wait := func(ctx context.Context) error {
		select {
		case <-f.gate:
			return nil
		case <-ended:
		case <-ctx.Done():
		}
		return errors.New("the test ended")
	}
	suggest, add := suggestOnYouTube, addSuggestionOnYouTube
	suggestOnYouTube = func(_ *shell, ctx context.Context, token string) (app.PlaylistSuggestions, error) {
		if err := wait(ctx); err != nil {
			return app.PlaylistSuggestions{}, err
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.asked = append(f.asked, token)
		if f.fail != nil {
			return app.PlaylistSuggestions{}, f.fail
		}
		return f.pages[token], nil
	}
	addSuggestionOnYouTube = func(_ *shell, ctx context.Context, playlistID string, tr media.Track) error {
		if err := wait(ctx); err != nil {
			return err
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.added = append(f.added, playlistID+" "+tr.ID)
		return nil
	}
	t.Cleanup(func() {
		close(ended)
		suggestOnYouTube, addSuggestionOnYouTube = suggest, add
	})
}

// ownPlaylistPage shows the page of a playlist of the user's on YouTube
// Music, or of another's when editable isn't set, signed in, which
// suggests the songs f gives, from token S1.
func ownPlaylistPage(t *testing.T, editable bool) (*shell, *ui.Tester, *fakeSuggestions) {
	t.Helper()
	f := &fakeSuggestions{pages: map[string]app.PlaylistSuggestions{
		"S1": {Refresh: "R1", Tracks: []media.Track{
			suggestedTrack("s1", "Harbour Lights"), suggestedTrack("s2", "Paper Kites"), suggestedTrack("s3", "Slow Orbit"),
			suggestedTrack("s4", "Coastline"), suggestedTrack("s5", "Ember"),
		}},
		"R1": {Refresh: "R2", Tracks: []media.Track{suggestedTrack("s6", "Northbound"), suggestedTrack("s7", "Glass Rain")}},
	}}
	suggestionsFor(t, f)
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true}}}
	s.loads["youtube:playlists"] = &loaded[[]media.Playlist]{done: true}
	s.loads["playlist:"+ownPlaylistID] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: ownPlaylistID, Source: media.SourceYouTube,
		Title: "Road trip", Owner: "You", TrackCount: 2, Tracks: []media.Track{testTracks[0], testTracks[2]},
		Editable: editable, Deletable: editable, Suggest: "S1"}}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, playlistPath(ownPlaylistID))
	return s, tt, f
}

// suggestionsShown is what the page of the playlist of ownPlaylistPage
// shows of its suggestions.
func suggestionsShown(s *shell) *suggested {
	st, _ := s.loads["playlist:"+ownPlaylistID+" suggestions"].(*suggested)
	return st
}

// answer lets YouTube Music answer what the page asked, and draws the page
// once it took the answer, which done tells.
func (f *fakeSuggestions) answer(t *testing.T, s *shell, tt *ui.Tester, done func() bool) {
	t.Helper()
	f.let(t)
	waitFor(t, s, done)
	tt.Frame()
}

// TestPlaylistSuggestions shows the songs YouTube Music suggests below the
// tracks of a playlist of the user's, five at a time: each plays alone,
// Add puts it in the playlist at once, Refresh shows the next ones, and
// playing the playlist leaves them out.
func TestPlaylistSuggestions(t *testing.T) {
	s, tt, f := ownPlaylistPage(t, true)
	sg := suggestionsShown(s)
	f.answer(t, s, tt, func() bool { return sg.done })
	if !tt.HasText(s.lk.words("Suggestions")) {
		t.Errorf("the suggestions have no heading: %q", tt.Texts())
	}
	for _, title := range []string{"Harbour Lights", "Paper Kites", "Slow Orbit", "Coastline", "Ember"} {
		if _, ok := tt.Find("Add " + title + " to Road trip"); !ok {
			t.Errorf("%s has no Add: %q", title, tt.Texts())
		}
	}
	if got := f.tokens(); !slices.Equal(got, []string{"S1"}) {
		t.Errorf("asked for %q", got)
	}

	// The playlist plays its own tracks alone.
	click(t, s, tt, "play")
	s.settle(t, func(st player.State) bool { return st.Playing && len(st.Queue) == 2 })
	for _, it := range s.p.State().Queue {
		if it.Track.ID != testTracks[0].ID && it.Track.ID != testTracks[2].ID {
			t.Errorf("playing the playlist queued %s", it.Track.Title)
		}
	}
	// A suggestion plays alone.
	click(t, s, tt, "Play Slow Orbit")
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && st.Playing && cur.Track.ID == "yt:s3" && len(st.Queue) == 1
	})

	// Added, a song leaves the suggestions and ends the playlist's tracks
	// at once, and the playlist is read again, for its place there.
	key := "playlist:" + ownPlaylistID
	click(t, s, tt, "Add Paper Kites to Road trip")
	if !sg.adding["yt:s2"] {
		t.Error("the song is not on its way")
	}
	head := s.loads[key].(*loaded[media.Playlist])
	f.let(t)
	waitFor(t, s, func() bool { return !sg.adding["yt:s2"] })
	if got := f.adds(); len(got) != 1 || got[0] != ownPlaylistID+" yt:s2" {
		t.Errorf("added %q", got)
	}
	if !head.again {
		t.Error("the playlist isn't read again")
	}
	// The test answers that, as YouTube Music would.
	head.again = false
	tt.Frame()
	if _, ok := tt.Find("Add Paper Kites to Road trip"); ok || len(sg.tracks) != 4 {
		t.Errorf("a song added is still suggested: %d suggestions", len(sg.tracks))
	}
	r := s.loads[key+" read"].(*playlistRead)
	if n := len(r.p.Tracks); n != 3 || r.p.Tracks[2].ID != "yt:s2" || r.p.TrackCount != 3 {
		t.Errorf("the playlist holds %d tracks, of %d", n, r.p.TrackCount)
	}
	if !tt.HasText("3 tracks") || !tt.HasText("Added Paper Kites to Road trip") {
		t.Errorf("the page shows %q", tt.Texts())
	}

	// Refresh shows the next ones, by the token of the last.
	click(t, s, tt, "Refresh the suggestions")
	f.answer(t, s, tt, func() bool { return !sg.asking })
	if got := f.tokens(); !slices.Equal(got, []string{"S1", "R1"}) {
		t.Errorf("asked for %q", got)
	}
	if !tt.HasText("Northbound") || tt.HasText("Harbour Lights") {
		t.Errorf("refreshed, the suggestions show %q", tt.Texts())
	}

	// The last one added, the next come.
	for i, title := range []string{"Northbound", "Glass Rain"} {
		click(t, s, tt, "Add "+title+" to Road trip")
		f.let(t)
		waitFor(t, s, func() bool { return len(f.adds()) == 2+i && !sg.adding[suggestedTrack("s"+count(6+i), title).ID] })
		head.again = false
		tt.Frame()
	}
	if !sg.asking {
		t.Fatal("the last suggestion added, no more are asked for")
	}
	f.answer(t, s, tt, func() bool { return !sg.asking })
	if got := f.tokens(); len(got) != 3 || got[2] != "R2" {
		t.Errorf("asked for %q once the last was added", got)
	}
}

// TestPlaylistSuggestionsOnlyYours shows none below a playlist the user
// can't change, and asks for none.
func TestPlaylistSuggestionsOnlyYours(t *testing.T) {
	s, tt, f := ownPlaylistPage(t, false)
	tt.Frame()
	if tt.HasText(s.lk.words("Suggestions")) || suggestionsShown(s) != nil {
		t.Errorf("another's playlist suggests %q", tt.Texts())
	}
	select {
	case f.gate <- struct{}{}:
		t.Error("another's playlist asked for suggestions")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestPlaylistSuggestionsFail keeps the suggestions shown through a
// refresh that fails, which a toast tells, and says why the first didn't
// come, with a way to try again.
func TestPlaylistSuggestionsFail(t *testing.T) {
	s, tt, f := ownPlaylistPage(t, true)
	sg := suggestionsShown(s)
	f.answer(t, s, tt, func() bool { return sg.done })
	f.mu.Lock()
	f.fail = errors.New("youtube music: HTTP 503")
	f.mu.Unlock()
	click(t, s, tt, "Refresh the suggestions")
	f.let(t)
	waitFor(t, s, func() bool { return !sg.asking })
	if len(s.notes) != 1 || !strings.Contains(s.notes[0].Message, "Couldn't refresh the suggestions: youtube music: HTTP 503") {
		t.Errorf("the toasts say %+v", s.notes)
	}
	tt.Frame()
	if !tt.HasText("Harbour Lights") {
		t.Errorf("a failed refresh lost the suggestions: %q", tt.Texts())
	}

	// The first failing says so.
	s.forget("playlist:" + ownPlaylistID + " suggestions")
	tt.Frame()
	sg = suggestionsShown(s)
	f.answer(t, s, tt, func() bool { return sg.done })
	if !tt.HasText("Couldn't load the suggestions: youtube music: HTTP 503") {
		t.Fatalf("the page shows %q", tt.Texts())
	}
	f.mu.Lock()
	f.fail = nil
	f.mu.Unlock()
	click(t, s, tt, s.lk.sentence("try again"))
	f.answer(t, s, tt, func() bool { return !sg.asking })
	if got := f.tokens(); got[len(got)-1] != "S1" || !tt.HasText("Harbour Lights") {
		t.Errorf("trying again asked for %q, and shows %q", got, tt.Texts())
	}
}
