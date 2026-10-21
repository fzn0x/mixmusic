//go:build windows && (amd64 || arm64)

package nowplaying

// WinRT from Go, without cgo, through combase.dll: the activation of
// runtime classes, HSTRINGs, and delegates made in Go. Each interface is a
// pointer to its vtable, which lists the methods after those of IUnknown
// and IInspectable, in the order of the Windows SDK's headers. TimeSpan
// arguments, 64 bits, pass in one slot as they do on 64-bit Windows only:
// hence the build constraint.

import (
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

var (
	iidIUnknown = guid{0x00000000, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	// iidIAgileObject marks an object any thread may call, as a delegate
	// made in Go is.
	iidIAgileObject = guid{0x94EA2B94, 0xE9CC, 0x49E0, [8]byte{0xC0, 0xFF, 0xEE, 0x64, 0xCA, 0x8F, 0x5B, 0x90}}
)

const (
	roInitMultithreaded = 1

	// HRESULTs.
	sOK          = 0
	eNoInterface = 0x80004002
)

var (
	combase                    = syscall.NewLazyDLL("combase.dll")
	procRoInitialize           = combase.NewProc("RoInitialize")
	procRoUninitialize         = combase.NewProc("RoUninitialize")
	procRoGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	procRoActivateInstance     = combase.NewProc("RoActivateInstance")
	procWindowsCreateString    = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString    = combase.NewProc("WindowsDeleteString")
)

// hresult is a failed HRESULT.
type hresult uint32

func (h hresult) Error() string { return fmt.Sprintf("nowplaying: Windows: error %#08x", uint32(h)) }

// check turns the HRESULT a call returned into an error.
func check(r uintptr) error {
	if int32(r) < 0 {
		return hresult(uint32(r))
	}
	return nil
}

type iUnknownVtbl struct {
	queryInterface, addRef, release uintptr
}

type iInspectableVtbl struct {
	iUnknownVtbl
	getIids, getRuntimeClassName, getTrustLevel uintptr
}

// iInspectable is a runtime object MixMusic only passes on.
type iInspectable struct{ vtbl *iInspectableVtbl }

// release releases the object p points to, if any.
func release(p unsafe.Pointer) {
	if p != nil {
		syscall.SyscallN((*iInspectable)(p).vtbl.release, uintptr(p))
	}
}

// queryInterface asks the object p for its interface iid, which it puts
// in *out.
func queryInterface(p unsafe.Pointer, iid *guid, out unsafe.Pointer) error {
	r, _, _ := syscall.SyscallN((*iInspectable)(p).vtbl.queryInterface, uintptr(p), uintptr(unsafe.Pointer(iid)), uintptr(out))
	return check(r)
}

// hstring is an HSTRING, which free frees. The empty string is 0.
type hstring uintptr

func newHString(s string) (hstring, error) {
	if s == "" {
		return 0, nil
	}
	u := utf16.Encode([]rune(s))
	var h hstring
	r, _, _ := syscall.SyscallN(procWindowsCreateString.Addr(), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&h)))
	return h, check(r)
}

func (h hstring) free() {
	if h != 0 {
		syscall.SyscallN(procWindowsDeleteString.Addr(), uintptr(h))
	}
}

// activationFactory puts the activation factory of the runtime class name
// in *out, as its interface iid.
func activationFactory(name string, iid *guid, out unsafe.Pointer) error {
	h, err := newHString(name)
	if err != nil {
		return err
	}
	defer h.free()
	r, _, _ := syscall.SyscallN(procRoGetActivationFactory.Addr(), uintptr(h), uintptr(unsafe.Pointer(iid)), uintptr(out))
	return check(r)
}

// activate makes an object of the runtime class name, and asks it for its
// interface iid, which it puts in *out.
func activate(name string, iid *guid, out unsafe.Pointer) error {
	h, err := newHString(name)
	if err != nil {
		return err
	}
	defer h.free()
	var obj *iInspectable
	r, _, _ := syscall.SyscallN(procRoActivateInstance.Addr(), uintptr(h), uintptr(unsafe.Pointer(&obj)))
	if err := check(r); err != nil {
		return err
	}
	defer release(unsafe.Pointer(obj))
	return queryInterface(unsafe.Pointer(obj), iid, out)
}

// delegate is a delegate of WinRT made in Go, as a TypedEventHandler: a
// COM object whose Invoke calls fn with the event's arguments, on a thread
// of the system's. It lives as long as its maker holds it: the references
// COM counts keep nothing alive.
type delegate struct {
	vtbl *delegateVtbl
	iid  guid
	refs atomic.Int32
	fn   func(args unsafe.Pointer)
}

type delegateVtbl struct {
	iUnknownVtbl
	invoke uintptr
}

// delegateMethods are the methods of every delegate, as callbacks, which
// Go makes a limited number of: once.
var delegateMethods = sync.OnceValue(func() *delegateVtbl {
	return &delegateVtbl{
		iUnknownVtbl: iUnknownVtbl{
			queryInterface: syscall.NewCallback(delegateQueryInterface),
			addRef:         syscall.NewCallback(delegateAddRef),
			release:        syscall.NewCallback(delegateRelease),
		},
		invoke: syscall.NewCallback(delegateInvoke),
	}
})

// newDelegate returns a delegate of the interface iid, which calls fn.
func newDelegate(iid guid, fn func(args unsafe.Pointer)) *delegate {
	d := &delegate{vtbl: delegateMethods(), iid: iid, fn: fn}
	d.refs.Store(1)
	return d
}

func delegateQueryInterface(d *delegate, iid *guid, out *unsafe.Pointer) uintptr {
	if *iid != iidIUnknown && *iid != iidIAgileObject && *iid != d.iid {
		*out = nil
		return eNoInterface
	}
	d.refs.Add(1)
	*out = unsafe.Pointer(d)
	return sOK
}

func delegateAddRef(d *delegate) uintptr  { return uintptr(d.refs.Add(1)) }
func delegateRelease(d *delegate) uintptr { return uintptr(d.refs.Add(-1)) }

func delegateInvoke(d *delegate, sender, args unsafe.Pointer) uintptr {
	d.fn(args)
	return sOK
}
