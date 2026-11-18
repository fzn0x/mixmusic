package localfiles

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestMoveID moves the IDs of a file's track and of a song with copies,
// numbered or not, and leaves those of albums and of files move leaves.
func TestMoveID(t *testing.T) {
	from, to := filepath.Join(t.TempDir(), "old"), filepath.Join(t.TempDir(), "new")
	move := func(p string) (string, bool) {
		rest, ok := strings.CutPrefix(p, from)
		if !ok {
			return "", false
		}
		return to + rest, true
	}
	old, moved := filepath.Join(from, "a.mp3"), filepath.Join(to, "a.mp3")
	if got, ok := MoveID(TrackID(old), move); !ok || got != TrackID(moved) {
		t.Errorf("track: %q %v", got, ok)
	}
	if got, ok := MoveID(SongID(old), move); !ok || got != SongID(moved) {
		t.Errorf("song: %q %v", got, ok)
	}
	if got, ok := MoveID(SongID(old)+".2", move); !ok || got != SongID(moved)+".2" {
		t.Errorf("numbered song: %q %v", got, ok)
	}
	for _, id := range []string{albumID("x"), artistID("y"), TrackID(filepath.Join(t.TempDir(), "b.mp3")), "yt:abc"} {
		if got, ok := MoveID(id, move); ok {
			t.Errorf("%s moved to %s", id, got)
		}
	}
	links, _ := json.Marshal(savedLinks{Version: 1, Songs: []savedSong{{ID: SongID(old), Copies: []string{old}, Linked: []string{old}}},
		Apart: []string{old}, Facts: []savedFacts{{Path: old}}})
	data, err := MoveLinks(links, move)
	if err != nil {
		t.Fatal(err)
	}
	var s savedLinks
	json.Unmarshal(data, &s)
	s = decodePaths(s)
	if s.Songs[0].ID != SongID(moved) || s.Songs[0].Copies[0] != moved || s.Songs[0].Linked[0] != moved || s.Apart[0] != moved || s.Facts[0].Path != moved {
		t.Errorf("links moved to %s", data)
	}
}
