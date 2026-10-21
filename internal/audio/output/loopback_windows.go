//go:build windows && (amd64 || arm64)

package output

// Process loopback: from Windows 10 build 20348 on, WASAPI hears what one
// process and its children play, and nothing of the others. Its client
// comes from a virtual device, activated asynchronously:
// ActivateAudioInterfaceAsync tells a completion handler, a COM object made
// here, once the client is ready.

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	mmdevapi                        = syscall.NewLazyDLL("mmdevapi.dll")
	procActivateAudioInterfaceAsync = mmdevapi.NewProc("ActivateAudioInterfaceAsync")
	procSetEvent                    = kernel32.NewProc("SetEvent")
)

var (
	iidIUnknown                                 = guid{0x00000000, 0x0000, 0x0000, [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIAgileObject                             = guid{0x94EA2B94, 0xE9CC, 0x49E0, [8]byte{0xC0, 0xFF, 0xEE, 0x64, 0xCA, 0x8F, 0x5B, 0x90}}
	iidIActivateAudioInterfaceCompletionHandler = guid{0x41D949AB, 0x9862, 0x444A, [8]byte{0x80, 0xF6, 0xC2, 0x61, 0x33, 0x4D, 0xA5, 0xEB}}
	iidIAudioCaptureClient                      = guid{0xC8ADBD64, 0xE71E, 0x48A0, [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
)

const (
	// processLoopbackDevice is VIRTUAL_AUDIO_DEVICE_PROCESS_LOOPBACK.
	processLoopbackDevice = `VAD\Process_Loopback`
	// activationProcessLoopback is AUDIOCLIENT_ACTIVATION_TYPE_PROCESS_LOOPBACK,
	// and loopbackIncludeTree PROCESS_LOOPBACK_MODE_INCLUDE_TARGET_PROCESS_TREE.
	activationProcessLoopback = 1
	loopbackIncludeTree       = 0

	vtBlob              = 65
	waveFormatPCM       = 1
	streamflagsLoopback = 0x00020000
	bufferflagsSilent   = 0x2
	eNoInterface        = 0x80004002

	// activationWait is how long the activation may take.
	activationWait = 5 * time.Second
	// quietAfter is how long the processes may give no audio before the
	// capture hears silence: packets come every 10 ms while they play.
	quietAfter = 100 * time.Millisecond
	// period is the silence the capture hears at a time, as a stream's.
	period = time.Second * periodFrames / Rate
)

// activationParams is AUDIOCLIENT_ACTIVATION_PARAMS, of process loopback.
type activationParams struct {
	activationType  uint32
	targetProcessID uint32
	loopbackMode    uint32
}

// blobVariant is a PROPVARIANT that holds a BLOB, laid out as C's is on
// 64-bit Windows: the BLOB's size after 8 bytes, its data after 8 more.
type blobVariant struct {
	vt   uint16
	_    [3]uint16
	size uint32
	_    uint32
	data unsafe.Pointer
}

// activation is the completion handler of an activation, a COM object: its
// first word is its vtable. Windows calls it from a thread of its own, and
// holds it only while it activates.
type activation struct {
	vtbl *activationVtbl
	// done is an event, set once the activation completed.
	done syscall.Handle
	// params are what is activated, in their variant, which Windows reads
	// as it activates.
	params  activationParams
	variant blobVariant
	// op receives the activation's operation. Not on the stack: Windows
	// calls the handler on the activating goroutine's stack, which may move
	// before it writes op.
	op *iActivateOperation
}

type activationVtbl struct {
	iUnknownVtbl
	activateCompleted uintptr
}

// activationMethods are the vtable of every activation. A handler is agile,
// as Windows asks of it: callable from any thread. It counts no references:
// Go keeps it while it activates.
var activationMethods = sync.OnceValue(func() *activationVtbl {
	return &activationVtbl{
		iUnknownVtbl: iUnknownVtbl{
			queryInterface: syscall.NewCallback(func(this *activation, iid *guid, out *uintptr) uintptr {
				switch *iid {
				case iidIUnknown, iidIAgileObject, iidIActivateAudioInterfaceCompletionHandler:
					*out = uintptr(unsafe.Pointer(this))
					return sOK
				}
				*out = 0
				return eNoInterface
			}),
			addRef:  syscall.NewCallback(func(*activation) uintptr { return 1 }),
			release: syscall.NewCallback(func(*activation) uintptr { return 1 }),
		},
		activateCompleted: syscall.NewCallback(func(this *activation, _ uintptr) uintptr {
			syscall.SyscallN(procSetEvent.Addr(), uintptr(this.done))
			return sOK
		}),
	}
})

// abandoned keeps the handlers of activations that took too long, which
// Windows may still call.
var abandoned struct {
	sync.Mutex
	handlers []*activation
}

type iActivateOperation struct{ vtbl *iActivateOperationVtbl }

type iActivateOperationVtbl struct {
	iUnknownVtbl
	getActivateResult uintptr
}

func (o *iActivateOperation) release() { release(unsafe.Pointer(o), &o.vtbl.iUnknownVtbl) }

// result returns the client the activation made, or why it made none.
func (o *iActivateOperation) result() (*iAudioClient, error) {
	var hr uint32
	var c *iAudioClient
	r, _, _ := syscall.SyscallN(o.vtbl.getActivateResult, uintptr(unsafe.Pointer(o)),
		uintptr(unsafe.Pointer(&hr)), uintptr(unsafe.Pointer(&c)))
	if err := check(r); err != nil {
		return nil, err
	}
	if err := check(uintptr(hr)); err != nil {
		return nil, err
	}
	return c, nil
}

type iAudioCaptureClient struct{ vtbl *iAudioCaptureClientVtbl }

type iAudioCaptureClientVtbl struct {
	iUnknownVtbl
	getBuffer, releaseBuffer, getNextPacketSize uintptr
}

func (cc *iAudioCaptureClient) release() { release(unsafe.Pointer(cc), &cc.vtbl.iUnknownVtbl) }

func (c *iAudioClient) captureClient() (*iAudioCaptureClient, error) {
	var cc *iAudioCaptureClient
	r, _, _ := syscall.SyscallN(c.vtbl.getService, uintptr(unsafe.Pointer(c)),
		uintptr(unsafe.Pointer(&iidIAudioCaptureClient)), uintptr(unsafe.Pointer(&cc)))
	if err := check(r); err != nil {
		return nil, err
	}
	return cc, nil
}

// nextPacket returns the frames of the next packet, 0 for none yet.
func (cc *iAudioCaptureClient) nextPacket() (uint32, error) {
	var n uint32
	r, _, _ := syscall.SyscallN(cc.vtbl.getNextPacketSize, uintptr(unsafe.Pointer(cc)), uintptr(unsafe.Pointer(&n)))
	return n, check(r)
}

// buffer returns the next packet: its data, its frames and its flags.
func (cc *iAudioCaptureClient) buffer() (*byte, uint32, uint32, error) {
	var p *byte
	var n, flags uint32
	r, _, _ := syscall.SyscallN(cc.vtbl.getBuffer, uintptr(unsafe.Pointer(cc)),
		uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&flags)), 0, 0)
	return p, n, flags, check(r)
}

func (cc *iAudioCaptureClient) releaseBuffer(n uint32) error {
	r, _, _ := syscall.SyscallN(cc.vtbl.releaseBuffer, uintptr(unsafe.Pointer(cc)), uintptr(n))
	return check(r)
}

// Capture hears what a process and its children play.
type Capture struct {
	stopping atomic.Bool
	done     chan struct{}
}

// CaptureProcess starts hearing what the process pid and its children
// play, through process loopback, as interleaved float32 stereo at Rate,
// which the capture's goroutine hands to write about every 10 ms; write
// must not block. While they play nothing, write is given silence at the
// same pace, so that what it feeds hears the sound stop. It returns once
// the capture runs, or why it can't, as ErrNoLoopback before Windows 10
// build 20348.
func CaptureProcess(pid uint32, write func([]float32)) (*Capture, error) {
	c := &Capture{done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		defer close(c.done)
		c.run(pid, write, ready)
	}()
	if err := <-ready; err != nil {
		<-c.done
		return nil, err
	}
	return c, nil
}

// Stop stops the capture: write is not called once it returns.
func (c *Capture) Stop() {
	c.stopping.Store(true)
	<-c.done
}

// run captures until Stop, after it sends to ready whether it started.
func (c *Capture) run(pid uint32, write func([]float32), ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := syscall.SyscallN(procCoInitializeEx.Addr(), 0, coinitMultithreaded)
	switch r {
	case sOK, sFalse:
		defer syscall.SyscallN(procCoUninitialize.Addr())
	case rpcEChangedMode:
		// The thread's COM was set up otherwise: WASAPI works in either.
	default:
		ready <- check(r)
		return
	}
	client, err := activateLoopback(pid)
	if err != nil {
		ready <- err
		return
	}
	defer client.release()
	l, err := startLoopback(client)
	if err != nil {
		ready <- err
		return
	}
	defer l.close()
	ready <- nil
	c.loop(l, write)
}

// activateLoopback activates a client that hears what pid and its children
// play.
func activateLoopback(pid uint32) (*iAudioClient, error) {
	if err := procActivateAudioInterfaceAsync.Find(); err != nil {
		return nil, ErrNoLoopback
	}
	h, _, errno := syscall.SyscallN(procCreateEventW.Addr(), 0, 0, 0, 0)
	if h == 0 {
		return nil, fmt.Errorf("output: WASAPI: CreateEvent: %w", errno)
	}
	handler := &activation{vtbl: activationMethods(), done: syscall.Handle(h)}
	handler.params = activationParams{activationType: activationProcessLoopback, targetProcessID: pid, loopbackMode: loopbackIncludeTree}
	handler.variant = blobVariant{vt: vtBlob, size: uint32(unsafe.Sizeof(handler.params)), data: unsafe.Pointer(&handler.params)}
	// Windows calls the handler, and reads its params, from a thread of its
	// own: they stay where they are meanwhile.
	var pinner runtime.Pinner
	pinner.Pin(handler)
	path, _ := syscall.UTF16PtrFromString(processLoopbackDevice)
	r, _, _ := syscall.SyscallN(procActivateAudioInterfaceAsync.Addr(), uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&iidIAudioClient)), uintptr(unsafe.Pointer(&handler.variant)),
		uintptr(unsafe.Pointer(handler)), uintptr(unsafe.Pointer(&handler.op)))
	if err := check(r); err != nil || handler.op == nil {
		pinner.Unpin()
		syscall.CloseHandle(handler.done)
		if err == nil {
			err = errors.New("output: WASAPI: the loopback's activation gave no operation")
		}
		return nil, fmt.Errorf("%w (%v)", ErrNoLoopback, err)
	}
	op := handler.op
	defer op.release()
	if ev, _ := syscall.WaitForSingleObject(handler.done, uint32(activationWait/time.Millisecond)); ev != syscall.WAIT_OBJECT_0 {
		// Windows may still complete it: its handler and event stay, pinned.
		abandoned.Lock()
		abandoned.handlers = append(abandoned.handlers, handler)
		abandoned.Unlock()
		return nil, errors.New("output: WASAPI: the loopback's activation didn't complete")
	}
	syscall.CloseHandle(handler.done)
	client, err := op.result()
	pinner.Unpin()
	if err != nil {
		return nil, fmt.Errorf("%w (%v)", ErrNoLoopback, err)
	}
	return client, nil
}

