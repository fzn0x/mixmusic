//go:build windows && (amd64 || arm64)

package output

// The output devices of Windows: WASAPI's render endpoints that are
// active, by their friendly names, as the sound settings name them, and
// the IMMDevice of one, by its endpoint ID, for a stream to play on.

import (
	"runtime"
	"syscall"
	"unsafe"
)

var (
	// pkeyDeviceFriendlyName is PKEY_Device_FriendlyName, as "Speakers
	// (Realtek(R) Audio)".
	pkeyDeviceFriendlyName = propertyKey{guid{0xA45C254E, 0xDF1C, 0x4EFD, [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}}, 14}

	procPropVariantClear = ole32.NewProc("PropVariantClear")
)

const (
	// deviceStateActive is DEVICE_STATE_ACTIVE: an endpoint that is there
	// and enabled.
	deviceStateActive = 0x1
	stgmRead          = 0
	// vtLPWSTR is VT_LPWSTR, a PROPVARIANT of a string of UTF-16.
	vtLPWSTR = 31
)

// propertyKey is PROPERTYKEY.
type propertyKey struct {
	fmtid guid
	pid   uint32
}

// propVariant is PROPVARIANT, as 64-bit Windows lays it out: 24 bytes, of
// its type and a union of 16 bytes, whose first field is a pointer when the
// type is a string's, as the properties read here are.
type propVariant struct {
	vt  uint16
	_   [3]uint16
	str *uint16
	_   uintptr
}

type iMMDeviceCollection struct{ vtbl *iMMDeviceCollectionVtbl }

type iMMDeviceCollectionVtbl struct {
	iUnknownVtbl
	getCount, item uintptr
}

type iPropertyStore struct{ vtbl *iPropertyStoreVtbl }

type iPropertyStoreVtbl struct {
	iUnknownVtbl
	getCount, getAt, getValue, setValue, commit uintptr
}

func (c *iMMDeviceCollection) release() { release(unsafe.Pointer(c), &c.vtbl.iUnknownVtbl) }
func (p *iPropertyStore) release()      { release(unsafe.Pointer(p), &p.vtbl.iUnknownVtbl) }

// withCOM runs fn on a thread of its own with COM set up, as WASAPI needs,
// and an enumerator of the devices, and returns what fn returns.
func withCOM(fn func(e *iMMDeviceEnumerator) error) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		undo, err := initCOM()
		if err != nil {
			done <- err
			return
		}
		defer undo()
		e, err := newDeviceEnumerator()
		if err != nil {
			done <- err
			return
		}
		defer e.release()
		done <- fn(e)
	}()
	return <-done
}

// initCOM sets up COM on the calling thread, which must be locked to it,
// and returns how to undo it.
func initCOM() (undo func(), err error) {
	r, _, _ := syscall.SyscallN(procCoInitializeEx.Addr(), 0, coinitMultithreaded)
	switch r {
	case sOK, sFalse:
		return func() { syscall.SyscallN(procCoUninitialize.Addr()) }, nil
	case rpcEChangedMode:
		// The thread's COM was set up otherwise: WASAPI works in either.
		return func() {}, nil
	}
	return nil, check(r)
}

// Devices returns the active output devices of Windows, sorted by name,
// the default one marked.
func Devices() ([]Device, error) {
	var devices []Device
	err := withCOM(func(e *iMMDeviceEnumerator) error {
		defaultID := ""
		if d, err := e.defaultRender(); err == nil {
			defaultID, _ = d.id()
			d.release()
		}
		all, err := e.renderDevices()
		if err != nil {
			return err
		}
		defer all.release()
		n, err := all.count()
		if err != nil {
			return err
		}
		var endpoints []Device
		for i := range n {
			d, err := all.item(i)
			if err != nil {
				continue
			}
			id, err := d.id()
			if err == nil {
				name, _ := d.friendlyName()
				endpoints = append(endpoints, Device{ID: id, Name: name})
			}
			d.release()
		}
		devices = endpointDevices(endpoints, defaultID)
		return nil
	})
	return devices, err
}

// renderDevices returns the render endpoints that are active.
func (e *iMMDeviceEnumerator) renderDevices() (*iMMDeviceCollection, error) {
	var c *iMMDeviceCollection
	r, _, _ := syscall.SyscallN(e.vtbl.enumAudioEndpoints, uintptr(unsafe.Pointer(e)),
		eRender, deviceStateActive, uintptr(unsafe.Pointer(&c)))
	if err := check(r); err != nil {
		return nil, err
	}
	return c, nil
}

// device returns the endpoint of the ID id, in whatever state it is.
func (e *iMMDeviceEnumerator) device(id string) (*iMMDevice, error) {
	p, err := syscall.UTF16PtrFromString(id)
	if err != nil {
		return nil, hresult(eNotFound)
	}
	var d *iMMDevice
	r, _, _ := syscall.SyscallN(e.vtbl.getDevice, uintptr(unsafe.Pointer(e)), uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&d)))
	if err := check(r); err != nil {
		return nil, err
	}
	return d, nil
}

func (c *iMMDeviceCollection) count() (uint32, error) {
	var n uint32
	r, _, _ := syscall.SyscallN(c.vtbl.getCount, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(&n)))
	return n, check(r)
}

func (c *iMMDeviceCollection) item(i uint32) (*iMMDevice, error) {
	var d *iMMDevice
	r, _, _ := syscall.SyscallN(c.vtbl.item, uintptr(unsafe.Pointer(c)), uintptr(i), uintptr(unsafe.Pointer(&d)))
	if err := check(r); err != nil {
		return nil, err
	}
	return d, nil
}

// state returns the DEVICE_STATE_ of the endpoint.
func (d *iMMDevice) state() (uint32, error) {
	var st uint32
	r, _, _ := syscall.SyscallN(d.vtbl.getState, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&st)))
	return st, check(r)
}

// friendlyName returns the endpoint's name, as the sound settings show it.
func (d *iMMDevice) friendlyName() (string, error) {
	var store *iPropertyStore
	r, _, _ := syscall.SyscallN(d.vtbl.openPropertyStore, uintptr(unsafe.Pointer(d)), stgmRead, uintptr(unsafe.Pointer(&store)))
	if err := check(r); err != nil {
		return "", err
	}
	defer store.release()
	var v propVariant
	r, _, _ = syscall.SyscallN(store.vtbl.getValue, uintptr(unsafe.Pointer(store)),
		uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&v)))
	if err := check(r); err != nil {
		return "", err
	}
	defer syscall.SyscallN(procPropVariantClear.Addr(), uintptr(unsafe.Pointer(&v)))
	return propString(&v), nil
}

// propString returns the string a PROPVARIANT holds, "" when it holds
// another type.
func propString(v *propVariant) string {
	if v.vt != vtLPWSTR {
		return ""
	}
	return utf16String(v.str)
}

// utf16String returns the string of UTF-16 that ends at the first 0 from p.
func utf16String(p *uint16) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), 2*n)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}
