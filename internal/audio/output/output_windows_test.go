//go:build windows && (amd64 || arm64)

package output

import (
	"runtime"
	"testing"
)

// TestBoostThread checks that the stream's thread runs ahead of threads of
// normal priority while it plays, as a game's or a busy browser's, which
// kept it waiting until the audio stuttered, and that it is normal again
// after.
func TestBoostThread(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	getThreadPriority := kernel32.NewProc("GetThreadPriority")
	thread, _, _ := procGetCurrentThread.Call()
	priority := func() int32 {
		p, _, _ := getThreadPriority.Call(thread)
		return int32(p)
	}
	undo := boostThread()
	if p := priority(); p <= threadPriorityNormal {
		t.Errorf("boosted, the thread's priority is %d", p)
	}
	undo()
	if p := priority(); p != threadPriorityNormal {
		t.Errorf("after, the thread's priority is %d", p)
	}
}
