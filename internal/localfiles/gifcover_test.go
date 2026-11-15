package localfiles

import (
	"path/filepath"
	"testing"
	"time"
)

// TestGIFCoverBeside takes a cover.gif, or a GIF of another cover name,
// beside files that hold no cover, as their album's cover, which moves on
// the now playing page (#569); a still of the same name comes first.
func TestGIFCoverBeside(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "A/1.mp3", "One|Ada Field|Kites", time.Hour)
	ti.write(t, "A/cover.gif", "GIF89a a cover that moves", time.Hour)
	ti.write(t, "B/1.mp3", "Two|Orin Vale|Paper", time.Hour)
	ti.write(t, "B/Folder.GIF", "GIF89a another", time.Hour)
	ti.write(t, "C/1.mp3", "Three|Mira Holt|Glass", time.Hour)
	ti.write(t, "C/cover.gif", "GIF89a beside a still", time.Hour)
	ti.write(t, "C/cover.jpg", "\xff\xd8\xff the still", time.Hour)
	ti.scan(t)
	want := map[string]string{"One": ".gif", "Two": ".gif", "Three": ".jpg"}
	for _, tr := range ti.Tracks() {
		if len(tr.Artwork) != 1 {
			t.Fatalf("%s has no cover", tr.Title)
		}
		path, ok := ti.ArtFile(tr.Artwork[0].URL)
		if !ok {
			t.Fatalf("no file of %s", tr.Artwork[0].URL)
		}
		if ext := filepath.Ext(path); ext != want[tr.Title] {
			t.Errorf("%s has a cover of %s, not %s", tr.Title, ext, want[tr.Title])
		}
	}
}
