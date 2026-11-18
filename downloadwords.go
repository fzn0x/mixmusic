package main

import (
	"errors"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/locales"
)

// The downloads tell why they wait, why a track failed and why a move
// didn't start in English, for the logs; the windows say each in their
// language here (#800). A failure of no reason known is the error's own
// words, after the message that leads it.

// downloadHold is why the downloads of st wait, or "" when they don't.
func downloadHold(st downloads.Status) string {
	arg := func(i int) string {
		if i < len(st.HoldArgs) {
			return st.HoldArgs[i]
		}
		return ""
	}
	switch st.HoldKind {
	case downloads.HoldPaused:
		return locales.DownloadsHoldPaused()
	case downloads.HoldOffline:
		return locales.DownloadsHoldOffline()
	case downloads.HoldMetered:
		return locales.DownloadsHoldMetered()
	case downloads.HoldThrottled:
		return locales.DownloadsHoldThrottled()
	case downloads.HoldDisk:
		return locales.DownloadsHoldDisk()
	case downloads.HoldFolder:
		if len(st.HoldArgs) > 1 {
			return locales.DownloadsHoldUnreadable(arg(0), arg(1))
		}
		return locales.DownloadsHoldUnreachable(arg(0))
	case downloads.HoldFailing:
		return locales.DownloadsHoldFailing(downloadReason(arg(0), arg(1)))
	case downloads.HoldMoving:
		return locales.DownloadsHoldMoving(arg(0))
	}
	return st.Hold
}

// downloadReason is why a download failed: its reason, of the
// downloads.Reason constants, or, of another, why, the error's words.
func downloadReason(reason, why string) string {
	switch reason {
	case downloads.ReasonNeedsYTDLP:
		return locales.DownloadsReasonNeedsYTDLP()
	case downloads.ReasonNoAudio:
		return locales.DownloadsReasonNoAudio()
	case downloads.ReasonNoMatch:
		return locales.DownloadsReasonNoMatch()
	case downloads.ReasonBadAudio:
		return locales.DownloadsReasonBadAudio()
	case downloads.ReasonNotYouTube:
		return locales.DownloadsReasonNotYouTube()
	case downloads.ReasonNoMatcher:
		return locales.DownloadsReasonNoMatcher()
	case downloads.ReasonNetwork:
		return locales.DownloadsReasonNetwork()
	case downloads.ReasonDiskFull:
		return locales.DownloadsReasonDiskFull()
	case downloads.ReasonExplicit:
		return locales.DownloadsReasonExplicit()
	}
	return why
}

// downloadPhase is what a download under way does, of its phase.
func downloadPhase(phase string) string {
	if phase == downloads.PhaseFinding {
		return locales.DownloadsPhaseFinding()
	}
	return phase
}

// downloadMoveError is why the downloads didn't move, for err.
func downloadMoveError(err error) string {
	var write *downloads.WriteError
	switch {
	case errors.Is(err, downloads.ErrNoDownloads):
		return locales.DownloadsMoveNoDownloads()
	case errors.Is(err, downloads.ErrMoving):
		return locales.DownloadsMoveMovingAlready()
	case errors.Is(err, downloads.ErrSameFolder):
		return locales.DownloadsMoveSameFolder()
	case errors.Is(err, downloads.ErrIntoTheirs):
		return locales.DownloadsMoveIntoTheirs()
	case errors.Is(err, downloads.ErrCacheFolder):
		return locales.DownloadsMoveCacheFolder()
	case errors.As(err, &write):
		return locales.DownloadsMoveCantWrite(write.Folder, errorText(write.Err))
	}
	return errorText(err)
}
