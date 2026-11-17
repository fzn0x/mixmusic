package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/downloads"
)

// TestLikedDownloadTitle names the download of the liked songs in the
// app's language, though the index keeps it as written, and the others as
// their services name them (#775).
func TestLikedDownloadTitle(t *testing.T) {
	usePseudoLocale(t)
	liked := likedDownload(nil).c
	if got := collectionTitle(liked); !strings.HasPrefix(got, "[") {
		t.Errorf("the liked songs' download is %q", got)
	}
	if liked.Title != "Liked songs" {
		t.Errorf("the index keeps %q", liked.Title)
	}
	if got := collectionTitle(downloads.Collection{Kind: "album", Title: "Mezzanine"}); got != "Mezzanine" {
		t.Errorf("an album's download is %q", got)
	}
}
