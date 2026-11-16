package lyrics

import (
	"testing"
	"time"
)

// TestShifted moves each line and word by the shift, a time before 0:00 to
// 0:00, and leaves the lyrics it came from and plain lyrics as they are.
func TestShifted(t *testing.T) {
	l := Lyrics{Synced: true, Source: "LRCLIB", Lines: []Line{
		{TimeMs: 1000, Text: "Early"},
		{TimeMs: 5000, Text: "Sung word", Words: []Word{{Text: "Sung", StartMs: 5000, EndMs: 5400}, {Text: "word", StartMs: 5400, EndMs: 6000}}},
	}}
	later := l.Shifted(29100 * time.Millisecond)
	if later.Lines[0].TimeMs != 30100 || later.Lines[1].TimeMs != 34100 || later.Lines[1].Words[1].StartMs != 34500 || later.Lines[1].Words[1].EndMs != 35100 {
		t.Errorf("29.1 s later: %+v", later.Lines)
	}
	if !later.Synced || later.Source != "LRCLIB" {
		t.Errorf("shifted to %+v", later)
	}
	earlier := l.Shifted(-2 * time.Second)
	if earlier.Lines[0].TimeMs != 0 || earlier.Lines[1].TimeMs != 3000 || earlier.Lines[1].Words[0].StartMs != 3000 {
		t.Errorf("2 s earlier: %+v", earlier.Lines)
	}
	if l.Lines[0].TimeMs != 1000 || l.Lines[1].Words[0].StartMs != 5000 {
		t.Errorf("the lyrics shifted changed: %+v", l.Lines)
	}
	plain := Plain("One\nTwo", "YouTube Music")
	if got := plain.Shifted(time.Minute); got.Lines[1].TimeMs != 0 {
		t.Errorf("plain lyrics shifted: %+v", got.Lines)
	}
}
