package main

import (
	"testing"
	"time"
)

// TestClockLength: a length is written rounded up to the whole second, as
// YouTube Music lists it (#718), and a position isn't.
func TestClockLength(t *testing.T) {
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		// アイドル by YOASOBI: its stream is 213.261 s long, and YouTube
		// Music lists it as 3:34.
		{213261 * time.Millisecond, "3:34"},
		{248026 * time.Millisecond, "4:09"},
		// A catalog's length in whole seconds stays as it is.
		{214 * time.Second, "3:34"},
		// Less than a millisecond past the second doesn't count.
		{214*time.Second + 400*time.Microsecond, "3:34"},
		{214*time.Second + time.Millisecond, "3:35"},
		{59500 * time.Millisecond, "1:00"},
		{3599200 * time.Millisecond, "1:00:00"},
		{0, "0:00"},
		{-time.Second, "0:00"},
	} {
		if got := clockLength(tc.d); got != tc.want {
			t.Errorf("clockLength(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
	if got := clock(213261 * time.Millisecond); got != "3:33" {
		t.Errorf("clock(213.261s) = %q, want 3:33: a position isn't rounded up", got)
	}
}
