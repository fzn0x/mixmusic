package browser

import (
	"maps"
	"os"
	"os/exec"
	"slices"
	"sync"
	"time"
)

// process is a browser MixMusic started, with the processes it starts.
type process struct {
	cmd  *exec.Cmd
	done chan struct{}
	// tree is what kills the browser's processes, once they no longer end
	// by themselves.
	tree processTree
}

// running are the browsers started that haven't exited.
var (
	runningMu sync.Mutex
	running   = map[*process]bool{}
)

// KillAll ends at once every browser a sign-in started, as MixMusic
// quits: their profiles go at its next start (RemoveLeftovers).
func KillAll() {
	runningMu.Lock()
	list := slices.Collect(maps.Keys(running))
	runningMu.Unlock()
	for _, p := range list {
		p.tree.kill()
		p.cmd.Process.Kill()
	}
}

// startProcess starts the program path with args, once prepare readied
// its command, as for the pipes of the DevTools protocol.
func startProcess(path string, args []string, prepare func(cmd *exec.Cmd)) (*process, error) {
	cmd := exec.Command(path, args...)
	readyCommand(cmd)
	if prepare != nil {
		prepare(cmd)
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &process{cmd: cmd, done: make(chan struct{}), tree: treeOf(cmd.Process)}
	runningMu.Lock()
	running[p] = true
	runningMu.Unlock()
	go func() {
		cmd.Wait()
		runningMu.Lock()
		delete(running, p)
		runningMu.Unlock()
		close(p.done)
	}()
	// The processes the browser starts are noted while it runs: once it
	// ends, which they were may no longer show.
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-p.done:
				return
			case <-tick.C:
				p.tree.remember()
			}
		}
	}()
	return p, nil
}

// stop waits grace for the browser to exit, as asked, then ends it and the
// processes it started, and waits for it.
func (p *process) stop(grace time.Duration) {
	p.tree.remember()
	select {
	case <-p.done:
	case <-time.After(grace):
	}
	// Even once the browser exited, a process of its may linger, holding
	// files of the profile.
	p.tree.kill()
	p.cmd.Process.Kill()
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
	}
}

// setEnv adds variables to the environment of a command.
func setEnv(cmd *exec.Cmd, vars ...string) {
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, vars...)
}
