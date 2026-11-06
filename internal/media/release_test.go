package media

import (
	"testing"
	"time"
)

// TestReleaseSpan reads a release's day, month or year as the days it may
// have come out on, and nothing of what tells none.
func TestReleaseSpan(t *testing.T) {
	day := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	for _, c := range []struct {
		released    string
		first, last time.Time
		ok          bool
	}{
		{"2026-10-31", day("2026-10-31"), day("2026-10-31"), true},
		{"2026-02", day("2026-02-01"), day("2026-02-28"), true},
		{"2024-02", day("2024-02-01"), day("2024-02-29"), true},
		{"2025", day("2025-01-01"), day("2025-12-31"), true},
		{"", time.Time{}, time.Time{}, false},
		{"soon", time.Time{}, time.Time{}, false},
		{"2026-13", time.Time{}, time.Time{}, false},
	} {
		first, last, ok := Release{Released: c.released}.Span()
		if !first.Equal(c.first) || !last.Equal(c.last) || ok != c.ok {
			t.Errorf("Span of %q = %v, %v, %v", c.released, first, last, ok)
		}
	}
}
