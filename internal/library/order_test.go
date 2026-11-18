package library

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// titlesOf returns the titles of playlists, in their order.
func titlesOf(ps []media.Playlist) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Title)
	}
	return out
}

// mixesAt makes the mixes of titles in a library whose clock moves a
// minute at each change, the first made first, and returns their library
// IDs, by title.
func mixesAt(t *testing.T, l *Library, titles ...string) map[string]string {
	t.Helper()
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	l.now = func() time.Time { at = at.Add(time.Minute); return at }
	ids := map[string]string{}
	for _, title := range titles {
		p, err := l.CreatePlaylist(title, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, ids[title], _ = media.ParseID(p.ID)
	}
	return ids
}

// TestEditKeepsTheMixsPlace changes mixes, which stay where they were in
// the user's order, as a new one goes first; what changed is first in
// Recently changed.
func TestEditKeepsTheMixsPlace(t *testing.T) {
	l := open(t)
	ids := mixesAt(t, l, "A", "B", "C")
	if got, want := titlesOf(l.Playlists()), []string{"C", "B", "A"}; !slices.Equal(got, want) {
		t.Fatalf("made, the mixes are %q, want %q", got, want)
	}
	l.AddToPlaylist(ids["A"], []media.Track{tr("yt:a", media.SourceYouTube, "A"), tr("yt:b", media.SourceYouTube, "B")})
	l.MoveInPlaylist(ids["A"], 1, 0)
	l.UpdatePlaylist(ids["B"], "B2", "")
	if got, want := titlesOf(l.Playlists()), []string{"C", "B2", "A"}; !slices.Equal(got, want) {
		t.Errorf("changed, the mixes are %q, want %q", got, want)
	}
	if got, want := titlesOf(l.SortedPlaylists(SortChanged, true)), []string{"B2", "A", "C"}; !slices.Equal(got, want) {
		t.Errorf("recently changed, the mixes are %q, want %q", got, want)
	}
	if got, want := titlesOf(l.SortedPlaylists(SortAdded, true)), []string{"C", "B2", "A"}; !slices.Equal(got, want) {
		t.Errorf("recently added, the mixes are %q, want %q", got, want)
	}
	if got, want := titlesOf(l.SortedPlaylists(SortAdded, false)), []string{"A", "B2", "C"}; !slices.Equal(got, want) {
		t.Errorf("added, oldest first, the mixes are %q, want %q", got, want)
	}
}

// TestSortedMixes sorts mixes by title, numbers by value, and by their
// tracks, both ways, those that sort the same in the user's order, and
// the pinned ones first, in the order of the pins.
func TestSortedMixes(t *testing.T) {
	l := open(t)
	ids := mixesAt(t, l, "mix 10", "Mix 2", "élan", "Zed")
	l.AddToPlaylist(ids["Zed"], []media.Track{tr("yt:a", media.SourceYouTube, "A"), tr("yt:b", media.SourceYouTube, "B")})
	l.AddToPlaylist(ids["mix 10"], []media.Track{tr("yt:a", media.SourceYouTube, "A")})
	for _, c := range []struct {
		sort string
		desc bool
		want []string
	}{
		{SortTitle, false, []string{"élan", "Mix 2", "mix 10", "Zed"}},
		{SortTitle, true, []string{"Zed", "mix 10", "Mix 2", "élan"}},
		{SortTracks, true, []string{"Zed", "mix 10", "élan", "Mix 2"}},
		{SortTracks, false, []string{"élan", "Mix 2", "mix 10", "Zed"}},
		{SortOwn, false, []string{"Zed", "élan", "Mix 2", "mix 10"}},
		{"nonsense", true, []string{"Zed", "élan", "Mix 2", "mix 10"}},
	} {
		if got := titlesOf(l.SortedPlaylists(c.sort, c.desc)); !slices.Equal(got, c.want) {
			t.Errorf("by %s (descending %v): %q, want %q", c.sort, c.desc, got, c.want)
		}
	}
	for _, title := range []string{"Mix 2", "mix 10"} {
		if err := l.SetPinned(Mark{ID: media.ID(media.SourceLocal, ids[title]), Kind: MarkPlaylist, Title: title}, true); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := titlesOf(l.SortedPlaylists(SortTitle, true)), []string{"mix 10", "Mix 2", "Zed", "élan"}; !slices.Equal(got, want) {
		t.Errorf("pinned, by title: %q, want %q", got, want)
	}
	// Deleted, a mix's pin goes.
	if err := l.DeletePlaylist(ids["mix 10"]); err != nil {
		t.Fatal(err)
	}
	if pins := l.Pins(); len(pins) != 1 || pins[0].Title != "Mix 2" || l.IsPinned(media.ID(media.SourceLocal, ids["mix 10"])) {
		t.Errorf("after a delete, the pins are %+v", pins)
	}
}

// TestMoveMixes moves mixes in the user's order, all shown and among
// those a search shows, whose moves keep the places of the others.
func TestMoveMixes(t *testing.T) {
	l := open(t)
	ids := mixesAt(t, l, "E", "D", "C", "B", "A")
	if err := l.MovePlaylist(ids["A"], ids["C"], nil); err != nil {
		t.Fatal(err)
	}
	if got, want := titlesOf(l.Playlists()), []string{"B", "C", "A", "D", "E"}; !slices.Equal(got, want) {
		t.Errorf("A onto C: %q, want %q", got, want)
	}
	if err := l.MovePlaylist(ids["E"], ids["B"], nil); err != nil {
		t.Fatal(err)
	}
	if got, want := titlesOf(l.Playlists()), []string{"E", "B", "C", "A", "D"}; !slices.Equal(got, want) {
		t.Errorf("E onto B: %q, want %q", got, want)
	}
	// A search shows E, C and D: B and A stay where they are.
	if err := l.MovePlaylist(ids["E"], ids["D"], []string{ids["E"], ids["C"], ids["D"]}); err != nil {
		t.Fatal(err)
	}
	if got, want := titlesOf(l.Playlists()), []string{"C", "B", "D", "A", "E"}; !slices.Equal(got, want) {
		t.Errorf("E onto D while searching: %q, want %q", got, want)
	}
	if err := l.MovePlaylist("nope", ids["D"], nil); err == nil {
		t.Error("a mix that isn't moved")
	}
	// A move changes no mix.
	if got := titlesOf(l.SortedPlaylists(SortChanged, true)); got[0] != "A" {
		t.Errorf("recently changed after moves: %q", got)
	}
}

// TestPins pins items, the last first, moves them, among those a search
// shows too, and hides one, which unpins it; pinned again, it shows.
func TestPins(t *testing.T) {
	l := open(t)
	marks := []Mark{
		{ID: "yt:PL1", Kind: MarkPlaylist, Title: "One"},
		{ID: "sp:album2", Kind: MarkAlbum, Title: "Two"},
		{ID: "yt:UC3", Kind: MarkArtist, Title: "Three"},
	}
	for _, m := range marks {
		if err := l.SetPinned(m, true); err != nil {
			t.Fatal(err)
		}
	}
	ids := func() []string {
		var out []string
		for _, m := range l.Pins() {
			out = append(out, m.ID)
		}
		return out
	}
	if got, want := ids(), []string{"yt:UC3", "sp:album2", "yt:PL1"}; !slices.Equal(got, want) {
		t.Fatalf("pinned: %q, want %q", got, want)
	}
	if l.PinAt("sp:album2") != 1 || l.PinAt("yt:none") != -1 {
		t.Error("PinAt doesn't find the pins")
	}
	if err := l.MovePin("yt:UC3", "yt:PL1", nil); err != nil {
		t.Fatal(err)
	}
	if got, want := ids(), []string{"sp:album2", "yt:PL1", "yt:UC3"}; !slices.Equal(got, want) {
		t.Errorf("moved: %q, want %q", got, want)
	}
	if err := l.MovePin("yt:UC3", "sp:album2", []string{"yt:UC3", "sp:album2"}); err != nil {
		t.Fatal(err)
	}
	if got, want := ids(), []string{"yt:UC3", "yt:PL1", "sp:album2"}; !slices.Equal(got, want) {
		t.Errorf("moved while searching: %q, want %q", got, want)
	}
	if err := l.SetHidden(marks[0], true); err != nil {
		t.Fatal(err)
	}
	if !l.IsHidden("yt:PL1") || l.IsPinned("yt:PL1") || len(l.Hidden()) != 1 {
		t.Errorf("hidden: pins %q, hidden %+v", ids(), l.Hidden())
	}
	if err := l.SetPinned(marks[0], true); err != nil {
		t.Fatal(err)
	}
	if l.IsHidden("yt:PL1") || !l.IsPinned("yt:PL1") {
		t.Error("pinned again, the playlist stays hidden")
	}
	if err := l.SetPinned(marks[1], false); err != nil {
		t.Fatal(err)
	}
	if got, want := ids(), []string{"yt:PL1", "yt:UC3"}; !slices.Equal(got, want) {
		t.Errorf("unpinned: %q, want %q", got, want)
	}
}

// TestManyPins keeps 600 pins, in their order, across a restart: they
// have no cap.
func TestManyPins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 600 {
		if err := l.SetPinned(Mark{ID: fmt.Sprintf("yt:PL%d", i), Kind: MarkPlaylist}, true); err != nil {
			t.Fatal(err)
		}
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if pins := again.Pins(); len(pins) != 600 || pins[0].ID != "yt:PL599" || again.PinAt("yt:PL0") != 599 {
		t.Errorf("read back %d pins", len(pins))
	}
}

// TestKeepListed drops the pins and hidden items a whole list of their
// account lacks, and no others: of other accounts, kinds and services,
// those of no account yet, which the list gives one, and those keep
// keeps.
func TestKeepListed(t *testing.T) {
	l := open(t)
	pin := func(m Mark) {
		t.Helper()
		if err := l.SetPinned(m, true); err != nil {
			t.Fatal(err)
		}
	}
	pin(Mark{ID: "yt:PLgone", Kind: MarkPlaylist, Title: "Gone", Account: "me"})
	pin(Mark{ID: "yt:PLkept", Kind: MarkPlaylist, Title: "Old title", Account: "me"})
	pin(Mark{ID: "yt:PLbrand", Kind: MarkPlaylist, Title: "Of the brand channel", Account: "brand"})
	pin(Mark{ID: "yt:PLnew", Kind: MarkPlaylist, Title: "Pinned from its page"})
	pin(Mark{ID: "yt:PLunknown", Kind: MarkPlaylist, Title: "Pinned before any list"})
	pin(Mark{ID: "sp:PLgone", Kind: MarkPlaylist, Title: "Of Spotify", Account: "me"})
	pin(Mark{ID: "yt:MPREalbum", Kind: MarkAlbum, Title: "An album", Account: "me"})
	pin(Mark{ID: "yt:UCfollowed", Kind: MarkArtist, Title: "Followed in MixMusic", Account: "me"})
	if err := l.SetHidden(Mark{ID: "yt:PLhidden", Kind: MarkPlaylist, Title: "Hidden", Account: "me"}, true); err != nil {
		t.Fatal(err)
	}
	if err := l.SetHidden(Mark{ID: "yt:PLhiddenGone", Kind: MarkPlaylist, Account: "me"}, true); err != nil {
		t.Fatal(err)
	}
	// A list of no known account drops nothing.
	if err := l.KeepListed(media.SourceYouTube, MarkPlaylist, "", nil, nil); err != nil || len(l.Pins()) != 8 {
		t.Fatalf("a list of no account: %d pins, %v", len(l.Pins()), err)
	}
	listed := []Mark{{ID: "yt:PLkept", Title: "New title"}, {ID: "yt:PLnew", Title: "Pinned from its page"}, {ID: "yt:PLhidden"}}
	if err := l.KeepListed(media.SourceYouTube, MarkPlaylist, "me", listed, nil); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range l.Pins() {
		got = append(got, m.ID+"@"+m.Account+":"+m.Title)
	}
	want := []string{"yt:UCfollowed@me:Followed in MixMusic", "yt:MPREalbum@me:An album", "sp:PLgone@me:Of Spotify",
		"yt:PLunknown@:Pinned before any list", "yt:PLnew@me:Pinned from its page", "yt:PLbrand@brand:Of the brand channel",
		"yt:PLkept@me:New title"}
	if !slices.Equal(got, want) {
		t.Errorf("after a whole list, the pins are\n%q\nwant\n%q", got, want)
	}
	if h := l.Hidden(); len(h) != 1 || h[0].ID != "yt:PLhidden" || !l.IsHidden("yt:PLhidden") || l.IsHidden("yt:PLhiddenGone") {
		t.Errorf("after a whole list, hidden: %+v", h)
	}
	// The artists the user follows in MixMusic stay.
	if err := l.KeepListed(media.SourceYouTube, MarkArtist, "me", nil, func(id string) bool { return id == "yt:UCfollowed" }); err != nil {
		t.Fatal(err)
	}
	if !l.IsPinned("yt:UCfollowed") {
		t.Error("an artist followed in MixMusic lost its pin")
	}
	if err := l.KeepListed(media.SourceYouTube, MarkArtist, "me", nil, nil); err != nil {
		t.Fatal(err)
	}
	if l.IsPinned("yt:UCfollowed") {
		t.Error("an artist followed nowhere kept its pin")
	}
}
