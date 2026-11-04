// Package proc starts helper programs, such as yt-dlp, without showing a
// console window for them on Windows.
package proc

import (
	"context"
	"os/exec"
	"time"
)

// waitDelay is how long Wait waits for the output of a program to close
// once the program exited, or was killed as its context ended: a program
// it started may hold it, as yt-dlp for macOS's Python does, and would
// keep Wait until that one ends too (#246).
const waitDelay = time.Second

// Command returns a command that runs name with args, killed when ctx is
// done.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = waitDelay
	hideWindow(cmd)
	return cmd
}
