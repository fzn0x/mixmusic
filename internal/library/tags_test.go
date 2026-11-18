package library

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// tagList returns the tags, in their order, each with its colour and the
// IDs of what it's on.
func tagList(l *Library) []string {
	out := []string{}
	for _, t := range l.Tags() {
		var ids []string
		for _, m := range t.Items {
			ids = append(ids, m.ID)
		}
		out = append(out, t.Name+" ("+t.Color+"): "+strings.Join(ids, " "))
	}
	return out
}

// TestTagName trims a name, makes each run of spaces one, normalises it
// and cuts it to 64 characters.
func TestTagName(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"  Work   out \t", "Work out"},
		{"Ｒｕｎ  fast", "Run fast"},
		{"\n \t", ""},
		{strings.Repeat("é", 70), strings.Repeat("é", 64)},
		{strings.Repeat("a", 63) + " b", strings.Repeat("a", 63)},
	} {
		if got := TagName(c.in); got != c.want {
			t.Errorf("TagName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestTags makes tags, last of them, takes one of the same name but for
// case and spacing as that tag, puts them on items and takes them off,
// renames, recolours, moves and deletes them.
func TestTags(t *testing.T) {
	l := open(t)
	run, err := l.NewTag("  Work   out ", "red")
	if err != nil || run.Name != "Work out" || run.Color != "red" || run.Items == nil {
		t.Fatalf("made %+v, %v", run, err)
	}
	calm, _ := l.NewTag("Calm", "teal")
	same, err := l.NewTag("WORK OUT", "blue")
	if err != nil || same.ID != run.ID || same.Color != "red" {
		t.Errorf("a name of another case made %+v, %v", same, err)
	}
	if _, err := l.NewTag(" \t", "red"); err != ErrTagName {
		t.Errorf("a name of spaces: %v", err)
	}
	mix := Mark{ID: "mx:a", Kind: MarkPlaylist}
	yt := Mark{ID: "yt:PLc", Kind: MarkPlaylist, Title: "Chill", Account: "me"}
	album := Mark{ID: "sp:al", Kind: MarkAlbum, Title: "Blue"}
	if err := l.ChangeTagged(run.ID, []Mark{mix, yt}, nil); err != nil {
		t.Fatal(err)
	}
	if err := l.ChangeTagged(calm.ID, []Mark{yt, album}, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := tagList(l), []string{"Work out (red): mx:a yt:PLc", "Calm (teal): yt:PLc sp:al"}; !slices.Equal(got, want) {
		t.Errorf("the tags are %q, want %q", got, want)
	}
	if got := l.TagsOf("yt:PLc"); !slices.Equal(got, []string{run.ID, calm.ID}) {
		t.Errorf("the playlist has the tags %q", got)
	}
	if got := l.TagsOf("mx:none"); len(got) != 0 {
		t.Errorf("an item untagged has %q", got)
	}

	// Tagged again, an item keeps its place, and what its mark knew.
	if err := l.ChangeTagged(run.ID, []Mark{{ID: "yt:PLc", Kind: MarkPlaylist}}, []string{"mx:a"}); err != nil {
		t.Fatal(err)
	}
	if m := l.Tags()[0].Items; len(m) != 1 || m[0].Account != "me" || m[0].Title != "Chill" {
		t.Errorf("tagged again, the playlist is %+v", m)
	}
	if err := l.RecolorTag(calm.ID, "indigo"); err != nil {
		t.Fatal(err)
	}
	if err := l.MoveTag(calm.ID, run.ID, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := tagList(l), []string{"Calm (indigo): yt:PLc sp:al", "Work out (red): yt:PLc"}; !slices.Equal(got, want) {
		t.Errorf("recoloured and moved, the tags are %q, want %q", got, want)
	}

	// A rename to the name of another tag makes them one.
	into, err := l.RenameTag(run.ID, "Running")
	if err != nil || into != run.ID {
		t.Fatalf("renamed into %q, %v", into, err)
	}
	if err := l.ChangeTagged(run.ID, []Mark{mix}, nil); err != nil {
		t.Fatal(err)
	}
	into, err = l.RenameTag(run.ID, " calm ")
	if err != nil || into != calm.ID {
		t.Fatalf("renamed as another into %q, %v", into, err)
	}
	if got, want := tagList(l), []string{"calm (indigo): yt:PLc sp:al mx:a"}; !slices.Equal(got, want) {
		t.Errorf("renamed as another, the tags are %q, want %q", got, want)
	}
	if _, err := l.RenameTag(run.ID, "x"); err != ErrNoTag {
		t.Errorf("renaming a tag gone: %v", err)
	}
	if err := l.ChangeTagged(run.ID, []Mark{mix}, nil); err != ErrNoTag {
		t.Errorf("tagging with a tag gone: %v", err)
	}
	if err := l.DeleteTag(calm.ID); err != nil || len(l.Tags()) != 0 || len(l.TagsOf("yt:PLc")) != 0 {
		t.Errorf("deleted, the tags are %q, %v", tagList(l), err)
	}
	if err := l.DeleteTag(calm.ID); err != ErrNoTag {
		t.Errorf("deleting a tag gone: %v", err)
	}
}

// TestTagsBound keeps 200 tags at most, and a colour's name short.
func TestTagsBound(t *testing.T) {
	l := open(t)
	for i := range MaxTags {
		if _, err := l.NewTag(fmt.Sprintf("Tag %d", i), "red"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.NewTag("One more", "red"); err != ErrTooManyTags {
		t.Errorf("the tag past the most: %v", err)
	}
	if got, err := l.NewTag("tag 7", "red"); err != nil || got.Name != "Tag 7" {
		t.Errorf("a tag of a name taken, at the most: %+v, %v", got, err)
	}
	tags := l.Tags()
	if err := l.RecolorTag(tags[0].ID, strings.Repeat("x", 100)); err != nil {
		t.Fatal(err)
	}
	if got := l.Tags()[0].Color; len(got) != maxTagColor {
		t.Errorf("the colour kept is %d bytes", len(got))
	}
}

// TestTagsFollowTheLibrary takes the tags off a mix deleted and a
// playlist hidden, and off a playlist of a service only once a whole list
// of its account lacks it, as its pin; a tag on nothing stays, and what
// it's on stays in library.json, opened again.
func TestTagsFollowTheLibrary(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	ids := mixesAt(t, l, "A")
	mid := media.ID(media.SourceLocal, ids["A"])
	mood, _ := l.NewTag("Mood", "red")
	gone, _ := l.NewTag("Gone", "blue")
	tag := func(id string, m ...Mark) {
		t.Helper()
		if err := l.ChangeTagged(id, m, nil); err != nil {
			t.Fatal(err)
		}
	}
	tag(mood.ID, Mark{ID: mid, Kind: MarkPlaylist},
		Mark{ID: "yt:PLkept", Kind: MarkPlaylist, Title: "Old title", Account: "me"},
		Mark{ID: "yt:PLbrand", Kind: MarkPlaylist, Account: "brand"},
		Mark{ID: "yt:PLunknown", Kind: MarkPlaylist},
		Mark{ID: "yt:MPal", Kind: MarkAlbum, Account: "me"},
		Mark{ID: "file:album:a2V5", Kind: MarkAlbum})
	tag(gone.ID, Mark{ID: "yt:PLgone", Kind: MarkPlaylist, Account: "me"}, Mark{ID: "yt:PLhidden", Kind: MarkPlaylist, Account: "me"})

	// A list that came of no account, or of albums, drops no playlist.
	if err := l.KeepListed(media.SourceYouTube, MarkPlaylist, "", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := l.KeepListed(media.SourceYouTube, MarkAlbum, "me", []Mark{{ID: "yt:MPal"}}, nil); err != nil {
		t.Fatal(err)
	}
	listed := []Mark{{ID: "yt:PLkept", Title: "New title"}, {ID: "yt:PLhidden"}}
	if err := l.KeepListed(media.SourceYouTube, MarkPlaylist, "me", listed, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"Mood (red): " + mid + " yt:PLkept yt:PLbrand yt:PLunknown yt:MPal file:album:a2V5", "Gone (blue): yt:PLhidden"}
	if got := tagList(l); !slices.Equal(got, want) {
		t.Errorf("after a whole list, the tags are %q, want %q", got, want)
	}
	if m := l.Tags()[0].Items[1]; m.Title != "New title" {
		t.Errorf("the list's title didn't reach the tag: %+v", m)
	}
	if err := l.SetHidden(Mark{ID: "yt:PLhidden", Kind: MarkPlaylist}, true); err != nil {
		t.Fatal(err)
	}
	if err := l.DeletePlaylist(ids["A"]); err != nil {
		t.Fatal(err)
	}
	want = []string{"Mood (red): yt:PLkept yt:PLbrand yt:PLunknown yt:MPal file:album:a2V5", "Gone (blue): "}
	if got := tagList(l); !slices.Equal(got, want) {
		t.Errorf("hidden and deleted, the tags are %q, want %q", got, want)
	}
	again, err := Open(filepath.Join(dir, "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := tagList(again); !slices.Equal(got, want) {
		t.Errorf("opened again, library.json keeps %q, want %q", got, want)
	}
	if got := again.TagsOf("file:album:a2V5"); !slices.Equal(got, []string{mood.ID}) {
		t.Errorf("opened again, the album has the tags %q", got)
	}
}
