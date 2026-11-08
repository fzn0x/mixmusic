//go:build linux && (amd64 || arm64)

package output

// Linux plays through PulseAudio's simple API, which PipeWire serves as
// well, or else ALSA's default device, as MyGo's audio does. Both block
// while the device holds enough: the stream's goroutine renders a period,
// writes it, and goes on. PulseAudio and PipeWire move the stream to the
// default device when it changes. A stream of 5.1 is one where PulseAudio's
// sink has a center, an LFE and surrounds, as its introspection tells;
// ALSA's stays stereo. A stream that asks for a device plays on that sink,
// or that ALSA device, and checks every second, beside its goroutine, that
// the device is there: when it went away, the stream plays on the default
// device, and goes back to the device once it's there again.

import (
	"errors"
	"fmt"
	"strings"
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

	introspectOnce sync.Once
	introspectErr  error

	paMainloopNew              func() uintptr
	paMainloopGetAPI           func(m uintptr) uintptr
	paMainloopIterate          func(m uintptr, block int32, retval *int32) int32
	paMainloopFree             func(m uintptr)
	paContextNew               func(api uintptr, name *byte) uintptr
	paContextConnect           func(c uintptr, server *byte, flags int32, api uintptr) int32
	paContextGetState          func(c uintptr) int32
	paContextDisconnect        func(c uintptr)
	paContextUnref             func(c uintptr)
	paContextGetSinkInfoByName func(c uintptr, name *byte, cb uintptr, userdata uintptr) uintptr
	paContextGetSinkInfoList   func(c uintptr, cb uintptr, userdata uintptr) uintptr
	paOperationGetState        func(op uintptr) int32
	paOperationUnref           func(op uintptr)
	// sinkCallback is the pa_sink_info_cb_t of the queries, made once, as
	// callbacks are limited; it keeps the sinks it hears of in sinkHeard,
	// which sinkMu guards with the queries.
	sinkCallback uintptr
	sinkMu       sync.Mutex
	sinkHeard    []pulseSink

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

// paChannelMap is pa_channel_map, of PA_CHANNELS_MAX positions.
type paChannelMap struct {
	channels uint8
	position [32]int32
}

// paSinkInfo is the start of pa_sink_info, as 64-bit systems lay it out:
// all its fields a query reads.
type paSinkInfo struct {
	name        *byte
	index       uint32
	description *byte
	sampleSpec  paSampleSpec
	channelMap  paChannelMap
}

// paBufferAttr is pa_buffer_attr.
type paBufferAttr struct {
	maxLength, tLength, preBuf, minReq, fragSize uint32
}

const (
	paStreamPlayback  = 1
	paSampleFloat32LE = 5

	paContextNoAutospawn = 1
	paContextReady       = 4
	paContextFailed      = 5
	paContextTerminated  = 6
	paOperationRunning   = 0

	sndPcmStreamPlayback      = 0
	sndPcmFormatFloatLE       = 14
	sndPcmAccessRWInterleaved = 3

	// sinkTimeout bounds a query of PulseAudio's sinks.
	sinkTimeout = time.Second
	// defaultSink is the name PulseAudio's queries take for the default
	// sink.
	defaultSink = "@DEFAULT_SINK@"
)

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

// loadIntrospect loads what of PulseAudio's library tells its sinks: their
// names and channels.
func loadIntrospect() error {
	introspectOnce.Do(func() {
		lib, err := purego.Dlopen("libpulse.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			introspectErr = fmt.Errorf("output: PulseAudio: %w", err)
			return
		}
		purego.RegisterLibFunc(&paMainloopNew, lib, "pa_mainloop_new")
		purego.RegisterLibFunc(&paMainloopGetAPI, lib, "pa_mainloop_get_api")
		purego.RegisterLibFunc(&paMainloopIterate, lib, "pa_mainloop_iterate")
		purego.RegisterLibFunc(&paMainloopFree, lib, "pa_mainloop_free")
		purego.RegisterLibFunc(&paContextNew, lib, "pa_context_new")
		purego.RegisterLibFunc(&paContextConnect, lib, "pa_context_connect")
		purego.RegisterLibFunc(&paContextGetState, lib, "pa_context_get_state")
		purego.RegisterLibFunc(&paContextDisconnect, lib, "pa_context_disconnect")
		purego.RegisterLibFunc(&paContextUnref, lib, "pa_context_unref")
		purego.RegisterLibFunc(&paContextGetSinkInfoByName, lib, "pa_context_get_sink_info_by_name")
		purego.RegisterLibFunc(&paContextGetSinkInfoList, lib, "pa_context_get_sink_info_list")
		purego.RegisterLibFunc(&paOperationGetState, lib, "pa_operation_get_state")
		purego.RegisterLibFunc(&paOperationUnref, lib, "pa_operation_unref")
		sinkCallback = purego.NewCallback(func(c uintptr, info *paSinkInfo, eol int32, user uintptr) {
			if eol != 0 || info == nil {
				return
			}
			sinkHeard = append(sinkHeard, sinkOf(info))
		})
	})
	return introspectErr
}

// sinkOf returns what a pa_sink_info tells of its sink.
func sinkOf(info *paSinkInfo) pulseSink {
	n := min(int(info.channelMap.channels), len(info.channelMap.position))
	return pulseSink{
		name:        cString(info.name),
		description: cString(info.description),
		positions:   append([]int32(nil), info.channelMap.position[:n]...),
	}
}

// cString returns the string of C at p, which ends at its first 0.
func cString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// pulseQuery asks PulseAudio, as PipeWire's PulseAudio server answers too,
// for the sinks of the query that start begins on a context, and returns
// those it heard of. It reports false when no server answered within
// sinkTimeout.
func pulseQuery(start func(c uintptr) uintptr) ([]pulseSink, bool) {
	if loadIntrospect() != nil {
		return nil, false
	}
	sinkMu.Lock()
	defer sinkMu.Unlock()
	m := paMainloopNew()
	if m == 0 {
		return nil, false
	}
	defer paMainloopFree(m)
	c := paContextNew(paMainloopGetAPI(m), cstr("MixMusic"))
	if c == 0 {
		return nil, false
	}
	defer paContextUnref(c)
	if paContextConnect(c, nil, paContextNoAutospawn, 0) < 0 {
		return nil, false
	}
	defer paContextDisconnect(c)
	deadline := time.Now().Add(sinkTimeout)
	// iterate runs the main loop, without blocking in it, until done.
	iterate := func(done func() bool) bool {
		for !done() {
			if time.Now().After(deadline) || paMainloopIterate(m, 0, nil) < 0 {
				return false
			}
			if !done() {
				time.Sleep(2 * time.Millisecond)
			}
		}
		return true
	}
	settled := func() bool {
		st := paContextGetState(c)
		return st == paContextReady || st == paContextFailed || st == paContextTerminated
	}
	if !iterate(settled) || paContextGetState(c) != paContextReady {
		return nil, false
	}
	sinkHeard = nil
	op := start(c)
	if op == 0 {
		return nil, false
	}
	defer paOperationUnref(op)
	if !iterate(func() bool { return paOperationGetState(op) != paOperationRunning }) {
		return nil, false
	}
	return sinkHeard, true
}

// pulseSinks returns PulseAudio's sinks, false when no server answers.
func pulseSinks() ([]pulseSink, bool) {
	return pulseQuery(func(c uintptr) uintptr { return paContextGetSinkInfoList(c, sinkCallback, 0) })
}

// sinkNamed returns PulseAudio's sink of the name, defaultSink for the
// default one, false when there is none or no server answers.
func sinkNamed(name string) (pulseSink, bool) {
	sinks, ok := pulseQuery(func(c uintptr) uintptr { return paContextGetSinkInfoByName(c, cstr(name), sinkCallback, 0) })
	if !ok || len(sinks) == 0 {
		return pulseSink{}, false
	}
	return sinks[0], true
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
		registerHints(lib)
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

// Devices returns PulseAudio's sinks, sorted by name, the default one
// marked, or, where no PulseAudio server answers, ALSA's output devices.
func Devices() ([]Device, error) {
	if err := Available(); err != nil {
		return nil, err
	}
	if sinks, ok := pulseSinks(); ok {
		def, _ := sinkNamed(defaultSink)
		return pulseDevices(sinks, def.name), nil
	}
	if loadALSA() == nil {
		return alsaDevices(alsaHints()), nil
	}
	return nil, ErrNoDevice
}

// present reports whether the device of the ID id is there, as Devices
// would list it, and false for ok when the system doesn't tell.
func present(id string) (here, ok bool) {
	if name, isALSA := strings.CutPrefix(id, alsaPrefix); isALSA {
		if loadALSA() != nil {
			return false, false
		}
		for _, h := range alsaHints() {
			if h.name == name {
				return true, true
			}
		}
		return false, true
	}
	sinks, ok := pulseSinks()
	if !ok {
		return false, false
	}
	for _, s := range sinks {
		if s.name == id {
			return true, true
		}
	}
	return false, true
}

// device is PulseAudio's stream or ALSA's.
type device interface {
	// channels returns the channels of its frames: 2, or Surround.
	channels() int
	// write plays interleaved frames after those before, waiting while the
	// device holds enough.
	write(samples []float32) error
	// delay returns the frames written that the device has yet to play.
	delay() int64
	// close drops what the device holds, and closes it.
	close()
}

// openDevice opens the device a stream that asks for the device want plays
// on: want, while it's there and the stream may try it, else the default
// device, and reports whether that is in want's place. When want is there
// but fails to open, the default device plays, and a keeps the stream from
// trying want for a while.
func openDevice(want string, a *avoid, channels int) (device, bool, error) {
	if err := Available(); err != nil {
		return nil, false, err
	}
	if want != "" && a.may(time.Now()) {
		if d, ok := openWanted(want, channels); ok {
			return d, false, nil
		}
		if here, _ := present(want); here {
			a.failed(time.Now())
		}
	}
	var perr, aerr error
	if loadPulse() == nil {
		d, err := openPulse("", channels)
		if err == nil {
			return d, want != "", nil
		}
		perr = err
	}
	if loadALSA() == nil {
		d, err := openALSA("default")
		if err == nil {
			return d, want != "", nil
		}
		aerr = err
	}
	return nil, false, fmt.Errorf("%w: %w", ErrNoDevice, errors.Join(perr, aerr))
}

// openWanted opens the device of the ID want, false when it isn't there or
// can't open.
func openWanted(want string, channels int) (device, bool) {
	if name, isALSA := strings.CutPrefix(want, alsaPrefix); isALSA {
		if loadALSA() != nil {
			return nil, false
		}
		d, err := openALSA(name)
		return d, err == nil
	}
	if loadPulse() != nil {
		return nil, false
	}
	// A sound server may put a stream asked for a sink that isn't there on
	// another: only a sink that is there is asked for.
	if here, _ := present(want); !here {
		return nil, false
	}
	d, err := openPulse(want, channels)
	return d, err == nil
}

type pulseDevice struct {
	s  uintptr
	ch int
}

// openPulse opens a stream of channels on the sink of the name, the
// default sink for "": of Surround where the sink has 5.1 or more, else
// stereo.
func openPulse(sink string, channels int) (*pulseDevice, error) {
	ch := 2
	var chmap unsafe.Pointer
	if channels == Surround {
		name := sink
		if name == "" {
			name = defaultSink
		}
		if s, ok := sinkNamed(name); ok {
			if m, ok := pulseSurroundMap(s.positions); ok {
				cm := &paChannelMap{channels: Surround}
				copy(cm.position[:], m[:])
				ch, chmap = Surround, unsafe.Pointer(cm)
			}
		}
	}
	spec := paSampleSpec{format: paSampleFloat32LE, rate: Rate, channels: uint8(ch)}
	frameBytes := uint32(4 * ch)
	// bufferDuration ahead, asked for more every period.
	attr := paBufferAttr{
		maxLength: ^uint32(0),
		tLength:   uint32(int64(Rate)*int64(bufferDuration)/int64(time.Second)) * frameBytes,
		preBuf:    ^uint32(0),
		minReq:    periodFrames * frameBytes,
		fragSize:  ^uint32(0),
	}
	var dev *byte
	if sink != "" {
		dev = cstr(sink)
	}
	var code int32
	s := paSimpleNew(nil, cstr("MixMusic"), paStreamPlayback, dev, cstr("Music"), &spec, chmap, unsafe.Pointer(&attr), &code)
	if s == 0 {
		if ch == Surround {
			return openPulse(sink, 2)
		}
		return nil, fmt.Errorf("output: PulseAudio: %s", paStrerror(code))
	}
	return &pulseDevice{s, ch}, nil
}

func (d *pulseDevice) channels() int { return d.ch }

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

// openALSA opens ALSA's device of the name, in stereo.
func openALSA(name string) (*alsaDevice, error) {
	var pcm uintptr
	if e := sndPcmOpen(&pcm, cstr(name), sndPcmStreamPlayback, 0); e < 0 {
		return nil, fmt.Errorf("output: ALSA: %s", sndStrerror(e))
	}
	latency := uint32(bufferDuration / time.Microsecond)
	if e := sndPcmSetParams(pcm, sndPcmFormatFloatLE, sndPcmAccessRWInterleaved, 2, Rate, 1, latency); e < 0 {
		sndPcmClose(pcm)
		return nil, fmt.Errorf("output: ALSA: %s", sndStrerror(e))
	}
	return &alsaDevice{pcm}, nil
}

func (d *alsaDevice) channels() int { return 2 }

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

// Stream plays what its render function makes on the default output, or
// on the device its Options name, while that one is there.
type Stream struct {
	render func([]float32, int)
	// asked is the channels the stream asks for, and want the device it
	// asks for, "" for the default.
	asked int
	want  string
	avoid avoid
	mover *mover
	// dev is the device that plays, which the stream's goroutine changes;
	// fallback is set while it is the default device in want's place, and
	// move once the stream should open its device again, as the device it
	// asks for went away or came back.
	dev      device
	fallback atomic.Bool
	move     atomic.Bool
	stopping atomic.Bool
	done     chan struct{}
	delay    atomic.Int64 // frames
}

// Open starts a stream as o tells: of o.Channels, 2, or Surround for 5.1,
// which it plays where PulseAudio's sink has 5.1 or more, else stereo, on
// o.Device or the default device. A stream of 5.1 on the default device
// stays one when the default sink changes, which the sound server then
// mixes down. Open returns once the device is open.
func Open(o Options) (*Stream, error) {
	s := &Stream{render: o.Render, asked: o.Channels, want: o.Device, mover: newMover(o), done: make(chan struct{})}
	dev, fallback, err := openDevice(s.want, &s.avoid, s.asked)
	if err != nil {
		return nil, err
	}
	s.dev = dev
	s.fallback.Store(fallback)
	s.mover.tell(fallback)
	if s.want != "" {
		go s.watch()
	}
	go func() {
		err := s.run()
		s.dev.close()
		close(s.done)
		if err != nil && o.Failed != nil {
			o.Failed(err)
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

// watch checks, every checkInterval until the stream ends, whether the
// device the stream asks for went away, or came back, and has the stream
// open its device again then. It asks the sound server beside the stream's
// goroutine, which a slow answer would hold up.
func (s *Stream) watch() {
	t := time.NewTicker(checkInterval)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
		}
		here, ok := present(s.want)
		if ok && here == s.fallback.Load() {
			s.move.Store(true)
		}
	}
}

func (s *Stream) run() error {
	var buf []float32
	for !s.stopping.Load() {
		if s.move.Swap(false) && s.moves() {
			if err := s.reopen(); err != nil {
				return err
			}
		}
		ch := s.dev.channels()
		if len(buf) != ch*periodFrames {
			buf = make([]float32, ch*periodFrames)
		}
		if err := s.fill(buf, ch); err != nil {
			return err
		}
		if err := s.dev.write(buf); err != nil {
			if s.want == "" || s.fallback.Load() {
				return err
			}
			// The device asked for went away: the default device plays.
			if err := s.reopen(); err != nil {
				return err
			}
			continue
		}
		s.delay.Store(s.dev.delay())
	}
	return nil
}

// moves reports whether the stream, which watch asked to move, opens its
// device again: not while it plays on the default device after the device
// it asks for failed to open, until it may try that one again.
func (s *Stream) moves() bool {
	return !s.fallback.Load() || s.avoid.may(time.Now())
}

// reopen closes the device that plays, and opens the one the stream should
// play on: what the device before held is dropped.
func (s *Stream) reopen() error {
	s.dev.close()
	s.delay.Store(0)
	dev, fallback, err := openDevice(s.want, &s.avoid, s.asked)
	if err != nil {
		// The stream's end closes a device: this one is closed already.
		s.dev = closed{}
		return err
	}
	s.dev = dev
	s.fallback.Store(fallback)
	s.mover.tell(fallback)
	return nil
}

// closed is the device of a stream whose device closed, and couldn't open
// again.
type closed struct{}

func (closed) channels() int         { return 2 }
func (closed) write([]float32) error { return ErrNoDevice }
func (closed) delay() int64          { return 0 }
func (closed) close()                {}

// fill renders into buf, of ch channels, turning a panic of render into an
// error.
func (s *Stream) fill(buf []float32, ch int) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("output: rendering the audio failed: %v", v)
		}
	}()
	s.render(buf, ch)
	return nil
}
