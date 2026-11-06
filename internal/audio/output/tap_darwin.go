//go:build darwin && (amd64 || arm64)

package output

// A process tap: from macOS 14.2 on, Core Audio hears what chosen processes
// play, and nothing of the others, through a tap
// (AudioHardwareCreateProcessTap of a CATapDescription). A private
// aggregate device of the tap alone gives what it hears as its input, which
// an IOProc of the device reads on Core Audio's I/O thread and hands to the
// capture's goroutine.
//
// A tap hears only once the user allowed MixMusic to record what other
// apps play, which macOS asks once, with the reason Info.plist gives
// (NSAudioCaptureUsageDescription). Until they answer, and once they
// refuse, it hears silence, and no call fails. So MixMusic asks first,
// through TCC's private TCCAccessRequest, and taps once they allowed it:
// a tap that has macOS ask itself starts only 90 s later, though the user
// answered at once, and TCCAccessPreflight goes on telling the process
// that asked that it didn't ask yet.

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/fzn0x/mixmusic/resample"
)

const (
	kAudioHardwarePropertyProcessObjectList           = 0x70727323 // 'prs#'
	kAudioHardwarePropertyTranslatePIDToProcessObject = 0x69643270 // 'id2p'
	kAudioProcessPropertyPID                          = 0x70706964 // 'ppid'
	kAudioProcessPropertyBundleID                     = 0x70626964 // 'pbid'
	kAudioTapPropertyFormat                           = 0x74666D74 // 'tfmt'
	kAudioFormatFlagIsFloat                           = 0x1

	// tapUnmuted is CATapMuteBehavior's CATapUnmuted: the processes play on
	// their device as well; tapMuted is CATapMuted, into the tap alone.
	tapUnmuted = 0
	tapMuted   = 1

	// tccAllowed and tccRefused are what TCCAccessPreflight answers of a
	// service the user allowed or refused; before it asked, it answers 2.
	tccAllowed = 0
	tccRefused = 1

	// tapBuffers is how many buffers of what a tap heard may wait for the
	// capture's goroutine, of tapFrames frames at most: the device's I/O
	// gives 512 at a time.
	tapBuffers = 8
	tapFrames  = 4096
)

var (
	tapOnce sync.Once
	tapErr  error

	audioHardwareCreateProcessTap       func(desc objc.ID, out *uint32) int32
	audioHardwareDestroyProcessTap      func(tap uint32) int32
	audioHardwareCreateAggregateDevice  func(desc objc.ID, out *uint32) int32
	audioHardwareDestroyAggregateDevice func(device uint32) int32
	audioDeviceCreateIOProcID           func(device uint32, proc, client uintptr, out *uintptr) int32
	audioDeviceDestroyIOProcID          func(device uint32, proc uintptr) int32
	audioDeviceStart                    func(device uint32, proc uintptr) int32
	audioDeviceStop                     func(device uint32, proc uintptr) int32

	// tccAccessPreflight and tccAccessRequest are TCC's, nil where they
	// aren't there.
	tccAccessPreflight func(service, options objc.ID) int32
	tccAccessRequest   func(service, options objc.ID, answer objc.Block)
	// allowed is set once the user allowed MixMusic to record what other
	// apps play, as TCCAccessRequest answered.
	allowed atomic.Bool

	tapPoolPush func() uintptr
	tapPoolPop  func(pool uintptr)

	// tapCallback is the IOProc of every tap's device, made once, as
	// callbacks are limited: it hands what a device heard to its tap, in
	// taps.
	tapCallback uintptr
	taps        sync.Map // device → *tap
)

