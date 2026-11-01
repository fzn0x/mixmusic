package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestDisliked keeps disliked tracks of both services, the last disliked
// first: a track is liked or disliked, not both, and a reopened library
// still has them.
func TestDisliked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b := tr("yt:a", media.SourceYouTube, "A"), tr("sp:b", media.SourceSpotify, "B")
	if err := l.SetLiked(a, true); err != nil {
		t.Fatal(err)
	}
	unliked, err := l.SetDisliked(a, true)
	if err != nil || !unliked {
		t.Fatalf("disliking a liked track: unliked %v, %v", unliked, err)
	}
	if l.IsLiked("yt:a") || !l.IsDisliked("yt:a") {
		t.Errorf("disliking a liked track left it liked %v, disliked %v", l.IsLiked("yt:a"), l.IsDisliked("yt:a"))
	}
	if unliked, err := l.SetDisliked(b, true); err != nil || unliked {
		t.Fatalf("disliking a track not liked: unliked %v, %v", unliked, err)
	}
	if d := l.Disliked(); len(d) != 2 || d[0].ID != "sp:b" || d[1].ID != "yt:a" || d[1].Title != a.Title {
		t.Errorf("disliked = %+v", d)
	}

	// Liking a disliked track takes the dislike back.
	if err := l.SetLiked(b, true); err != nil {
		t.Fatal(err)
	}
	if l.IsDisliked("sp:b") || !l.IsLiked("sp:b") {
		t.Error("liking a disliked track left it disliked")
	}
	// Unliking takes no dislike back, and leaves no other track.
	if err := l.SetLiked(b, false); err != nil {
		t.Fatal(err)
	}
	if !l.IsDisliked("yt:a") {
		t.Error("unliking another track took the dislike of yt:a back")
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if d := reopened.Disliked(); len(d) != 1 || d[0].ID != "yt:a" || !reopened.IsDisliked("yt:a") {
		t.Errorf("reopened, disliked = %+v", d)
	}
	if _, err := reopened.SetDisliked(a, false); err != nil {
		t.Fatal(err)
	}
	if reopened.IsDisliked("yt:a") || len(reopened.Disliked()) != 0 || reopened.IsLiked("yt:a") {
		t.Error("taking the dislike back left it disliked, or liked it")
	}
}

// TestDislikedOfOldLibrary opens a library.json kept before dislikes, and
// leaves the file without them while there are none.
func TestDislikedOfOldLibrary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	if err := os.WriteFile(path, []byte(`{"liked":[{"id":"yt:a","title":"A"}],"playlists":[],"history":[],"searches":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if d := l.Disliked(); d == nil || len(d) != 0 || l.IsDisliked("yt:a") || !l.IsLiked("yt:a") {
		t.Errorf("disliked = %+v", d)
	}
	if err := l.AddSearch("x"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "disliked") {
		t.Errorf("library.json holds disliked without any: %s", data)
	}
}
