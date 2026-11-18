package player

// Keeping albums gapless (#601): with Crossfade on, the songs of one album
// play straight on, as they do with Crossfade off, while the settings keep
// albums gapless, as a live album's or a concept album's run into each
// other. The step out of an album still crossfades. The player asks again
// at each time the engine tells, until the fade would start, so that a
// late change of the queue counts.

import "github.com/fzn0x/mixmusic/internal/media"

// gaplessState is how the next entry plays, as it was prepared while the
// current one plays: a Spotify track's recording on YouTube Music.
type gaplessState struct {
	uid string
	src Source
}

// preparedNext keeps how the entry of uid plays, prepared as the next
// one.
func (p *Player) preparedNext(uid string, src Source) {
	p.albums = gaplessState{uid: uid, src: src}
}

// gapless reports whether next, which plays after entry, the current
// entry, follows it without a crossfade, as the settings keep albums
// gapless: both are of one album, whose ID tells its service, next comes
// right after entry in the queue's own order, and neither is the radio's.
// When both tell their place on the album, next is the following track of
// entry's disc, or the first of the next disc. A Spotify track that plays
// from its recording on YouTube Music follows only when both recordings
// are songs: a music video often starts or ends otherwise.
func (p *Player) gapless(entry, next Item) bool {
	if entry.Radio || next.Radio || !sameAlbum(entry.Track, next.Track) || !follows(entry.Track, next.Track) {
		return false
	}
	if !p.svc.Settings().GaplessAlbums || p.ownNext(entry.UID) != next.UID {
		return false
	}
	if entry.Track.Source != media.SourceSpotify {
		return true
	}
	return p.s.Source != nil && recordedSong(*p.s.Source) && p.albums.uid == next.UID && recordedSong(p.albums.src)
}

// sameAlbum reports whether a and b are of one album: albums of one title
// on two services, or of two artists, have IDs of their own.
func sameAlbum(a, b media.Track) bool {
	return a.Album != nil && b.Album != nil && a.Album.ID != "" && a.Album.ID == b.Album.ID
}

// follows reports whether b comes right after a on their album, when both
// tell their place: the next track of a's disc, or the first of the next
// disc. A track whose number isn't known may follow any. A disc not told
// is the first.
func follows(a, b media.Track) bool {
	if a.TrackNumber == 0 || b.TrackNumber == 0 {
		return true
	}
	da, db := max(a.DiscNumber, 1), max(b.DiscNumber, 1)
	return db == da && b.TrackNumber == a.TrackNumber+1 || db == da+1 && b.TrackNumber == 1
}

// ownNext is the UID of the entry after the one of uid in the queue's own
// order, or "": while the queue is shuffled, the order it had before,
// which turning Shuffle off brings back, of the entries still there.
func (p *Player) ownNext(uid string) string {
	if p.unshuffled == nil {
		i := p.indexOf(uid)
		if i < 0 || i+1 >= len(p.s.Queue) {
			return ""
		}
		return p.s.Queue[i+1].UID
	}
	i := indexIn(p.unshuffled, uid)
	if i < 0 {
		return ""
	}
	for _, it := range p.unshuffled[i+1:] {
		if p.indexOf(it.UID) >= 0 {
			return it.UID
		}
	}
	return ""
}

// recordedSong reports whether src plays a Spotify track from a song of
// YouTube Music, not a music video, nor what isn't known to be either.
func recordedSong(src Source) bool {
	return src.Mode == ModeStream && src.Track.Source == media.SourceYouTube && !src.Track.Video
}
