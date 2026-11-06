package output

import (
	"testing"
	"time"
)

// TestQuiet gives a capture silence once its processes gave no audio for a
// moment, at a stream's pace, and no more than a moment of it after a long
// gap, as after the computer slept.
func TestQuiet(t *testing.T) {
	start := time.Now()
	q := newQuiet(start)
	frames := 0
	write := func(f []float32) { frames += len(f) / 2 }
	at := func(d time.Duration) time.Time { return start.Add(d) }

	q.fill(at(quietAfter/2), write)
	if frames != 0 {
		t.Fatalf("gave %d frames of silence before the processes were quiet a moment", frames)
	}
	q.fill(at(quietAfter), write)
	if want := int(quietAfter / period * periodFrames); frames != want {
		t.Errorf("gave %d frames of silence once quiet, want %d", frames, want)
	}
	frames = 0
	q.fill(at(quietAfter+50*time.Millisecond), write)
	if want := 5 * periodFrames; frames != want {
		t.Errorf("gave %d frames in the next 50 ms, want %d", frames, want)
	}

	q.audio(at(time.Second))
	frames = 0
	q.fill(at(time.Second+quietAfter/2), write)
	if frames != 0 {
		t.Errorf("gave %d frames of silence just after audio came", frames)
	}

	frames = 0
	q.fill(at(time.Hour), write)
	if most := int(2 * quietAfter / period * periodFrames); frames > most {
		t.Errorf("gave %d frames of silence after an hour asleep, more than a moment (%d)", frames, most)
	}
}
