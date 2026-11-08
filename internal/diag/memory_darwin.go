//go:build darwin && (amd64 || arm64)

package diag

import (
	"os"
	"sync"

	"github.com/ebitengine/purego"
)

// rusageInfo is libproc's rusage_info_v0.
type rusageInfo struct {
	uuid                                   [16]byte
	userTime, systemTime                   uint64
	pkgIdleWkups, interruptWkups, pageins  uint64
	wiredSize, residentSize, physFootprint uint64
	procStartAbstime, procExitAbstime      uint64
}

// pidRusage is libSystem's proc_pid_rusage, nil where it isn't there.
var pidRusage = sync.OnceValue(func() func(pid, flavor int32, info *rusageInfo) int32 {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return nil
	}
	sym, err := purego.Dlsym(lib, "proc_pid_rusage")
	if err != nil {
		return nil
	}
	var fn func(pid, flavor int32, info *rusageInfo) int32
	purego.RegisterFunc(&fn, sym)
	return fn
})

// processMemory returns the process's resident size, and its physical
// footprint, which Activity Monitor shows as its Memory, as
// proc_pid_rusage tells them (RUSAGE_INFO_V0).
func processMemory() (resident, footprint, private uint64) {
	fn := pidRusage()
	if fn == nil {
		return 0, 0, 0
	}
	var info rusageInfo
	if fn(int32(os.Getpid()), 0, &info) != 0 {
		return 0, 0, 0
	}
	return info.residentSize, info.physFootprint, 0
}
