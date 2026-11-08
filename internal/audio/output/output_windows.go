//go:build windows && (amd64 || arm64)

package output

import (
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procCreateEventW      = kernel32.NewProc("CreateEventW")
	procGetCurrentThread  = kernel32.NewProc("GetCurrentThread")
	procSetThreadPriority = kernel32.NewProc("SetThreadPriority")

	avrt                                = syscall.NewLazyDLL("avrt.dll")
	procAvSetMmThreadCharacteristicsW   = avrt.NewProc("AvSetMmThreadCharacteristicsW")
	procAvRevertMmThreadCharacteristics = avrt.NewProc("AvRevertMmThreadCharacteristics")
)

const (
	threadPriorityNormal       = 0
	threadPriorityTimeCritical = 15
)

// boostThread has Windows run the calling thread ahead of other programs'
// threads, as it does audio threads: the stream renders only
// bufferDuration ahead, and a game or a busy browser kept a thread of
// normal priority waiting for longer, which the user heard as stutters.
// It registers the thread with the multimedia scheduler (MMCSS) as Pro
// Audio, or, without MMCSS, raises its priority. It returns how to undo
// it.
func boostThread() (undo func()) {
	if procAvSetMmThreadCharacteristicsW.Find() == nil {
		task, _ := syscall.UTF16PtrFromString("Pro Audio")
		var index uint32
		h, _, _ := procAvSetMmThreadCharacteristicsW.Call(uintptr(unsafe.Pointer(task)), uintptr(unsafe.Pointer(&index)))
		if h != 0 {
			return func() { procAvRevertMmThreadCharacteristics.Call(h) }
		}
	}
	thread, _, _ := procGetCurrentThread.Call()
	if ok, _, _ := procSetThreadPriority.Call(thread, threadPriorityTimeCritical); ok == 0 {
		return func() {}
	}
	return func() { procSetThreadPriority.Call(thread, threadPriorityNormal) }
}

// device is an opened output device.
type device struct {
	id     string
	client *iAudioClient
	render *iAudioRenderClient
	clock  *iAudioClock
	event  syscall.Handle
	frames uint32 // the size of its buffer
	freq   uint64 // the frequency of its clock
	// channels is the channels of the stream: 2, or Surround on a device
	// of 5.1 or more, when the stream asks for them.
	channels int
	// fallback is set on the default device that plays in place of the
	// one the stream asks for, which isn't there or can't play.
	fallback bool
}

func (d *device) close() {
	if d.client != nil {
		d.client.stop()
	}
	if d.clock != nil {
		d.clock.release()
	}
	if d.render != nil {
		d.render.release()
	}
	if d.client != nil {
		d.client.release()
	}
	if d.event != 0 {
		syscall.CloseHandle(d.event)
	}
}

// pick returns the endpoint a stream that asks for the device want plays
// on: want, while it is there and active, and the stream may try it, else
// the default device, and whether that is in want's place.
func pick(e *iMMDeviceEnumerator, want string, may bool) (dev *iMMDevice, fallback bool, err error) {
	if want != "" && may {
		if dev, err := e.device(want); err == nil {
			if st, err := dev.state(); err == nil && st == deviceStateActive {
				return dev, false, nil
			}
			dev.release()
		}
	}
	dev, err = e.defaultRender()
	return dev, want != "", err
}

// openOn opens the device a stream that asks for the device want plays on,
// as pick picks it. When want is there but fails to open, the default
// device plays, and avoid keeps the stream from trying want for a while.
func openOn(e *iMMDeviceEnumerator, want string, a *avoid, channels int) (*device, error) {
	dev, fallback, err := pick(e, want, a.may(time.Now()))
	if err != nil {
		return nil, err
	}
	d, err := openEndpoint(dev, channels)
	dev.release()
	if err != nil && !fallback && want != "" {
		a.failed(time.Now())
		return openOn(e, want, a, channels)
	}
	if d != nil {
		d.fallback = fallback
	}
	return d, err
}

