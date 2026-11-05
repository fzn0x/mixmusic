//go:build !windows && !(linux && (amd64 || arm64))

package keypad

func watch(uintptr, func() bool) (func(), error) { return nil, ErrUnsupported }
