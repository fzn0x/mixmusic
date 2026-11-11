package browser

import (
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// openKey opens a key of the registry, as "HKLM\SOFTWARE\…", to read.
func openKey(key string) (syscall.Handle, bool) {
	hive, path, _ := strings.Cut(key, `\`)
	var root syscall.Handle
	switch hive {
	case "HKLM":
		root = syscall.HKEY_LOCAL_MACHINE
	case "HKCU":
		root = syscall.HKEY_CURRENT_USER
	default:
		return 0, false
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var h syscall.Handle
	if syscall.RegOpenKeyEx(root, p, 0, syscall.KEY_READ, &h) != nil {
		return 0, false
	}
	return h, true
}

// registryValue reads a text value of a key, "" for the key's default.
func registryValue(key, name string) string {
	h, ok := openKey(key)
	if !ok {
		return ""
	}
	defer syscall.RegCloseKey(h)
	n16, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return ""
	}
	var typ, size uint32
	if syscall.RegQueryValueEx(h, n16, nil, &typ, nil, &size) != nil || size == 0 || size > 1<<16 {
		return ""
	}
	if typ != syscall.REG_SZ && typ != syscall.REG_EXPAND_SZ {
		return ""
	}
	buf := make([]uint16, size/2+1)
	if syscall.RegQueryValueEx(h, n16, nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &size) != nil {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// registrySubkeys lists the keys of a key.
func registrySubkeys(key string) []string {
	h, ok := openKey(key)
	if !ok {
		return nil
	}
	defer syscall.RegCloseKey(h)
	// Windows wants the keys listed from one thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var names []string
	for i := uint32(0); i < 1024; i++ {
		buf := make([]uint16, 256) // the longest name a key has, and its end
		n := uint32(len(buf))
		if syscall.RegEnumKeyEx(h, i, &buf[0], &n, nil, nil, nil, nil) != nil {
			break
		}
		names = append(names, syscall.UTF16ToString(buf[:n]))
	}
	return names
}
