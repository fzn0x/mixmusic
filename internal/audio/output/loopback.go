package output

import "errors"

// ErrNoLoopback is returned where the system can't hear what a process
// plays: on Windows before build 20348, and on other systems.
var ErrNoLoopback = errors.New("output: this system can't hear what a process plays")
