package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// pickTracks are the made-up tracks of the album the tests select in.
var pickTracks = []media.Track{
	testTrack("yt:pick0000001", "Paper Lanterns", "Ola Brine", "Harbour Lights", media.SourceYouTube),
	testTrack("yt:pick0000002", "Slow Ferry", "Ola Brine", "Harbour Lights", media.SourceYouTube),
	testTrack("yt:pick0000003", "Tin Roof Rain", "Ola Brine", "Harbour Lights", media.SourceYouTube),
	testTrack("yt:pick0000004", "Lighthouse Keeper", "Ola Brine", "Harbour Lights", media.SourceYouTube),
	testTrack("yt:pick0000005", "Gulls at Dawn", "Ola Brine", "Harbour Lights", media.SourceYouTube),
}

// pickPage shows the page of an album of pickTracks.
func pickPage(t *testing.T) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	s.loads["album:al-harbour"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-harbour", Title: "Harbour Lights",
		Source: media.SourceYouTube, Tracks: pickTracks}}
	tt := tester(t, s)
	show(s, tt, "/album/al-harbour")
	until(t, s, tt, "the album's tracks", func() bool { return tt.HasText(pickTracks[4].Title) })
	return s, tt
}

// clickRow clicks the row of the track titled title with mods held.
func clickRow(t *testing.T, s *shell, tt *ui.Tester, mods ui.Modifiers, title string) {
	t.Helper()
	if err := tt.ClickWith(mods, title); err != nil {
		t.Fatalf("clicking %q: %v; the window shows %q", title, err, tt.Texts())
	}
	s.drain()
	tt.Frame()
	// The next click is one of its own, not the second of a double click.
	tt.Advance(time.Second)
}

// picked are the titles of the tracks selected, in their list's order.
func picked(s *shell) []string {
	var out []string
	for _, t := range s.picks.inOrder() {
		out = append(out, t.Title)
	}
	return out
}

// titlesOf are the titles of pickTracks at is.
func titlesOf(is ...int) []string {
	var out []string
	for _, i := range is {
		out = append(out, pickTracks[i].Title)
	}
	return out
}