// openEndpoint opens an output device for a stream of float32 at Rate,
// which Windows converts to the device's own format: of Surround channels
// when asked for them and the device mixes 5.1 or more, its surrounds as
// the device's mix names them, else stereo.
func openEndpoint(dev *iMMDevice, channels int) (*device, error) {
	var err error
	d := &device{}
	if d.id, err = dev.id(); err != nil {
		return nil, err
	}
	if d.client, err = dev.activateClient(); err != nil {
		return nil, err
	}
	fail := func(err error) (*device, error) {
		d.close()
		return nil, err
	}
	mask := uint32(speakerStereo)
	d.channels = 2
	if channels == Surround {
		if n, m, err := d.client.mixFormat(); err == nil {
			if sm, ok := surroundMask(n, m); ok {
				d.channels, mask = Surround, sm
			}
		}
	}
	format := streamFormat(d.channels, mask)
	const flags = streamflagsEventCallback | streamflagsAutoConvert | streamflagsSRCQuality
	if err := d.client.initialize(flags, int64(bufferDuration/100), &format); err != nil {
		if d.channels == Surround {
			// A device that takes no stream of 5.1, whatever its mix says,
			// plays stereo, through a client of its own.
			d.close()
			return openEndpoint(dev, 2)
		}
		return fail(err)
	}
	h, _, errno := syscall.SyscallN(procCreateEventW.Addr(), 0, 0, 0, 0)
	if h == 0 {
		return fail(fmt.Errorf("output: WASAPI: CreateEvent: %w", errno))
	}
	d.event = syscall.Handle(h)
	if err := d.client.setEventHandle(d.event); err != nil {
		return fail(err)
	}
	if d.frames, err = d.client.bufferSize(); err != nil {
		return fail(err)
	}
	if d.render, err = d.client.renderClient(); err != nil {
		return fail(err)
	}
	if d.clock, err = d.client.clock(); err != nil {
		return fail(err)
	}
	if d.freq, err = d.clock.frequency(); err != nil {
		return fail(err)
	}
	if d.freq == 0 {
		return fail(errors.New("output: WASAPI: the device's clock has no frequency"))
	}
	return d, nil
}

// streamFormat is the WAVEFORMATEXTENSIBLE of a stream of float32 at Rate
// of channels on the speakers of mask.
func streamFormat(channels int, mask uint32) waveFormat {
	return waveFormat{
		formatTag:      waveFormatExtensible,
		channels:       uint16(channels),
		samplesPerSec:  Rate,
		avgBytesPerSec: uint32(Rate * 4 * channels),
		blockAlign:     uint16(4 * channels),
		bitsPerSample:  32,
		size:           22,
		validBits:      32,
		channelMask:    mask,
		subFormat:      subtypeIEEEFloat,
	}
}

// Available returns nil: Windows always has WASAPI, if not a device.
func Available() error { return nil }

// Stream plays what its render function makes on an output device of
// Windows: the default device, following it when it changes, or the device
// its Options name, while that one is there.
type Stream struct {
	render func([]float32, int)
	// channels is what the stream asks devices for, and want the device it
	// asks for, "" for the default.
	channels int
	want     string
	avoid    avoid
	mover    *mover

	stopping atomic.Bool
	done     chan struct{}
	// delay is the frames rendered that the device has not played yet.
	delay atomic.Int64
}

// Open starts a stream as o tells: of o.Channels, 2, or Surround for 5.1,
// which it plays where the device mixes 5.1 or more, else stereo, on
// o.Device or the default device. It returns once the device plays, or why
// it can't.
func Open(o Options) (*Stream, error) {
	s := &Stream{render: o.Render, channels: o.Channels, want: o.Device, mover: newMover(o), done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		err := s.run(ready)
		close(s.done)
		if err != nil && o.Failed != nil {
			o.Failed(err)
		}
	}()
	if err := <-ready; err != nil {
		<-s.done
		return nil, err
	}
	return s, nil
}

// Stop stops the stream and releases the device, so that Windows may sleep.
// It waits for the stream's goroutine to end; render is not called after.
func (s *Stream) Stop() {
	s.stopping.Store(true)
	<-s.done
}

// Delay returns how long what render makes now waits before it plays.
func (s *Stream) Delay() time.Duration {
	return time.Duration(s.delay.Load()) * time.Second / Rate
}

