//go:build linux && (amd64 || arm64)

package output

// ALSA's devices, for a Linux without a PulseAudio server: the hints of
// snd_device_name_hint, as `aplay -L` lists them.

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	sndDeviceNameHint     func(card int32, iface *byte, hints *unsafe.Pointer) int32
	sndDeviceNameGetHint  func(hint unsafe.Pointer, id *byte) unsafe.Pointer
	sndDeviceNameFreeHint func(hints unsafe.Pointer) int32
	// free is libc's, which frees the strings snd_device_name_get_hint
	// returns; nil where libc didn't load, which leaves them.
	free func(p unsafe.Pointer)
)

// registerHints registers ALSA's hints, of its library lib.
func registerHints(lib uintptr) {
	purego.RegisterLibFunc(&sndDeviceNameHint, lib, "snd_device_name_hint")
	purego.RegisterLibFunc(&sndDeviceNameGetHint, lib, "snd_device_name_get_hint")
	purego.RegisterLibFunc(&sndDeviceNameFreeHint, lib, "snd_device_name_free_hint")
	if libc, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
		purego.RegisterLibFunc(&free, libc, "free")
	}
}

// alsaHints returns the hints of ALSA's PCM devices.
func alsaHints() []alsaHint {
	if loadALSA() != nil {
		return nil
	}
	var hints unsafe.Pointer
	if sndDeviceNameHint(-1, cstr("pcm"), &hints) < 0 || hints == nil {
		return nil
	}
	defer sndDeviceNameFreeHint(hints)
	return hintsOf(hints, func(hint unsafe.Pointer, id string) string {
		p := sndDeviceNameGetHint(hint, cstr(id))
		if p == nil {
			return ""
		}
		s := cString((*byte)(p))
		if free != nil {
			free(p)
		}
		return s
	})
}

// hintsOf returns the hints of an array of them that ends with a nil, as
// snd_device_name_hint gives it, each of whose values get tells.
func hintsOf(hints unsafe.Pointer, get func(hint unsafe.Pointer, id string) string) []alsaHint {
	var out []alsaHint
	for i := 0; ; i++ {
		h := *(*unsafe.Pointer)(unsafe.Add(hints, i*int(unsafe.Sizeof(hints))))
		if h == nil {
			return out
		}
		out = append(out, alsaHint{name: get(h, "NAME"), desc: get(h, "DESC"), ioid: get(h, "IOID")})
	}
}
