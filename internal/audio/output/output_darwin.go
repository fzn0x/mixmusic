//go:build darwin && (amd64 || arm64)

package output

// macOS plays through an Audio Queue on the default output, which follows
// it when it changes, as MyGo's audio does: the queue's own thread hands
// back each buffer it played, and the stream's goroutine renders into it
// and queues it again. A queue of 5.1 tells Core Audio its layout, which
// maps its channels to the device's speakers. A stream that asks for a
// device plays on it through a queue of its UID, and checks every second
// that the device is there: when it went away, a queue on the default
// output takes over, and gives way to one on the device once it's back.

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// audioStreamBasicDescription is AudioStreamBasicDescription.
type audioStreamBasicDescription struct {
	sampleRate       float64
	formatID         uint32
	formatFlags      uint32
	bytesPerPacket   uint32
	framesPerPacket  uint32
	bytesPerFrame    uint32
	channelsPerFrame uint32
	bitsPerChannel   uint32
	reserved         uint32
}

// audioChannelLayout is AudioChannelLayout, of one channel description,
// which a layout of a tag leaves out.
type audioChannelLayout struct {
	tag          uint32
	bitmap       uint32
	descriptions uint32
	description  [5]uint32 // label, flags and three float32 coordinates
}

// audioObjectPropertyAddress is AudioObjectPropertyAddress.
type audioObjectPropertyAddress struct {
	selector, scope, element uint32
}

// audioQueueBuffer is AudioQueueBuffer, whose fields Go lays out at the
// offsets of C's.
type audioQueueBuffer struct {
	audioDataBytesCapacity uint32
	audioData              unsafe.Pointer
	audioDataByteSize      uint32
	userData               uintptr
	packetDescCapacity     uint32
	packetDescs            uintptr
	packetDescCount        uint32
}

const (
	kAudioFormatLinearPCM = 0x6C70636D // 'lpcm'
	// kAudioFormatFlagsNativeFloatPacked: kAudioFormatFlagIsFloat |
	// kAudioFormatFlagIsPacked.
	kAudioFormatFlagsNativeFloatPacked = 1 | 8
	// kAudioQueueErr_CannotStart and kAudioQueueErr_InvalidDevice, the
	// errors of a system without an output device.
	errCannotStart   = -66681
	errInvalidDevice = -66683
	// aqBuffers is how many buffers a queue holds, of periodFrames each:
	// bufferDuration in all.
	aqBuffers = int(int64(Rate)*int64(bufferDuration)/int64(time.Second)) / periodFrames

	kAudioQueuePropertyChannelLayout = 0x6171636C // 'aqcl'
	kAudioObjectSystemObject         = 1
	// The properties of the default output device, and of its streams'
	// channels: 'dOut', 'slay', in the scopes 'glob' and 'outp'.
	kAudioHardwarePropertyDefaultOutputDevice = 0x644F7574
	kAudioDevicePropertyStreamConfiguration   = 0x736C6179
	kAudioObjectPropertyScopeGlobal           = 0x676C6F62
	kAudioDevicePropertyScopeOutput           = 0x6F757470
)

var (
	aqOnce sync.Once
	aqErr  error

	audioQueueNewOutput      func(format *audioStreamBasicDescription, callback uintptr, user uintptr, runLoop, runLoopMode uintptr, flags uint32, out *uintptr) int32
	audioQueueAllocateBuffer func(q uintptr, size uint32, out **audioQueueBuffer) int32
	audioQueueEnqueueBuffer  func(q uintptr, b *audioQueueBuffer, n uint32, descs uintptr) int32
	audioQueueStart          func(q uintptr, at uintptr) int32
	audioQueueStop           func(q uintptr, immediate bool) int32
	audioQueueDispose        func(q uintptr, immediate bool) int32
	audioQueueSetProperty    func(q uintptr, id uint32, data unsafe.Pointer, size uint32) int32

	audioObjectGetPropertyDataSize func(object uint32, addr *audioObjectPropertyAddress, qualifierSize uint32, qualifier unsafe.Pointer, size *uint32) int32
	audioObjectGetPropertyData     func(object uint32, addr *audioObjectPropertyAddress, qualifierSize uint32, qualifier unsafe.Pointer, size *uint32, data unsafe.Pointer) int32
	// coreAudio tells whether Core Audio's functions loaded, which tell
	// the channels of the device.
	coreAudio bool

	// aqCallback is made once, as callbacks are limited; it hands the
	// buffers a queue played back to the stream's queue, in aqStreams.
	aqCallback uintptr
	aqStreams  sync.Map // queue → *queue
)

