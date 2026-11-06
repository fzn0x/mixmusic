package app

import (
	"os"
	"syscall"
	"unsafe"
)

// webViewAudio returns the browser process of MixMusic's WebView2, whose
// children play what its web views play, or 0 for none: Windows hears a
// process and its children, but not theirs, so not WebView2's audio from
// MixMusic's own process.
func webViewAudio(procs []proc) uint32 {
	return childNamed(procs, uint32(os.Getpid()), "msedgewebview2.exe")
}

// processes returns the processes that run.
func processes() []proc {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(snap)
	var procs []proc
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		procs = append(procs, proc{pid: e.ProcessID, parent: e.ParentProcessID, name: syscall.UTF16ToString(e.ExeFile[:])})
	}
	return procs
}
