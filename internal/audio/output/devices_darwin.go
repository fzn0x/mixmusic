//go:build darwin && (amd64 || arm64)

package output

// The output devices of macOS: Core Audio's devices with output channels,
// by their names, as Sound settings shows them, and their UIDs, which an
// Audio Queue takes as its device.

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	// The properties of the devices: 'dev#', 'uid ', 'lnam', 'hidn' and
	// 'livn'.
	kAudioHardwarePropertyDevices     = 0x64657623
	kAudioDevicePropertyDeviceUID     = 0x75696420
	kAudioObjectPropertyName          = 0x6C6E616D
	kAudioDevicePropertyIsHidden      = 0x6869646E
	kAudioDevicePropertyDeviceIsAlive = 0x6C69766E
	// kAudioQueuePropertyCurrentDevice is the UID of the device a queue
	// plays on: 'aqcd'.
	kAudioQueuePropertyCurrentDevice = 0x61716364

	kCFStringEncodingUTF8 = 0x08000100
)

var (
	cfOnce sync.Once
	cfErr  error

	cfStringGetLength                 func(s uintptr) int
	cfStringGetMaximumSizeForEncoding func(n int, encoding uint32) int
	cfStringGetCString                func(s uintptr, buf *byte, size int, encoding uint32) bool
	cfStringCreateWithCString         func(alloc uintptr, s *byte, encoding uint32) uintptr
	cfRelease                         func(p uintptr)
)

// loadCoreFoundation loads CoreFoundation's strings, which Core Audio
// tells names and UIDs in.
func loadCoreFoundation() error {
	cfOnce.Do(func() {
		if err := loadAudioToolbox(); err != nil {
			cfErr = err
			return
		}
		if !coreAudio {
			cfErr = ErrUnsupported
			return
		}
		lib, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			cfErr = err
			return
		}
		purego.RegisterLibFunc(&cfStringGetLength, lib, "CFStringGetLength")
		purego.RegisterLibFunc(&cfStringGetMaximumSizeForEncoding, lib, "CFStringGetMaximumSizeForEncoding")
		purego.RegisterLibFunc(&cfStringGetCString, lib, "CFStringGetCString")
		purego.RegisterLibFunc(&cfStringCreateWithCString, lib, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&cfRelease, lib, "CFRelease")
	})
	return cfErr
}

// goString returns the string of a CFString.
func goString(s uintptr) string {
	if s == 0 {
		return ""
	}
	size := cfStringGetMaximumSizeForEncoding(cfStringGetLength(s), kCFStringEncodingUTF8) + 1
	if size <= 1 {
		return ""
	}
	buf := make([]byte, size)
	if !cfStringGetCString(s, &buf[0], size, kCFStringEncodingUTF8) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// cfString returns a CFString of s, which the caller releases.
func cfString(s string) uintptr { return cfStringCreateWithCString(0, cstr(s), kCFStringEncodingUTF8) }

// Devices returns Core Audio's output devices, sorted by name, the default
// one marked.
func Devices() ([]Device, error) {
	if err := loadCoreFoundation(); err != nil {
		return nil, err
	}
	return coreAudioDevices(audioDevices(), defaultOutputDevice()), nil
}

// deviceIDs returns the objects of Core Audio's devices.
func deviceIDs() []uint32 {
	addr := audioObjectPropertyAddress{kAudioHardwarePropertyDevices, kAudioObjectPropertyScopeGlobal, 0}
	var size uint32
	if audioObjectGetPropertyDataSize(kAudioObjectSystemObject, &addr, 0, nil, &size) != 0 || size < 4 {
		return nil
	}
	ids := make([]uint32, size/4)
	if audioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, nil, &size, unsafe.Pointer(&ids[0])) != 0 {
		return nil
	}
	return ids[:min(int(size/4), len(ids))]
}

// audioDevices returns what Core Audio tells of each of its devices.
func audioDevices() []coreAudioDevice {
	ids := deviceIDs()
	devices := make([]coreAudioDevice, 0, len(ids))
	for _, id := range ids {
		devices = append(devices, coreAudioDevice{
			id:      id,
			uid:     deviceString(id, kAudioDevicePropertyDeviceUID),
			name:    deviceString(id, kAudioObjectPropertyName),
			outputs: deviceChannels(id),
			hidden:  deviceFlag(id, kAudioDevicePropertyIsHidden, false),
		})
	}
	return devices
}

// defaultOutputDevice returns the object of the default output device, 0
// for none.
func defaultOutputDevice() uint32 {
	addr := audioObjectPropertyAddress{kAudioHardwarePropertyDefaultOutputDevice, kAudioObjectPropertyScopeGlobal, 0}
	var dev uint32
	size := uint32(unsafe.Sizeof(dev))
	if audioObjectGetPropertyData(kAudioObjectSystemObject, &addr, 0, nil, &size, unsafe.Pointer(&dev)) != 0 {
		return 0
	}
	return dev
}

// deviceString returns a property of a device that is a CFString, "" when
// it tells none.
func deviceString(dev, selector uint32) string {
	addr := audioObjectPropertyAddress{selector, kAudioObjectPropertyScopeGlobal, 0}
	var s uintptr
	size := uint32(unsafe.Sizeof(s))
	if audioObjectGetPropertyData(dev, &addr, 0, nil, &size, unsafe.Pointer(&s)) != 0 || s == 0 {
		return ""
	}
	defer cfRelease(s)
	return goString(s)
}

// deviceFlag returns a property of a device that is a UInt32 of 0 or 1, or
// otherwise when it tells none.
func deviceFlag(dev, selector uint32, otherwise bool) bool {
	addr := audioObjectPropertyAddress{selector, kAudioObjectPropertyScopeGlobal, 0}
	var v uint32
	size := uint32(unsafe.Sizeof(v))
	if audioObjectGetPropertyData(dev, &addr, 0, nil, &size, unsafe.Pointer(&v)) != 0 {
		return otherwise
	}
	return v != 0
}

// deviceChannels returns the output channels of a device, 0 when it has
// none or Core Audio doesn't tell.
func deviceChannels(dev uint32) int {
	// An AudioBufferList: the number of buffers, then each AudioBuffer of
	// 16 bytes, after 4 bytes of padding, whose first field is its
	// channels.
	addr := audioObjectPropertyAddress{kAudioDevicePropertyStreamConfiguration, kAudioDevicePropertyScopeOutput, 0}
	var size uint32
	if audioObjectGetPropertyDataSize(dev, &addr, 0, nil, &size) != 0 || size < 8 || size > 1<<16 {
		return 0
	}
	buf := make([]byte, size)
	if audioObjectGetPropertyData(dev, &addr, 0, nil, &size, unsafe.Pointer(&buf[0])) != 0 {
		return 0
	}
	return bufferListChannels(buf[:min(int(size), len(buf))])
}

// outputDevice returns the object of the output device of the UID uid and
// its output channels, while it is there and alive.
func outputDevice(uid string) (id uint32, channels int, ok bool) {
	if loadCoreFoundation() != nil {
		return 0, 0, false
	}
	for _, id := range deviceIDs() {
		if deviceString(id, kAudioDevicePropertyDeviceUID) != uid {
			continue
		}
		channels := deviceChannels(id)
		if channels > 0 && deviceFlag(id, kAudioDevicePropertyDeviceIsAlive, true) {
			return id, channels, true
		}
	}
	return 0, 0, false
}