// loadTaps loads what a tap needs: Core Audio's process taps, which
// macOS 14.2 brought, Foundation's objects to describe them, and TCC's
// answer, where it is there.
func loadTaps() error {
	tapOnce.Do(func() {
		if err := loadAudioToolbox(); err != nil {
			tapErr = fmt.Errorf("%w (%v)", ErrNoLoopback, err)
			return
		}
		ca, err := purego.Dlopen("/System/Library/Frameworks/CoreAudio.framework/CoreAudio", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil || !coreAudio {
			tapErr = fmt.Errorf("%w (CoreAudio: %v)", ErrNoLoopback, err)
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			tapErr = fmt.Errorf("%w (Foundation: %v)", ErrNoLoopback, err)
			return
		}
		if _, err := purego.Dlsym(ca, "AudioHardwareCreateProcessTap"); err != nil || objc.GetClass("CATapDescription") == 0 {
			tapErr = fmt.Errorf("%w (macOS hears what an app plays from 14.2 on)", ErrNoLoopback)
			return
		}
		purego.RegisterLibFunc(&audioHardwareCreateProcessTap, ca, "AudioHardwareCreateProcessTap")
		purego.RegisterLibFunc(&audioHardwareDestroyProcessTap, ca, "AudioHardwareDestroyProcessTap")
		purego.RegisterLibFunc(&audioHardwareCreateAggregateDevice, ca, "AudioHardwareCreateAggregateDevice")
		purego.RegisterLibFunc(&audioHardwareDestroyAggregateDevice, ca, "AudioHardwareDestroyAggregateDevice")
		purego.RegisterLibFunc(&audioDeviceCreateIOProcID, ca, "AudioDeviceCreateIOProcID")
		purego.RegisterLibFunc(&audioDeviceDestroyIOProcID, ca, "AudioDeviceDestroyIOProcID")
		purego.RegisterLibFunc(&audioDeviceStart, ca, "AudioDeviceStart")
		purego.RegisterLibFunc(&audioDeviceStop, ca, "AudioDeviceStop")
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			tapErr = fmt.Errorf("%w (libobjc: %v)", ErrNoLoopback, err)
			return
		}
		purego.RegisterLibFunc(&tapPoolPush, lib, "objc_autoreleasePoolPush")
		purego.RegisterLibFunc(&tapPoolPop, lib, "objc_autoreleasePoolPop")
		if tcc, err := purego.Dlopen("/System/Library/PrivateFrameworks/TCC.framework/Versions/A/TCC", purego.RTLD_LAZY); err == nil {
			if sym, err := purego.Dlsym(tcc, "TCCAccessPreflight"); err == nil {
				purego.RegisterFunc(&tccAccessPreflight, sym)
			}
			if sym, err := purego.Dlsym(tcc, "TCCAccessRequest"); err == nil {
				purego.RegisterFunc(&tccAccessRequest, sym)
			}
		}
		tapCallback = purego.NewCallback(func(device uint32, _ unsafe.Pointer, in *audioBufferList, _, _, _, _ unsafe.Pointer) int32 {
			if v, ok := taps.Load(device); ok && in != nil {
				v.(*tap).heard(in)
			}
			return 0
		})
	})
	return tapErr
}

var (
	selMu    sync.Mutex
	selCache = map[string]objc.SEL{}
)

func sel(name string) objc.SEL {
	selMu.Lock()
	defer selMu.Unlock()
	s, ok := selCache[name]
	if !ok {
		s = objc.RegisterName(name)
		selCache[name] = s
	}
	return s
}

func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

// withPool runs fn in an autorelease pool, on one thread, which releases
// the objects fn made that nothing keeps.
func withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := tapPoolPush()
	defer tapPoolPop(pool)
	fn()
}

func nsString(s string) objc.ID { return class("NSString").Send(sel("stringWithUTF8String:"), s) }

func nsNumber(v int) objc.ID { return class("NSNumber").Send(sel("numberWithInt:"), v) }

// nsDictionary returns an NSDictionary of the keys and values of kv, in
// turn.
func nsDictionary(kv ...any) objc.ID {
	d := class("NSMutableDictionary").Send(sel("dictionary"))
	for i := 0; i+1 < len(kv); i += 2 {
		d.Send(sel("setObject:forKey:"), kv[i+1], nsString(kv[i].(string)))
	}
	return d
}

// permission returns what the user answered when macOS asked whether
// MixMusic may record what other apps play: tccAllowed, tccRefused, or
// another answer before macOS asked, and where TCC doesn't tell. A process
// that asked goes on being told it didn't.
func permission() int32 {
	if loadTaps() != nil || tccAccessPreflight == nil {
		return -1
	}
	var st int32
	withPool(func() { st = tccAccessPreflight(nsString("kTCCServiceAudioCapture"), 0) })
	return st
}

// ask has macOS ask the user whether MixMusic may record what other apps
// play, unless they answered already, and waits for their answer: true
// once they allowed it, and where TCC can't ask, as Core Audio then asks
// at the tap.
func ask() bool {
	if allowed.Load() {
		return true
	}
	switch permission() {
	case tccAllowed:
		allowed.Store(true)
		return true
	case tccRefused:
		return false
	}
	if tccAccessRequest == nil {
		return true
	}
	answer := make(chan bool, 1)
	// TCC answers on a queue of its own.
	block := objc.NewBlock(func(_ objc.Block, ok bool) {
		select {
		case answer <- ok:
		default:
		}
	})
	withPool(func() { tccAccessRequest(nsString("kTCCServiceAudioCapture"), 0, block) })
	ok := <-answer
	block.Release()
	if ok {
		allowed.Store(true)
	}
	return ok
}

// AudioProcess is a process Core Audio knows: one that plays or records,
// or did.
type AudioProcess struct {
	PID uint32
	// BundleID is the identifier of its bundle, "" for none.
	BundleID string
}

