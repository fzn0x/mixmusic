package app

// The network, as the player waits for it, and the computer's sleep.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/player"
)

// resolveCheck is how long yt-dlp looks for a stream before MixMusic
// checks the network beside it: yt-dlp takes a minute and more to give up
// on a network that drops what it sends.
var resolveCheck = 3 * time.Second

// watchNetwork has the player follow the network and the computer's
// sleep, as MyGo tells them. A change of the network the system tells only
// has MixMusic check it: the system counts a network behind a captive
// portal as online, and one without a route out may still answer. The
// handlers run on the main thread, and only pass on what came.
func (a *App) watchNetwork() {
	mygo.Network.OnChanged(func(mygo.NetworkStatus) { a.online.Kick() })
	mygo.Power.OnSuspend(func() {
		if p := a.goPlayer.Load(); p != nil {
			p.Sleeping()
		}
	})
	mygo.Power.OnResume(func() {
		// Sleep may have left the connections kept for reuse dead.
		if t, ok := http.DefaultTransport.(*http.Transport); ok {
			t.CloseIdleConnections()
		}
		a.proxy.HTTP.CloseIdleConnections()
		a.online.Kick()
		if p := a.goPlayer.Load(); p != nil {
			p.Woke()
		}
	})
}

// offlineErr returns err, why something that needs the network failed, as
// player.ErrOffline when a check finds the network gone: the player waits
// for it then.
func (a *App) offlineErr(ctx context.Context, err error) error {
	if err == nil || a.online == nil || errors.Is(err, player.ErrOffline) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if a.online.Check(ctx) {
		return err
	}
	return fmt.Errorf("%w: %v", player.ErrOffline, err)
}

// resolveStream has yt-dlp find the stream of a video. Once it took
// resolveCheck, the network is checked beside it, and when it doesn't
// answer, resolveStream returns player.ErrOffline at once: yt-dlp's run
// goes on, and what it finds stays for the next try.
func (a *App) resolveStream(ctx context.Context, videoID string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := a.resolver.Resolve(ctx, videoID)
		done <- err
	}()
	if a.online == nil {
		return <-done
	}
	t := time.NewTimer(resolveCheck)
	defer t.Stop()
	select {
	case err := <-done:
		return a.offlineErr(ctx, err)
	case <-t.C:
	}
	gone := make(chan struct{})
	go func() {
		if !a.online.Check(ctx) && ctx.Err() == nil {
			close(gone)
		}
	}()
	select {
	case err := <-done:
		return a.offlineErr(ctx, err)
	case <-gone:
		return player.ErrOffline
	}
}
