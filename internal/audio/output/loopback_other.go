//go:build !(windows && (amd64 || arm64))

package output

// Capture hears what a process and its children play: only on Windows.
type Capture struct{}

// CaptureProcess returns ErrNoLoopback: only Windows hears what a process
// plays.
func CaptureProcess(pid uint32, write func([]float32)) (*Capture, error) {
	return nil, ErrNoLoopback
}

// Stop does nothing.
func (c *Capture) Stop() {}
