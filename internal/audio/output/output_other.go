//go:build !((windows || darwin || linux) && (amd64 || arm64))

package output

import "time"

// Stream plays audio; there are none on this system.
type Stream struct{}

// Available returns ErrUnsupported.
func Available() error { return ErrUnsupported }

// Start returns ErrUnsupported.
func Start(render func(out []float32), failed func(error)) (*Stream, error) {
	return nil, ErrUnsupported
}

// Stop does nothing.
func (s *Stream) Stop() {}

// Delay returns 0.
func (s *Stream) Delay() time.Duration { return 0 }
