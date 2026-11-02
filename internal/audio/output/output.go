// Package output plays the native engine's audio on the system's default
// output device, as float32 stereo at 48 kHz, or 5.1 where the device has
// it, which the system converts to the device's own format, following the
// default device when the user changes it: through WASAPI on Windows, an
// Audio Queue on macOS, and PulseAudio, which PipeWire serves as well, or
// else ALSA on Linux. It needs no cgo: it calls the system's libraries at
// run time. Elsewhere, and on 32-bit systems, Start returns ErrUnsupported.
package output

import (
	"errors"
	"time"
)

var (
	// ErrNoDevice is returned when the system has no audio output device.
	ErrNoDevice = errors.New("output: no audio output device")
	// ErrUnsupported is returned where there is no output.
	ErrUnsupported = errors.New("output: no audio output on this system")
)

// Rate is the sample rate of a Stream, in Hz.
const Rate = 48000

// bufferDuration is how much a Stream renders ahead of the device: the most
// the audio waits before it plays.
const bufferDuration = 40 * time.Millisecond

// periodFrames is how many frames a Stream renders at a time where the
// output itself doesn't say: 10 ms.
const periodFrames = Rate / 100
