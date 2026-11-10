//go:build unix

package tagwrite

import (
	"os"
	"testing"
)

// denyWrite takes away the permission to make files in dir until the end
// of the test. A test run by root, whom permissions don't stop, is
// skipped.
func denyWrite(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if f, err := os.CreateTemp(dir, "probe"); err == nil {
		f.Close()
		os.Remove(f.Name())
		t.Skip("permissions don't stop this user, as root")
	}
}
