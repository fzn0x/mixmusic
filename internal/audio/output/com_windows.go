//go:build windows && (amd64 || arm64)

package output

// The COM interfaces of WASAPI that the stream uses. Each is a pointer to
// its vtable, which lists the methods after those of IUnknown, in order.
// The REFERENCE_TIME arguments, 64 bits, pass in one slot as they do on
// 64-bit Windows only: hence the build constraint.

import (
	"syscall"
	"unsafe"
)

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

var (
	clsidMMDeviceEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidIAudioClient         = guid{0x1CB9AD4C, 0xDBFA, 0x4C32, [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidIAudioRenderClient   = guid{0xF294ACFC, 0x3146, 0x4483, [8]byte{0xA7, 0xBF, 0xAD, 0xDC, 0xA7, 0xC2, 0x60, 0xE2}}
	iidIAudioClock          = guid{0xCD63314F, 0x3FBA, 0x4A1B, [8]byte{0x81, 0x2C, 0xEF, 0x96, 0x35, 0x87, 0x28, 0xE7}}
	// subtypeIEEEFloat is KSDATAFORMAT_SUBTYPE_IEEE_FLOAT.
	subtypeIEEEFloat = guid{0x00000003, 0x0000, 0x0010, [8]byte{0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71}}
)

const (
	coinitMultithreaded = 0x0
	clsctxAll           = 0x17
	eRender             = 0
	eConsole            = 0

	sharemodeShared          = 0
	streamflagsEventCallback = 0x00040000
	streamflagsAutoConvert   = 0x80000000 // AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM
	streamflagsSRCQuality    = 0x08000000 // AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY

	waveFormatExtensible = 0xFFFE

	// HRESULTs.
	sOK                       = 0
	sFalse                    = 1
	rpcEChangedMode           = 0x80010106
	eNotFound                 = 0x80070490
	audclntEInvalidated       = 0x88890004
	audclntEServiceNotRunning = 0x88890010
)

// waveFormatExtensible is WAVEFORMATEXTENSIBLE, whose fields Go lays out
// at the offsets of C's: 40 bytes.
type waveFormat struct {
	formatTag      uint16
	channels       uint16
	samplesPerSec  uint32
	avgBytesPerSec uint32
	blockAlign     uint16
	bitsPerSample  uint16
	size           uint16 // cbSize: 22, the bytes after it
	validBits      uint16
	channelMask    uint32
	subFormat      guid
}

type iUnknownVtbl struct {
	queryInterface, addRef, release uintptr
}

type iMMDeviceEnumerator struct{ vtbl *iMMDeviceEnumeratorVtbl }

type iMMDeviceEnumeratorVtbl struct {
	iUnknownVtbl
	enumAudioEndpoints, getDefaultAudioEndpoint, getDevice,
	registerEndpointNotificationCallback, unregisterEndpointNotificationCallback uintptr
}

type iMMDevice struct{ vtbl *iMMDeviceVtbl }

type iMMDeviceVtbl struct {
	iUnknownVtbl
	activate, openPropertyStore, getID, getState uintptr
}

type iAudioClient struct{ vtbl *iAudioClientVtbl }

type iAudioClientVtbl struct {
	iUnknownVtbl
	initialize, getBufferSize, getStreamLatency, getCurrentPadding, isFormatSupported,
	getMixFormat, getDevicePeriod, start, stop, reset, setEventHandle, getService uintptr
}

type iAudioRenderClient struct{ vtbl *iAudioRenderClientVtbl }

type iAudioRenderClientVtbl struct {
	iUnknownVtbl
	getBuffer, releaseBuffer uintptr
}

type iAudioClock struct{ vtbl *iAudioClockVtbl }

type iAudioClockVtbl struct {
	iUnknownVtbl
	getFrequency, getPosition, getCharacteristics uintptr
}

var (
	ole32                = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
)

// hresult is a failed HRESULT.
type hresult uint32

func (h hresult) Error() string {
	switch h {
	case eNotFound:
		return "output: WASAPI: no audio output device"
	case audclntEInvalidated:
		return "output: WASAPI: the audio device was removed or changed"
	case audclntEServiceNotRunning:
		return "output: WASAPI: the Windows audio service is not running"
	}
	return "output: WASAPI: error " + hex(uint32(h))
}

func hex(v uint32) string {
	const digits = "0123456789ABCDEF"
	b := []byte("0x00000000")
	for i := 9; i >= 2; i-- {
		b[i] = digits[v&0xF]
		v >>= 4
	}
	return string(b)
}

// Is makes the HRESULT of a missing device ErrNoDevice.
func (h hresult) Is(target error) bool { return target == ErrNoDevice && h == eNotFound }

// check turns the HRESULT a COM call returned into an error.
func check(r uintptr) error {
	if int32(r) < 0 {
		return hresult(uint32(r))
	}
	return nil
}

func release(p unsafe.Pointer, vtbl *iUnknownVtbl) {
	syscall.SyscallN(vtbl.release, uintptr(p))
}

func (e *iMMDeviceEnumerator) release() { release(unsafe.Pointer(e), &e.vtbl.iUnknownVtbl) }
func (d *iMMDevice) release()           { release(unsafe.Pointer(d), &d.vtbl.iUnknownVtbl) }
func (c *iAudioClient) release()        { release(unsafe.Pointer(c), &c.vtbl.iUnknownVtbl) }
func (c *iAudioRenderClient) release()  { release(unsafe.Pointer(c), &c.vtbl.iUnknownVtbl) }
func (c *iAudioClock) release()         { release(unsafe.Pointer(c), &c.vtbl.iUnknownVtbl) }

func newDeviceEnumerator() (*iMMDeviceEnumerator, error) {
	var e *iMMDeviceEnumerator
	r, _, _ := syscall.SyscallN(procCoCreateInstance.Addr(),
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&e)))
	if err := check(r); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *iMMDeviceEnumerator) defaultRender() (*iMMDevice, error) {
	var d *iMMDevice
	r, _, _ := syscall.SyscallN(e.vtbl.getDefaultAudioEndpoint, uintptr(unsafe.Pointer(e)),
		eRender, eConsole, uintptr(unsafe.Pointer(&d)))
	if err := check(r); err != nil {
		return nil, err
	}
	return d, nil
}

