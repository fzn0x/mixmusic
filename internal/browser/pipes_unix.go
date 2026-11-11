//go:build !windows

package browser

import (
	"os"
	"os/exec"
)

// pipeArgs are the arguments that give the browser its pipes: none, as it
// finds them as its file descriptors 3 and 4.
func pipeArgs(read, write *os.File) []string { return nil }

// givePipes has the browser read from read as its file descriptor 3, and
// write to write as its 4, where --remote-debugging-pipe looks for them.
func givePipes(cmd *exec.Cmd, read, write *os.File) {
	cmd.ExtraFiles = []*os.File{read, write}
}
