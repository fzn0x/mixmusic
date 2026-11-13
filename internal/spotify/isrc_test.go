package spotify

import (
	"context"
	"testing"
)

// TestTrackISRC reads the ISRC of a whole track, as a search gives it,
// which an export writes into a copy's tags; an album's page gives none.
func TestTrackISRC(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /search": `{"tracks": {"items": [{"id": "t9", "name": "Paper Lanterns", "type": "track", "duration_ms": 201000,
			"artists": [{"id": "a9", "name": "Ola Brine"}], "album": {"id": "al9", "name": "Harbour Lights", "images": []},
			"external_ids": {"isrc": "QZES81900004"}}]}}`,
		"GET /albums/al9": `{"id": "al9", "name": "Harbour Lights", "album_type": "album", "images": [], "artists": [{"id": "a9", "name": "Ola Brine"}],
			"tracks": {"items": [{"id": "t9", "name": "Paper Lanterns", "duration_ms": 201000, "artists": [{"id": "a9", "name": "Ola Brine"}]}]}}`,
	})
	res, err := c.Search(context.Background(), "paper lanterns", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tracks) != 1 || res.Tracks[0].ISRC != "QZES81900004" {
		t.Errorf("tracks = %+v", res.Tracks)
	}
	al, err := c.Album(context.Background(), "al9")
	if err != nil {
		t.Fatal(err)
	}
	if len(al.Tracks) != 1 || al.Tracks[0].ISRC != "" {
		t.Errorf("the album's tracks = %+v", al.Tracks)
	}
}
