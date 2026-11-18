package player

// Explicit songs, while the settings hide explicit content (#427): the
// queue passes over their entries as it moves on, as it does over those of
// the tracks the user disliked, but those the user chose too. None of them
// plays: choosing one tells why instead.
//
// The player keeps the invariant that every entry of a track the settings
// hide is marked SkipExplicit: as entries are made (skipOf), as the user
// chooses one (keptSkip), and as the settings change (hideChanged).
//
// A Spotify track that plays from its recording on YouTube Music is
// explicit when that recording is (#583), as the app found it before
// (Recordings), or as its stream is found: its entries are marked then
// (recordingFound), and a move that reaches one passes over it (goTo).
// Spotify's own audio goes by Spotify's mark.

import (
	"slices"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// SkipExplicit is why the queue passes over an entry of an explicit song,
// while the settings hide explicit content. Unlike the other reasons, the
// user choosing the entry doesn't lift it.
const SkipExplicit = "explicit"

// Recordings are what the player asks of the app of the recordings on
// YouTube Music that Spotify tracks play from, when its Services have them
// too: without, a Spotify track goes by Spotify's mark alone.
type Recordings interface {
	// KnownRecording is the recording on YouTube Music that track plays
	// from, as found before, without searching for one.
	KnownRecording(track media.Track) (media.Track, bool)
}

// hides reports whether the settings hide t: an explicit song or video,
// or a Spotify track that plays from an explicit recording, while they
// hide explicit content.
func (p *Player) hides(t media.Track) bool {
	return p.hideExplicit && (t.ExplicitSong() || p.explicitRecording(t))
}

// explicitRecording reports whether t is a Spotify track that plays from
// its recording on YouTube Music, as the settings play it, and the app
// found that recording explicit.
func (p *Player) explicitRecording(t media.Track) bool {
	r, ok := p.svc.(Recordings)
	if !ok || t.Source != media.SourceSpotify || !streams(t, p.spotifyMode, p.here) {
		return false
	}
	rec, ok := r.KnownRecording(t)
	return ok && rec.ExplicitSong()
}

// hidesFound reports whether src, found for t to play, is a recording the
// settings hide: an explicit one, of a Spotify track, while they hide
// explicit content. A Spotify track that Spotify can't play plays from it
// too.
func (p *Player) hidesFound(t media.Track, src Source) bool {
	return p.hideExplicit && t.Source == media.SourceSpotify && src.Mode == ModeStream && src.Track.ExplicitSong()
}

// markExplicit has the queue pass over the entries of the track id, whose
// recording was found explicit while the settings hide explicit content:
// those the user chose too.
func (p *Player) markExplicit(id string) {
	p.setSkips(id, func(Item) string { return SkipExplicit })
}

// recordingFound marks the entries of t, once the app may have found its
// recording, as a check of its label does, while the settings hide it.
func (p *Player) recordingFound(t media.Track) {
	if p.hides(t) {
		p.markExplicit(t.ID)
	}
}

// leaveFound leaves the current entry, whose stream was found from an
// explicit recording as it was to play: its entries are passed over from
// then on, a user who chose it is told why it doesn't play, and the queue
// moves on from it, as when the settings come to hide it.
func (p *Player) leaveFound(entry Item) {
	if entry.Chosen {
		p.notify(Notice{Message: locales.PlayerExplicitRefused(entry.Track.Title), Settings: true})
	}
	p.markExplicit(entry.Track.ID)
	p.leave()
}

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
	p.notify(Notice{Message: locales.PlayerExplicitRefused(t.Title), Settings: true})
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
		return locales.PlayerExplicitAll()
	}
	return locales.PlayerExplicitOrSkipped()
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
