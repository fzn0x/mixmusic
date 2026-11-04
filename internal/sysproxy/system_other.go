//go:build !windows

package sysproxy

// install does nothing: macOS and Linux keep the environment's proxy.
func install() {}
