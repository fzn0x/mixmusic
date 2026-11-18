package spotify

import (
	"context"
	"testing"
)

// TestTrackNumbers keeps each track's place on its album, as an album's
// page and a whole track tell it.
func TestTrackNumbers(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /tracks/t2": `{"id": "t2", "name": "Low Tide", "type": "track", "duration_ms": 180000, "track_number": 1, "disc_number": 2,
			"artists": [{"id": "a9", "name": "Ola Brine"}], "album": {"id": "al9", "name": "Harbour Lights", "images": []}}`,
		"GET /albums/al9": `{"id": "al9", "name": "Harbour Lights", "album_type": "album", "images": [], "artists": [{"id": "a9", "name": "Ola Brine"}],
			"tracks": {"items": [
				{"id": "t1", "name": "Paper Lanterns", "duration_ms": 201000, "track_number": 7, "disc_number": 1, "artists": [{"id": "a9", "name": "Ola Brine"}]},
				{"id": "t2", "name": "Low Tide", "duration_ms": 180000, "track_number": 1, "disc_number": 2, "artists": [{"id": "a9", "name": "Ola Brine"}]}]}}`,
	})
	al, err := c.Album(context.Background(), "al9")
	if err != nil {
		t.Fatal(err)
	}
	if len(al.Tracks) != 2 || al.Tracks[0].TrackNumber != 7 || al.Tracks[0].DiscNumber != 1 ||
		al.Tracks[1].TrackNumber != 1 || al.Tracks[1].DiscNumber != 2 {
		t.Errorf("the album's tracks = %+v", al.Tracks)
	}
	tr, err := c.Track(context.Background(), "t2")
	if err != nil {
		t.Fatal(err)
	}
	if tr.TrackNumber != 1 || tr.DiscNumber != 2 {
		t.Errorf("the track is number %d of disc %d", tr.TrackNumber, tr.DiscNumber)
	}
}
