package app

import (
	"log"
	"slices"
)

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
// hold back, then the queue and the volume, then settings.json where it
// holds no settings MixMusic can read: gone, or with a mistake, which is
// kept as settings.json.broken, so that the next start doesn't fall back
// to the settings of a new install. The windows go first, as the main
// window saves the settings it changed, whose volume may be older than the
// player's. It runs on the main thread as MixMusic quits, and never waits
// on the player's goroutine, which may wait on the main thread itself.
func (a *App) flush() {
	a.mu.Lock()
	fns, stop := a.flushers, a.stopWatch
	a.mu.Unlock()
	if stop != nil {
		stop()
	}
	for _, fn := range fns {
		(*fn)()
	}
	if p := a.goPlayer.Load(); p != nil {
		p.Flush()
	}
	if err := a.settings.Flush(); err != nil {
		log.Printf("mixmusic: saving settings.json: %v", err)
	}
}
