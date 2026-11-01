package app

import "slices"

// OnFlush has fn save at once what a window holds back, as MixMusic quits,
// until cancel. It runs on the main thread.
func (a *App) OnFlush(fn func()) (cancel func()) {
	f := &fn
	a.mu.Lock()
	a.flushers = append(slices.Clone(a.flushers), f)
	a.mu.Unlock()
	return func() {
		a.mu.Lock()
		a.flushers = slices.DeleteFunc(slices.Clone(a.flushers), func(g *func()) bool { return g == f })
		a.mu.Unlock()
	}
}

// flush saves at once what waits a moment to be saved: what the windows
// hold back, then the queue and the volume. The windows go first, as the
// main window saves the whole of the settings it holds, whose volume may
// be older than the player's. It runs on the main thread as MixMusic
// quits, and never waits on the player's goroutine, which may wait on the
// main thread itself.
func (a *App) flush() {
	a.mu.Lock()
	fns := a.flushers
	a.mu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
	if p := a.goPlayer.Load(); p != nil {
		p.Flush()
	}
	a.labels.save()
}
