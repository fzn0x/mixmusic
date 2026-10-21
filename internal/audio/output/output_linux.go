//go:build linux && (amd64 || arm64)

package output

// Linux plays through PulseAudio's simple API, which PipeWire serves as
// well, or else ALSA's default device, as MyGo's audio does. Both block
// while the device holds enough: the stream's goroutine renders a period,
// writes it, and goes on. PulseAudio and PipeWire move the stream to the
// default device when it changes.

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	pulseOnce sync.Once
	pulseErr  error

	paSimpleNew        func(server, name *byte, dir int32, dev, stream *byte, spec *paSampleSpec, chmap, attr unsafe.Pointer, err *int32) uintptr
	paSimpleWrite      func(s uintptr, data unsafe.Pointer, bytes uintptr, err *int32) int32
	paSimpleFlush      func(s uintptr, err *int32) int32
	paSimpleGetLatency func(s uintptr, err *int32) uint64
	paSimpleFree       func(s uintptr)
	paStrerror         func(err int32) string

	alsaOnce sync.Once
	alsaErr  error

	sndPcmOpen      func(pcm *uintptr, name *byte, stream int32, mode int32) int32
	sndPcmSetParams func(pcm uintptr, format, access int32, channels, rate uint32, softResample int32, latency uint32) int32
	sndPcmWritei    func(pcm uintptr, buf unsafe.Pointer, frames uintptr) int
	sndPcmRecover   func(pcm uintptr, err int32, silent int32) int32
	sndPcmDelay     func(pcm uintptr, delay *int) int32
	sndPcmDrop      func(pcm uintptr) int32
	sndPcmClose     func(pcm uintptr) int32
	sndStrerror     func(err int32) string
)

// paSampleSpec is pa_sample_spec.
type paSampleSpec struct {
	format   int32
	rate     uint32
	channels uint8
}

// paBufferAttr is pa_buffer_attr.
type paBufferAttr struct {
	maxLength, tLength, preBuf, minReq, fragSize uint32
}

const (
	paStreamPlayback  = 1
	paSampleFloat32LE = 5

	sndPcmStreamPlayback      = 0
	sndPcmFormatFloatLE       = 14
	sndPcmAccessRWInterleaved = 3

	frameBytes = 2 * 4
)

func cstr(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}

func loadPulse() error {
	pulseOnce.Do(func() {
		lib, err := purego.Dlopen("libpulse-simple.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			pulseErr = fmt.Errorf("output: PulseAudio: %w", err)
			return
		}
		purego.RegisterLibFunc(&paSimpleNew, lib, "pa_simple_new")
		purego.RegisterLibFunc(&paSimpleWrite, lib, "pa_simple_write")
		purego.RegisterLibFunc(&paSimpleFlush, lib, "pa_simple_flush")
		purego.RegisterLibFunc(&paSimpleGetLatency, lib, "pa_simple_get_latency")
		purego.RegisterLibFunc(&paSimpleFree, lib, "pa_simple_free")
		purego.RegisterLibFunc(&paStrerror, lib, "pa_strerror")
	})
	return pulseErr
}

func loadALSA() error {
	alsaOnce.Do(func() {
		lib, err := purego.Dlopen("libasound.so.2", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			alsaErr = fmt.Errorf("output: ALSA: %w", err)
			return
		}
		purego.RegisterLibFunc(&sndPcmOpen, lib, "snd_pcm_open")
		purego.RegisterLibFunc(&sndPcmSetParams, lib, "snd_pcm_set_params")
		purego.RegisterLibFunc(&sndPcmWritei, lib, "snd_pcm_writei")
		purego.RegisterLibFunc(&sndPcmRecover, lib, "snd_pcm_recover")
		purego.RegisterLibFunc(&sndPcmDelay, lib, "snd_pcm_delay")
		purego.RegisterLibFunc(&sndPcmDrop, lib, "snd_pcm_drop")
		purego.RegisterLibFunc(&sndPcmClose, lib, "snd_pcm_close")
		purego.RegisterLibFunc(&sndStrerror, lib, "snd_strerror")
	})
	return alsaErr
}

// Available returns nil where PulseAudio's or ALSA's library is installed,
// else why there is no output.
func Available() error {
	perr := loadPulse()
	if perr == nil {
		return nil
	}
	aerr := loadALSA()
	if aerr == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrUnsupported, errors.Join(perr, aerr))
}

// device is PulseAudio's stream or ALSA's.
type device interface {
	// write plays interleaved stereo frames after those before, waiting
	// while the device holds enough.
	write(samples []float32) error
	// delay returns the frames written that the device has yet to play.
	delay() int64
	// close drops what the device holds, and closes it.
	close()
}

func openDevice() (device, error) {
	if err := Available(); err != nil {
		return nil, err
	}
	var perr, aerr error
	if loadPulse() == nil {
		d, err := openPulse()
		if err == nil {
			return d, nil
		}
		perr = err
	}
	if loadALSA() == nil {
		d, err := openALSA()
		if err == nil {
			return d, nil
		}
		aerr = err
	}
	return nil, fmt.Errorf("%w: %w", ErrNoDevice, errors.Join(perr, aerr))
}