// loopback is a running client of process loopback.
type loopback struct {
	client  *iAudioClient
	capture *iAudioCaptureClient
	event   syscall.Handle
	// float is set when its frames are float32; else they are 16-bit PCM.
	float bool
}

func (l *loopback) close() {
	l.client.stop()
	if l.capture != nil {
		l.capture.release()
	}
	if l.event != 0 {
		syscall.CloseHandle(l.event)
	}
}

// startLoopback starts a client to hear in float32 stereo at Rate, which
// Windows converts what plays to, or else in 16-bit PCM, as Microsoft's
// sample of process loopback does.
func startLoopback(client *iAudioClient) (*loopback, error) {
	l := &loopback{client: client}
	const flags = streamflagsLoopback | streamflagsEventCallback
	float := waveFormat{
		formatTag:      waveFormatExtensible,
		channels:       2,
		samplesPerSec:  Rate,
		avgBytesPerSec: Rate * 8,
		blockAlign:     8,
		bitsPerSample:  32,
		size:           22,
		validBits:      32,
		channelMask:    speakerStereo,
		subFormat:      subtypeIEEEFloat,
	}
	pcm := waveFormat{formatTag: waveFormatPCM, channels: 2, samplesPerSec: Rate, avgBytesPerSec: Rate * 4, blockAlign: 4, bitsPerSample: 16}
	var err error
	for _, f := range []struct {
		format *waveFormat
		flags  uint32
	}{
		{&float, flags | streamflagsAutoConvert | streamflagsSRCQuality},
		{&float, flags},
		{&pcm, flags},
	} {
		if err = client.initialize(f.flags, int64(bufferDuration/100), f.format); err == nil {
			l.float = f.format == &float
			break
		}
	}
	if err != nil {
		return nil, err
	}
	h, _, errno := syscall.SyscallN(procCreateEventW.Addr(), 0, 0, 0, 0)
	if h == 0 {
		return nil, fmt.Errorf("output: WASAPI: CreateEvent: %w", errno)
	}
	l.event = syscall.Handle(h)
	fail := func(err error) (*loopback, error) {
		l.close()
		return nil, err
	}
	if err := client.setEventHandle(l.event); err != nil {
		return fail(err)
	}
	if l.capture, err = client.captureClient(); err != nil {
		return fail(err)
	}
	if err := client.start(); err != nil {
		return fail(err)
	}
	return l, nil
}

