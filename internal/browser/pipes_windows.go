package browser

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// pipeArgs tells the browser the handles of its pipes, as Windows has no
// file descriptors 3 and 4 to give it: the handles a process inherits
// keep their values.
func pipeArgs(read, write *os.File) []string {
	return []string{fmt.Sprintf("--remote-debugging-io-pipes=%d,%d", uint32(read.Fd()), uint32(write.Fd()))}
}

// givePipes has the browser inherit its ends of the pipes, and those
// alone.
func givePipes(cmd *exec.Cmd, read, write *os.File) {
	cmd.SysProcAttr.AdditionalInheritedHandles = append(cmd.SysProcAttr.AdditionalInheritedHandles,
		syscall.Handle(read.Fd()), syscall.Handle(write.Fd()))
}
