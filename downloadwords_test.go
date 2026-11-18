package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/downloads"
)

// TestDownloadWords says each hold, failure reason, phase and move error
// of the downloads in the language: en-XA brackets each message.
func TestDownloadWords(t *testing.T) {
	usePseudoLocale(t)
	said := map[string]string{}
	for _, st := range []downloads.Status{
		{HoldKind: downloads.HoldPaused}, {HoldKind: downloads.HoldOffline}, {HoldKind: downloads.HoldMetered},
		{HoldKind: downloads.HoldThrottled}, {HoldKind: downloads.HoldDisk},
		{HoldKind: downloads.HoldFolder, HoldArgs: []string{"/media/usb"}},
		{HoldKind: downloads.HoldFolder, HoldArgs: []string{"/media/usb", "permission denied"}},
		{HoldKind: downloads.HoldFailing, HoldArgs: []string{downloads.ReasonNetwork, "dial tcp: timeout"}},
		{HoldKind: downloads.HoldMoving, HoldArgs: []string{"/media/usb"}},
	} {
		said[st.HoldKind+strings.Join(st.HoldArgs, ",")] = downloadHold(st)
	}
	for _, r := range []string{downloads.ReasonNeedsYTDLP, downloads.ReasonNoAudio, downloads.ReasonNoMatch, downloads.ReasonBadAudio,
		downloads.ReasonNotYouTube, downloads.ReasonNoMatcher, downloads.ReasonNetwork, downloads.ReasonDiskFull, downloads.ReasonExplicit} {
		said[r] = downloadReason(r, "the English words")
	}
	said["phase"] = downloadPhase(downloads.PhaseFinding)
	for _, err := range []error{downloads.ErrNoDownloads, downloads.ErrMoving, downloads.ErrSameFolder, downloads.ErrIntoTheirs,
		downloads.ErrCacheFolder, &downloads.WriteError{Folder: "/media/usb", Err: errors.New("read-only file system")}} {
		said[err.Error()] = downloadMoveError(err)
	}
	for what, txt := range said {
		if !strings.HasPrefix(txt, "[") {
			t.Errorf("%s says %q, which no catalog translates", what, txt)
		}
	}
	// A failure of no reason known is its own words, which the message
	// around it leads.
	if got := downloadReason("", "YouTube answered HTTP 500"); got != "YouTube answered HTTP 500" {
		t.Errorf("another failure says %q", got)
	}
}
