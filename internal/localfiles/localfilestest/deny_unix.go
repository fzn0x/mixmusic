//go:build unix

package localfilestest

import (
	"os"
	"sync"
	"testing"
)

// DenyRead takes away every permission of dir, as chmod 000 does, so that
// nothing of it can be read, neither its list of files nor the files
// themselves, until allow or the end of the test gives them back. A test
// run by root, whom permissions don't stop, is skipped.
func DenyRead(t testing.TB, dir string) (allow func()) {
	t.Helper()
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	allow = sync.OnceFunc(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("giving %s its permissions back: %v", dir, err)
		}
	})
	t.Cleanup(allow)
	if _, err := os.ReadDir(dir); err == nil {
		t.Skip("permissions don't stop this user, as root")
	}
	return allow
}
