package discord

import (
	"context"
	"errors"
	"testing"
)

// TestTestsNeverReachDiscord: a client made in a test binary, as the app's
// tests make one when they play a track, never opens the pipe or socket of
// the Discord app on the computer, so no test's track shows as the
// person's status.
func TestTestsNeverReachDiscord(t *testing.T) {
	c := New("1234")
	if _, err := c.dial(context.Background()); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("dial in a test binary: %v, want ErrNotRunning", err)
	}
	if err := c.SetActivity(context.Background(), track("Song Title")); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("SetActivity in a test binary: %v, want ErrNotRunning", err)
	}
}