// TestSelectTracks selects rows with Ctrl or ⌘, a plain click once some
// are, and Shift, which the bar counts, then adds them to the queue in
// their list's order, and Escape lets them go.
func TestSelectTracks(t *testing.T) {
	s, tt := pickPage(t)
	// A plain click selects nothing while nothing is.
	clickRow(t, s, tt, 0, pickTracks[3].Title)
	if len(s.picks.ids) != 0 || tt.HasText("1 selected") {
		t.Fatalf("a plain click selected %q", picked(s))
	}
	clickRow(t, s, tt, ui.Cmd, pickTracks[3].Title)
	if got, want := picked(s), titlesOf(3); !slices.Equal(got, want) || !tt.HasText("1 selected") {
		t.Fatalf("selected %q, want %q; the window shows %q", got, want, tt.Texts())
	}
	if _, ok := tt.Access("Select " + pickTracks[0].Title); !ok {
		t.Error("the rows show no check boxes")
	}
	// Once one is, a plain click selects another.
	clickRow(t, s, tt, 0, pickTracks[1].Title)
	// Shift selects from the one clicked last.
	clickRow(t, s, tt, ui.Shift, pickTracks[0].Title)
	if got, want := picked(s), titlesOf(0, 1, 3); !slices.Equal(got, want) || !tt.HasText("3 selected") {
		t.Fatalf("selected %q, want %q; the window shows %q", got, want, tt.Texts())
	}
	// Ctrl or ⌘ lets one go.
	clickRow(t, s, tt, ui.Cmd, pickTracks[1].Title)
	if got, want := picked(s), titlesOf(0, 3); !slices.Equal(got, want) {
		t.Fatalf("selected %q, want %q", got, want)
	}
	click(t, s, tt, "Add the selected tracks to the queue")
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 2 })
	var queued []string
	for _, it := range s.p.State().Queue {
		queued = append(queued, it.Track.Title)
	}
	if want := titlesOf(0, 3); !slices.Equal(queued, want) {
		t.Errorf("the queue holds %q, want %q", queued, want)
	}
	if !tt.HasText("Added 2 tracks to the queue") {
		t.Errorf("the window shows %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	s.drain()
	tt.Frame()
	if len(s.picks.ids) != 0 || tt.HasText("2 selected") {
		t.Errorf("after Escape, %q are selected; the window shows %q", picked(s), tt.Texts())
	}
	if _, ok := tt.Access("Select " + pickTracks[0].Title); ok {
		t.Error("the check boxes stay")
	}
}

// TestSelectFromMenu starts a selection with Select in a track's menu,
// goes on with the rows' check boxes, Shift on one too, and lets a track
// go with Deselect and all of them with the bar's close button.
func TestSelectFromMenu(t *testing.T) {
	s, tt := pickPage(t)
	menu := func(title, item string) {
		t.Helper()
		if err := tt.RightClick(title); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(item); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		s.drain()
		tt.Frame()
	}
	menu(pickTracks[1].Title, "Select")
	if got, want := picked(s), titlesOf(1); !slices.Equal(got, want) {
		t.Fatalf("selected %q, want %q", got, want)
	}
	click(t, s, tt, "Select "+pickTracks[2].Title)
	if err := tt.ClickWith(ui.Shift, "Select "+pickTracks[4].Title); err != nil {
		t.Fatal(err)
	}
	s.drain()
	tt.Frame()
	if got, want := picked(s), titlesOf(1, 2, 3, 4); !slices.Equal(got, want) || !tt.HasText("4 selected") {
		t.Fatalf("selected %q, want %q; the window shows %q", got, want, tt.Texts())
	}
	// A check box clicked again lets its track go.
	click(t, s, tt, "Select "+pickTracks[3].Title)
	menu(pickTracks[2].Title, "Deselect")
	if got, want := picked(s), titlesOf(1, 4); !slices.Equal(got, want) {
		t.Fatalf("selected %q, want %q", got, want)
	}
	click(t, s, tt, "Clear the selection")
	if len(s.picks.ids) != 0 || tt.HasText("2 selected") {
		t.Errorf("after the close button, %q are selected", picked(s))
	}
}

// TestSelectAll selects every track the list shows from the bar, and
// another page lets them go.
func TestSelectAll(t *testing.T) {
	s, tt := pickPage(t)
	clickRow(t, s, tt, ui.Cmd, pickTracks[2].Title)
	click(t, s, tt, "select all")
	if got, want := picked(s), titlesOf(0, 1, 2, 3, 4); !slices.Equal(got, want) || !tt.HasText("5 selected") {
		t.Fatalf("selected %q, want %q; the window shows %q", got, want, tt.Texts())
	}
	if tt.HasText("select all") {
		t.Error("select all shows with all selected")
	}
	show(s, tt, "/library")
	if len(s.picks.ids) != 0 || tt.HasText("5 selected") {
		t.Errorf("on another page, %q are selected", picked(s))
	}
	show(s, tt, "/album/al-harbour")
	if len(s.picks.ids) != 0 || tt.HasText("5 selected") {
		t.Errorf("back on the album, %q are selected", picked(s))
	}
}

// TestSelectToMix adds the tracks selected to a mix of the user's from the
// bar's Add to mix.
func TestSelectToMix(t *testing.T) {
	s, tt := pickPage(t)
	mix, err := s.svc.Collection.CreatePlaylist("Late Bus", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	clickRow(t, s, tt, ui.Cmd, pickTracks[4].Title)
	clickRow(t, s, tt, ui.Cmd, pickTracks[2].Title)
	click(t, s, tt, "Add the selected tracks to a mix")
	if err := tt.ChooseMenuItem("Late Bus"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	mixTracks := func() []media.Track {
		p, err := s.svc.Catalog.PlaylistStart(context.Background(), mix.ID)
		if err != nil {
			t.Fatal(err)
		}
		return p.Tracks
	}
	until(t, s, tt, "the mix's tracks", func() bool { return len(mixTracks()) == 2 })
	var got []string
	for _, tr := range mixTracks() {
		got = append(got, tr.Title)
	}
	if want := titlesOf(2, 4); !slices.Equal(got, want) {
		t.Errorf("the mix holds %q, want %q", got, want)
	}
}

// TestSelectDownload downloads the tracks selected that aren't, and the
// bar's Download goes once they all wait.
func TestSelectDownload(t *testing.T) {
	s, tt := pickPage(t)
	// The downloads wait, so that nothing asks YouTube.
	s.svc.Downloads.SetPaused(true)
	clickRow(t, s, tt, ui.Cmd, pickTracks[0].Title)
	clickRow(t, s, tt, ui.Shift, pickTracks[2].Title)
	click(t, s, tt, "Download the selected tracks")
	until(t, s, tt, "the downloads", func() bool {
		for _, tr := range pickTracks[:3] {
			if s.kept.State(tr.ID).Kind != downloads.Waiting {
				return false
			}
		}
		return true
	})
	if k := s.kept.State(pickTracks[3].ID).Kind; k != downloads.None {
		t.Errorf("a track not selected is %v", k)
	}
	if _, ok := tt.Access("Download the selected tracks"); ok {
		t.Error("Download stays with all of them waiting")
	}
}

// TestPickActionsInOrder keeps the bar's actions in their orders, and
// refuses two at one.
func TestPickActionsInOrder(t *testing.T) {
	saved := slices.Clone(pickActions)
	t.Cleanup(func() { pickActions = saved })
	for i := 1; i < len(pickActions); i++ {
		if pickActions[i-1].order >= pickActions[i].order {
			t.Errorf("the actions are out of order: %d before %d", pickActions[i-1].order, pickActions[i].order)
		}
	}
	addPickAction(15, func(*shell, *ui.Context, []media.Track) {})
	if pickActions[1].order != 15 {
		t.Errorf("an action at 15 went to %d", slices.IndexFunc(pickActions, func(a pickAction) bool { return a.order == 15 }))
	}
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "two actions") {
			t.Errorf("two actions at one order: %v", r)
		}
	}()
	addPickAction(15, func(*shell, *ui.Context, []media.Track) {})
}

// TestSelectDoubleClick plays a list from a row double-clicked, but while
// the list has tracks selected, when a double click is two clicks that
// select the row and let it go.
func TestSelectDoubleClick(t *testing.T) {
	s, tt := pickPage(t)
	clickRow(t, s, tt, ui.Cmd, pickTracks[0].Title)
	for range 2 {
		if err := tt.Click(pickTracks[2].Title); err != nil {
			t.Fatal(err)
		}
		s.drain()
		tt.Frame()
	}
	if got, want := picked(s), titlesOf(0); !slices.Equal(got, want) {
		t.Errorf("after a double click, %q are selected, want %q", got, want)
	}
	if q := s.p.State().Queue; len(q) != 0 {
		t.Errorf("a double click while selecting played %d tracks", len(q))
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(time.Second)
	for range 2 {
		if err := tt.Click(pickTracks[2].Title); err != nil {
			t.Fatal(err)
		}
		s.drain()
		tt.Frame()
	}
	s.settle(t, func(st player.State) bool { return len(st.Queue) == len(pickTracks) && st.Index == 2 })
	if len(s.picks.ids) != 0 {
		t.Errorf("a double click selected %q", picked(s))
	}
}

// TestSelectInMix selects in a mix, whose rows also drag: a click with
// Ctrl or ⌘ selects, and doesn't move the track.
func TestSelectInMix(t *testing.T) {
	s := newTestShell(t)
	mix, err := s.svc.Collection.CreatePlaylist("Night Shift", "", pickTracks[:3])
	if err != nil {
		t.Fatal(err)
	}
	tt := tester(t, s)
	show(s, tt, playlistPath(mix.ID))
	until(t, s, tt, "the mix's tracks", func() bool { return tt.HasText(pickTracks[2].Title) })
	clickRow(t, s, tt, ui.Cmd, pickTracks[1].Title)
	clickRow(t, s, tt, ui.Cmd, pickTracks[2].Title)
	if got, want := picked(s), titlesOf(1, 2); !slices.Equal(got, want) || !tt.HasText("2 selected") {
		t.Fatalf("selected %q, want %q; the window shows %q", got, want, tt.Texts())
	}
	p, err := s.svc.Catalog.PlaylistStart(context.Background(), mix.ID)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, tr := range p.Tracks {
		order = append(order, tr.Title)
	}
	if want := titlesOf(0, 1, 2); !slices.Equal(order, want) {
		t.Errorf("the mix holds %q, want %q", order, want)
	}
}
