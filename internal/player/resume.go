package player

import (
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Resumer is what the player asks of the app of where a podcast's episode
// starts, when its Services have it too: without, every track starts from
// its start.
type Resumer interface {
	// ResumeAt is where track plays from as it becomes current: where the
	// user left it, for an episode, and 0 for a track that starts from its
	// start.
	ResumeAt(track media.Track) time.Duration
}

// resumeAt is where track plays from as it becomes current.
func (p *Player) resumeAt(track media.Track) time.Duration {
	if r, ok := p.svc.(Resumer); ok && track.IsEpisode() {
		return max(r.ResumeAt(track), 0)
	}
	return 0
}

// spoken reports whether a track is a podcast's episode, which neither
// crossfades nor has its audio fetched ahead: an episode of hours is
// hundreds of megabytes.
func spoken(t media.Track) bool { return t.IsEpisode() }
