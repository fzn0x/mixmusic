//go:build !((windows || darwin || linux) && (amd64 || arm64))

package output

import "time"

// Stream plays audio; there are none on this system.
type Stream struct{}

// Available returns ErrUnsupported.
func Available() error { return ErrUnsupported }

// Open returns ErrUnsupported.
func Open(o Options) (*Stream, error) { return nil, ErrUnsupported }

// Devices returns ErrUnsupported.
func Devices() ([]Device, error) { return nil, ErrUnsupported }

// Stop does nothing.
func (s *Stream) Stop() {}

// Delay returns 0.
func (s *Stream) Delay() time.Duration { return 0 }
