package main

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// mixPage makes a mix of the first four test tracks, and shows its page.
func mixPage(t *testing.T) (*shell, *ui.Tester, string) {
	t.Helper()
	s := newTestShell(t)
	mix, err := s.svc.Collection.CreatePlaylist("Night drive", "", testTracks[:4])
	if err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	show(s, tt, playlistPath(mix.ID))
	until(t, s, tt, "the mix's tracks", func() bool { return tt.HasText(testTracks[3].Title) })
	return s, tt, mix.ID
}

// mixOrder is the order of the tracks of the mix id, by their indices in
// testTracks, as the library keeps it.
func mixOrder(t *testing.T, s *shell, id string) []int {
	t.Helper()
	p, err := s.svc.Catalog.PlaylistStart(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return trackIndices(p.Tracks)
}

func trackIndices(tracks []media.Track) []int {
	var out []int
	for _, tr := range tracks {
		out = append(out, slices.IndexFunc(testTracks, func(x media.Track) bool { return x.ID == tr.ID }))
	}
	return out
}

// shownOrder is the order the window shows the first four test tracks
// in, by their indices in testTracks.
func shownOrder(t *testing.T, tt *ui.Tester) []int {
	t.Helper()
	var out []int
	for _, title := range titlesShown(t, tt, testTracks[:4]) {
		out = append(out, slices.IndexFunc(testTracks, func(x media.Track) bool { return x.Title == title }))
	}
	return out
}

// dragTrack drags the row of the track titled from onto the row of the
// track titled onto.
func dragTrack(t *testing.T, s *shell, tt *ui.Tester, from, onto string) {
	t.Helper()
	a, ok := tt.Find(from)
	b, ok2 := tt.Find(onto)
	if !ok || !ok2 {
		t.Fatalf("the list doesn't show %q and %q: %q", from, onto, tt.Texts())
	}
	tt.Press(a.X+4, a.Y+4)
	for step := float32(1); step <= 8; step++ {
		tt.Move(a.X+4, a.Y+4+(b.Y-a.Y)*step/8)
		tt.Frame()
	}
	tt.Release(b.X+4, b.Y+4)
	s.drain()
	tt.Frame()
}

// TestDragInMix drags a track of a mix onto another, up and down, which
// it takes the place of, on the page at once and in the library.
func TestDragInMix(t *testing.T) {
	s, tt, id := mixPage(t)
	dragTrack(t, s, tt, testTracks[3].Title, testTracks[1].Title)
	if got, want := mixOrder(t, s, id), []int{0, 3, 1, 2}; !slices.Equal(got, want) {
		t.Errorf("dragged up, the mix holds %v, want %v", got, want)
	}
	if got, want := shownOrder(t, tt), []int{0, 3, 1, 2}; !slices.Equal(got, want) {
		t.Errorf("dragged up, the page shows %v, want %v", got, want)
	}
	dragTrack(t, s, tt, testTracks[0].Title, testTracks[1].Title)
	if got, want := mixOrder(t, s, id), []int{3, 1, 0, 2}; !slices.Equal(got, want) {
		t.Errorf("dragged down, the mix holds %v, want %v", got, want)
	}
	if got, want := shownOrder(t, tt), []int{3, 1, 0, 2}; !slices.Equal(got, want) {
		t.Errorf("dragged down, the page shows %v, want %v", got, want)
	}
}

// TestMoveInMixWithKeys moves a track of a mix with Alt+Down and Alt+Up
// from its play button, which keeps the focus on the track as it moves.
func TestMoveInMixWithKeys(t *testing.T) {
	s, tt, id := mixPage(t)
	lead := "Play " + testTracks[0].Title
	focusOn(t, tt, lead)
	for range 2 {
		tt.Key(ui.Alt, ui.KeyDown)
		s.drain()
		tt.Frame()
		tt.Frame()
	}
	if got, want := mixOrder(t, s, id), []int{1, 2, 0, 3}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Down twice, the mix holds %v, want %v", got, want)
	}
	if info, _ := tt.Access(lead); !info.Focused {
		t.Errorf("the focus didn't follow the track down")
	}
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	tt.Frame()
	if got, want := mixOrder(t, s, id), []int{1, 0, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Up, the mix holds %v, want %v", got, want)
	}
	if got, want := shownOrder(t, tt), []int{1, 0, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Up, the page shows %v, want %v", got, want)
	}
	// The first track goes no higher.
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	if got, want := mixOrder(t, s, id), []int{0, 1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("past the top, the mix holds %v, want %v", got, want)
	}
}

