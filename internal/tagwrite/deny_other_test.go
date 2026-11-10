//go:build !unix && !windows

package tagwrite

import "testing"

// denyWrite skips the test: this system has no permissions to take away.
func denyWrite(t *testing.T, dir string) {
	t.Helper()
	t.Skip("no permissions to take away here")
}
