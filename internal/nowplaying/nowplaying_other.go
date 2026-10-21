//go:build !((darwin || linux || windows) && (amd64 || arm64))

package nowplaying

// Other systems aren't reached yet, nor 32-bit ones: there, MixMusic's own
// audio engine has no place in the system's media controls.
func open(func(Request), func(func())) (platform, error) { return nil, ErrUnsupported }
