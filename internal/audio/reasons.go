package audio

import "errors"

// The failures of a deck that the windows tell in their language (#800),
// as Reason names them. The errors keep their English, for the logs and
// the failure details' Copy.
const (
	// ReasonNetwork: the network failed while loading the audio.
	ReasonNetwork = "network"
	// ReasonDecode: the audio could not be decoded.
	ReasonDecode = "decode"
	// ReasonUnsupported: the audio's format is not one the engine plays.
	ReasonUnsupported = "unsupported"
	// ReasonFile: the file of a track on this computer can't be read.
	ReasonFile = "file"
)

// deckFailures are the failures of a deck, by their reasons.
var deckFailures = []struct {
	reason string
	err    error
}{{ReasonNetwork, errNetwork}, {ReasonDecode, errDecode}, {ReasonUnsupported, errUnsupported}, {ReasonFile, errFile}}

// Reason names the failure of a deck that err is, or wraps, and returns
// part, the words of err that tell it: the details err adds after them
// are the decoder's. It returns "", "" for another error.
func Reason(err error) (reason, part string) {
	for _, f := range deckFailures {
		if errors.Is(err, f.err) {
			return f.reason, f.err.Error()
		}
	}
	return "", ""
}