func loadAudioToolbox() error {
	aqOnce.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/AudioToolbox.framework/AudioToolbox", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			aqErr = fmt.Errorf("output: AudioToolbox: %w", err)
			return
		}
		purego.RegisterLibFunc(&audioQueueNewOutput, lib, "AudioQueueNewOutput")
		purego.RegisterLibFunc(&audioQueueAllocateBuffer, lib, "AudioQueueAllocateBuffer")
		purego.RegisterLibFunc(&audioQueueEnqueueBuffer, lib, "AudioQueueEnqueueBuffer")
		purego.RegisterLibFunc(&audioQueueStart, lib, "AudioQueueStart")
		purego.RegisterLibFunc(&audioQueueStop, lib, "AudioQueueStop")
		purego.RegisterLibFunc(&audioQueueDispose, lib, "AudioQueueDispose")
		purego.RegisterLibFunc(&audioQueueSetProperty, lib, "AudioQueueSetProperty")
		if ca, err := purego.Dlopen("/System/Library/Frameworks/CoreAudio.framework/CoreAudio", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err == nil {
			purego.RegisterLibFunc(&audioObjectGetPropertyDataSize, ca, "AudioObjectGetPropertyDataSize")
			purego.RegisterLibFunc(&audioObjectGetPropertyData, ca, "AudioObjectGetPropertyData")
			coreAudio = true
		}
		aqCallback = purego.NewCallback(func(user, q uintptr, b *audioQueueBuffer) {
			if v, ok := aqStreams.Load(q); ok {
				qq := v.(*queue)
				qq.queued.Add(-1)
				select {
				case qq.free <- b:
				default:
				}
			}
		})
	})
	return aqErr
}

// Available returns nil where AudioToolbox loads, as on every Mac.
func Available() error { return loadAudioToolbox() }

func osStatus(call string, st int32) error {
	if st == errCannotStart || st == errInvalidDevice {
		return fmt.Errorf("%w (%s: OSStatus %d)", ErrNoDevice, call, st)
	}
	return fmt.Errorf("output: %s failed (OSStatus %d)", call, st)
}

// outputChannels returns the output channels of the default output device,
// 0 when Core Audio doesn't tell.
func outputChannels() int {
	if !coreAudio {
		return 0
	}
	dev := defaultOutputDevice()
	if dev == 0 {
		return 0
	}
	return deviceChannels(dev)
}

// Stream plays what its render function makes on the default output, or
// on the device its Options name, while that one is there.
type Stream struct {
	render func([]float32, int)
	// asked is the channels the stream asks for, and want the UID of the
	// device it asks for, "" for the default.
	asked int
	want  string
	avoid avoid
	mover *mover
	// q is the queue that plays, which the stream's goroutine changes as
	// the device does.
	q    atomic.Pointer[queue]
	stop chan struct{}
	once sync.Once
	done chan struct{}
}

// queue is an Audio Queue of a stream, on one device.
type queue struct {
	q uintptr
	// channels is the queue's: 2, or Surround.
	channels int
	// device is the UID of the device it plays on, "" for the default
	// device, which it follows; fallback is set while it plays there in
	// place of the device the stream asks for.
	device   string
	fallback bool
	free     chan *audioQueueBuffer
	queued   atomic.Int64 // buffers the queue holds
}

// Open starts a stream as o tells: of o.Channels, 2, or Surround for 5.1,
// which it plays where the device has six output channels or more, else
// stereo, on o.Device or the default device. A queue of 5.1 on the default
// device stays one when the default device changes, which Core Audio then
// mixes down. Open returns once the queue plays.
func Open(o Options) (*Stream, error) {
	if err := loadAudioToolbox(); err != nil {
		return nil, err
	}
	s := &Stream{render: o.Render, asked: o.Channels, want: o.Device, mover: newMover(o),
		stop: make(chan struct{}), done: make(chan struct{})}
	q, err := s.open()
	if err != nil {
		return nil, err
	}
	s.q.Store(q)
	s.mover.tell(q.fallback)
	go func() {
		err := s.run()
		s.q.Load().dispose()
		close(s.done)
		if err != nil && o.Failed != nil {
			o.Failed(err)
		}
	}()
	return s, nil
}

// target returns the UID of the device the stream plays on, "" for the
// default one, its output channels, and whether it plays on the default in
// place of the device the stream asks for.
func (s *Stream) target() (uid string, channels int, fallback bool) {
	if s.want != "" && s.avoid.may(time.Now()) {
		if _, ch, ok := outputDevice(s.want); ok {
			return s.want, ch, false
		}
	}
	return "", outputChannels(), s.want != ""
}

// open starts a queue on the device the stream plays on, as target tells.
// When the device asked for is there but fails to start, the default
// device plays, and avoid keeps the stream from trying it for a while.
func (s *Stream) open() (*queue, error) {
	uid, ch, fallback := s.target()
	channels := 2
	if s.asked == Surround {
		channels = coreAudioChannels(Surround, ch)
	}
	q, err := s.start(channels, uid)
	if err != nil && channels == Surround {
		// A device that takes no queue of 5.1 plays stereo.
		q, err = s.start(2, uid)
	}
	if err != nil && uid != "" {
		s.avoid.failed(time.Now())
		return s.open()
	}
	if err != nil {
		return nil, err
	}
	q.fallback = fallback
	return q, nil
}

