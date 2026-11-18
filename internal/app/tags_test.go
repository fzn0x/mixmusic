package app

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// TestTagsFollowWholeLists keeps the tag of a playlist of YouTube Music
// while its list comes in part, or another channel's whole list comes,
// and takes it off once a whole list of its account lacks it: the tag,
// on nothing then, stays.
func TestTagsFollowWholeLists(t *testing.T) {
	quickPageWaits(t)
	var partial atomic.Bool
	kept := youtubeCard("MUSIC_PAGE_TYPE_PLAYLIST", "VLPLkept", "Kept")
	yt := partialYouTube(t, &partial, []any{kept}, []any{kept})
	a := newQuietApp(t, nil)
	a.youtube.BaseURL = yt.URL
	a.youtube.SetCookies(testCookies)
	a.mu.Lock()
	a.youtubeMe = &youtube.Account{Name: "Ana", Handle: "@ana"}
	a.mu.Unlock()
	c := &Collection{a}
	mood, err := c.NewTag("Mood", "teal")
	if err != nil {
		t.Fatal(err)
	}
	gone, _ := c.NewTag("Gone", "red")
	if err := c.ChangeTagged(mood.ID, []library.Mark{{ID: "yt:PLkept", Kind: library.MarkPlaylist, Title: "Old title"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.ChangeTagged(gone.ID, []library.Mark{{ID: "yt:PLgone", Kind: library.MarkPlaylist}}, nil); err != nil {
		t.Fatal(err)
	}
	if m := c.Tags()[1].Items[0]; m.Kind != library.MarkPlaylist || m.Account == "" {
		t.Errorf("the tag keeps %+v, without the kind or the account", m)
	}
	on := func() int { return len(c.TagsOf("yt:PLkept")) + len(c.TagsOf("yt:PLgone")) }

	partial.Store(true)
	if _, err := c.YouTubePlaylists(context.Background()); err == nil {
		t.Fatal("the partial list came whole")
	}
	if on() != 2 {
		t.Errorf("after a partial list, the tags are %+v", c.Tags())
	}
	a.youtube.SetChannel("UCbrand")
	partial.Store(false)
	if _, err := c.YouTubePlaylists(context.Background()); err != nil {
		t.Fatal(err)
	}
	if on() != 2 {
		t.Errorf("after another channel's list, the tags are %+v", c.Tags())
	}
	a.youtube.SetChannel("")
	if _, err := c.YouTubePlaylists(context.Background()); err != nil {
		t.Fatal(err)
	}
	tags := c.Tags()
	if len(tags) != 2 || len(tags[0].Items) != 1 || tags[0].Items[0].Title != "Kept" || len(tags[1].Items) != 0 {
		t.Errorf("after a whole list, the tags are %+v", tags)
	}
}

// TestTagErrorsWorded says the tags' errors in the interface's words.
func TestTagErrorsWorded(t *testing.T) {
	c := &Collection{newQuietApp(t, nil)}
	if _, err := c.NewTag("\t", "red"); ErrorWords(err) != locales.LibraryTagsNeedName() {
		t.Errorf("a name of spaces: %v, said %q", err, ErrorWords(err))
	}
	if _, err := c.RenameTag("gone", "Name"); ErrorWords(err) != locales.LibraryTagsGone() {
		t.Errorf("a tag gone: %v, said %q", err, ErrorWords(err))
	}
	for i := range library.MaxTags {
		if _, err := c.NewTag(fmt.Sprint("Tag ", i), "red"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.NewTag("One more", "red"); ErrorWords(err) != locales.LibraryTagsTooMany(library.MaxTags) {
		t.Errorf("a tag past the most: %v, said %q", err, ErrorWords(err))
	}
}
