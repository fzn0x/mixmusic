package tagwrite

import (
	"os"
	"os/exec"
	"testing"
)

// denyWrite denies everyone, administrators too, making files and folders
// in dir, as icacls does, until the end of the test.
func denyWrite(t *testing.T, dir string) {
	t.Helper()
	const everyone = "*S-1-1-0"
	if out, err := exec.Command("icacls", dir, "/deny", everyone+":(WD,AD)").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v: %s", err, out)
	}
	t.Cleanup(func() {
		if out, err := exec.Command("icacls", dir, "/remove:d", everyone).CombinedOutput(); err != nil {
			t.Errorf("icacls: %v: %s", err, out)
		}
	})
	if f, err := os.CreateTemp(dir, "probe"); err == nil {
		f.Close()
		os.Remove(f.Name())
		t.Skip("denying the folder doesn't stop this user")
	}
}