// start starts a queue of channels on the device of the UID uid, or on the
// default device when it's "".
func (s *Stream) start(channels int, uid string) (*queue, error) {
	desc := audioStreamBasicDescription{
		sampleRate:       Rate,
		formatID:         kAudioFormatLinearPCM,
		formatFlags:      kAudioFormatFlagsNativeFloatPacked,
		bytesPerPacket:   uint32(channels * 4),
		framesPerPacket:  1,
		bytesPerFrame:    uint32(channels * 4),
		channelsPerFrame: uint32(channels),
		bitsPerChannel:   32,
	}
	q := &queue{channels: channels, device: uid, free: make(chan *audioQueueBuffer, aqBuffers)}
	if st := audioQueueNewOutput(&desc, aqCallback, 0, 0, 0, 0, &q.q); st != 0 {
		return nil, osStatus("AudioQueueNewOutput", st)
	}
	aqStreams.Store(q.q, q)
	fail := func(err error) (*queue, error) {
		q.dispose()
		return nil, err
	}
	if uid != "" {
		ref := cfString(uid)
		st := audioQueueSetProperty(q.q, kAudioQueuePropertyCurrentDevice, unsafe.Pointer(&ref), uint32(unsafe.Sizeof(ref)))
		cfRelease(ref)
		if st != 0 {
			return fail(osStatus("AudioQueueSetProperty", st))
		}
	}
	if channels == Surround {
		layout := audioChannelLayout{tag: kAudioChannelLayoutTagMPEG51A}
		if st := audioQueueSetProperty(q.q, kAudioQueuePropertyChannelLayout, unsafe.Pointer(&layout), uint32(unsafe.Sizeof(layout))); st != 0 {
			return fail(osStatus("AudioQueueSetProperty", st))
		}
	}
	// Each buffer holds something before the queue starts, not to start
	// with an underrun.
	for range aqBuffers {
		var b *audioQueueBuffer
		if st := audioQueueAllocateBuffer(q.q, uint32(periodFrames*channels*4), &b); st != 0 {
			return fail(osStatus("AudioQueueAllocateBuffer", st))
		}
		if err := s.fill(q, b); err != nil {
			return fail(err)
		}
	}
	if st := audioQueueStart(q.q, 0); st != 0 {
		return fail(osStatus("AudioQueueStart", st))
	}
	return q, nil
}

// dispose stops the queue and disposes of it.
func (q *queue) dispose() {
	if q.q == 0 {
		return
	}
	audioQueueStop(q.q, true)
	audioQueueDispose(q.q, true)
	aqStreams.Delete(q.q)
}

// Stop stops the stream and disposes of its queue. It waits for the
// stream's goroutine to end; render is not called after.
func (s *Stream) Stop() {
	s.once.Do(func() { close(s.stop) })
	<-s.done
}

// Delay returns how long what render makes now waits before it plays.
func (s *Stream) Delay() time.Duration {
	q := s.q.Load()
	if q == nil {
		return 0
	}
	return time.Duration(q.queued.Load()*periodFrames) * time.Second / Rate
}

func (s *Stream) run() error {
	// A queue that stopped taking buffers, as when its device went away.
	stalled := time.NewTimer(time.Second)
	defer stalled.Stop()
	// While the stream asks for a device, it checks whether it went away,
	// or came back.
	var check <-chan time.Time
	if s.want != "" {
		t := time.NewTicker(checkInterval)
		defer t.Stop()
		check = t.C
	}
	for {
		q := s.q.Load()
		select {
		case <-s.stop:
			return nil
		case b := <-q.free:
			if err := s.fill(q, b); err != nil {
				return err
			}
			stalled.Reset(time.Second)
		case <-stalled.C:
			if uid, _, _ := s.target(); s.want == "" || q.device == "" || uid == q.device {
				return errors.New("output: the audio queue stopped playing")
			}
			// The device asked for went away: the default device plays.
			if err := s.move(); err != nil {
				return err
			}
			stalled.Reset(time.Second)
		case <-check:
			if uid, _, _ := s.target(); uid == q.device {
				continue
			}
			if err := s.move(); err != nil {
				return err
			}
			stalled.Reset(time.Second)
		}
	}
}

// move has the stream play on the device it should, in a queue of its
// own: what the queue before held is dropped.
func (s *Stream) move() error {
	s.q.Load().dispose()
	q, err := s.open()
	if err != nil {
		// Stop and the stream's end dispose of a queue that never played.
		s.q.Store(&queue{free: make(chan *audioQueueBuffer)})
		return err
	}
	s.q.Store(q)
	s.mover.tell(q.fallback)
	return nil
}

// fill renders a period into b and queues it on q, turning a panic of
// render into an error.
func (s *Stream) fill(q *queue, b *audioQueueBuffer) (err error) {
	out := unsafe.Slice((*float32)(b.audioData), periodFrames*q.channels)
	defer func() {
		if v := recover(); v != nil {
			clear(out)
			err = fmt.Errorf("output: rendering the audio failed: %v", v)
		}
	}()
	s.render(out, q.channels)
	b.audioDataByteSize = uint32(len(out) * 4)
	q.queued.Add(1)
	if st := audioQueueEnqueueBuffer(q.q, b, 0, 0); st != 0 {
		q.queued.Add(-1)
		return osStatus("AudioQueueEnqueueBuffer", st)
	}
	return nil
}
