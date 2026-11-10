package player

// Explicit songs, while the settings hide explicit content (#427): the
// queue passes over their entries as it moves on, as it does over those of
// the tracks the user disliked, but those the user chose too. None of them
// plays: choosing one tells why instead.
//
// The player keeps the invariant that every entry of a track the settings
// hide is marked SkipExplicit: as entries are made (skipOf), as the user
// chooses one (keptSkip), and as the settings change (hideChanged).

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// SkipExplicit is why the queue passes over an entry of an explicit song,
// while the settings hide explicit content. Unlike the other reasons, the
// user choosing the entry doesn't lift it.
const SkipExplicit = "explicit"

// hides reports whether the settings hide t: an explicit song or video,
// while they hide explicit content.
func (p *Player) hides(t media.Track) bool { return p.hideExplicit && t.ExplicitSong() }

// keptSkip is why the queue still passes over an entry of t that the user
// chose: SkipExplicit for a track the settings hide, else "".
func (p *Player) keptSkip(t media.Track) string {
	if p.hides(t) {
		return SkipExplicit
	}
	return ""
}

// refuses reports whether t, which the user chose to play, doesn't, as the
// settings hide it, and tells them why.
func (p *Player) refuses(t media.Track) bool {
	if !p.hides(t) {
		return false
	}
	p.notify(Notice{Message: t.Title + " is explicit, and your settings hide explicit content", Settings: true})
	return true
}

// explicitPassedOver tells why the queue would pass over every entry of
// items, when it passes over some for the settings hiding explicit
// content: "" when it passes over none for that.
func explicitPassedOver(items []Item) string {
	explicit := 0
	for _, it := range items {
		if it.Skip == SkipExplicit {
			explicit++
		}
	}
	switch {
	case explicit == 0:
		return ""
	case explicit == len(items):
		return "Every song here is explicit, and your settings hide explicit content"
	}
	return "Every song here is explicit, which your settings hide, or one they skip"
}

// hideChanged takes in the settings hiding explicit content, or no longer:
// the queue passes over the entries of explicit songs, those the user
// chose too, or no longer, and moves on from the current one when it's
// one of them, playing or paused as it was.
func (p *Player) hideChanged(s settings.Settings) {
	if s.HideExplicit == p.hideExplicit {
		return
	}
	p.hideExplicit = s.HideExplicit
	p.reflag()
	cur, ok := p.s.Current()
	if !ok || !p.hides(cur.Track) {
		return
	}
	q := slices.Clone(p.s.Queue)
	q[p.s.Index].Chosen, q[p.s.Index].Skip = false, SkipExplicit
	p.s.Queue = q
	p.emit(QueueChange)
	p.leave()
}