// TestSortedMixStays moves nothing while the mix's tracks show sorted:
// they have no place of their own there.
func TestSortedMixStays(t *testing.T) {
	s, tt, id := mixPage(t)
	click(t, s, tt, "Sort by")
	click(t, s, tt, "Title")
	dragTrack(t, s, tt, testTracks[3].Title, testTracks[1].Title)
	if got, want := mixOrder(t, s, id), []int{0, 1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("a drag in the sorted list made the mix %v", got)
	}
}

// focusOn tabs to the control labelled label, until it has the focus
// itself, rather than what holds it.
func focusOn(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for range 150 {
		if info, _ := tt.Access(label); info.Focused {
			return
		}
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	}
	t.Fatalf("Tab doesn't reach %q", label)
}

// youtubeMoves records the moves the page asks of YouTube, which answers
// with err, once gate, if set, lets it.
type youtubeMoves struct {
	mu    sync.Mutex
	moves [][2]string
	err   error
	gate  chan struct{}
}

func (m *youtubeMoves) asked() [][2]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.moves)
}

// youtubePlaylistPage shows a playlist of the user's on YouTube Music,
// signed in, whose moves YouTube answers through m.
func youtubePlaylistPage(t *testing.T, m *youtubeMoves) (*shell, *ui.Tester, string) {
	t.Helper()
	old := moveOnYouTube
	moveOnYouTube = func(s *shell, ctx context.Context, playlistID string, tr, before media.Track) error {
		if m.gate != nil {
			<-m.gate
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		m.moves = append(m.moves, [2]string{tr.EntryID, before.EntryID})
		return m.err
	}
	t.Cleanup(func() { moveOnYouTube = old })
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true}}}
	tracks := slices.Clone(testTracks[:4])
	for i := range tracks {
		tracks[i].EntryID = "set" + count(i)
	}
	id := "yt:PLmine"
	s.loads["playlist:"+id] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: id, Source: media.SourceYouTube,
		Title: "Mine", Owner: "Ada", Editable: true, TrackCount: len(tracks), Tracks: tracks}}
	tt := tester(t, s)
	show(s, tt, playlistPath(id))
	until(t, s, tt, "the playlist's tracks", func() bool { return tt.HasText(testTracks[3].Title) })
	return s, tt, id
}

// TestDragInYouTubePlaylist moves a track of a playlist of the user's on
// YouTube Music there too, by the places of its entries: before the track
// after it, or to the end.
func TestDragInYouTubePlaylist(t *testing.T) {
	m := &youtubeMoves{}
	s, tt, _ := youtubePlaylistPage(t, m)
	dragTrack(t, s, tt, testTracks[3].Title, testTracks[1].Title)
	dragTrack(t, s, tt, testTracks[0].Title, testTracks[2].Title)
	until(t, s, tt, "YouTube to hear of both moves", func() bool { return len(m.asked()) == 2 })
	if got, want := m.asked(), [][2]string{{"set3", "set1"}, {"set0", ""}}; !slices.Equal(got, want) {
		t.Errorf("YouTube was asked %v, want %v", got, want)
	}
	if got, want := shownOrder(t, tt), []int{3, 1, 2, 0}; !slices.Equal(got, want) {
		t.Errorf("the page shows %v, want %v", got, want)
	}
}

// TestRefusedYouTubeMove tells why YouTube refused a move, and has the
// page read the playlist again, as YouTube keeps it. No frame follows the
// refusal, so that nothing asks YouTube itself.
func TestRefusedYouTubeMove(t *testing.T) {
	m := &youtubeMoves{err: errors.New("YouTube Music did not move it in the playlist"), gate: make(chan struct{})}
	s, tt, id := youtubePlaylistPage(t, m)
	dragTrack(t, s, tt, testTracks[3].Title, testTracks[1].Title)
	close(m.gate)
	for deadline := time.Now().Add(5 * time.Second); !told(s, m.err.Error(), true); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no refusal told: %+v", s.notes)
		}
		s.drain()
	}
	if l := s.loads["playlist:"+id].(*loaded[media.Playlist]); !l.again {
		t.Error("the page doesn't read the playlist again")
	}
}
