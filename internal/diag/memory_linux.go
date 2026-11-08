//go:build linux

package diag

import (
	"os"
	"strconv"
	"strings"
)

// processMemory returns the process's resident size, as the second field
// of /proc/self/statm counts its pages. Linux tells no footprint, nor
// private bytes, as cheaply.
func processMemory() (resident, footprint, private uint64) {
	b, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 2 {
		return 0, 0, 0
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, 0
	}
	return pages * uint64(os.Getpagesize()), 0, 0
}
