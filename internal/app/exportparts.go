package app

// Leaving the parts SponsorBlock marks out of exports (#735): while
// Settings › Playback › Skip parts of videos skips them, and "Leave marked
// parts out of exports" is on, the copy of a download of Opus leaves out
// the parts the player passes over, as the engine does (audio.Spans), cut
// in the remux (tagwrite.Remux). M4A copies keep them, as their sample
// tables aren't written anew. The parts are those the player asks for, of
// the parts cache (sponsorblock.go), and asked for only of the copies of
// Opus. A copy whose parts can't be found goes as it is; once two lookups
// in a row failed, as without the network, the export asks no more.

import (
	"context"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// partLookups are the lookups of the parts of the videos an export
// copies: how many failed in a row.
type partLookups struct{ failed int }

// get returns the parts of video, asking server when the cache doesn't
// know them, unless two lookups in a row failed, and false when they
// can't be found. Offline, it doesn't ask: the parts kept count however
// old.
func (l *partLookups) get(ctx context.Context, a *App, server, video string) ([]sponsorblock.Segment, bool) {
	if a.offline() || l.failed >= 2 {
		return a.parts.Known(video, true)
	}
	parts, err := a.parts.Get(ctx, server, video)
	if err != nil {
		l.failed++
		return nil, false
	}
	l.failed = 0
	return parts, true
}

// exportLeave returns what the copy of t's download leaves out, as
// export.Track.Leave asks, or nil while the settings leave nothing out,
// and for a track that isn't a recording on YouTube.
func (a *App) exportLeave(ctx context.Context, t media.Track, lookups *partLookups) func(time.Duration) []tagwrite.Cut {
	set := a.settings.Get().Clean().SkipParts
	video := a.exportVideo(t)
	if !set.On || !set.Exports || video == "" {
		return nil
	}
	return func(length time.Duration) []tagwrite.Cut {
		parts, ok := lookups.get(ctx, a, set.Server, video)
		if !ok {
			return nil
		}
		return leftOut(parts, set, length)
	}
}

// exportVideo is the video on YouTube whose audio t's download holds: ""
// for none.
func (a *App) exportVideo(t media.Track) string {
	if t.Source == media.SourceFile {
		return ""
	}
	if k, ok := a.downloads.Lookup(t.ID); ok && k.Path != "" {
		return k.Video
	}
	if source, video, err := media.ParseID(t.ID); err == nil && source == media.SourceYouTube {
		return video
	}
	return ""
}

// leftOut returns the stretches a copy of an audio of length leaves out
// of parts: those of the kinds set skips, as the engine passes over them,
// which fit the audio, and joined; one that ends the track, as the player
// ends it where the part starts, to the end. It leaves out none when they
// would leave less than a second.
func leftOut(parts []sponsorblock.Segment, set settings.SkipParts, length time.Duration) []tagwrite.Cut {
	var skips []audio.Skip
	for _, p := range parts {
		if p.Kind != sponsorblock.Highlight && set.Skips(p.Kind) {
			skips = append(skips, audio.Skip{From: p.From, To: p.To, Of: p.Of})
		}
	}
	var cuts []tagwrite.Cut
	left := length
	for _, s := range audio.Spans(skips, length) {
		if s.Ends(length) {
			s.To = length
		}
		cuts = append(cuts, tagwrite.Cut{From: s.From, To: s.To})
		left -= s.To - s.From
	}
	if length > 0 && left < time.Second {
		return nil
	}
	return cuts
}
