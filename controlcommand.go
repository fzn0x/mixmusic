package main

// The flags of a Stream Deck key or a script (#590): MixMusic --next and
// the like hand their commands over to the MixMusic that runs, as a second
// start does, and end. With none running, MixMusic doesn't start.

import (
	"fmt"
	"io"

	"github.com/fzn0x/mixmusic/internal/app"
)

// controlRunning hands the flags of args over to the MixMusic that runs,
// when they give a command's, with handOver, which reports whether one
// ran to take them. It reports whether args gave one, and the status to
// exit with, having told errOut when no MixMusic runs.
func controlRunning(args []string, handOver func() bool, errOut io.Writer) (bool, int) {
	if !app.HasControlFlag(args) {
		return false, 0
	}
	if handOver() {
		return true, 0
	}
	fmt.Fprintln(errOut, "MixMusic isn't running")
	return true, 1
}
