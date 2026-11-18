package library

import (
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestFolderName trims a name, makes each run of spaces one, normalises
// it and cuts it to 64 characters.
func TestFolderName(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"  Road   trips \t", "Road trips"},
		{"Ｃｈｉｌｌ  mix", "Chill mix"},
		{"\n \t", ""},
		{strings.Repeat("é", 70), strings.Repeat("é", 64)},
		{strings.Repeat("a", 63) + " b", strings.Repeat("a", 63)},
	} {
		if got := FolderName(c.in); got != c.want {
			t.Errorf("FolderName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// folderNames returns the names of the folders, as they list, with the
// IDs of what each holds.
func folderNames(l *Library) []string {
	out := []string{}
	for _, f := range l.Folders() {
		var ids []string
		for _, m := range f.Items {
			ids = append(ids, m.ID)
		}
		out = append(out, f.Name+": "+strings.Join(ids, " "))
	}
	return out
}

// TestFolders files playlists in folders, which list by name, takes one
// of the same name, but for case and spacing, as that folder, moves them
// between folders, renames and ungroups them, and lets a folder go once
// it holds nothing.
func TestFolders(t *testing.T) {
	l := open(t)
	mix := func(id string) Mark { return Mark{ID: "mx:" + id, Kind: MarkPlaylist} }
	road, err := l.NewFolder("  Road   trips ", mix("a"))
	if err != nil || road.Name != "Road trips" || len(road.Items) != 1 {
		t.Fatalf("made %+v, %v", road, err)
	}
	if _, err := l.NewFolder("chill", Mark{ID: "yt:PLc", Kind: MarkPlaylist, Title: "Chill", Account: "me"}); err != nil {
		t.Fatal(err)
	}
	same, err := l.NewFolder("ROAD TRIPS", mix("b"))
	if err != nil || same.ID != road.ID {
		t.Errorf("a name of another case made %+v, %v", same, err)
	}
	if _, err := l.NewFolder(" \t", mix("c")); err != ErrFolderName {
		t.Errorf("a name of spaces: %v", err)
	}
	if got, want := folderNames(l), []string{"chill: yt:PLc", "Road trips: mx:a mx:b"}; !slices.Equal(got, want) {
		t.Errorf("the folders are %q, want %q", got, want)
	}
	if l.FolderOf("mx:b") != road.ID || l.FolderOf("mx:c") != "" {
		t.Errorf("mx:b is in %q, mx:c in %q", l.FolderOf("mx:b"), l.FolderOf("mx:c"))
	}

	// Filed again, a playlist moves, and keeps what its mark knew.
	chill := l.FolderOf("yt:PLc")
	if err := l.FileIn(road.ID, Mark{ID: "yt:PLc", Kind: MarkPlaylist}); err != nil {
		t.Fatal(err)
	}
	if got, want := folderNames(l), []string{"Road trips: mx:a mx:b yt:PLc"}; !slices.Equal(got, want) {
		t.Errorf("chill emptied, the folders are %q, want %q", got, want)
	}
	if m := l.Folders()[0].Items[2]; m.Account != "me" || m.Title != "Chill" {
		t.Errorf("the playlist moved lost its account or title: %+v", m)
	}
	if err := l.FileIn(chill, mix("c")); err != ErrNoFolder {
		t.Errorf("filing in a folder gone: %v", err)
	}
	if err := l.MoveFiled(road.ID, "yt:PLc", "mx:a", nil); err != nil {
		t.Fatal(err)
	}
	if err := l.Unfile("mx:b"); err != nil {
		t.Fatal(err)
	}
	if got, want := folderNames(l), []string{"Road trips: yt:PLc mx:a"}; !slices.Equal(got, want) {
		t.Errorf("moved and unfiled, the folders are %q, want %q", got, want)
	}

	// A rename to the name of another folder makes them one.
	other, _ := l.NewFolder("Autumn", mix("b"))
	if err := l.RenameFolder(other.ID, "Rainy days"); err != nil {
		t.Fatal(err)
	}
	if err := l.RenameFolder(other.ID, "road  TRIPS"); err != nil {
		t.Fatal(err)
	}
	if got, want := folderNames(l), []string{"road TRIPS: yt:PLc mx:a mx:b"}; !slices.Equal(got, want) {
		t.Errorf("renamed as another, the folders are %q, want %q", got, want)
	}
	if err := l.RenameFolder(other.ID, "x"); err != ErrNoFolder {
		t.Errorf("renaming a folder gone: %v", err)
	}
	if err := l.UngroupFolder(road.ID); err != nil || len(l.Folders()) != 0 || l.FolderOf("mx:a") != "" {
		t.Errorf("ungrouped, the folders are %q, %v", folderNames(l), err)
	}
}

// TestMoveFolders lists the folders by name until one moves, and keeps
// the order the user put them in from then on, a new one first.
func TestMoveFolders(t *testing.T) {
	l := open(t)
	ids := map[string]string{}
	for i, name := range []string{"Banjo", "apples", "Cello", "Drums"} {
		f, err := l.NewFolder(name, Mark{ID: "mx:" + string(rune('a'+i)), Kind: MarkPlaylist})
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = f.ID
	}
	names := func() []string {
		out := []string{}
		for _, f := range l.Folders() {
			out = append(out, f.Name)
		}
		return out
	}
	if got, want := names(), []string{"apples", "Banjo", "Cello", "Drums"}; !slices.Equal(got, want) {
		t.Fatalf("by name, the folders are %q, want %q", got, want)
	}
	// Moved among those a search shows, the others keep their places.
	if err := l.MoveFolder(ids["Drums"], ids["apples"], []string{ids["apples"], ids["Drums"]}); err != nil {
		t.Fatal(err)
	}
	if got, want := names(), []string{"Drums", "Banjo", "Cello", "apples"}; !slices.Equal(got, want) {
		t.Errorf("moved, the folders are %q, want %q", got, want)
	}
	if _, err := l.NewFolder("Accordion", Mark{ID: "mx:z", Kind: MarkPlaylist}); err != nil {
		t.Fatal(err)
	}
	if got, want := names(), []string{"Accordion", "Drums", "Banjo", "Cello", "apples"}; !slices.Equal(got, want) {
		t.Errorf("a new one, the folders are %q, want %q", got, want)
	}
	if err := l.MoveFolder("gone", ids["Drums"], nil); err != ErrNoFolder {
		t.Errorf("moving a folder gone: %v", err)
	}
}

// TestFoldersFollowTheLibrary takes a mix deleted, and a playlist hidden,
// out of their folders, and a playlist of a service only once a whole
// list of its account lacks it, as its pin.
func TestFoldersFollowTheLibrary(t *testing.T) {
	l := open(t)
	ids := mixesAt(t, l, "A")
	mid := media.ID(media.SourceLocal, ids["A"])
	file := func(name string, m Mark) {
		t.Helper()
		if _, err := l.NewFolder(name, m); err != nil {
			t.Fatal(err)
		}
	}
	file("Mixes", Mark{ID: mid, Kind: MarkPlaylist})
	file("Mixes", Mark{ID: "yt:PLkept", Kind: MarkPlaylist, Title: "Old title", Account: "me"})
	file("Gone", Mark{ID: "yt:PLgone", Kind: MarkPlaylist, Account: "me"})
	file("Brand", Mark{ID: "yt:PLbrand", Kind: MarkPlaylist, Account: "brand"})
	file("Brand", Mark{ID: "yt:PLunknown", Kind: MarkPlaylist})
	file("Hidden", Mark{ID: "yt:PLhidden", Kind: MarkPlaylist, Account: "me"})

	if err := l.KeepListed(media.SourceYouTube, MarkPlaylist, "", nil, nil); err != nil || len(l.Folders()) != 4 {
		t.Fatalf("a list of no account: %q, %v", folderNames(l), err)
	}
	if err := l.KeepListed(media.SourceYouTube, MarkAlbum, "me", nil, nil); err != nil || len(l.Folders()) != 4 {
		t.Fatalf("a list of albums: %q, %v", folderNames(l), err)
	}
	listed := []Mark{{ID: "yt:PLkept", Title: "New title"}, {ID: "yt:PLhidden"}}
	if err := l.KeepListed(media.SourceYouTube, MarkPlaylist, "me", listed, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := folderNames(l), []string{"Brand: yt:PLbrand yt:PLunknown", "Hidden: yt:PLhidden", "Mixes: " + mid + " yt:PLkept"}; !slices.Equal(got, want) {
		t.Errorf("after a whole list, the folders are %q, want %q", got, want)
	}
	if m := l.Folders()[2].Items[1]; m.Title != "New title" {
		t.Errorf("the list's title didn't reach the folder: %+v", m)
	}
	if err := l.SetHidden(Mark{ID: "yt:PLhidden", Kind: MarkPlaylist}, true); err != nil {
		t.Fatal(err)
	}
	if err := l.DeletePlaylist(ids["A"]); err != nil {
		t.Fatal(err)
	}
	if got, want := folderNames(l), []string{"Brand: yt:PLbrand yt:PLunknown", "Mixes: yt:PLkept"}; !slices.Equal(got, want) {
		t.Errorf("hidden and deleted, the folders are %q, want %q", got, want)
	}
}
