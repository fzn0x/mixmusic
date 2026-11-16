package settings

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// SkipParts is how MixMusic passes over the parts of videos SponsorBlock
// marks as they play, in Settings › Playback › Skip parts of videos
// (#548).
type SkipParts struct {
	// On skips the parts of the kinds Kinds ticks, and marks them on the
	// seek bar. It is off at first: it asks SponsorBlock of each video,
	// and nothing is asked while it's off.
	On bool `json:"on"`
	// Kinds are the kinds of parts skipped, by SponsorBlock's names, in
	// the order of sponsorblock.Kinds: Non-music parts, Sponsor and
	// Self-promotion at first.
	Kinds []sponsorblock.Kind `json:"kinds"`
	// Highlight marks the highlight SponsorBlock tells of a video, on the
	// seek bar, for Jump to the highlight.
	Highlight bool `json:"highlight"`
	// Server is the https address of a mirror of SponsorBlock to ask in
	// its place: "" for SponsorBlock's own.
	Server string `json:"server"`
}

// defaultSkipParts is how a new install skips parts: not at all, until the
// user turns it on.
func defaultSkipParts() SkipParts {
	return SkipParts{Kinds: slices.Clone(sponsorblock.DefaultKinds), Highlight: true}
}

// Skips reports whether the parts of kind k are skipped, while On.
func (p SkipParts) Skips(k sponsorblock.Kind) bool { return slices.Contains(p.Kinds, k) }

// clean is p with the kinds MixMusic knows, once each and in their order,
// and the server's address an https one. Settings that name no kinds, as
// those from before, take the first ones.
func (p SkipParts) clean() SkipParts {
	if p.Kinds == nil {
		p.Kinds = slices.Clone(sponsorblock.DefaultKinds)
	} else {
		kinds := make([]sponsorblock.Kind, 0, len(p.Kinds))
		for _, k := range sponsorblock.Kinds {
			if slices.Contains(p.Kinds, k) {
				kinds = append(kinds, k)
			}
		}
		p.Kinds = kinds
	}
	p.Server = sponsorblock.Server(p.Server)
	return p
}
