//go:build darwin && (amd64 || arm64)

package output

// macOS plays through an Audio Queue on the default output, which follows
// it when it changes, as MyGo's audio does: the queue's own thread hands
// back each buffer it played, and the stream's goroutine renders into it
// and queues it again. A queue of 5.1 tells Core Audio its layout, which
// maps its channels to the device's speakers.

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
	// buffers a queue played back to the stream of the queue, in aqStreams.
	aqCallback uintptr
	aqStreams  sync.Map // queue → *Stream
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
				s := v.(*Stream)
				s.queued.Add(-1)
				select {
				case s.free <- b:
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
	addr := audioObjectPropertyAddress{kAudioHardwarePropertyDefaultOutputDevice, kAudioObjectPropertyScopeGlobal, 0}
	var dev uint32
	size := uint32(unsafe.Sizeof(dev))
	if audioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, nil, &size, unsafe.Pointer(&dev)) != 0 || dev == 0 {
		return 0
	}
	// An AudioBufferList: the number of buffers, then each AudioBuffer of
	// 16 bytes, after 4 bytes of padding, whose first field is its
	// channels.
	addr = audioObjectPropertyAddress{kAudioDevicePropertyStreamConfiguration, kAudioDevicePropertyScopeOutput, 0}
	if audioObjectGetPropertyDataSize(dev, &addr, 0, nil, &size) != 0 || size < 8 || size > 1<<16 {
		return 0
	}
	buf := make([]byte, size)
	if audioObjectGetPropertyData(dev, &addr, 0, nil, &size, unsafe.Pointer(&buf[0])) != 0 {
		return 0
	}
	buf = buf[:min(int(size), len(buf))]
	return bufferListChannels(buf)
}

// Stream plays what its render function makes on the default output.
type Stream struct {
	render func([]float32, int)
	// channels is the queue's: 2, or Surround.
	channels int
	q        uintptr
	free     chan *audioQueueBuffer
	queued   atomic.Int64 // buffers the queue holds
	stop     chan struct{}
	once     sync.Once
	done     chan struct{}
}

// Start starts a stream of channels, 2, or Surround for 5.1, which it
// plays where the default device has six output channels or more, else
// stereo. Its goroutine calls render for each period, 10 ms, to fill out
// with frames of the channels it tells, interleaved; render must not
// block. A queue of 5.1 stays one when the default device changes, which
// Core Audio then mixes down. When the stream stops of itself, as when the
// device goes away, failed is called with why, from that goroutine once
// the stream has ended; Stop is not needed then. Start returns once the
// queue plays.
func Start(channels int, render func(out []float32, channels int), failed func(error)) (*Stream, error) {
	if err := loadAudioToolbox(); err != nil {
		return nil, err
	}
	if channels == Surround {
		channels = coreAudioChannels(channels, outputChannels())
	} else {
		channels = 2
	}
	s, err := start(channels, render, failed)
	if err != nil && channels == Surround {
		// A device that takes no queue of 5.1 plays stereo.
		return start(2, render, failed)
	}
	return s, err
}

func start(channels int, render func(out []float32, channels int), failed func(error)) (*Stream, error) {
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
	s := &Stream{
		render:   render,
		channels: channels,
		free:     make(chan *audioQueueBuffer, aqBuffers),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	if st := audioQueueNewOutput(&desc, aqCallback, 0, 0, 0, 0, &s.q); st != 0 {
		return nil, osStatus("AudioQueueNewOutput", st)
	}
	aqStreams.Store(s.q, s)
	fail := func(err error) (*Stream, error) {
		audioQueueDispose(s.q, true)
		aqStreams.Delete(s.q)
		return nil, err
	}
	if channels == Surround {
		layout := audioChannelLayout{tag: kAudioChannelLayoutTagMPEG51A}
		if st := audioQueueSetProperty(s.q, kAudioQueuePropertyChannelLayout, unsafe.Pointer(&layout), uint32(unsafe.Sizeof(layout))); st != 0 {
			return fail(osStatus("AudioQueueSetProperty", st))
		}
	}
	// Each buffer holds something before the queue starts, not to start
	// with an underrun.
	for range aqBuffers {
		var b *audioQueueBuffer
		if st := audioQueueAllocateBuffer(s.q, uint32(periodFrames*channels*4), &b); st != 0 {
			return fail(osStatus("AudioQueueAllocateBuffer", st))
		}
		if err := s.fill(b); err != nil {
			return fail(err)
		}
	}
	if st := audioQueueStart(s.q, 0); st != 0 {
		return fail(osStatus("AudioQueueStart", st))
	}
	go func() {
		err := s.run()
		audioQueueStop(s.q, true)
		audioQueueDispose(s.q, true)
		aqStreams.Delete(s.q)
		close(s.done)
		if err != nil && failed != nil {
			failed(err)
		}
	}()
	return s, nil
}

// Stop stops the stream and disposes of its queue. It waits for the
// stream's goroutine to end; render is not called after.
func (s *Stream) Stop() {
	s.once.Do(func() { close(s.stop) })
	<-s.done
}

// Delay returns how long what render makes now waits before it plays.
func (s *Stream) Delay() time.Duration {
	return time.Duration(s.queued.Load()*periodFrames) * time.Second / Rate
}

func (s *Stream) run() error {
	// A queue that stopped taking buffers, as when its device went away.
	stalled := time.NewTimer(time.Second)
	defer stalled.Stop()
	for {
		select {
		case <-s.stop:
			return nil
		case b := <-s.free:
			if err := s.fill(b); err != nil {
				return err
			}
			stalled.Reset(time.Second)
		case <-stalled.C:
			return errors.New("output: the audio queue stopped playing")
		}
	}
}

// fill renders a period into b and queues it, turning a panic of render
// into an error.
func (s *Stream) fill(b *audioQueueBuffer) (err error) {
	out := unsafe.Slice((*float32)(b.audioData), periodFrames*s.channels)
	defer func() {
		if v := recover(); v != nil {
			clear(out)
			err = fmt.Errorf("output: rendering the audio failed: %v", v)
		}
	}()
	s.render(out, s.channels)
	b.audioDataByteSize = uint32(len(out) * 4)
	s.queued.Add(1)
	if st := audioQueueEnqueueBuffer(s.q, b, 0, 0); st != 0 {
		s.queued.Add(-1)
		return osStatus("AudioQueueEnqueueBuffer", st)
	}
	return nil
}
