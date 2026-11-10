package settings

import (
	"encoding/json"
	"testing"
)

// TestHideExplicit has new installs and the settings of earlier versions
// show explicit content, until the user hides it; while it's hidden, clean
// versions are preferred, whatever Prefer clean versions holds.
func TestHideExplicit(t *testing.T) {
	for _, c := range []struct {
		file         string
		hide, prefer bool
	}{
		{`{}`, false, false},
		{`{"hideExplicit":true}`, true, true},
		{`{"hideExplicit":true,"cleanVersions":false}`, true, true},
		{`{"hideExplicit":false,"cleanVersions":true}`, false, true},
		{`{"hideExplicit":false}`, false, false},
	} {
		s := Default()
		if err := json.Unmarshal([]byte(c.file), &s); err != nil {
			t.Fatal(err)
		}
		s = s.Clean()
		if s.HideExplicit != c.hide || s.PrefersClean() != c.prefer {
			t.Errorf("%s: HideExplicit %v, PrefersClean %v; want %v, %v", c.file, s.HideExplicit, s.PrefersClean(), c.hide, c.prefer)
		}
		if c.hide && s.CleanVersions {
			t.Errorf("%s: hiding explicit content set CleanVersions", c.file)
		}
	}
}
