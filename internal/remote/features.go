package remote

import "slices"

// What a computer can do beyond version 1 of the protocol, which it tells
// in its hello and its welcome. An older MixMusic, which doesn't know the
// field, tells none, and ignores the others': it runs no command it
// doesn't know, and skips a message of a type it doesn't know. So the
// version changes only when the two can no longer understand each other,
// and what is added since goes by these names.
const (
	// FeatureQueue: the computer controlled takes the commands on its
	// queue (OpPlayEntry, OpRemove, OpMove, OpClear), and its states name
	// the entries of their tracks.
	FeatureQueue = "queue"
	// FeatureHandOver: the computer controlled takes what the other hands
	// over to it (handover).
	FeatureHandOver = "handover"
	// FeatureBring: the computer controlled hands what plays there over to
	// the one that controls it, as that one asks (Bring).
	FeatureBring = "bring"
	// FeatureModes: the computer controlled takes OpShuffle, OpRepeat,
	// OpMute and OpLike, and its states tell its shuffle and repeat,
	// whether the track that plays is liked, and up to MaxNext tracks
	// after it, where an older MixMusic tells 20.
	FeatureModes = "modes"
	// FeatureOutputs: the states of the computer controlled tell where it
	// can play, its sound outputs and the cast devices it found, and it
	// takes OpOutput.
	FeatureOutputs = "outputs"
)

// features are those of this MixMusic.
var features = []string{FeatureQueue, FeatureHandOver, FeatureBring, FeatureModes, FeatureOutputs}

// FeatureKeep: the computer that pairs, which tells it in its hello to
// pair, waits for paired as long as the one controlled may take to keep
// the pairing (Keep), and a silence more. An older one waits a silence.
const FeatureKeep = "keep"

// pairFeatures are what this MixMusic tells as it pairs.
var pairFeatures = []string{FeatureKeep}

// cleanFeatures are the names of the features a computer told, at most 32
// of them, each of at most 32 bytes: those this MixMusic doesn't know are
// no use, but harmless.
func cleanFeatures(fs []string) []string {
	var out []string
	for _, f := range fs {
		if len(out) == 32 {
			break
		}
		if f != "" && len(f) <= 32 && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// Can reports whether the computer controlled does the feature f, as its
// welcome told.
func (s *Session) Can(f string) bool { return slices.Contains(s.features, f) }
