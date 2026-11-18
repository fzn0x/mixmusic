package main

import (
	"errors"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// errorText is err as the windows show it, in their language where
// MixMusic knows the error: app.ErrorText.
func errorText(err error) string { return app.ErrorText(err) }

// settingsError reports whether the playback settings change err, which
// the toast of a track that couldn't play for it offers then: a track
// YouTube plays only to signed-in viewers, whose words tell what to
// change for the user's account to be asked (#577).
func settingsError(err error) bool { return errors.Is(err, stream.ErrNeedsAccount) }

// colorFileFault is why the colour file of st couldn't be read, as the
// windows show it.
func colorFileFault(st app.ColorFileStatus) string {
	if st.Fault != nil {
		return errorText(st.Fault)
	}
	return st.Err
}
