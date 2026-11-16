package app

import (
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// remotePartsOf are the parts of the track that plays which the player
// passes over, and its highlight, as a state tells them to the computer
// that controls this one (#735), whose page marks them on its seek bar:
// those the seek bars here mark.
func remotePartsOf(st player.State) []remote.Part {
	var out []remote.Part
	for _, p := range st.Parts {
		out = append(out, remote.Part{Kind: p.Kind, FromMs: p.From.Milliseconds(), ToMs: p.To.Milliseconds()})
	}
	if st.Highlight > 0 {
		out = append(out, remote.Part{Kind: sponsorblock.Highlight, FromMs: st.Highlight.Milliseconds()})
	}
	return out
}
