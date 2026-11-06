package output

import (
	"errors"
	"time"
)

// ErrNoLoopback is returned where the system can't hear what a process
// plays: on Windows before build 20348, on macOS before 14.2, and on other
// systems.
var ErrNoLoopback = errors.New("output: this system can't hear what a process plays")

// ErrRefused is returned where the user refused MixMusic the permission to
// hear what other apps play: on macOS, which asks at the first capture.
var ErrRefused = errors.New("output: the permission to record what other apps play was refused " +
	"(System Settings › Privacy & Security › Screen & System Audio Recording)")

const (
	// quietAfter is how long the processes may give no audio before the
	// capture hears silence: packets come every 10 ms while they play.
	quietAfter = 100 * time.Millisecond
	// period is the silence the capture hears at a time, as a stream's.
	period = time.Second * periodFrames / Rate
)

// quiet gives a capture's write silence while the processes it hears give
// no audio, at the pace of a stream, so that what it feeds hears the sound
// stop.
type quiet struct {
	heard   time.Time // when the last audio came
	filled  time.Time // how far silence was given since
	silence []float32
}

func newQuiet(now time.Time) *quiet {
	return &quiet{heard: now, filled: now, silence: make([]float32, 2*periodFrames)}
}

// audio notes that audio came at now.
func (q *quiet) audio(now time.Time) { q.heard, q.filled = now, now }

// fill gives write the silence due by now, once no audio came for
// quietAfter, and no more than a moment of it at once, as after the
// computer slept.
func (q *quiet) fill(now time.Time, write func([]float32)) {
	if now.Sub(q.heard) < quietAfter {
		return
	}
	if earliest := now.Add(-2 * quietAfter); q.filled.Before(earliest) {
		q.filled = earliest
	}
	for ; now.Sub(q.filled) >= period; q.filled = q.filled.Add(period) {
		write(q.silence)
	}
}
