//go:build windows && (amd64 || arm64)

package output

import (
	"syscall"
	"testing"
	"unsafe"
)

// TestPropString reads the friendly name of an endpoint from a PROPVARIANT
// as its property store fills it: a string of UTF-16, and nothing from a
// PROPVARIANT of another type. PROPVARIANT and PROPERTYKEY take the bytes
// they take in C.
func TestPropString(t *testing.T) {
	if n := unsafe.Sizeof(propVariant{}); n != 24 {
		t.Errorf("PROPVARIANT of %d bytes", n)
	}
	if n := unsafe.Sizeof(propertyKey{}); n != 20 {
		t.Errorf("PROPERTYKEY of %d bytes", n)
	}
	name, err := syscall.UTF16FromString("Kopfhörer (USB-DAC) 🎧")
	if err != nil {
		t.Fatal(err)
	}
	if got := propString(&propVariant{vt: vtLPWSTR, str: &name[0]}); got != "Kopfhörer (USB-DAC) 🎧" {
		t.Errorf("the name read %q", got)
	}
	if got := propString(&propVariant{}); got != "" {
		t.Errorf("an empty PROPVARIANT read %q", got)
	}
	if got := propString(&propVariant{vt: vtLPWSTR}); got != "" {
		t.Errorf("a string of no pointer read %q", got)
	}
}
