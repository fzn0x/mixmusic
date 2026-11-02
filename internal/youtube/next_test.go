package youtube

import "testing"

func TestParseQueue(t *testing.T) {
	n := fixture(t, "next")
	tracks := parseQueue(n)
	if len(tracks) < 2 {
		t.Fatalf("tracks = %d", len(tracks))
	}
	first := tracks[0]
	if first.ID != "yt:w3XzgjlKFjc" || first.Title != "Draft Punk" || first.DurationMs != 208000 {
		t.Errorf("first = %+v", first)
	}
	if len(first.Artists) != 1 || first.Artists[0].Name != "DJT" {
		t.Errorf("artists = %+v", first.Artists)
	}
	if first.Album == nil || first.Album.Name != "Q-ENA II" {
		t.Errorf("album = %+v", first.Album)
	}
	if got := lyricsBrowseID(n); got != "MPLYt_I0lA998lMLT-8" {
		t.Errorf("lyrics browse id = %q", got)
	}
}

func TestParseLyrics(t *testing.T) {
	text, source := parseLyrics(fixture(t, "lyrics"))
	if text == "" || source != "Source: Example Lyrics" {
		t.Errorf("lyrics = %q, %q", text, source)
	}
}

// TestParseVideoRadio reads the radio of a music video, which YouTube
// Music makes of music videos and other uploads, and no song.
func TestParseVideoRadio(t *testing.T) {
	tracks := parseQueue(fixture(t, "next-video"))
	if len(tracks) != 5 || tracks[0].ID != "yt:a5uQMwRMHcs" {
		t.Fatalf("tracks = %+v", tracks)
	}
	for _, tr := range tracks {
		if !tr.Video {
			t.Errorf("%s (%s) is not a video", tr.Title, tr.ID)
		}
	}
	if up := tracks[4]; up.ID != "yt:GPZ5fnYFI4Q" || len(up.Artists) != 1 || up.Artists[0].Name != "MyDriveThru" {
		t.Errorf("upload = %+v", up)
	}
}

// TestParseSongRadio reads the radio of that video's song: songs, and an
// upload YouTube Music suggests beside them.
func TestParseSongRadio(t *testing.T) {
	tracks := parseQueue(fixture(t, "next-song"))
	if len(tracks) != 5 || tracks[0].ID != "yt:khnokW3Mw24" {
		t.Fatalf("tracks = %+v", tracks)
	}
	for i, tr := range tracks {
		if tr.Video != (i == 4) {
			t.Errorf("%s (%s): video = %v", tr.Title, tr.ID, tr.Video)
		}
	}
}
