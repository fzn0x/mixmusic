//go:build linux && (amd64 || arm64)

package output

import (
	"slices"
	"testing"
	"unsafe"
)

// TestSinkOf reads a sink from a pa_sink_info as PulseAudio's query hands
// it, laid out as C lays it out on 64-bit systems.
func TestSinkOf(t *testing.T) {
	if off := unsafe.Offsetof(paSinkInfo{}.description); off != 16 {
		t.Errorf("description at %d", off)
	}
	if off := unsafe.Offsetof(paSinkInfo{}.sampleSpec); off != 24 {
		t.Errorf("sample_spec at %d", off)
	}
	if off := unsafe.Offsetof(paSinkInfo{}.channelMap); off != 36 {
		t.Errorf("channel_map at %d", off)
	}
	name := []byte("alsa_output.usb-DAC-00.analog-surround-51\x00")
	desc := []byte("USB DAC Analog Surround 5.1\x00")
	info := paSinkInfo{name: &name[0], index: 3, description: &desc[0]}
	info.channelMap.channels = 6
	copy(info.channelMap.position[:], []int32{paFrontLeft, paFrontRight, paRearLeft, paRearRight, paFrontCenter, paLFE})
	s := sinkOf(&info)
	if s.name != "alsa_output.usb-DAC-00.analog-surround-51" || s.description != "USB DAC Analog Surround 5.1" {
		t.Errorf("sink %q, %q", s.name, s.description)
	}
	if want := []int32{paFrontLeft, paFrontRight, paRearLeft, paRearRight, paFrontCenter, paLFE}; !slices.Equal(s.positions, want) {
		t.Errorf("positions %v", s.positions)
	}
	if s := sinkOf(&paSinkInfo{}); s.name != "" || s.description != "" || len(s.positions) != 0 {
		t.Errorf("an empty pa_sink_info read %+v", s)
	}
}

// TestHintsOf reads ALSA's hints from an array of them that ends with a
// nil, as snd_device_name_hint gives it.
func TestHintsOf(t *testing.T) {
	type fake struct{ name, desc, ioid string }
	fakes := []*fake{
		{"sysdefault:CARD=PCH", "HDA Intel PCH, ALC892 Analog\nDefault Audio Device", ""},
		{"dsnoop:CARD=PCH,DEV=0", "HDA Intel PCH\nDirect sample snooping device", "Input"},
	}
	array := []unsafe.Pointer{unsafe.Pointer(fakes[0]), unsafe.Pointer(fakes[1]), nil}
	got := hintsOf(unsafe.Pointer(&array[0]), func(hint unsafe.Pointer, id string) string {
		f := (*fake)(hint)
		switch id {
		case "NAME":
			return f.name
		case "DESC":
			return f.desc
		case "IOID":
			return f.ioid
		}
		t.Errorf("asked for %q", id)
		return ""
	})
	want := []alsaHint{{fakes[0].name, fakes[0].desc, ""}, {fakes[1].name, fakes[1].desc, "Input"}}
	if !slices.Equal(got, want) {
		t.Errorf("hints %+v, want %+v", got, want)
	}
	if got := hintsOf(unsafe.Pointer(&array[2]), nil); got != nil {
		t.Errorf("no hints read %+v", got)
	}
	s := []byte("Default Audio Device\x00more")
	if got := cString(&s[0]); got != "Default Audio Device" {
		t.Errorf("the string of C read %q", got)
	}
	if cString(nil) != "" {
		t.Error("a nil string of C read something")
	}
}
