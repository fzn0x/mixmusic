package media

import (
	"context"
	"time"
)

// MaxPageWait is the longest a background read of a long list waits for a
// service that asks for a wait, as Spotify does with Retry-After.
const MaxPageWait = 60 * time.Second

type pageWaitKey struct{}

// WithPageWait marks ctx as that of a read in the background, which waits
// as long as a service asks, up to MaxPageWait, instead of failing at
// once. While it waits, notice is given the time left, each second, and
// 0 when the wait ends. Whoever drops the read cancels the context.
func WithPageWait(ctx context.Context, notice func(left time.Duration)) context.Context {
	if notice == nil {
		notice = func(time.Duration) {}
	}
	return context.WithValue(ctx, pageWaitKey{}, notice)
}

// PageWait waits d if ctx is a read in the background (see WithPageWait)
// and asked, the wait a service asked for, is within MaxPageWait: d is
// asked, but tests wait less. It tells whether it waited, and fails with
// the error of ctx if that ends first.
func PageWait(ctx context.Context, asked, d time.Duration) (bool, error) {
	notice, ok := ctx.Value(pageWaitKey{}).(func(time.Duration))
	if !ok || asked > MaxPageWait {
		return false, nil
	}
	defer notice(0)
	end := time.Now().Add(d)
	for {
		left := time.Until(end)
		if left <= 0 {
			return true, nil
		}
		notice(left)
		select {
		case <-time.After(min(left, time.Second)):
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}