// AudioProcesses returns the processes Core Audio knows, from macOS 14.2
// on; none before.
func AudioProcesses() []AudioProcess {
	if loadTaps() != nil {
		return nil
	}
	addr := audioObjectPropertyAddress{kAudioHardwarePropertyProcessObjectList, kAudioObjectPropertyScopeGlobal, 0}
	var size uint32
	if audioObjectGetPropertyDataSize(kAudioObjectSystemObject, &addr, 0, nil, &size) != 0 || size < 4 {
		return nil
	}
	ids := make([]uint32, size/4)
	if audioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, nil, &size, unsafe.Pointer(&ids[0])) != 0 {
		return nil
	}
	var procs []AudioProcess
	withPool(func() {
		for _, id := range ids[:min(int(size/4), len(ids))] {
			var pid int32
			addr := audioObjectPropertyAddress{kAudioProcessPropertyPID, kAudioObjectPropertyScopeGlobal, 0}
			size := uint32(unsafe.Sizeof(pid))
			if audioObjectGetPropertyData(id, &addr, 0, nil, &size, unsafe.Pointer(&pid)) != 0 || pid <= 0 {
				continue
			}
			p := AudioProcess{PID: uint32(pid)}
			// A CFString, which the caller releases.
			var bundle objc.ID
			addr.selector = kAudioProcessPropertyBundleID
			size = uint32(unsafe.Sizeof(bundle))
			if audioObjectGetPropertyData(id, &addr, 0, nil, &size, unsafe.Pointer(&bundle)) == 0 && bundle != 0 {
				p.BundleID = objc.Send[string](bundle, sel("UTF8String"))
				bundle.Send(sel("release"))
			}
			procs = append(procs, p)
		}
	})
	return procs
}

// processObject returns Core Audio's object of the process pid, 0 for none.
func processObject(pid uint32) uint32 {
	addr := audioObjectPropertyAddress{kAudioHardwarePropertyTranslatePIDToProcessObject, kAudioObjectPropertyScopeGlobal, 0}
	p := int32(pid)
	var object uint32
	size := uint32(unsafe.Sizeof(object))
	if audioObjectGetPropertyData(kAudioObjectSystemObject, &addr, uint32(unsafe.Sizeof(p)), unsafe.Pointer(&p), &size, unsafe.Pointer(&object)) != 0 {
		return 0
	}
	return object
}

// tap is a process tap, which its own aggregate device reads.
type tap struct {
	id, device uint32
	proc       uintptr // the device's IOProc
	desc       objc.ID // its CATapDescription
	// rate is the sample rate of what it hears.
	rate float64
	// full brings what the tap heard, as stereo, to the capture's
	// goroutine, which hands each buffer back in free.
	full, free chan []float32
}

// newTap taps the process object, muted or not as mute tells, and starts
// its device.
func newTap(process uint32, mute int) (*tap, error) {
	t := &tap{full: make(chan []float32, tapBuffers), free: make(chan []float32, tapBuffers)}
	for range tapBuffers {
		t.free <- make([]float32, 0, 2*tapFrames)
	}
	var err error
	withPool(func() { err = t.open(process, mute) })
	if err != nil {
		t.close()
		return nil, err
	}
	return t, nil
}

func (t *tap) open(process uint32, mute int) error {
	procs := class("NSArray").Send(sel("arrayWithObject:"), class("NSNumber").Send(sel("numberWithUnsignedInt:"), process))
	t.desc = class("CATapDescription").Send(sel("alloc")).Send(sel("initStereoMixdownOfProcesses:"), procs)
	if t.desc == 0 {
		return errors.New("output: the tap's CATapDescription wasn't made")
	}
	t.desc.Send(sel("setMuteBehavior:"), mute)
	t.desc.Send(sel("setPrivate:"), true)
	if st := audioHardwareCreateProcessTap(t.desc, &t.id); st != 0 {
		t.id = 0
		return osStatus("AudioHardwareCreateProcessTap", st)
	}
	var f audioStreamBasicDescription
	addr := audioObjectPropertyAddress{kAudioTapPropertyFormat, kAudioObjectPropertyScopeGlobal, 0}
	size := uint32(unsafe.Sizeof(f))
	if st := audioObjectGetPropertyData(t.id, &addr, 0, nil, &size, unsafe.Pointer(&f)); st != 0 {
		return osStatus("the tap's format", st)
	}
	if f.formatID != kAudioFormatLinearPCM || f.formatFlags&kAudioFormatFlagIsFloat == 0 || f.bitsPerChannel != 32 || f.sampleRate < 8000 {
		return fmt.Errorf("output: the tap hears %d-bit audio of format %#x, flags %#x, at %g Hz", f.bitsPerChannel, f.formatID, f.formatFlags, f.sampleRate)
	}
	t.rate = f.sampleRate
	// The device of the tap alone, which starts the tap as it starts.
	desc := nsDictionary(
		"name", nsString("MixMusic's visualizer"),
		"uid", class("NSUUID").Send(sel("UUID")).Send(sel("UUIDString")),
		"private", nsNumber(1),
		"stacked", nsNumber(0),
		"tapautostart", nsNumber(1),
		"taps", class("NSArray").Send(sel("arrayWithObject:"), nsDictionary(
			"uid", t.desc.Send(sel("UUID")).Send(sel("UUIDString")),
			"drift", nsNumber(1),
		)),
	)
	if st := audioHardwareCreateAggregateDevice(desc, &t.device); st != 0 {
		t.device = 0
		return osStatus("AudioHardwareCreateAggregateDevice", st)
	}
	taps.Store(t.device, t)
	if st := audioDeviceCreateIOProcID(t.device, tapCallback, 0, &t.proc); st != 0 {
		t.proc = 0
		return osStatus("AudioDeviceCreateIOProcID", st)
	}
	if st := audioDeviceStart(t.device, t.proc); st != 0 {
		return osStatus("AudioDeviceStart", st)
	}
	return nil
}