// id returns the endpoint ID string of the device.
func (d *iMMDevice) id() (string, error) {
	var p *uint16
	r, _, _ := syscall.SyscallN(d.vtbl.getID, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&p)))
	if err := check(r); err != nil {
		return "", err
	}
	defer syscall.SyscallN(procCoTaskMemFree.Addr(), uintptr(unsafe.Pointer(p)))
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), 2*n)) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n)), nil
}

func (d *iMMDevice) activateClient() (*iAudioClient, error) {
	var c *iAudioClient
	r, _, _ := syscall.SyscallN(d.vtbl.activate, uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(&iidIAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&c)))
	if err := check(r); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *iAudioClient) initialize(flags uint32, bufferDuration int64, format *waveFormat) error {
	r, _, _ := syscall.SyscallN(c.vtbl.initialize, uintptr(unsafe.Pointer(c)),
		sharemodeShared, uintptr(flags), uintptr(bufferDuration), 0, uintptr(unsafe.Pointer(format)), 0)
	return check(r)
}

// mixFormat returns the channels of the format the device mixes in, and
// their speakers, 0 when it names none.
func (c *iAudioClient) mixFormat() (channels int, mask uint32, err error) {
	var p *waveFormat
	r, _, _ := syscall.SyscallN(c.vtbl.getMixFormat, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(&p)))
	if err := check(r); err != nil {
		return 0, 0, err
	}
	defer syscall.SyscallN(procCoTaskMemFree.Addr(), uintptr(unsafe.Pointer(p)))
	channels = int(p.channels)
	if p.formatTag == waveFormatExtensible && p.size >= 22 {
		mask = p.channelMask
	}
	return channels, mask, nil
}

func (c *iAudioClient) bufferSize() (uint32, error) {
	var n uint32
	r, _, _ := syscall.SyscallN(c.vtbl.getBufferSize, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(&n)))
	return n, check(r)
}

func (c *iAudioClient) currentPadding() (uint32, error) {
	var n uint32
	r, _, _ := syscall.SyscallN(c.vtbl.getCurrentPadding, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(&n)))
	return n, check(r)
}

func (c *iAudioClient) start() error {
	r, _, _ := syscall.SyscallN(c.vtbl.start, uintptr(unsafe.Pointer(c)))
	return check(r)
}

func (c *iAudioClient) stop() error {
	r, _, _ := syscall.SyscallN(c.vtbl.stop, uintptr(unsafe.Pointer(c)))
	return check(r)
}

func (c *iAudioClient) setEventHandle(h syscall.Handle) error {
	r, _, _ := syscall.SyscallN(c.vtbl.setEventHandle, uintptr(unsafe.Pointer(c)), uintptr(h))
	return check(r)
}

func (c *iAudioClient) renderClient() (*iAudioRenderClient, error) {
	var rc *iAudioRenderClient
	r, _, _ := syscall.SyscallN(c.vtbl.getService, uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(&iidIAudioRenderClient)), uintptr(unsafe.Pointer(&rc)))
	if err := check(r); err != nil {
		return nil, err
	}
	return rc, nil
}

func (c *iAudioClient) clock() (*iAudioClock, error) {
	var ck *iAudioClock
	r, _, _ := syscall.SyscallN(c.vtbl.getService, uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(&iidIAudioClock)), uintptr(unsafe.Pointer(&ck)))
	if err := check(r); err != nil {
		return nil, err
	}
	return ck, nil
}

// buffer returns the n frames of the device buffer to write next.
func (rc *iAudioRenderClient) buffer(n uint32) (*byte, error) {
	var p *byte
	r, _, _ := syscall.SyscallN(rc.vtbl.getBuffer, uintptr(unsafe.Pointer(rc)), uintptr(n), uintptr(unsafe.Pointer(&p)))
	if err := check(r); err != nil {
		return nil, err
	}
	return p, nil
}

func (rc *iAudioRenderClient) releaseBuffer(n uint32) error {
	r, _, _ := syscall.SyscallN(rc.vtbl.releaseBuffer, uintptr(unsafe.Pointer(rc)), uintptr(n), 0)
	return check(r)
}

func (ck *iAudioClock) frequency() (uint64, error) {
	var f uint64
	r, _, _ := syscall.SyscallN(ck.vtbl.getFrequency, uintptr(unsafe.Pointer(ck)), uintptr(unsafe.Pointer(&f)))
	return f, check(r)
}

func (ck *iAudioClock) position() (uint64, error) {
	var p uint64
	r, _, _ := syscall.SyscallN(ck.vtbl.getPosition, uintptr(unsafe.Pointer(ck)), uintptr(unsafe.Pointer(&p)), 0)
	return p, check(r)
}
