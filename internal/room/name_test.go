package room

import "testing"

// TestCleanNameTurnsNoText drops the marks that turn the way text runs,
// with which a name would show as another, and keeps the joiners of
// emoji and scripts.
func TestCleanNameTurnsNoText(t *testing.T) {
	for in, want := range map[string]string{
		"Ana\u202Etxt.exe":              "Anatxt.exe",
		"\u2067Bo\u2069":                "Bo",
		"Cy\u200F\u200E":                "Cy",
		"\U0001F469\u200D\U0001F4BB Di": "\U0001F469\u200D\U0001F4BB Di",
	} {
		if got := CleanName(in); got != want {
			t.Errorf("CleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