type pulseDevice struct{ s uintptr }

func openPulse() (*pulseDevice, error) {
	spec := paSampleSpec{format: paSampleFloat32LE, rate: Rate, channels: 2}
	// bufferDuration ahead, asked for more every period.
	attr := paBufferAttr{
		maxLength: ^uint32(0),
		tLength:   uint32(int64(Rate)*int64(bufferDuration)/int64(time.Second)) * frameBytes,
		preBuf:    ^uint32(0),
		minReq:    periodFrames * frameBytes,
		fragSize:  ^uint32(0),
	}
	var code int32
	s := paSimpleNew(nil, cstr("MixMusic"), paStreamPlayback, nil, cstr("Music"), &spec, nil, unsafe.Pointer(&attr), &code)
	if s == 0 {
		return nil, fmt.Errorf("output: PulseAudio: %s", paStrerror(code))
	}
	return &pulseDevice{s}, nil
}

func (d *pulseDevice) write(samples []float32) error {
	var code int32
	if paSimpleWrite(d.s, unsafe.Pointer(&samples[0]), uintptr(len(samples)*4), &code) < 0 {
		return fmt.Errorf("output: PulseAudio: %s", paStrerror(code))
	}
	return nil
}

func (d *pulseDevice) delay() int64 {
	var code int32
	us := paSimpleGetLatency(d.s, &code)
	if us == ^uint64(0) {
		return 0
	}
	return int64(us) * Rate / 1_000_000
}

func (d *pulseDevice) close() {
	var code int32
	paSimpleFlush(d.s, &code)
	paSimpleFree(d.s)
}

type alsaDevice struct{ pcm uintptr }

func openALSA() (*alsaDevice, error) {
	var pcm uintptr
	if e := sndPcmOpen(&pcm, cstr("default"), sndPcmStreamPlayback, 0); e < 0 {
		return nil, fmt.Errorf("output: ALSA: %s", sndStrerror(e))
	}
	latency := uint32(bufferDuration / time.Microsecond)
	if e := sndPcmSetParams(pcm, sndPcmFormatFloatLE, sndPcmAccessRWInterleaved, 2, Rate, 1, latency); e < 0 {
		sndPcmClose(pcm)
		return nil, fmt.Errorf("output: ALSA: %s", sndStrerror(e))
	}
	return &alsaDevice{pcm}, nil
}

func (d *alsaDevice) write(samples []float32) error {
	for len(samples) > 0 {
		n := sndPcmWritei(d.pcm, unsafe.Pointer(&samples[0]), uintptr(len(samples)/2))
		if n < 0 {
			// An underrun or a suspend, recovered from by preparing the
			// device again.
			if e := sndPcmRecover(d.pcm, int32(n), 1); e < 0 {
				return fmt.Errorf("output: ALSA: %s", sndStrerror(e))
			}
			continue
		}
		samples = samples[2*n:]
	}
	return nil
}

func (d *alsaDevice) delay() int64 {
	var n int
	if sndPcmDelay(d.pcm, &n) < 0 {
		return 0
	}
	return int64(max(n, 0))
}

func (d *alsaDevice) close() {
	sndPcmDrop(d.pcm)
	sndPcmClose(d.pcm)
}

// Stream plays what its render function makes on the default output.
type Stream struct {
	render   func([]float32)
	dev      device
	stopping atomic.Bool
	done     chan struct{}
	delay    atomic.Int64 // frames
}

// Start starts a stream. Its goroutine calls render for each period, 10
// ms, to fill out with interleaved stereo frames; render must not block.
// When the stream stops of itself, as when the sound server goes away,
// failed is called with why, from that goroutine once the stream has
// ended; Stop is not needed then. Start returns once the device is open.
func Start(render func(out []float32), failed func(error)) (*Stream, error) {
	dev, err := openDevice()
	if err != nil {
		return nil, err
	}
	s := &Stream{render: render, dev: dev, done: make(chan struct{})}
	go func() {
		err := s.run()
		s.dev.close()
		close(s.done)
		if err != nil && failed != nil {
			failed(err)
		}
	}()
	return s, nil
}

// Stop stops the stream and closes the device. It waits for the stream's
// goroutine to end; render is not called after.
func (s *Stream) Stop() {
	s.stopping.Store(true)
	<-s.done
}

// Delay returns how long what render makes now waits before it plays.
func (s *Stream) Delay() time.Duration {
	return time.Duration(s.delay.Load()) * time.Second / Rate
}

func (s *Stream) run() error {
	buf := make([]float32, 2*periodFrames)
	for !s.stopping.Load() {
		if err := s.fill(buf); err != nil {
			return err
		}
		if err := s.dev.write(buf); err != nil {
			return err
		}
		s.delay.Store(s.dev.delay())
	}
	return nil
}

// fill renders into buf, turning a panic of render into an error.
func (s *Stream) fill(buf []float32) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("output: rendering the audio failed: %v", v)
		}
	}()
	s.render(buf)
	return nil
}
