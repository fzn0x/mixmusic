//go:build windows

package app

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// userSealer seals secrets for the Windows user, with DPAPI: what the
// Credential Manager encrypts its own entries with.
var userSealer sealer = dpapi{}

// errNoLogonSession is ERROR_NO_SUCH_LOGON_SESSION, the Credential
// Manager's answer in a logon session that has no credentials, as a
// network logon.
const errNoLogonSession = syscall.Errno(1312)

// unavailable reports whether err means the Credential Manager isn't
// available to MixMusic: not to this logon session, or not at all.
func unavailable(err error) bool {
	return errors.Is(err, errors.ErrUnsupported) || errors.Is(err, errNoLogonSession)
}

var (
	// crypt32.dll is one of the system libraries the syscall package
	// loads from System32 only.
	crypt32                = syscall.NewLazyDLL("crypt32.dll")
	procCryptProtectData   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
)

// cryptprotectUIForbidden is CRYPTPROTECT_UI_FORBIDDEN: DPAPI fails rather
// than ask the user anything. MixMusic never passes
// CRYPTPROTECT_LOCAL_MACHINE, with which every account of the computer
// could open the files.
const cryptprotectUIForbidden = 0x1

// dataBlob is DATA_BLOB.
type dataBlob struct {
	size uint32
	data *byte
}

func blobOf(b []byte) dataBlob { return dataBlob{size: uint32(len(b)), data: unsafe.SliceData(b)} }

// dpapi seals data for the Windows user, the purpose as DPAPI's entropy.
type dpapi struct{}

func (dpapi) protect(data, purpose []byte) ([]byte, error) {
	if len(data) == 0 {
		// DPAPI refuses it, with ERROR_INVALID_PARAMETER.
		return nil, errors.New("dpapi: nothing to seal")
	}
	return cryptData(procCryptProtectData, data, purpose)
}

func (dpapi) unprotect(sealed, purpose []byte) ([]byte, error) {
	return cryptData(procCryptUnprotectData, sealed, purpose)
}

// cryptData calls CryptProtectData or CryptUnprotectData, which take the
// same arguments, and returns a copy of what it made, which DPAPI's buffer
// leaves cleared.
func cryptData(proc *syscall.LazyProc, in, purpose []byte) ([]byte, error) {
	inBlob, entropy := blobOf(in), blobOf(purpose)
	var out dataBlob
	r, _, err := proc.Call(uintptr(unsafe.Pointer(&inBlob)), 0, uintptr(unsafe.Pointer(&entropy)), 0, 0,
		cryptprotectUIForbidden, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(in)
	runtime.KeepAlive(purpose)
	if r == 0 {
		return nil, fmt.Errorf("dpapi: %w", err)
	}
	buf := unsafe.Slice(out.data, out.size)
	data := append([]byte(nil), buf...)
	clear(buf)
	syscall.LocalFree(syscall.Handle(unsafe.Pointer(out.data)))
	return data, nil
}
