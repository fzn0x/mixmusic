package youtube

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

func TestParseRelated(t *testing.T) {
	if id := relatedBrowseID(fixture(t, "next")); id != "MPTRt_I0lA998lMLT-8" {
		t.Errorf("related browse ID = %q", id)
	}
	shelves := sectionShelves(fixture(t, "related").get("contents", "sectionListRenderer", "contents"))
	if len(shelves) < 3 {
		t.Fatalf("related = %d shelves", len(shelves))
	}
	if shelves[0].Title != "You might also like" || shelves[0].Items[0].Kind != media.ItemTrack {
		t.Errorf("first shelf = %q, %s", shelves[0].Title, shelves[0].Items[0].Kind)
	}
	kinds := map[media.ItemKind]bool{}
	for _, s := range shelves {
		for _, it := range s.Items {
			kinds[it.Kind] = true
		}
	}
	if !kinds[media.ItemPlaylist] || !kinds[media.ItemArtist] {
		t.Errorf("related misses playlists or artists: %v", kinds)
	}
}

// TestTrackVideoType keeps the kind of video YouTube Music gives each
// track, of videos only.
func TestTrackVideoType(t *testing.T) {
	shelves := sectionShelves(fixture(t, "related").get("contents", "sectionListRenderer", "contents"))
	got := map[string]media.Track{}
	for _, s := range shelves {
		for _, it := range s.Items {
			if it.Track != nil {
				got[it.Track.ID] = *it.Track
			}
		}
	}
	for id, want := range map[string]string{
		"yt:2fJGJdjWxGE": "MUSIC_VIDEO_TYPE_OFFICIAL_SOURCE_MUSIC",
		"yt:TDeKxDNMfww": media.VideoUser,
	} {
		tr, ok := got[id]
		if !ok {
			t.Errorf("no track %s", id)
			continue
		}
		if !tr.Video || tr.VideoType != want {
			t.Errorf("%s: Video %v, VideoType %q, want %q", id, tr.Video, tr.VideoType, want)
		}
	}
	for id, tr := range got {
		if !tr.Video && tr.VideoType != "" {
			t.Errorf("the song %s has the video type %q", id, tr.VideoType)
		}
	}
}
