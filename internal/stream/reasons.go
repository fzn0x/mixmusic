package stream

import "errors"

// The reasons a stream wasn't found or failed that the windows tell in
// their language (#800), as Reason names them. The errors keep their
// English, for the logs and the failure details' Copy.
const (
	// ReasonBot: YouTube asks to confirm this isn't a bot.
	ReasonBot = "bot"
	// ReasonThrottled: YouTube turns down requests from this network.
	ReasonThrottled = "throttled"
	// ReasonAdults: YouTube plays the track to signed-in adults only.
	ReasonAdults = "adults"
	// ReasonSignedIn: YouTube plays the track to signed-in viewers only.
	ReasonSignedIn = "signedIn"
	// ReasonCountry: the track isn't available in the user's country.
	ReasonCountry = "country"
	// ReasonPrivate: the track is private.
	ReasonPrivate = "private"
	// ReasonMembers: only the members of its channel can play it.
	ReasonMembers = "members"
	// ReasonPremium: only YouTube Music Premium members can play it.
	ReasonPremium = "premium"
	// ReasonNoFormat: YouTube gave no audio; a newer yt-dlp may.
	ReasonNoFormat = "noFormat"
	// ReasonGone: the track is no longer on YouTube Music.
	ReasonGone = "gone"
	// ReasonRefused: YouTube refused to send its audio.
	ReasonRefused = "refused"
	// ReasonStalled: YouTube stopped sending the audio.
	ReasonStalled = "stalled"
)

// plainReasons are the reasons of plainErrors, by their plain words.
var plainReasons = map[string]string{
	"YouTube asks to confirm this is not a bot; try again in a while":                             ReasonBot,
	"YouTube is turning down requests from this network for now; try again in a while":            ReasonThrottled,
	"YouTube plays this track only to signed-in adults":                                           ReasonAdults,
	"YouTube plays this track only to signed-in viewers":                                          ReasonSignedIn,
	"this track is not available in your country":                                                 ReasonCountry,
	"this track is private":                                                                       ReasonPrivate,
	"only the members of its channel can play this track":                                         ReasonMembers,
	"only YouTube Music Premium members can play this track":                                      ReasonPremium,
	"YouTube gave no audio for this track; updating yt-dlp in Settings › Playback often fixes it": ReasonNoFormat,
	"this track is no longer on YouTube Music":                                                    ReasonGone,
}

// Reason names why err, of finding or fetching a stream, failed, when the
// windows tell it in their language: one of the Reason constants, or ""
// for an error whose own words tell it. HTTP's errors aren't the stream's
// alone, so they have none.
func Reason(err error) string {
	var f failure
	switch {
	case errors.As(err, &f):
		return plainReasons[f.msg]
	case errors.Is(err, errExpired):
		return ReasonRefused
	case errors.Is(err, errStalled):
		return ReasonStalled
	}
	return ""
}
