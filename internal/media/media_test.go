package media

import (
	"errors"
	"testing"
)

func TestIDRoundTrip(t *testing.T) {
	for _, source := range []Source{SourceYouTube, SourceSpotify, SourceLocal} {
		id := ID(source, "abc:def")
		got, native, err := ParseID(id)
		if err != nil {
			t.Fatalf("ParseID(%q): %v", id, err)
		}
		if got != source || native != "abc:def" {
			t.Errorf("ParseID(%q) = %q, %q", id, got, native)
		}
	}
}

func TestParseIDRejectsUnknown(t *testing.T) {
	for _, id := range []string{"", "yt:", "zz:abc", "abc"} {
		if _, _, err := ParseID(id); !errors.Is(err, ErrBadID) {
			t.Errorf("ParseID(%q) error = %v, want ErrBadID", id, err)
		}
	}
}

func TestArtistNames(t *testing.T) {
	tr := Track{Artists: []Ref{{Name: "A"}, {Name: "B"}}}
	if got := tr.ArtistNames(); got != "A, B" {
		t.Errorf("ArtistNames() = %q", got)
	}
}

// TestMusicVideo shows the picture of official music videos and of those
// someone uploaded only: never of a song, nor of a podcast's episode. A
// video kept before its kind was read counts as one.
func TestMusicVideo(t *testing.T) {
	for _, tc := range []struct {
		name string
		t    Track
		want bool
	}{
		{"song", Track{}, false},
		{"official", Track{Video: true, VideoType: VideoOfficial}, true},
		{"upload", Track{Video: true, VideoType: VideoUser}, true},
		{"podcast", Track{Video: true, VideoType: "MUSIC_VIDEO_TYPE_PODCAST_EPISODE"}, false},
		{"official source", Track{Video: true, VideoType: "MUSIC_VIDEO_TYPE_OFFICIAL_SOURCE_MUSIC"}, false},
		{"kept before", Track{Video: true}, true},
	} {
		if got := tc.t.MusicVideo(); got != tc.want {
			t.Errorf("%s: MusicVideo() = %v", tc.name, got)
		}
	}
}
