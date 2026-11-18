package main

import (
	"github.com/fzn0x/mixmusic/internal/app"
)

// errorText is err as the windows show it, in their language where
// MixMusic knows the error: app.ErrorText.
func errorText(err error) string { return app.ErrorText(err) }

// colorFileFault is why the colour file of st couldn't be read, as the
// windows show it.
func colorFileFault(st app.ColorFileStatus) string {
	if st.Fault != nil {
		return errorText(st.Fault)
	}
	return st.Err
}
