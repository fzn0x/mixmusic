package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// homeReads answers for the services as Home reads the playlists and the
// artists of the shelves the user added: with what the test put in them,
// and an error for the others.
type homeReads struct {
	mu        sync.Mutex
	playlists map[string]media.Playlist
	artists   map[string]media.Artist
	errs      map[string]error
}

// fakeHomeReads has Home read the shelves the user added of the reads
// returned, until the test ends.
func fakeHomeReads(t *testing.T) *homeReads {
	r := &homeReads{playlists: map[string]media.Playlist{}, artists: map[string]media.Artist{}, errs: map[string]error{}}
	playlist, artist := readHomePlaylist, readHomeArtist
	t.Cleanup(func() { readHomePlaylist, readHomeArtist = playlist, artist })
	readHomePlaylist = func(_ *shell, _ context.Context, id string) (media.Playlist, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if p, ok := r.playlists[id]; ok {
			return p, r.errs[id]
		}
		return media.Playlist{}, errors.New("no playlist " + id + " in the test")
	}
	readHomeArtist = func(_ *shell, _ context.Context, id string) (media.Artist, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if a, ok := r.artists[id]; ok {
			return a, r.errs[id]
		}
		return media.Artist{}, errors.New("no artist " + id + " in the test")
	}
	return r
}

// homeShell is a test shell whose YouTube Music home has shelves, in a
// window tall enough to build them all, showing Home.
func homeShell(t *testing.T, feed []media.Shelf) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	return s, homeWindow(t, s, feed)
}

// homeWindow is a window of s, showing Home, whose YouTube Music home
// has shelves.
func homeWindow(t *testing.T, s *shell, feed []media.Shelf) *ui.Tester {
	t.Helper()
	tt := testerOf(t, s, 1280, 2400)
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: feed}
	show(s, tt, "/")
	return tt
}

// homeShelfOrder returns the titles, of those given, of the shelves Home
// shows, from the top down.
func homeShelfOrder(tt *ui.Tester, titles ...string) []string {
	return byTop(tt, titles, func(title string) string { return "Scroll " + title + " on" })
}

// homeRowOrder returns the titles, of those given, of the rows the edit of
// Home lists, from the top down, of shelves of YouTube Music.
func homeRowOrder(tt *ui.Tester, titles ...string) []string {
	return byTop(tt, titles, func(title string) string { return title + ", YouTube Music" })
}

// byTop returns those of titles whose element, labelled label(title),
// shows, from the top down.
func byTop(tt *ui.Tester, titles []string, label func(string) string) []string {
	type at struct {
		title string
		y     float32
	}
	var shown []at
	for _, title := range titles {
		if r, ok := tt.Find(label(title)); ok {
			shown = append(shown, at{title, r.Y})
		}
	}
	slices.SortFunc(shown, func(a, b at) int {
		switch {
		case a.y < b.y:
			return -1
		case a.y > b.y:
			return 1
		}
		return 0
	})
	out := []string{}
	for _, a := range shown {
		out = append(out, a.title)
	}
	return out
}

