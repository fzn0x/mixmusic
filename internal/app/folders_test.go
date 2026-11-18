package app

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// TestFoldersFollowWholeLists keeps a playlist of YouTube Music in its
// folder while its list comes in part, or another channel's whole list
// comes, and takes it out once a whole list of its account lacks it: the
// folder, left empty, goes.
func TestFoldersFollowWholeLists(t *testing.T) {
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
	kept1, err := c.NewFolder("Kept", library.Mark{ID: "yt:PLkept", Title: "Old title"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.NewFolder("Gone", library.Mark{ID: "yt:PLgone"}); err != nil {
		t.Fatal(err)
	}
	if m := c.Folders()[1].Items[0]; m.Kind != library.MarkPlaylist || m.Account == "" {
		t.Errorf("the folder keeps %+v, without the kind or the account", m)
	}
	count := func() int { return len(c.Folders()) }

	partial.Store(true)
	if _, err := c.YouTubePlaylists(context.Background()); err == nil {
		t.Fatal("the partial list came whole")
	}
	if count() != 2 {
		t.Errorf("after a partial list, the folders are %+v", c.Folders())
	}
	a.youtube.SetChannel("UCbrand")
	partial.Store(false)
	if _, err := c.YouTubePlaylists(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count() != 2 {
		t.Errorf("after another channel's list, the folders are %+v", c.Folders())
	}
	a.youtube.SetChannel("")
	if _, err := c.YouTubePlaylists(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fs := c.Folders(); len(fs) != 1 || fs[0].ID != kept1.ID || fs[0].Items[0].Title != "Kept" {
		t.Errorf("after a whole list, the folders are %+v", fs)
	}
}

// TestFolderErrorsWorded says the folders' errors in the interface's
// words.
func TestFolderErrorsWorded(t *testing.T) {
	c := &Collection{newQuietApp(t, nil)}
	if _, err := c.NewFolder("\t", library.Mark{ID: "mx:a"}); ErrorWords(err) != locales.FoldersNeedName() {
		t.Errorf("a name of spaces: %v, said %q", err, ErrorWords(err))
	}
	if err := c.RenameFolder("gone", "Name"); ErrorWords(err) != locales.FoldersGone() {
		t.Errorf("a folder gone: %v, said %q", err, ErrorWords(err))
	}
}
