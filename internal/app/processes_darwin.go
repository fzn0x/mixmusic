//go:build darwin && (amd64 || arm64)

package app

import (
	"os"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/fzn0x/mixmusic/internal/audio/output"
)

// processes returns the processes Core Audio knows, which play or record,
// named by their bundle's identifier, each with the process responsible
// for it as its parent: launchd starts WebKit's processes, for the app
// macOS holds responsible for them.
func processes() []proc {
	var procs []proc
	for _, p := range output.AudioProcesses() {
		procs = append(procs, proc{pid: p.PID, parent: responsible(p.PID), name: p.BundleID})
	}
	return procs
}

// webViewAudio returns the process that plays what MixMusic's web views
// play, or 0 for none: WebKit's GPU process of MixMusic's, which macOS
// holds MixMusic responsible for, as MixMusic itself, a new one at each
// start.
func webViewAudio(procs []proc) uint32 {
	self := responsible(uint32(os.Getpid()))
	if self == 0 {
		return 0
	}
	return childNamed(procs, self, "com.apple.WebKit.GPU")
}

// responsibleFor is libSystem's private
// responsibility_get_pid_responsible_for_pid, nil where it isn't there.
var responsibleFor = sync.OnceValue(func() func(pid int32) int32 {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return nil
	}
	sym, err := purego.Dlsym(lib, "responsibility_get_pid_responsible_for_pid")
	if err != nil {
		return nil
	}
	var fn func(pid int32) int32
	purego.RegisterFunc(&fn, sym)
	return fn
})

// responsible returns the process macOS holds responsible for pid, as for
// what it asks permission to do: pid itself for an app that was opened,
// the app it works for for one of WebKit's processes, and 0 when macOS
// doesn't tell.
func responsible(pid uint32) uint32 {
	fn := responsibleFor()
	if fn == nil {
		return 0
	}
	if r := fn(int32(pid)); r > 0 {
		return uint32(r)
	}
	return 0
}
