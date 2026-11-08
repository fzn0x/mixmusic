package localfilestest

import (
	"os"
	"os/exec"
	"sync"
	"testing"
)

// everyone is the security identifier of everyone, administrators too.
const everyone = "*S-1-1-0"

// DenyRead denies everyone the list of dir's files, as icacls does, until
// allow or the end of the test takes the denial back: its files can still
// be read by their paths, as their own permissions allow.
func DenyRead(t testing.TB, dir string) (allow func()) {
	t.Helper()
	if out, err := exec.Command("icacls", dir, "/deny", everyone+":(RD)").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v: %s", err, out)
	}
	allow = sync.OnceFunc(func() {
		if out, err := exec.Command("icacls", dir, "/remove:d", everyone).CombinedOutput(); err != nil {
			t.Errorf("icacls: %v: %s", err, out)
		}
	})
	t.Cleanup(allow)
	if _, err := os.ReadDir(dir); err == nil {
		t.Skip("denying the folder's list of files doesn't stop this user")
	}
	return allow
}
