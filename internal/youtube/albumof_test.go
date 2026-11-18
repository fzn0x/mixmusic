package youtube

import (
	"context"
	"errors"
	"testing"
)

// TestAlbumOf finds the album of the playlist of its tracks, which YouTube
// Music answers with no header, as a link to it names the album (#614),
// and none of a playlist of its own.
func TestAlbumOf(t *testing.T) {
	c, requests := labelServer(t, "playlist-album", false)
	got, err := c.AlbumOf(context.Background(), "OLAK5uy_mByMItZzRVpiPOPfqNdCyzqCEs1k7Ow14")
	if err != nil || got != "MPREb_I0lA998lMLT" {
		t.Fatalf("AlbumOf = %q, %v", got, err)
	}
	if r := requests(); len(r) != 1 || r[0].body["browseId"] != "VLOLAK5uy_mByMItZzRVpiPOPfqNdCyzqCEs1k7Ow14" {
		t.Errorf("asked %+v", r)
	}
	if got, err := albumOf(fixture(t, "playlist")); !errors.Is(err, ErrNotFound) {
		t.Errorf("a playlist of its own is of the album %q, %v", got, err)
	}
}
