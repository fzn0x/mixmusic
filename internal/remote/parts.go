package remote

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// Part is a part of the track that plays which the player of the computer
// controlled passes over, as SponsorBlock marks it (#735), of a kind of
// sponsorblock.Kinds, from FromMs to ToMs; or the track's highlight, of
// the kind sponsorblock.Highlight, at FromMs. A state tells them so that
// the page of the computer that controls it marks them on its seek bar. An
// older MixMusic sends none, and ignores those it gets.
type Part struct {
	Kind   sponsorblock.Kind `json:"kind"`
	FromMs int64             `json:"fromMs"`
	ToMs   int64             `json:"toMs,omitempty"`
}

// MaxParts bounds the parts a state tells, its highlight among them.
const MaxParts = 64

// cleanParts are parts as a state of a track of durationMs, 0 when not
// known, tells them: of the kinds this MixMusic knows, within the track,
// MaxParts of them at most, and one highlight at most, with no length.
// What a computer sends is shown, and bounds no more than the track.
func cleanParts(parts []Part, durationMs int64) []Part {
	var out []Part
	highlight := false
	for _, p := range parts[:min(len(parts), MaxParts)] {
		p.FromMs = max(p.FromMs, 0)
		if durationMs > 0 {
			p.ToMs = min(p.ToMs, durationMs)
		}
		switch {
		case p.Kind == sponsorblock.Highlight:
			if highlight || p.FromMs == 0 || durationMs > 0 && p.FromMs >= durationMs {
				continue
			}
			highlight, p.ToMs = true, 0
		case !slices.Contains(sponsorblock.Kinds, p.Kind) || p.ToMs <= p.FromMs:
			continue
		}
		out = append(out, p)
	}
	return out
}
