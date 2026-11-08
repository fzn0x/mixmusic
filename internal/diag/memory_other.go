//go:build !linux && !windows && !(darwin && (amd64 || arm64))

package diag

// processMemory tells nothing of the process where the system doesn't tell
// it cheaply.
func processMemory() (resident, footprint, private uint64) { return 0, 0, 0 }
