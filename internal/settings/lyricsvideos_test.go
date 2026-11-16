package settings

import "testing"

// TestLyricsTimeVideos times music videos' lyrics to them at first, and in
// a settings.json of before, which has no key of it; a file that turns it
// off keeps it off.
func TestLyricsTimeVideos(t *testing.T) {
	if !Default().LyricsTimeVideos {
		t.Error("off at first")
	}
	before, err := Decode([]byte(`{"lyricsPlain": true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !before.Clean().LyricsTimeVideos {
		t.Error("off in a file of before")
	}
	off, err := Decode([]byte(`{"lyricsTimeVideos": false}`))
	if err != nil {
		t.Fatal(err)
	}
	if off.Clean().LyricsTimeVideos {
		t.Error("on in a file that turns it off")
	}
}