// loop hands what the client hears to write until Stop, and silence while
// it hears nothing, or until the client fails.
func (c *Capture) loop(l *loopback, write func([]float32)) {
	var frames []float32
	silence := make([]float32, 2*periodFrames)
	heard := time.Now() // when the last packet came
	filled := heard     // how far silence was given since
	for !c.stopping.Load() {
		syscall.WaitForSingleObject(l.event, 20)
		got := false
		for {
			n, err := l.capture.nextPacket()
			if err != nil {
				return
			}
			if n == 0 {
				break
			}
			p, n, flags, err := l.capture.buffer()
			if err != nil {
				return
			}
			frames = frames[:0]
			switch {
			case flags&bufferflagsSilent != 0 || p == nil:
				frames = append(frames, make([]float32, 2*n)...)
			case l.float:
				frames = append(frames, unsafe.Slice((*float32)(unsafe.Pointer(p)), 2*n)...)
			default:
				for _, v := range unsafe.Slice((*int16)(unsafe.Pointer(p)), 2*n) {
					frames = append(frames, float32(v)/32768)
				}
			}
			if err := l.capture.releaseBuffer(n); err != nil {
				return
			}
			write(frames)
			got = true
		}
		now := time.Now()
		if got {
			heard, filled = now, now
			continue
		}
		if now.Sub(heard) < quietAfter {
			continue
		}
		// Silence, at the pace of a stream, and no more than a moment of it
		// at once, as after the computer slept.
		if earliest := now.Add(-2 * quietAfter); filled.Before(earliest) {
			filled = earliest
		}
		for ; now.Sub(filled) >= period; filled = filled.Add(period) {
			write(silence)
		}
	}
}