// heard hands what the device's input heard to the capture's goroutine,
// on Core Audio's I/O thread: it drops it when the goroutine lags.
func (t *tap) heard(in *audioBufferList) {
	select {
	case b := <-t.free:
		select {
		case t.full <- appendStereo(b[:0], in):
		default:
		}
	default:
	}
}

// close stops the tap's device, once its I/O ended, and destroys them.
func (t *tap) close() {
	if t.proc != 0 {
		audioDeviceStop(t.device, t.proc)
		audioDeviceDestroyIOProcID(t.device, t.proc)
	}
	if t.device != 0 {
		audioHardwareDestroyAggregateDevice(t.device)
		taps.Delete(t.device)
	}
	if t.id != 0 {
		audioHardwareDestroyProcessTap(t.id)
	}
	if t.desc != 0 {
		withPool(func() { t.desc.Send(sel("release")) })
	}
}

// Capture hears what a process plays.
type Capture struct {
	stopped
	once sync.Once
	stop chan struct{}
	done chan struct{}
}

// CaptureProcess starts hearing what the process pid plays, through a
// process tap, as interleaved float32 stereo at Rate, which the capture's
// goroutine hands to write about every 10 ms; write must not block. While
// it plays nothing, write is given silence at the same pace, so that what
// it feeds hears the sound stop. It returns once the capture runs, or why
// it can't: ErrNoLoopback before macOS 14.2, and ErrRefused once the user
// refused MixMusic to record what other apps play. The first capture has
// macOS ask them, and waits for their answer.
func CaptureProcess(pid uint32, write func([]float32)) (*Capture, error) {
	if err := loadTaps(); err != nil {
		return nil, err
	}
	if !ask() {
		return nil, ErrRefused
	}
	return startCapture(pid, write, tapUnmuted)
}

// startCapture is CaptureProcess once the user answered, with the tap's
// mute behavior: muted, the process plays into the tap alone, as tests
// have it.
func startCapture(pid uint32, write func([]float32), mute int) (*Capture, error) {
	if err := loadTaps(); err != nil {
		return nil, err
	}
	process := processObject(pid)
	if process == 0 {
		return nil, fmt.Errorf("output: Core Audio knows no process %d", pid)
	}
	t, err := newTap(process, mute)
	if err != nil {
		return nil, err
	}
	c := &Capture{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(c.done)
		defer t.close()
		c.loop(t, write)
	}()
	return c, nil
}

// Stop stops the capture: write is not called once it returns.
func (c *Capture) Stop() {
	c.once.Do(func() { close(c.stop) })
	<-c.done
}

// loop hands what the tap hears to write until Stop, at Rate, and silence
// while it hears nothing.
func (c *Capture) loop(t *tap, write func([]float32)) {
	// The tap hears at the rate of the output device.
	var conv *resample.Converter
	var out []float32
	if t.rate != Rate {
		if r, err := resample.New(int(t.rate), Rate, 2); err == nil {
			conv, out = r, make([]float32, 2*r.MaxOutput(tapFrames))
		}
	}
	q := newQuiet(time.Now())
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-c.stop:
			return
		case b := <-t.full:
			if conv != nil {
				_, n := conv.Process(b, out)
				write(out[:2*n])
			} else {
				write(b)
			}
			t.free <- b
			q.audio(time.Now())
		case now := <-tick.C:
			q.fill(now, write)
		}
	}
}
