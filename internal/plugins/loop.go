package plugins

import "sync"

// loop runs a plugin's jobs, one after the other, on a goroutine of its
// own: a JavaScript runtime belongs to one goroutine at a time, so
// whatever calls into a plugin's is a job of its loop.
type loop struct {
	mu      sync.Mutex
	jobs    []func()
	stopped bool
	wake    chan struct{}
	// done is closed once the loop ran its last job.
	done chan struct{}
}

func newLoop() *loop {
	l := &loop{wake: make(chan struct{}, 1), done: make(chan struct{})}
	go l.run()
	return l
}

// post runs fn on the loop, after the jobs posted before it, and tells
// whether the loop takes it: a stopped loop runs nothing more.
func (l *loop) post(fn func()) bool {
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return false
	}
	l.jobs = append(l.jobs, fn)
	l.mu.Unlock()
	l.signal()
	return true
}

// stop ends the loop once it ran the jobs posted before.
func (l *loop) stop() {
	l.mu.Lock()
	l.stopped = true
	l.mu.Unlock()
	l.signal()
}

func (l *loop) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *loop) run() {
	defer close(l.done)
	for {
		l.mu.Lock()
		jobs, stopped := l.jobs, l.stopped
		l.jobs = nil
		l.mu.Unlock()
		for _, job := range jobs {
			job()
		}
		if len(jobs) == 0 {
			if stopped {
				return
			}
			<-l.wake
		}
	}
}
