package settings

import (
	"encoding/json"
	"testing"
)

// TestSkipAI has new installs and the settings of earlier versions play
// what YouTube labels as made with AI, until the user says otherwise.
func TestSkipAI(t *testing.T) {
	for file, want := range map[string]bool{`{}`: false, `{"skipAi":true}`: true, `{"skipAi":false}`: false} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if s = s.Clean(); s.SkipAI != want {
			t.Errorf("%s: SkipAI %v, not %v", file, s.SkipAI, want)
		}
	}
}