// homeKept returns how Home is arranged, as settings.json keeps it on
// disk.
func homeKept(t *testing.T, s *shell) []settings.HomeShelf {
	t.Helper()
	data, err := os.ReadFile(s.svc.Preferences.File().Path)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		HomeShelves []settings.HomeShelf `json:"homeShelves"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file.HomeShelves
}

// keysOf returns the keys of an arrangement, those hidden after a "-".
func keysOf(layout []settings.HomeShelf) []string {
	var out []string
	for _, e := range layout {
		if e.Hidden {
			out = append(out, "-"+e.Key)
		} else {
			out = append(out, e.Key)
		}
	}
	return out
}

// arrangeHome keeps layout as Home's arrangement, as an edit does.
func arrangeHome(s *shell, layout ...settings.HomeShelf) {
	s.set.HomeShelves = layout
	s.save()
}

// TestHomeKeepsItsArrangement shows Home's shelves where the user put
// them, without those they hid. A shelf named by its service keeps its
// place when its title changes, as in another language, and the
// service's new shelves come after those placed.
func TestHomeKeepsItsArrangement(t *testing.T) {
	feed := shelves(3, 2)
	feed[1].Key, feed[1].Title = "FEmusic_listen_again", "Listen again"
	s, tt := homeShell(t, feed)
	if got, want := homeShelfOrder(tt, "Shelf 0", "Listen again", "Shelf 2"), []string{"Shelf 0", "Listen again", "Shelf 2"}; !slices.Equal(got, want) {
		t.Fatalf("Home as it comes shows %q, want %q", got, want)
	}

	arrangeHome(s,
		settings.HomeShelf{Key: "youtube:title:Shelf 2", Title: "Shelf 2"},
		settings.HomeShelf{Key: "youtube:FEmusic_listen_again", Title: "Listen again"},
		settings.HomeShelf{Key: "youtube:title:Shelf 0", Title: "Shelf 0", Hidden: true})
	tt.Frame()
	if got, want := homeShelfOrder(tt, "Shelf 0", "Listen again", "Shelf 2"), []string{"Shelf 2", "Listen again"}; !slices.Equal(got, want) {
		t.Errorf("Home arranged shows %q, want %q", got, want)
	}

	// Another day, in another language, with a new shelf before the others.
	next := append(shelves(1, 2), feed...)
	next[0].Title = "New today"
	next[2].Title = "Écouter à nouveau"
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: next}
	tt.Frame()
	if got, want := homeShelfOrder(tt, "New today", "Shelf 0", "Écouter à nouveau", "Shelf 2"), []string{"Shelf 2", "Écouter à nouveau", "New today"}; !slices.Equal(got, want) {
		t.Errorf("the next day, Home shows %q, want %q", got, want)
	}
}

// openHomeEdit opens Edit home, and waits for its rows of titles.
func openHomeEdit(t *testing.T, s *shell, tt *ui.Tester, titles ...string) {
	t.Helper()
	click(t, s, tt, "edit home")
	until(t, s, tt, "the edit's rows", func() bool { return len(homeRowOrder(tt, titles...)) == len(titles) })
}

// TestEditHomeHides hides a shelf, which the list under the others shows
// and settings.json keeps, and shows it again where it was.
func TestEditHomeHides(t *testing.T) {
	s, tt := homeShell(t, shelves(3, 2))
	openHomeEdit(t, s, tt, "Shelf 0", "Shelf 1", "Shelf 2")
	if shelfShows(tt, "Shelf 0") {
		t.Error("the shelves show while Home is edited")
	}
	click(t, s, tt, "Hide Shelf 1")
	if got, want := keysOf(homeKept(t, s)), []string{"youtube:title:Shelf 0", "-youtube:title:Shelf 1", "youtube:title:Shelf 2"}; !slices.Equal(got, want) {
		t.Errorf("hidden, settings.json keeps %q, want %q", got, want)
	}
	if got, want := homeRowOrder(tt, "Shelf 0", "Shelf 1", "Shelf 2"), []string{"Shelf 0", "Shelf 2"}; !slices.Equal(got, want) {
		t.Errorf("hidden, the edit lists %q, want %q", got, want)
	}
	click(t, s, tt, "done")
	if shelfShows(tt, "Shelf 1") || !shelfShows(tt, "Shelf 2") {
		t.Errorf("Home shows the shelf hidden: %q", tt.Texts())
	}

	openHomeEdit(t, s, tt, "Shelf 0", "Shelf 2")
	click(t, s, tt, "Show Shelf 1")
	if got, want := homeRowOrder(tt, "Shelf 0", "Shelf 1", "Shelf 2"), []string{"Shelf 0", "Shelf 1", "Shelf 2"}; !slices.Equal(got, want) {
		t.Errorf("shown again, the edit lists %q, want %q", got, want)
	}
	if got := keysOf(s.svc.Preferences.Get().HomeShelves); slices.Contains(got, "-youtube:title:Shelf 1") {
		t.Errorf("shown again, the settings keep %q", got)
	}
}

// dragHomeRow drags the row of the shelf titled from onto the row of the
// shelf titled onto, in the edit of Home.
func dragHomeRow(t *testing.T, s *shell, tt *ui.Tester, from, onto string) {
	t.Helper()
	a, ok := tt.Find(from + ", YouTube Music")
	b, ok2 := tt.Find(onto + ", YouTube Music")
	if !ok || !ok2 {
		t.Fatalf("the edit doesn't list %q and %q: %q", from, onto, tt.Texts())
	}
	x := a.X + a.W/3
	tt.Press(x, a.Y+a.H/2)
	for step := float32(1); step <= 8; step++ {
		tt.Move(x, a.Y+a.H/2+(b.Y-a.Y)*step/8)
		tt.Frame()
	}
	tt.Release(x, b.Y+b.H/2)
	s.drain()
	tt.Frame()
}

// TestEditHomeDrags drags a shelf onto another, up and down, which it
// takes the place of, in the edit, in settings.json and on Home.
func TestEditHomeDrags(t *testing.T) {
	s, tt := homeShell(t, shelves(3, 2))
	titles := []string{"Shelf 0", "Shelf 1", "Shelf 2"}
	openHomeEdit(t, s, tt, titles...)
	dragHomeRow(t, s, tt, "Shelf 2", "Shelf 0")
	if got, want := homeRowOrder(tt, titles...), []string{"Shelf 2", "Shelf 0", "Shelf 1"}; !slices.Equal(got, want) {
		t.Errorf("dragged up, the edit lists %q, want %q", got, want)
	}
	dragHomeRow(t, s, tt, "Shelf 2", "Shelf 1")
	if got, want := homeRowOrder(tt, titles...), []string{"Shelf 0", "Shelf 1", "Shelf 2"}; !slices.Equal(got, want) {
		t.Errorf("dragged down, the edit lists %q, want %q", got, want)
	}
	dragHomeRow(t, s, tt, "Shelf 0", "Shelf 1")
	want := []string{"youtube:title:Shelf 1", "youtube:title:Shelf 0", "youtube:title:Shelf 2"}
	if got := keysOf(homeKept(t, s)); !slices.Equal(got, want) {
		t.Errorf("settings.json keeps %q, want %q", got, want)
	}
	click(t, s, tt, "done")
	if got, want := homeShelfOrder(tt, titles...), []string{"Shelf 1", "Shelf 0", "Shelf 2"}; !slices.Equal(got, want) {
		t.Errorf("Home shows %q, want %q", got, want)
	}
}

// TestEditHomeKeys moves the shelf whose row has the focus with Alt+Down
// and Alt+Up, which keeps the focus on it as it moves, and no further
// than the top.
func TestEditHomeKeys(t *testing.T) {
	s, tt := homeShell(t, shelves(3, 2))
	titles := []string{"Shelf 0", "Shelf 1", "Shelf 2"}
	openHomeEdit(t, s, tt, titles...)
	row := "Shelf 0, YouTube Music"
	click(t, s, tt, row)
	if info, _ := tt.Access(row); !info.Focused {
		t.Fatal("a click doesn't give the row the focus")
	}
	press := func(key ui.Key) {
		tt.Key(ui.Alt, key)
		s.drain()
		tt.Frame()
		tt.Frame()
	}
	press(ui.KeyDown)
	press(ui.KeyDown)
	if got, want := homeRowOrder(tt, titles...), []string{"Shelf 1", "Shelf 2", "Shelf 0"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Down twice, the edit lists %q, want %q", got, want)
	}
	if info, _ := tt.Access(row); !info.Focused {
		t.Error("the focus didn't follow the shelf down")
	}
	press(ui.KeyUp)
	if got, want := homeRowOrder(tt, titles...), []string{"Shelf 1", "Shelf 0", "Shelf 2"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Up, the edit lists %q, want %q", got, want)
	}
	press(ui.KeyUp)
	press(ui.KeyUp)
	if got, want := keysOf(s.svc.Preferences.Get().HomeShelves), []string{"youtube:title:Shelf 0", "youtube:title:Shelf 1", "youtube:title:Shelf 2"}; !slices.Equal(got, want) {
		t.Errorf("past the top, the settings keep %q, want %q", got, want)
	}
}

// TestEditHomeAdds adds shelves of the user's own: a mix, an artist's
// newest releases, the tracks played most this month and the downloads,
// each first, and removes one.
func TestEditHomeAdds(t *testing.T) {
	s, _ := newKeptShell(t, keptTracks...)
	col := s.svc.Collection
	if _, err := col.CreatePlaylist("Night drive", "", testTracks[2:4]); err != nil {
		t.Fatal(err)
	}
	for _, tr := range []media.Track{testTracks[0], testTracks[5], testTracks[5]} {
		if err := col.RecordPlay(tr); err != nil {
			t.Fatal(err)
		}
	}
	m83 := media.Artist{ID: "yt:UCm83", Source: media.SourceYouTube, Name: "M83",
		Albums:  []media.Album{{ID: "yt:al-hurry", Source: media.SourceYouTube, Title: "Hurry Up, We're Dreaming", Year: "2011"}},
		Singles: []media.Album{{ID: "yt:al-oceans", Source: media.SourceYouTube, Title: "Oceans Niagara", Year: "2023"}}}
	if err := col.SetArtistFollowed(m83, true); err != nil {
		t.Fatal(err)
	}
	s.loads["releases"] = &loaded[[]media.Release]{done: true}
	fakeHomeReads(t).artists[m83.ID] = m83
	tt := homeWindow(t, s, nil)

	click(t, s, tt, "edit home")
	choose := func(path ...string) {
		t.Helper()
		click(t, s, tt, "add a shelf")
		if err := tt.ChooseMenuItem(path...); err != nil {
			t.Fatalf("choosing %q: %v; the menu holds %q", path, err, tt.Menu())
		}
		s.drain()
		tt.Frame()
	}
	choose("One of your mixes", "Night drive")
	choose("Most played this month")
	choose("Downloads")
	choose("An artist's new releases", "New from M83")
	kept := keysOf(homeKept(t, s))
	if len(kept) < 4 || !slices.Equal(kept[:4], []string{homeArtist + m83.ID, homeDownloads, homeTopMonth, homePlaylist + col.Playlists()[0].ID}) {
		t.Errorf("settings.json keeps %q", kept)
	}
	// Recently played is Home's own: it's on Home already, and choosing
	// it hides it, as choosing a shelf added again takes it off.
	choose("Recently played")
	choose("Recently played")
	choose("Most played this month")
	choose("Most played this month")
	kept = keysOf(homeKept(t, s))
	if i := slices.Index(kept, homeTopMonth); i != 0 || slices.Index(kept, homeRecent) != 1 {
		t.Errorf("chosen twice, settings.json keeps %q", kept)
	}
	choose("Recently played")
	if kept = keysOf(homeKept(t, s)); !slices.Contains(kept, "-"+homeRecent) {
		t.Errorf("Recently played chosen while on Home, settings.json keeps %q", kept)
	}
	choose("Recently played")
	choose("Most played this month")
	choose("Most played this month")

	click(t, s, tt, "done")
	titles := []string{"Most played this month", "Recently played", "New from M83", "Downloads", "Night drive"}
	if got := homeShelfOrder(tt, titles...); !slices.Equal(got, titles) {
		t.Errorf("Home shows %q, want %q", got, titles)
	}
	if got := homeShelfOrder(tt, "Your mixes"); len(got) != 1 {
		t.Error("Your mixes doesn't show after the shelves added")
	}
	// The artist's newest release first, the mix's tracks, the downloads
	// and what played most.
	for _, text := range []string{"Oceans Niagara", testTracks[3].Title, keptTracks[1].Title, testTracks[5].Title} {
		if !tt.HasText(text) {
			t.Errorf("Home doesn't show %q: %q", text, tt.Texts())
		}
	}
	if a, ok := tt.Find("Oceans Niagara"); !ok {
		t.Error("no newest single")
	} else if b, ok := tt.Find("Hurry Up, We're Dreaming"); ok && b.X < a.X {
		t.Error("the artist's older album comes before their newest single")
	}

	click(t, s, tt, "edit home")
	click(t, s, tt, "Remove Downloads from Home")
	if got := keysOf(s.svc.Preferences.Get().HomeShelves); slices.Contains(got, homeDownloads) {
		t.Errorf("removed, the settings keep %q", got)
	}
}

// TestHomeAddedPlaylist shows a playlist of the user's library the user
// added, as its page fetched it, and why one couldn't come, with a way to
// try again.
func TestHomeAddedPlaylist(t *testing.T) {
	s := newTestShell(t)
	reads := fakeHomeReads(t)
	reads.playlists["yt:PL1"] = media.Playlist{ID: "yt:PL1", Source: media.SourceYouTube, Title: "Road trip", Tracks: testTracks[:3]}
	reads.playlists["yt:PL2"] = media.Playlist{}
	reads.errs["yt:PL2"] = errors.New("youtube: this playlist is private")
	arrangeHome(s,
		settings.HomeShelf{Key: "playlist:yt:PL1", Title: "Road trip"},
		settings.HomeShelf{Key: "playlist:yt:PL2", Title: "Gone private"})
	tt := homeWindow(t, s, shelves(1, 2))
	until(t, s, tt, "the playlists", func() bool { return tt.HasText("youtube: this playlist is private") && shelfShows(tt, "Shelf 0") })
	if got, want := homeShelfOrder(tt, "Road trip", "Shelf 0"), []string{"Road trip", "Shelf 0"}; !slices.Equal(got, want) {
		t.Errorf("Home shows %q, want %q", got, want)
	}
	if !tt.HasText(testTracks[2].Title) {
		t.Errorf("the playlist's tracks don't show: %q", tt.Texts())
	}
	if !tt.HasText(s.lk.words("Gone private")) {
		t.Errorf("the playlist that failed isn't named: %q", tt.Texts())
	}
	reads.mu.Lock()
	reads.playlists["yt:PL2"] = media.Playlist{ID: "yt:PL2", Source: media.SourceYouTube, Title: "Public again", Tracks: testTracks[3:5]}
	delete(reads.errs, "yt:PL2")
	reads.mu.Unlock()
	click(t, s, tt, "try again")
	until(t, s, tt, "the playlist asked again", func() bool { return shelfShows(tt, "Public again") })
	if tt.HasText("youtube: this playlist is private") {
		t.Errorf("the playlist that came still says why it didn't: %q", tt.Texts())
	}
}

// TestEditHomeResets puts Home back as it comes, once the user said so.
func TestEditHomeResets(t *testing.T) {
	s := newTestShell(t)
	arrangeHome(s,
		settings.HomeShelf{Key: "youtube:title:Shelf 1", Title: "Shelf 1"},
		settings.HomeShelf{Key: "youtube:title:Shelf 0", Title: "Shelf 0", Hidden: true})
	tt := homeWindow(t, s, shelves(2, 2))
	openHomeEdit(t, s, tt, "Shelf 1")
	click(t, s, tt, "reset home")
	click(t, s, tt, "reset")
	if got := homeKept(t, s); got == nil || len(got) != 0 {
		t.Errorf("reset, settings.json keeps %+v", got)
	}
	if got, want := homeRowOrder(tt, "Shelf 0", "Shelf 1"), []string{"Shelf 0", "Shelf 1"}; !slices.Equal(got, want) {
		t.Errorf("reset, the edit lists %q, want %q", got, want)
	}
}
