//go:build !windows

package keypad

func watch(uintptr, func() bool) (func(), error) { return nil, ErrUnsupported }
