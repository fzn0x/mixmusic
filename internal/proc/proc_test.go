package proc

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// TestKilledProgramsChild has Output end soon after the program it runs is
// killed as its context ends, though a program it started still holds its
// output, as yt-dlp for macOS's Python does (#246).
func TestKilledProgramsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("needs sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := Command(ctx, "sh", "-c", "sleep 5 & sleep 5").Output(); err == nil {
		t.Error("the killed program succeeded")
	}
	if took := time.Since(start); took > 4*time.Second {
		t.Errorf("Output waited %s for the killed program's child", took)
	}
}
