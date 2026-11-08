package output

// The output devices: Devices lists them, and a Stream plays on the one
// its Options name, or on the system's default device. What each system
// tells of its devices is put together here, apart from the calls that
// ask it, so that tests give it what a system would.

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// Device is an audio output device of the system, as Devices lists it.
type Device struct {
	// ID tells the device from the others, from one run to the next: its
	// endpoint ID on Windows, its UID on macOS, and on Linux the name of
	// its sink in PulseAudio, or "alsa:" and its name in ALSA.
	ID string
	// Name is what the system calls it, as its sound settings show.
	Name string
	// Default is set on the device the system plays on by default, where
	// the system tells which it is.
	Default bool
}

// Options are those of a Stream: what Start takes, and the device it
// plays on.
type Options struct {
	// Channels is what the stream asks for: 2, or Surround for 5.1, which
	// it plays where the device mixes 5.1 or more, else stereo.
	Channels int
	// Render is called from the stream's goroutine for each period of the
	// device, about every 10 ms, to fill out with frames of the channels it
	// tells, interleaved: those of the device that plays, which change as
	// the device does. It must not block.
	Render func(out []float32, channels int)
	// Failed, when set, is called with why when the stream stops of
	// itself, as when no output device is left, from the stream's
	// goroutine once the stream has ended; Stop is not needed then.
	Failed func(error)
	// Device is the ID of the device to play on, as Devices tells it; ""
	// plays on the system's default device, and follows it as it changes.
	// While the device isn't there, or can't play, the stream plays on the
	// default device, and it goes back to the device once it can.
	Device string
	// Moved, when set, is told whether the stream plays on the default
	// device because Device isn't there or can't play: as the stream
	// starts, and each time that changes. It is never told while Device is
	// "". It is called from the stream's goroutines, and must not block.
	Moved func(fallback bool)
}

// Start starts a stream of channels on the system's default device, as
// Open does with Options of channels, render and failed.
func Start(channels int, render func(out []float32, channels int), failed func(error)) (*Stream, error) {
	return Open(Options{Channels: channels, Render: render, Failed: failed})
}

const (
	// checkInterval is how often a stream checks where it should play:
	// which device is the default, and whether the device it asks for is
	// there.
	checkInterval = time.Second
	// avoidFor is how long a stream plays on the default device after the
	// device it asks for was there but failed to open, before it tries
	// that device again.
	avoidFor = 10 * time.Second
)

// mover tells a stream's Moved where the stream plays, once for each
// change. One goroutine of the stream uses it at a time.
type mover struct {
	moved          func(bool)
	told, fallback bool
}

func newMover(o Options) *mover {
	if o.Device == "" {
		return &mover{}
	}
	return &mover{moved: o.Moved}
}

// tell tells Moved whether the stream plays on the default device in place
// of the one it asks for, unless it told so last.
func (m *mover) tell(fallback bool) {
	if m.moved == nil || m.told && m.fallback == fallback {
		return
	}
	m.told, m.fallback = true, fallback
	m.moved(fallback)
}

// avoid is when a stream may try the device it asks for again, after it
// failed to open: the zero time while it may.
type avoid struct{ until time.Time }

// failed has the stream play on the default device for avoidFor.
func (a *avoid) failed(now time.Time) { a.until = now.Add(avoidFor) }

// may reports whether the stream may try the device it asks for.
func (a *avoid) may(now time.Time) bool { return !now.Before(a.until) }

// cstr returns s as a string of C, which ends in a 0.
func cstr(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}

// sortDevices sorts devices by name, as people read them, then by ID.
func sortDevices(devices []Device) []Device {
	slices.SortStableFunc(devices, func(a, b Device) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return devices
}

// endpointDevices returns the devices of WASAPI's active render endpoints,
// whose names are their friendly names, the default one marked: the ID
// stands for a name the endpoint doesn't tell.
func endpointDevices(endpoints []Device, defaultID string) []Device {
	out := make([]Device, 0, len(endpoints))
	for _, d := range endpoints {
		if d.ID == "" {
			continue
		}
		if d.Name == "" {
			d.Name = d.ID
		}
		d.Default = d.ID == defaultID
		out = append(out, d)
	}
	return sortDevices(out)
}

// coreAudioDevice is what Core Audio tells of one of its devices.
type coreAudioDevice struct {
	id        uint32
	uid, name string
	// outputs is the device's output channels, hidden whether it's hidden
	// from the system's settings, as a private aggregate device is.
	outputs int
	hidden  bool
}

// coreAudioDevices returns the output devices of Core Audio's devices:
// those with output channels and a UID, which aren't hidden, the default
// output device marked.
func coreAudioDevices(devices []coreAudioDevice, defaultID uint32) []Device {
	var out []Device
	for _, d := range devices {
		if d.outputs <= 0 || d.uid == "" || d.hidden {
			continue
		}
		name := d.name
		if name == "" {
			name = d.uid
		}
		out = append(out, Device{ID: d.uid, Name: name, Default: d.id == defaultID})
	}
	return sortDevices(out)
}

// pulseSink is what PulseAudio tells of a sink: its name, its description
// and the positions of its channels.
type pulseSink struct {
	name, description string
	positions         []int32
}

// pulseDevices returns the devices of PulseAudio's sinks, the default sink,
// named def, marked.
func pulseDevices(sinks []pulseSink, def string) []Device {
	var out []Device
	for _, s := range sinks {
		if s.name == "" {
			continue
		}
		name := s.description
		if name == "" {
			name = s.name
		}
		out = append(out, Device{ID: s.name, Name: name, Default: s.name == def})
	}
	return sortDevices(out)
}

// alsaPrefix starts the IDs of ALSA's devices, which tells them apart from
// PulseAudio's sinks.
const alsaPrefix = "alsa:"

// alsaHint is a device of ALSA's configuration, as snd_device_name_hint
// tells it: its name, its description, of a line or two, and "Input" or
// "Output" for a device of one direction, "" for one of both.
type alsaHint struct{ name, desc, ioid string }

// alsaSkipped are the starts of the names of ALSA's devices that aren't
// listed: the raw devices of the cards, which take no float32 audio at
// 48 kHz but those that convert it; each card's devices for its channels,
// which play one card's speakers as its default device does; and the
// mixing and recording plugins.
var alsaSkipped = []string{"hw:", "plughw:", "dmix:", "dsnoop:", "front:", "rear:", "center_lfe:", "side:", "surround", "usbstream:"}

// alsaDevices returns the output devices of ALSA's hints: each card's
// default device, its HDMI and S/PDIF outputs, and the devices of the
// user's own configuration, but for "null" and "default", which plays as
// the system's default does. Of two names of the same device, as
// default:CARD= and sysdefault:CARD= are, the first stays.
func alsaDevices(hints []alsaHint) []Device {
	var out []Device
	seen := map[string]bool{}
	for _, h := range hints {
		if h.name == "" || h.name == "null" || h.name == "default" || h.ioid == "Input" ||
			slices.ContainsFunc(alsaSkipped, func(p string) bool { return strings.HasPrefix(h.name, p) }) {
			continue
		}
		name := alsaName(h)
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, Device{ID: alsaPrefix + h.name, Name: name})
	}
	return sortDevices(out)
}

// alsaName is the name of an ALSA device: its description, of the card and
// then the kind of output, on one line, else its name.
func alsaName(h alsaHint) string {
	var lines []string
	for line := range strings.SplitSeq(h.desc, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return h.name
	}
	return strings.Join(lines, " · ")
}
