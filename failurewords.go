package main

import (
	"strings"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// The details of a failure as the window shows them, in its language
// (#800): Copy's text, for an issue, stays English (failurereport.go).
// The ways the player tried are the player's names of them, which it
// gives in English; why each failed it gives in English and as the window
// said it then.

// failureAgain ends the name of a way tried once more, as the player
// names it.
const failureAgain = ", tried again"

// failureWay is a way the player tried, as it names it, in the language;
// one it doesn't know is as it came.
func failureWay(what string) string {
	if base, ok := strings.CutSuffix(what, failureAgain); ok {
		return locales.FailureWayAgain(failureWay(base))
	}
	switch what {
	case "the file":
		return locales.FailureWayFile()
	case "finding it on YouTube Music":
		return locales.FailureWayFindingRecording()
	case "finding its stream":
		return locales.FailureWayFindingStream()
	case "Spotify in MixMusic":
		return locales.FailureWaySpotifyHere()
	case "the live stream":
		return locales.FailureWayLive()
	case "the download, or the offline cache":
		return locales.FailureWayOffline()
	case "the stream":
		return locales.FailureWayStream()
	case "its 5.1 surround audio":
		return locales.FailureWaySurround()
	case "YouTube's player API":
		return locales.FailureWayPlayerAPI()
	}
	return sentenceStart(what)
}

// failureStepSaid is a step of a failure as the window tells it: the way
// tried, how many times, and why it failed.
func failureStepSaid(st player.Step, pr privacy) string {
	if st.Times > 1 {
		return locales.FailureStepTimes(failureWay(st.What), st.Times, pr.clean(failureWhy(st)))
	}
	return locales.FailureStep(failureWay(st.What), pr.clean(failureWhy(st)))
}

// failureWhy is why a step of a failure failed, as the window said it, in
// its language.
func failureWhy(st player.Step) string {
	if st.Said != "" {
		return st.Said
	}
	return st.Why
}

// failureTrackSaid names the track of a failure as the window does: its
// title, its artists and its album.
func failureTrackSaid(t media.Track) string {
	by := names(t.Artists)
	on := ""
	if t.Album != nil && t.Album.Name != t.Title {
		on = t.Album.Name
	}
	switch {
	case by != "" && on != "":
		return locales.FailureTrackByOn(t.Title, by, on)
	case by != "":
		return locales.FailureTrackBy(t.Title, by)
	case on != "":
		return locales.FailureTrackOn(t.Title, on)
	}
	return t.Title
}

// failureFromSaid tells where a track of a failure comes from, as the
// window does: its service, and its ID there, or, for a track on this
// computer, its file's path.
func failureFromSaid(t media.Track, pr privacy) string {
	if t.Source == media.SourceFile {
		if path, ok := localfiles.PathOf(t.ID); ok {
			return locales.FailureFromFileAt(pr.inHome(path))
		}
		return locales.FailureFromFile()
	}
	return failureFrom(t, pr)
}

// failurePlayedAsSaid tells the recording r a track of a failure played
// as, and where it comes from, as the window does. A file on this computer
// has messages of its own: after "from", it takes a case of its own in
// some languages.
func failurePlayedAsSaid(r media.Track, pr privacy) string {
	track := pr.clean(failureTrackSaid(r))
	if r.Source == media.SourceFile {
		if path, ok := localfiles.PathOf(r.ID); ok {
			return locales.FailurePlayedAsFileAt(track, pr.inHome(path))
		}
		return locales.FailurePlayedAsFile(track)
	}
	return locales.FailurePlayedAs(track, failureFrom(r, pr))
}
