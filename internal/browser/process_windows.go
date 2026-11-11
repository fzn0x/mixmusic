package browser

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// readyCommand readies the command's attributes, which the pipes add to,
// and its environment, without the compatibility layer a MixMusic run as
// administrator has (__COMPAT_LAYER=RunAsAdmin): Edge starts again
// without it, through another process, which would leave the pipes
// behind.
func readyCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.Env = slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(strings.ToUpper(kv), "__COMPAT_LAYER=")
	})
}

// processTree is the browser's process and the processes it started,
// which Windows doesn't end with it: Firefox's first process starts the
// browser's own, and waits for it (-wait-for-browser), and a browser that
// starts again, as to update, starts its new process from its old one.
type processTree struct {
	root winProc
	mu   *sync.Mutex
	// known are the processes the browser started, and those they
	// started, as seen while they ran.
	known *[]winProc
}

// winProc is a process, by its ID and when it started, which tells it
// from a later process given the same ID.
type winProc struct {
	pid     uint32
	created int64
}

const processQueryLimitedInformation = 0x1000

func treeOf(p *os.Process) processTree {
	t := processTree{root: winProc{pid: uint32(p.Pid)}, mu: new(sync.Mutex), known: new([]winProc)}
	t.root.created, _ = startedAt(t.root.pid)
	return t
}

// remember notes the processes the browser and those known started.
func (t processTree) remember() {
	if t.root.created == 0 {
		return // which processes are its is no longer known
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	*t.known = descendants(append([]winProc{t.root}, *t.known...), *t.known)
}

// kill ends the processes the browser started, then the browser.
func (t processTree) kill() {
	t.remember()
	t.mu.Lock()
	known := append([]winProc(nil), *t.known...)
	t.mu.Unlock()
	for _, p := range known {
		terminate(p)
	}
	terminate(t.root)
}

func containsProc(list []winProc, p winProc) bool {
	for _, q := range list {
		if q == p {
			return true
		}
	}
	return false
}

// startedAt returns when the process pid started.
func startedAt(pid uint32) (int64, error) {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(h)
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	return created.Nanoseconds(), nil
}

// descendants returns found with the processes that run now, that the
// processes of from started, and those they started. A process whose
// parent's ID is one of them, but that started before it, is another's,
// which got the ID once it was free.
func descendants(from, found []winProc) []winProc {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return found
	}
	defer syscall.CloseHandle(snap)
	children := map[uint32][]uint32{}
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		if e.ProcessID != e.ParentProcessID {
			children[e.ParentProcessID] = append(children[e.ParentProcessID], e.ProcessID)
		}
	}
	queue := append([]winProc(nil), from...)
	for len(queue) > 0 && len(found) < 1024 {
		parent := queue[0]
		queue = queue[1:]
		for _, pid := range children[parent.pid] {
			created, err := startedAt(pid)
			p := winProc{pid, created}
			if err != nil || created < parent.created || containsProc(found, p) {
				continue
			}
			found = append(found, p)
			queue = append(queue, p)
		}
	}
	return found
}

// terminate ends p, unless the process of its ID is another now.
func terminate(p winProc) {
	if p.created == 0 {
		return
	}
	h, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE|processQueryLimitedInformation, false, p.pid)
	if err != nil {
		return
	}
	defer syscall.CloseHandle(h)
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil || created.Nanoseconds() != p.created {
		return
	}
	syscall.TerminateProcess(h, 1)
}
