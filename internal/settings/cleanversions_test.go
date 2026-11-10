package settings

import (
	"encoding/json"
	"testing"
)

// TestCleanVersions has new installs and the settings of earlier versions
// open the explicit version of an album, until the user prefers clean ones.
func TestCleanVersions(t *testing.T) {
	for file, want := range map[string]bool{`{}`: false, `{"cleanVersions":true}`: true, `{"cleanVersions":false}`: false} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if s = s.Clean(); s.CleanVersions != want {
			t.Errorf("%s: CleanVersions %v, not %v", file, s.CleanVersions, want)
		}
	}
}
