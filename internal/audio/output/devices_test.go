package output

import (
	"slices"
	"testing"
	"time"
)

// TestEndpointDevices lists WASAPI's endpoints by their friendly names,
// sorted as people read them, the default one marked: an endpoint without
// a name shows its ID, and one without an ID isn't listed.
func TestEndpointDevices(t *testing.T) {
	got := endpointDevices([]Device{
		{ID: "{0.0.0.00000000}.{b}", Name: "Speakers (Realtek(R) Audio)"},
		{ID: "{0.0.0.00000000}.{a}", Name: "headphones (USB DAC)"},
		{ID: "{0.0.0.00000000}.{c}"},
		{Name: "No ID"},
	}, "{0.0.0.00000000}.{b}")
	want := []Device{
		{ID: "{0.0.0.00000000}.{a}", Name: "headphones (USB DAC)"},
		{ID: "{0.0.0.00000000}.{b}", Name: "Speakers (Realtek(R) Audio)", Default: true},
		{ID: "{0.0.0.00000000}.{c}", Name: "{0.0.0.00000000}.{c}"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("devices %+v, want %+v", got, want)
	}
}

// TestCoreAudioDevices lists Core Audio's devices that have output
// channels, by name, the default output marked: a microphone, a device
// hidden from the system's settings, as a private aggregate device is, and
// one without a UID aren't listed.
func TestCoreAudioDevices(t *testing.T) {
	got := coreAudioDevices([]coreAudioDevice{
		{id: 41, uid: "BuiltInMicrophoneDevice", name: "MacBook Pro Microphone", outputs: 0},
		{id: 48, uid: "BuiltInSpeakerDevice", name: "MacBook Pro Speakers", outputs: 2},
		{id: 90, uid: "BlackHole2ch_UID", name: "BlackHole 2ch", outputs: 2},
		{id: 95, uid: "F2-8C-1E-AA-00-01:output", name: "AirPods Pro", outputs: 2},
		{id: 120, uid: "AggregateTap-1", name: "MixMusic's visualizer", outputs: 2, hidden: true},
		{id: 130, name: "No UID", outputs: 2},
		{id: 140, uid: "Unnamed_UID", outputs: 6},
	}, 48)
	want := []Device{
		{ID: "F2-8C-1E-AA-00-01:output", Name: "AirPods Pro"},
		{ID: "BlackHole2ch_UID", Name: "BlackHole 2ch"},
		{ID: "BuiltInSpeakerDevice", Name: "MacBook Pro Speakers", Default: true},
		{ID: "Unnamed_UID", Name: "Unnamed_UID"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("devices %+v, want %+v", got, want)
	}
}

// TestPulseDevices lists PulseAudio's sinks by their descriptions, the
// default sink marked.
func TestPulseDevices(t *testing.T) {
	got := pulseDevices([]pulseSink{
		{name: "alsa_output.pci-0000_00_1f.3.analog-stereo", description: "Built-in Audio Analog Stereo"},
		{name: "bluez_output.AA_BB_CC_DD_EE_FF.1", description: "WH-1000XM4"},
		{name: "auto_null"},
		{description: "No name"},
	}, "bluez_output.AA_BB_CC_DD_EE_FF.1")
	want := []Device{
		{ID: "auto_null", Name: "auto_null"},
		{ID: "alsa_output.pci-0000_00_1f.3.analog-stereo", Name: "Built-in Audio Analog Stereo"},
		{ID: "bluez_output.AA_BB_CC_DD_EE_FF.1", Name: "WH-1000XM4", Default: true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("devices %+v, want %+v", got, want)
	}
}

// TestALSADevices lists the hints of `aplay -L` on a computer of an Intel
// card with HDMI, a USB DAC and a device of the user's own: each card's
// default device once, its HDMI and S/PDIF outputs and the user's device,
// and none of the raw devices, the per-channel ones, the plugins or the
// microphones.
func TestALSADevices(t *testing.T) {
	hints := []alsaHint{
		{"null", "Discard all samples (playback) or generate zero samples (capture)", ""},
		{"default", "Default ALSA Output (currently PipeWire Media Server)", ""},
		{"mydac", "My DAC, with its own settings", "Output"},
		{"default:CARD=PCH", "HDA Intel PCH, ALC892 Analog\nDefault Audio Device", ""},
		{"sysdefault:CARD=PCH", "HDA Intel PCH, ALC892 Analog\nDefault Audio Device", ""},
		{"front:CARD=PCH,DEV=0", "HDA Intel PCH, ALC892 Analog\nFront output / input", ""},
		{"surround51:CARD=PCH,DEV=0", "HDA Intel PCH, ALC892 Analog\n5.1 Surround output to Front, Center, Rear and Subwoofer speakers", "Output"},
		{"iec958:CARD=PCH,DEV=0", "HDA Intel PCH, ALC892 Digital\nIEC958 (S/PDIF) Digital Audio Output", "Output"},
		{"hdmi:CARD=PCH,DEV=0", "HDA Intel PCH, HDMI 0\nHDMI Audio Output", "Output"},
		{"dmix:CARD=PCH,DEV=0", "HDA Intel PCH, ALC892 Analog\nDirect sample mixing device", "Output"},
		{"dsnoop:CARD=PCH,DEV=0", "HDA Intel PCH, ALC892 Analog\nDirect sample snooping device", "Input"},
		{"hw:CARD=PCH,DEV=0", "HDA Intel PCH, ALC892 Analog\nDirect hardware device without any conversions", ""},
		{"plughw:CARD=PCH,DEV=0", "HDA Intel PCH, ALC892 Analog\nHardware device with all software conversions", ""},
		{"sysdefault:CARD=DAC", "USB Audio DAC, USB Audio\nDefault Audio Device", ""},
		{"usbstream:CARD=DAC", "USB Audio DAC\nUSB Stream Output", ""},
		{"sysdefault:CARD=Mic", "Webcam Microphone, USB Audio\nDefault Audio Device", "Input"},
	}
	got := alsaDevices(hints)
	want := []Device{
		{ID: "alsa:default:CARD=PCH", Name: "HDA Intel PCH, ALC892 Analog · Default Audio Device"},
		{ID: "alsa:iec958:CARD=PCH,DEV=0", Name: "HDA Intel PCH, ALC892 Digital · IEC958 (S/PDIF) Digital Audio Output"},
		{ID: "alsa:hdmi:CARD=PCH,DEV=0", Name: "HDA Intel PCH, HDMI 0 · HDMI Audio Output"},
		{ID: "alsa:mydac", Name: "My DAC, with its own settings"},
		{ID: "alsa:sysdefault:CARD=DAC", Name: "USB Audio DAC, USB Audio · Default Audio Device"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("devices:\n%+v\nwant\n%+v", got, want)
	}
	if got := alsaName(alsaHint{name: "plain"}); got != "plain" {
		t.Errorf("a device without a description is named %q", got)
	}
}

// TestMover tells Moved where a stream that asks for a device plays, as it
// starts and as that changes, once for each change, and never for a
// stream of the default device.
func TestMover(t *testing.T) {
	var told []bool
	m := newMover(Options{Device: "headphones", Moved: func(fallback bool) { told = append(told, fallback) }})
	for _, fallback := range []bool{false, false, true, true, false} {
		m.tell(fallback)
	}
	if want := []bool{false, true, false}; !slices.Equal(told, want) {
		t.Errorf("told %v, want %v", told, want)
	}
	told = nil
	m = newMover(Options{Moved: func(fallback bool) { told = append(told, fallback) }})
	m.tell(false)
	m.tell(true)
	if told != nil {
		t.Errorf("a stream of the default device told %v", told)
	}
	newMover(Options{Device: "headphones"}).tell(true) // without Moved
}

// TestAvoid keeps a stream off the device it asks for for a while after
// the device failed to open.
func TestAvoid(t *testing.T) {
	var a avoid
	now := time.Now()
	if !a.may(now) {
		t.Fatal("a device that never failed is avoided")
	}
	a.failed(now)
	if a.may(now.Add(avoidFor - time.Millisecond)) {
		t.Error("a device that failed is tried again at once")
	}
	if !a.may(now.Add(avoidFor)) {
		t.Errorf("a device that failed isn't tried again after %v", avoidFor)
	}
}
