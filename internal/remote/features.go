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
)

// features are those of this MixMusic.
var features = []string{FeatureQueue, FeatureHandOver}

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
