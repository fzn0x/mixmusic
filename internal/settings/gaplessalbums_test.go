package settings

import (
	"encoding/json"
	"testing"
)

// TestGaplessAlbums has Keep albums gapless on in new installs, and in the
// settings of earlier versions, which don't have it, until the user turns
// it off.
func TestGaplessAlbums(t *testing.T) {
	for file, want := range map[string]bool{
		`{}`:                                    true,
		`{"crossfade":6}`:                       true,
		`{"crossfade":6,"gaplessAlbums":false}`: false,
		`{"gaplessAlbums":true}`:                true,
	} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if s = s.Clean(); s.GaplessAlbums != want {
			t.Errorf("%s: Keep albums gapless %v, not %v", file, s.GaplessAlbums, want)
		}
	}
}
