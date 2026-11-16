package lyrics

import (
	"testing"
	"time"
)

// TestShiftedAtASpeed moves each line and word to the shift plus its time
// at the speed, to the millisecond: a line late in the song moves further
// than an early one.
func TestShiftedAtASpeed(t *testing.T) {
	l := Lyrics{Synced: true, Lines: []Line{
		{TimeMs: 0, Text: "Start"},
		{TimeMs: 200000, Text: "Late word", Words: []Word{{Text: "Late", StartMs: 200000, EndMs: 200600}, {Text: "word", StartMs: 200600, EndMs: 201001}}},
	}}
	slower := l.Shifted(2900*time.Millisecond, 1.0026)
	if slower.Lines[0].TimeMs != 2900 || slower.Lines[1].TimeMs != 203420 || slower.Lines[1].Words[0].EndMs != 204022 || slower.Lines[1].Words[1].EndMs != 204424 {
		t.Errorf("0.26%% slower: %+v", slower.Lines)
	}
	faster := l.Shifted(-1100*time.Millisecond, 0.999)
	if faster.Lines[0].TimeMs != 0 || faster.Lines[1].TimeMs != 198700 || faster.Lines[1].Words[1].StartMs != 199299 {
		t.Errorf("0.1%% faster: %+v", faster.Lines)
	}
	if l.Lines[1].TimeMs != 200000 || l.Lines[1].Words[1].EndMs != 201001 {
		t.Errorf("the lyrics shifted changed: %+v", l.Lines)
	}
}
