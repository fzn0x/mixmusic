//go:build windows

package diag

import (
	"syscall"
	"unsafe"
)

// processMemoryCounters is PROCESS_MEMORY_COUNTERS_EX.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
	privateUsage               uintptr
}

var getProcessMemoryInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

// processMemory returns the process's working set, its resident size, and
// its private bytes, as GetProcessMemoryInfo tells them.
func processMemory() (resident, footprint, private uint64) {
	if getProcessMemoryInfo.Find() != nil {
		return 0, 0, 0
	}
	p, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, 0, 0
	}
	c := processMemoryCounters{}
	c.cb = uint32(unsafe.Sizeof(c))
	if ok, _, _ := getProcessMemoryInfo.Call(uintptr(p), uintptr(unsafe.Pointer(&c)), uintptr(c.cb)); ok == 0 {
		return 0, 0, 0
	}
	return uint64(c.workingSetSize), 0, uint64(c.privateUsage)
}
