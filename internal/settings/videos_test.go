package settings

import (
	"encoding/json"
	"testing"
)

// TestMusicVideos keeps the picture of music videos off in new installs
// and in the settings of earlier versions, until the user turns it on.
func TestMusicVideos(t *testing.T) {
	for file, want := range map[string]bool{
		`{}`:                   false,
		`{"autoplay":true}`:    false,
		`{"musicVideos":true}`: true,
	} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if s = s.Clean(); s.MusicVideos != want {
			t.Errorf("%s: music videos %v, not %v", file, s.MusicVideos, want)
		}
	}
}