// run plays the stream until Stop, or until it fails: it sends to ready
// whether it started, and returns why it ended after it did.
func (s *Stream) run(ready chan<- error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer boostThread()()
	undo, err := initCOM()
	if err != nil {
		ready <- err
		return nil
	}
	defer undo()
	e, err := newDeviceEnumerator()
	if err != nil {
		ready <- err
		return nil
	}
	defer e.release()
	d, err := openOn(e, s.want, &s.avoid, s.channels)
	if err == nil {
		if err = s.prime(d); err != nil {
			d.close()
		}
	}
	if err == nil {
		s.mover.tell(d.fallback)
	}
	ready <- err
	if err != nil {
		return nil
	}
	return s.loop(e, d)
}

// prime fills the buffer of a new device, and starts it.
func (s *Stream) prime(d *device) error {
	if err := s.fill(d, d.frames); err != nil {
		return err
	}
	return d.client.start()
}

// fill renders n frames into the device's buffer.
func (s *Stream) fill(d *device, n uint32) (err error) {
	if n == 0 {
		return nil
	}
	p, err := d.render.buffer(n)
	if err != nil {
		return err
	}
	out := unsafe.Slice((*float32)(unsafe.Pointer(p)), d.channels*int(n))
	defer func() {
		if v := recover(); v != nil {
			clear(out)
			d.render.releaseBuffer(n)
			err = fmt.Errorf("output: WASAPI: rendering the audio failed: %v", v)
		}
	}()
	s.render(out, d.channels)
	return d.render.releaseBuffer(n)
}

// deviceWait is how long the stream waits for an output device when there
// is none, as while the user switches devices, before it fails.
const deviceWait = 2 * time.Second

func (s *Stream) loop(e *iMMDeviceEnumerator, d *device) error {
	defer func() { d.close() }()
	// written counts the frames given to the devices; played, those they
	// played, or dropped when the device changed.
	written, played := int64(d.frames), int64(0)
	var base int64 // the frames played on the devices before d
	lastCheck := time.Now()
	for !s.stopping.Load() {
		if _, err := syscall.WaitForSingleObject(d.event, 200); err != nil {
			return fmt.Errorf("output: WASAPI: waiting for the device: %w", err)
		}
		// Whether signaled or not, the device's padding tells what to
		// render, and whether it is still there.
		reopen := false
		padding, err := d.client.currentPadding()
		if err == nil {
			n := d.frames - padding
			if err = s.fill(d, n); err == nil {
				written += int64(n)
			}
		}
		if err != nil {
			if !errors.Is(err, hresult(audclntEInvalidated)) {
				return err
			}
			reopen = true
		}
		if pos, err := d.clock.position(); err == nil {
			played = base + int64(pos*Rate/d.freq)
			s.delay.Store(max(0, written-played))
		}
		if !reopen && time.Since(lastCheck) >= checkInterval {
			lastCheck = time.Now()
			reopen = s.elsewhere(e, d)
		}
		if !reopen {
			continue
		}
		// The device changed: what it held is dropped, and the next one
		// plays from here.
		d.close()
		d = &device{}
		base, played = written, written
		s.delay.Store(0)
		nd, err := s.reopen(e)
		if err != nil || nd == nil {
			return err
		}
		d = nd
		s.mover.tell(d.fallback)
		written += int64(d.frames)
		lastCheck = time.Now()
	}
	return nil
}

// elsewhere reports whether the stream should play on another device than
// d: as the default device changed, or the device the stream asks for went
// away, or came back.
func (s *Stream) elsewhere(e *iMMDeviceEnumerator, d *device) bool {
	dev, _, err := pick(e, s.want, s.avoid.may(time.Now()))
	if err != nil {
		return errors.Is(err, hresult(eNotFound))
	}
	id, err := dev.id()
	dev.release()
	return err == nil && id != d.id
}

// reopen opens and primes the device the stream plays on, waiting for one
// to appear for deviceWait. It returns nil, nil when the stream stops
// meanwhile.
func (s *Stream) reopen(e *iMMDeviceEnumerator) (*device, error) {
	deadline := time.Now().Add(deviceWait)
	for {
		d, err := openOn(e, s.want, &s.avoid, s.channels)
		if err == nil {
			if err = s.prime(d); err == nil {
				return d, nil
			}
			d.close()
		}
		missing := errors.Is(err, hresult(eNotFound)) || errors.Is(err, hresult(audclntEInvalidated))
		if !missing || time.Now().After(deadline) {
			return nil, err
		}
		if s.stopping.Load() {
			return nil, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}
