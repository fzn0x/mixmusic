//go:build !unix && !windows

package localfilestest

import "testing"

// DenyRead skips the test: this system has no permissions to take away.
func DenyRead(t testing.TB, dir string) (allow func()) {
	t.Helper()
	t.Skip("no permissions to take away here")
	return func() {}
}
