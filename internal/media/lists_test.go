package media

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestTryPage reads a page again after each of PageWaits while it fails,
// and stops trying once the context ends.
func TestTryPage(t *testing.T) {
	quickTries(t)
	down := errors.New("HTTP 503")
	tries := 0
	failing := func(n int) func(context.Context) error {
		return func(context.Context) error {
			if tries++; tries <= n {
				return down
			}
			return nil
		}
	}
	if err := TryPage(context.Background(), failing(len(PageWaits))); err != nil || tries != len(PageWaits)+1 {
		t.Errorf("a page that fails %d times: %v after %d tries", len(PageWaits), err, tries)
	}
	tries = 0
	if err := TryPage(context.Background(), failing(len(PageWaits)+1)); !errors.Is(err, down) || tries != len(PageWaits)+1 {
		t.Errorf("a page that always fails: %v after %d tries", err, tries)
	}

	// A read dropped stops at once, with what failed.
	PageWaits = []time.Duration{time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	tries = 0
	err := TryPage(ctx, func(context.Context) error {
		tries++
		cancel()
		return down
	})
	if !errors.Is(err, down) || tries != 1 {
		t.Errorf("a read dropped: %v after %d tries", err, tries)
	}
}

// TestShortList tells how much of a list came and why the rest didn't,
// and is found in an error that wraps it.
func TestShortList(t *testing.T) {
	down := errors.New("HTTP 503")
	err := fmt.Errorf("Spotify: %w", &ShortList{Got: 50, Total: 120, Err: down})
	short, ok := Short(err)
	if !ok || short.Got != 50 || short.Total != 120 || !errors.Is(err, down) {
		t.Fatalf("Short(%v) = %+v, %v", err, short, ok)
	}
	if got := err.Error(); got != "Spotify: only 50 of 120 came: HTTP 503" {
		t.Errorf("error %q", got)
	}
	// A list whose count the service didn't tell says what came.
	if got := (&ShortList{Got: 50, Err: down}).Error(); got != "only 50 came: HTTP 503" {
		t.Errorf("error %q", got)
	}
	if _, ok := Short(down); ok {
		t.Error("another error is a short list")
	}
	if _, ok := Short(nil); ok {
		t.Error("no error is a short list")
	}
}
