//go:build !windows

package browser

import (
	"os"
	"os/exec"
	"syscall"
)

// readyCommand starts the browser in a process group of its own, which
// the processes it starts join, so that they end together.
func readyCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// processTree is the browser's process group.
type processTree struct{ pid int }

func treeOf(p *os.Process) processTree { return processTree{pid: p.Pid} }

// remember has nothing to do: the group holds the browser's processes.
func (t processTree) remember() {}

// kill ends every process of the group.
func (t processTree) kill() {
	if t.pid > 0 {
		syscall.Kill(-t.pid, syscall.SIGKILL)
	}
}
