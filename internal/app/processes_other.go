//go:build !windows && !(darwin && (amd64 || arm64))

package app

// processes returns none: only Windows and macOS hear what another process
// plays.
func processes() []proc { return nil }

// webViewAudio returns 0: only Windows and macOS hear MixMusic's web views.
func webViewAudio([]proc) uint32 { return 0 }
