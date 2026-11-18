package tools

import (
	"context"
	"testing"
	"time"
)

// fakeClock stands in for the clock of fetch's tries: its waits pass at
// once and move its time on, and it records them.
type fakeClock struct {
	now   time.Time
	waits []time.Duration
}

// fakeWaits has fetch's tries wait on a fake clock until the test ends.
func fakeWaits(t *testing.T) *fakeClock {
	t.Helper()
	c := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	savedNow, savedWait := retryNow, retryWait
	retryNow = func() time.Time { return c.now }
	retryWait = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.waits = append(c.waits, d)
		c.now = c.now.Add(d)
		return nil
	}
	t.Cleanup(func() { retryNow, retryWait = savedNow, savedWait })
	return c
}
