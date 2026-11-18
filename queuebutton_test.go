package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// queueAlbum shows the page of an album of the test tracks; compact sets
// the compact density.
func queueAlbum(t *testing.T, compact bool) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShellOf(t, func(o *app.Options) {
		if !compact {
			return
		}
		if err := os.WriteFile(filepath.Join(o.DataDir, "settings.json"), []byte(`{"layout": "tiles", "density": "compact"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	s.loads["album:al-queue"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-queue", Title: "Night Bus",
		Source: media.SourceYouTube, Tracks: testTracks}}
	tt := tester(t, s)
	show(s, tt, "/album/al-queue")
	until(t, s, tt, "the album's tracks", func() bool { return tt.HasText(testTracks[5].Title) })
	return s, tt
}

// hoverRow puts the pointer on the row of the track or the episode titled
// title, over its … button.
func hoverRow(t *testing.T, s *shell, tt *ui.Tester, title string) {
	t.Helper()
	r, ok := tt.Find(locales.TracksMoreFor(title))
	if !ok {
		t.Fatalf("no row of %q; the window shows %q", title, tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	s.drain()
	tt.Frame()
}

// queueAdd is the label of the queue button of the track titled title.
func queueAdd(title string) string { return locales.QueueButtonAdd(title) }

// queuedIDs are the IDs of the tracks of the queue.
func queuedIDs(st player.State) []string {
	var out []string
	for _, it := range st.Queue {
		out = append(out, it.Track.ID)
	}
	return out
}

// TestRowQueueButton shows a queue button in the length's place of the row
// under the pointer only, whose target is 48px square, and which plays the
// song while nothing plays, then adds songs at the queue's end, one queued
// already too, without selecting the row.
func TestRowQueueButton(t *testing.T) {
	s, tt := queueAlbum(t, false)
	strobe, teardrop := testTracks[2], testTracks[1]
	if _, ok := tt.Find(queueAdd(strobe.Title)); ok {
		t.Fatal("the button shows with the pointer over no row, a Tab stop")
	}
	lengths := textCount(tt, "3:31")
	hoverRow(t, s, tt, strobe.Title)
	r, ok := tt.Find(queueAdd(strobe.Title))
	if !ok {
		t.Fatalf("no queue button on the row under the pointer; the window shows %q", tt.Texts())
	}
	if r.W != 48 || r.H != 48 {
		t.Errorf("the button's target is %v by %v, want 48 by 48", r.W, r.H)
	}
	if got := textCount(tt, "3:31"); got != lengths-1 {
		t.Errorf("%d rows show their length, want %d: the button takes it", got, lengths-1)
	}
	if _, ok := tt.Find(queueAdd(teardrop.Title)); ok {
		t.Error("a row the pointer isn't over shows its button")
	}

	click(t, s, tt, queueAdd(strobe.Title))
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == strobe.ID && len(st.Queue) == 1
	})
	if !toasted(t, s, tt, "Playing "+strobe.Title) {
		t.Errorf("no toast of the song playing: %q", tt.Texts())
	}

	hoverRow(t, s, tt, teardrop.Title)
	click(t, s, tt, queueAdd(teardrop.Title))
	if !toasted(t, s, tt, "Added "+teardrop.Title+" to the queue") {
		t.Errorf("no toast of the song added: %q", tt.Texts())
	}
	hoverRow(t, s, tt, strobe.Title)
	click(t, s, tt, queueAdd(strobe.Title))
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 3 })
	st := s.p.State()
	if got, want := queuedIDs(st), []string{strobe.ID, teardrop.ID, strobe.ID}; !slices.Equal(got, want) || st.Index != 0 {
		t.Errorf("the queue holds %q at %d, want %q at 0", got, st.Index, want)
	}
	if len(s.picks.ids) != 0 {
		t.Errorf("the button selected %q", picked(s))
	}
}

// TestRowQueueButtonInMix clicks the queue button of a row of a mix, whose
// rows play, select and drag: it neither plays the mix nor moves the row.
func TestRowQueueButtonInMix(t *testing.T) {
	s, tt, id := mixPage(t)
	s.p.PlayTracks(testTracks[5:], 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	hoverRow(t, s, tt, testTracks[2].Title)
	click(t, s, tt, queueAdd(testTracks[2].Title))
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 2 })
	st := s.p.State()
	if got, want := queuedIDs(st), []string{testTracks[5].ID, testTracks[2].ID}; !slices.Equal(got, want) || st.Index != 0 {
		t.Errorf("the queue holds %q at %d, want %q at 0", got, st.Index, want)
	}
	if got, want := mixOrder(t, s, id), []int{0, 1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("the mix holds %v, want %v", got, want)
	}
	if got, want := shownOrder(t, tt), []int{0, 1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("the page shows %v, want %v", got, want)
	}
	if len(s.picks.ids) != 0 {
		t.Errorf("the button selected %q", picked(s))
	}
}

// TestRowQueueButtonWhileSelected hides the button while the list has
// tracks selected, whose bar adds them to the queue.
func TestRowQueueButtonWhileSelected(t *testing.T) {
	s, tt := queueAlbum(t, false)
	hoverRow(t, s, tt, testTracks[3].Title)
	if _, ok := tt.Find(queueAdd(testTracks[3].Title)); !ok {
		t.Fatalf("no queue button; the window shows %q", tt.Texts())
	}
	clickRow(t, s, tt, ui.Cmd, testTracks[0].Title)
	hoverRow(t, s, tt, testTracks[3].Title)
	if _, ok := tt.Find(queueAdd(testTracks[3].Title)); ok {
		t.Error("the button shows while tracks are selected")
	}
}

// TestRowQueueButtonCompact gives the button the row's height, under 48px
// with the compact density.
func TestRowQueueButtonCompact(t *testing.T) {
	s, tt := queueAlbum(t, true)
	if s.lk.row >= 48 {
		t.Fatalf("compact rows are %vpx tall", s.lk.row)
	}
	hoverRow(t, s, tt, testTracks[2].Title)
	r, ok := tt.Find(queueAdd(testTracks[2].Title))
	if !ok {
		t.Fatalf("no queue button; the window shows %q", tt.Texts())
	}
	if r.W != 48 || r.H != s.lk.row {
		t.Errorf("the button's target is %v by %v, want 48 by %v", r.W, r.H, s.lk.row)
	}
}

// TestRowQueueKeys queues a row with Shift+Enter, which plays it while
// nothing plays, and puts one next with Alt+Enter, while the keyboard's
// focus is in the row.
func TestRowQueueKeys(t *testing.T) {
	s, tt := queueAlbum(t, false)
	teardrop, strobe, windowlicker := testTracks[1], testTracks[2], testTracks[3]
	key := func(mods ui.Modifiers, title string) {
		t.Helper()
		focusOn(t, tt, "Play "+title)
		tt.Key(mods, ui.KeyEnter)
		s.drain()
		tt.Frame()
	}
	key(ui.Shift, teardrop.Title)
	s.settle(t, func(st player.State) bool { cur, ok := st.Current(); return ok && cur.Track.ID == teardrop.ID })
	if !toasted(t, s, tt, "Playing "+teardrop.Title) {
		t.Errorf("no toast of the song playing: %q", tt.Texts())
	}
	key(ui.Shift, windowlicker.Title)
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 2 })
	if !toasted(t, s, tt, "Added "+windowlicker.Title+" to the queue") {
		t.Errorf("no toast of the song added: %q", tt.Texts())
	}
	focusOn(t, tt, "Pause "+teardrop.Title)
	key(ui.Alt, strobe.Title)
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 3 })
	st := s.p.State()
	if got, want := queuedIDs(st), []string{teardrop.ID, strobe.ID, windowlicker.ID}; !slices.Equal(got, want) || st.Index != 0 {
		t.Errorf("the queue holds %q at %d, want %q at 0", got, st.Index, want)
	}
	if !toasted(t, s, tt, strobe.Title+" plays next") {
		t.Errorf("no toast of the song next: %q", tt.Texts())
	}
}

// TestEpisodeQueueButton gives an episode's row the queue button in its
// length's 64px slot, and the keys.
func TestEpisodeQueueButton(t *testing.T) {
	s := newPodcastShell(t)
	s.loads["podcast:"+testShow.ID] = &loaded[media.Podcast]{done: true, value: testShow}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, podcastPath(testShow.ID))
	dream, light := testShow.Episodes[1], testShow.Episodes[2]
	hoverRow(t, s, tt, dream.Title)
	r, ok := tt.Find(queueAdd(dream.Title))
	if !ok {
		t.Fatalf("no queue button on the episode's row; the window shows %q", tt.Texts())
	}
	if r.W != 48 || r.H != 48 {
		t.Errorf("the button's target is %v by %v, want 48 by 48", r.W, r.H)
	}
	if tt.HasText("34:00") {
		t.Error("the row shows its length beside the button")
	}
	click(t, s, tt, queueAdd(dream.Title))
	s.settle(t, func(st player.State) bool { cur, ok := st.Current(); return ok && cur.Track.ID == dream.ID })
	if !toasted(t, s, tt, "Playing "+dream.Title) {
		t.Errorf("no toast of the episode playing: %q", tt.Texts())
	}
	tt.Move(4, 400)
	focusOn(t, tt, "Play "+light.Title)
	tt.Key(ui.Alt, ui.KeyEnter)
	s.drain()
	tt.Frame()
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 2 && st.Queue[1].Track.ID == light.ID })
}

// TestSongTileQueueButtons shows Add to queue and Play next beside the
// play circle of a song's tile while it is hovered or Tab reaches it,
// which neither open the tile nor play its shelf.
func TestSongTileQueueButtons(t *testing.T) {
	feed := []media.Shelf{{Title: "Quick picks", Items: itemsOfTracks(testTracks[:3]...)}}
	s, tt := homeShell(t, feed)
	city, teardrop, strobe := testTracks[0], testTracks[1], testTracks[2]
	next := func(title string) string { return "Play " + title + " next" }
	if _, ok := tt.Find(queueAdd(city.Title)); ok {
		t.Fatal("a tile shows its buttons with the pointer away")
	}
	hoverTile := func(title string) {
		t.Helper()
		r, ok := tt.Find(title)
		if !ok {
			t.Fatalf("no tile of %q; the window shows %q", title, tt.Texts())
		}
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
	}
	hoverTile(city.Title)
	for _, label := range []string{queueAdd(city.Title), next(city.Title)} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q; the window shows %q", label, tt.Texts())
		}
		if r.W != 32 || r.H != 32 {
			t.Errorf("%q is %v by %v, want 32 by 32", label, r.W, r.H)
		}
	}
	click(t, s, tt, queueAdd(city.Title))
	s.settle(t, func(st player.State) bool { cur, ok := st.Current(); return ok && cur.Track.ID == city.ID })
	if !toasted(t, s, tt, "Playing "+city.Title) {
		t.Errorf("no toast of the song playing: %q", tt.Texts())
	}
	hoverTile(teardrop.Title)
	click(t, s, tt, next(teardrop.Title))
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 2 })
	if got, want := queuedIDs(s.p.State()), []string{city.ID, teardrop.ID}; !slices.Equal(got, want) {
		t.Errorf("the queue holds %q, want %q: a tile played its shelf", got, want)
	}
	if !toasted(t, s, tt, teardrop.Title+" plays next") {
		t.Errorf("no toast of the song next: %q", tt.Texts())
	}
	if got := s.router.Path(); got != "/" {
		t.Errorf("a button opened the tile's album: the window is at %q", got)
	}

	// From the keyboard: the tile, then its buttons.
	tt.Move(4, 4)
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	focusOn(t, tt, strobe.Title)
	if _, ok := tt.Find(queueAdd(strobe.Title)); !ok {
		t.Fatal("the tile Tab reached shows no buttons")
	}
	// Tab goes on to Add to queue, then to Play next, which stay.
	press := func(key ui.Key) {
		tt.Key(0, key)
		s.drain()
		tt.Frame()
	}
	press(ui.KeyTab)
	press(ui.KeyEnter)
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 3 && st.Queue[2].Track.ID == strobe.ID })
	press(ui.KeyTab)
	press(ui.KeyEnter)
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 4 && st.Queue[1].Track.ID == strobe.ID })
	if got := s.router.Path(); got != "/" {
		t.Errorf("Enter on a button opened the tile's album: the window is at %q", got)
	}
}

// TestMenuAddToQueuePlays plays what an album's More menu adds to the
// queue while nothing plays, as a track's menu then queues what it adds.
func TestMenuAddToQueuePlays(t *testing.T) {
	s, tt := queueAlbum(t, false)
	choosePage(t, s, tt, "Add to queue")
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == testTracks[0].ID && len(st.Queue) == len(testTracks)
	})
	if !toasted(t, s, tt, "Playing Night Bus") {
		t.Errorf("no toast of the album playing: %q", tt.Texts())
	}
	if err := tt.RightClick(testTracks[4].Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Add to queue"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	s.settle(t, func(st player.State) bool { return len(st.Queue) == len(testTracks)+1 })
	if st := s.p.State(); st.Index != 0 || st.Queue[len(testTracks)].Track.ID != testTracks[4].ID {
		t.Errorf("the queue holds %q at %d", queuedIDs(st), st.Index)
	}
	if !toasted(t, s, tt, "Added "+testTracks[4].Title+" to the queue") {
		t.Errorf("no toast of the song added: %q", tt.Texts())
	}
}
