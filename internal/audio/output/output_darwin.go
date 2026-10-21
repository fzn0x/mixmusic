//go:build darwin && (amd64 || arm64)

package output

// macOS plays through an Audio Queue on the default output, which follows
// it when it changes, as MyGo's audio does: the queue's own thread hands
// back each buffer it played, and the stream's goroutine renders into it
// and queues it again.

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

// Stream plays what its render function makes on the default output.
type Stream struct {
	render func([]float32)
	q      uintptr
	free   chan *audioQueueBuffer
	queued atomic.Int64 // buffers the queue holds
	stop   chan struct{}
	once   sync.Once
	done   chan struct{}
}

// Start starts a stream. Its goroutine calls render for each period, 10
// ms, to fill out with interleaved stereo frames; render must not block.
// When the stream stops of itself, as when the device goes away, failed is
// called with why, from that goroutine once the stream has ended; Stop is
// not needed then. Start returns once the queue plays.
func Start(render func(out []float32), failed func(error)) (*Stream, error) {
	if err := loadAudioToolbox(); err != nil {
		return nil, err
	}
	desc := audioStreamBasicDescription{
		sampleRate:       Rate,
		formatID:         kAudioFormatLinearPCM,
		formatFlags:      kAudioFormatFlagsNativeFloatPacked,
		bytesPerPacket:   2 * 4,
		framesPerPacket:  1,
		bytesPerFrame:    2 * 4,
		channelsPerFrame: 2,
		bitsPerChannel:   32,
	}
	s := &Stream{
		render: render,
		free:   make(chan *audioQueueBuffer, aqBuffers),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
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
	// Each buffer holds something before the queue starts, not to start
	// with an underrun.
	for range aqBuffers {
		var b *audioQueueBuffer
		if st := audioQueueAllocateBuffer(s.q, periodFrames*2*4, &b); st != 0 {
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
	out := unsafe.Slice((*float32)(b.audioData), periodFrames*2)
	defer func() {
		if v := recover(); v != nil {
			clear(out)
			err = fmt.Errorf("output: rendering the audio failed: %v", v)
		}
	}()
	s.render(out)
	b.audioDataByteSize = uint32(len(out) * 4)
	s.queued.Add(1)
	if st := audioQueueEnqueueBuffer(s.q, b, 0, 0); st != 0 {
		s.queued.Add(-1)
		return osStatus("AudioQueueEnqueueBuffer", st)
	}
	return nil
}
