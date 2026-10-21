//go:build !((darwin || linux) && (amd64 || arm64))

package nowplaying

// Other systems aren't reached yet: on Windows, MixMusic's own audio engine
// has no place in the system's media controls.
func open(func(Request), func(func())) (platform, error) { return nil, ErrUnsupported }
