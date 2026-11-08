package downloads

import (
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestVideoFile tells the file of a video downloaded, for a track of
// Spotify too, which plays its recording's.
func TestVideoFile(t *testing.T) {
	dir := t.TempDir()
	video := yt("fffffffffff", "Teardrop")
	m := open(t, dir, Options{Fetcher: &fetcher{}, Matcher: &matcher{matches: map[string]media.Track{"sp:tear": video}}})
	m.Start()
	m.Add([]media.Track{sp("tear", "Teardrop")}, "best")
	until(t, "the download", func() bool { return kind(m, "sp:tear") == Done })
	if p, ok := m.Snapshot().VideoFile("fffffffffff"); !ok || p != filepath.Join(dir, "audio", "fffffffffff.webm") {
		t.Errorf("the file %q, %v", p, ok)
	}
	if p, ok := m.Snapshot().VideoFile("ggggggggggg"); ok {
		t.Errorf("a video not downloaded: %q", p)
	}
	var none *Snapshot
	if _, ok := none.VideoFile("fffffffffff"); ok {
		t.Error("a nil snapshot holds a file")
	}
}
