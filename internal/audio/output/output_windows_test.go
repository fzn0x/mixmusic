//go:build windows && (amd64 || arm64)

package output

import (
	"runtime"
	"testing"
	"unsafe"
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

// TestStreamFormat checks the WAVEFORMATEXTENSIBLE of streams of stereo and
// of 5.1: 40 bytes, as C lays it out, of float32 frames of each channel.
func TestStreamFormat(t *testing.T) {
	if n := unsafe.Sizeof(waveFormat{}); n != 40 {
		t.Errorf("WAVEFORMATEXTENSIBLE of %d bytes", n)
	}
	for _, c := range []struct {
		channels int
		mask     uint32
	}{{2, speakerStereo}, {Surround, speaker51Side}, {Surround, speaker51}} {
		f := streamFormat(c.channels, c.mask)
		if f.formatTag != waveFormatExtensible || int(f.channels) != c.channels || f.blockAlign != uint16(4*c.channels) ||
			f.avgBytesPerSec != uint32(Rate*4*c.channels) || f.channelMask != c.mask || f.subFormat != subtypeIEEEFloat ||
			f.bitsPerSample != 32 || f.validBits != 32 || f.size != 22 {
			t.Errorf("%d channels: %+v", c.channels, f)
		}
	}
}
