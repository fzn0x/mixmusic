package localfiles

import (
	"testing"
	"time"
)

// TestTrackNumbers gives a file's track its place on its album, as its
// tags tell it.
func TestTrackNumbers(t *testing.T) {
	tr := Entry{Path: "/music/Kites/03.flac", Title: "Paper", Album: "Kites", Track: 3, Disc: 2}.track()
	if tr.TrackNumber != 3 || tr.DiscNumber != 2 {
		t.Errorf("number %d of disc %d", tr.TrackNumber, tr.DiscNumber)
	}
	ti := newIndex(t)
	ti.write(t, "Kites/1.mp3", "Paper|Ada Field|Kites||1", time.Hour)
	ti.write(t, "Kites/2.mp3", "String|Ada Field|Kites||2", time.Hour)
	ti.scan(t)
	kites, ok := ti.Album(ti.Albums()[0].ID)
	if !ok || len(kites.Tracks) != 2 || kites.Tracks[0].TrackNumber != 1 || kites.Tracks[1].TrackNumber != 2 {
		t.Errorf("Kites: %+v", kites.Tracks)
	}
}
