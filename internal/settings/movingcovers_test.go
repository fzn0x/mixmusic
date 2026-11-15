package settings

import (
	"encoding/json"
	"testing"
)

// TestMovingCovers keeps moving covers off in new installs and in the
// settings of earlier versions, with 256 MB of their clips kept, until
// the user changes them: Off keeps none, and Unlimited stays so.
func TestMovingCovers(t *testing.T) {
	for _, c := range []struct {
		file string
		on   bool
		mb   int
	}{
		{`{}`, false, 256},
		{`{"musicVideos":true}`, false, 256},
		{`{"movingCovers":true}`, true, 256},
		{`{"movingCoversMb":0}`, false, 0},
		{`{"movingCoversMb":8192}`, false, 8192},
		{`{"movingCoversMb":-1}`, false, UnlimitedMovingCovers},
		{`{"movingCoversMb":-40}`, false, UnlimitedMovingCovers},
		{`{"movingCoversMb":1000000}`, false, 64 * 1024},
	} {
		s := Default()
		if err := json.Unmarshal([]byte(c.file), &s); err != nil {
			t.Fatal(err)
		}
		s = s.Clean()
		if s.MovingCovers != c.on || s.MovingCoversMB != c.mb {
			t.Errorf("%s: moving covers %v, %d MB, not %v, %d MB", c.file, s.MovingCovers, s.MovingCoversMB, c.on, c.mb)
		}
	}
}
