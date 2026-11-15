package main

import (
	"strings"
	"testing"
)

// TestControlRunning hands a command's flag over to the MixMusic that
// runs, and ends quietly; with none running it says so and fails, and
// MixMusic doesn't start. Without such a flag, MixMusic starts as it did.
func TestControlRunning(t *testing.T) {
	for _, c := range []struct {
		name    string
		args    []string
		running bool
		took    bool
		status  int
		out     string
	}{
		{"handed over", []string{"--next"}, true, true, 0, ""},
		{"not running", []string{"--volume-up"}, false, true, 1, "MixMusic isn't running\n"},
		{"among others", []string{"--some-flag", "--pause"}, true, true, 0, ""},
		{"a plain start", nil, true, false, 0, ""},
		{"a link", []string{"mixmusic://next"}, true, false, 0, ""},
		{"like", []string{"--like"}, true, false, 0, ""},
	} {
		var out strings.Builder
		asked := false
		handOver := func() bool { asked = true; return c.running }
		took, status := controlRunning(c.args, handOver, &out)
		if took != c.took || status != c.status || out.String() != c.out {
			t.Errorf("%s: took it %v, status %d, told %q; want %v, %d, %q", c.name, took, status, out.String(), c.took, c.status, c.out)
		}
		// Only a command's flag looks for the MixMusic that runs: a start
		// without one takes the lock and runs.
		if asked != c.took {
			t.Errorf("%s: looked for the MixMusic that runs: %v", c.name, asked)
		}
	}
}
