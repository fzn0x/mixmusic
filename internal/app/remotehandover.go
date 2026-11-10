package app

// Handing what plays over to another computer (remote control, #353):
// this computer sends its queue, the track that plays and where it is to
// the computer whose page shows, which plays them in place of its own,
// and pauses once that one took them.
//
// A track of YouTube Music or Spotify goes with its ID, and the other
// plays it as it plays its own: a Spotify track as its settings play
// those, from YouTube Music when it has no Spotify. A file of this
// computer goes without its path, which names nothing there, by its tags:
// the other plays its own file of the same song, and leaves the track out
// when it has none. A file is never looked for on a service, as on this
// computer.

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// handOverFor bounds a hand-over, from sending it to the other computer's
// answer.
var handOverFor = 30 * time.Second

// HandedOver is how a hand-over went: how many tracks the other computer
// plays of it, and how many it left out, as files of this computer it
// doesn't have. CurrentLeft is set when it left out the track that played
// here, and plays from the next one instead.
type HandedOver struct {
	Kept, Left  int
	CurrentLeft bool
}

// errNothingPlays: there is nothing to hand over.
var errNothingPlays = errors.New("Nothing plays on this computer")

// session is the session with the computer id, whose page shows.
func (rc *Remote) session(id string) (*remote.Session, error) {
	r := &rc.a.remote
	r.mu.Lock()
	ctl := r.control
	r.mu.Unlock()
	if ctl == nil || ctl.id != id {
		return nil, errors.New("Not connected to that computer")
	}
	ctl.mu.Lock()
	s := ctl.session
	ctl.mu.Unlock()
	if s == nil {
		return nil, errors.New("Not connected to that computer")
	}
	return s, nil
}

// HandOver hands what plays on this computer over to the computer id,
// whose page shows: its queue, the track that plays and where it is, in
// the interface's words when it fails. Once that computer took it, this
// one pauses, unless it took nothing.
func (rc *Remote) HandOver(id string) (HandedOver, error) {
	a := rc.a
	s, err := rc.session(id)
	if err != nil {
		return HandedOver{}, err
	}
	p := remoteHost{a}.player()
	if p == nil {
		return HandedOver{}, errNothingPlays
	}
	h, ok := handOverOf(p.State())
	if !ok {
		return HandedOver{}, errNothingPlays
	}
	ctx, cancel := context.WithTimeout(context.Background(), handOverFor)
	defer cancel()
	taken, err := s.HandOver(ctx, h)
	if err != nil {
		log.Printf("mixmusic: remote control: handing over to %s: %v", s.Name(), err)
		if errors.Is(err, context.DeadlineExceeded) {
			return HandedOver{}, errors.New(s.Name() + " didn't answer in time: what plays here plays on")
		}
		return HandedOver{}, errors.New(strings.TrimSuffix(remoteProblem(s.Name(), err), "."))
	}
	if taken.Kept > 0 {
		p.Pause()
	}
	log.Printf("mixmusic: remote control: handed %d tracks over to %s, which left out %d", taken.Kept, s.Name(), taken.Left)
	return HandedOver{Kept: taken.Kept, Left: taken.Left, CurrentLeft: taken.CurrentLeft}, nil
}

// handOverOf is what the player's st hands over: its queue around the
// track that plays, which the protocol cuts to its bounds, where that
// track is, whether it plays, and how the queue goes on. It is false when
// no track is current.
func handOverOf(st player.State) (remote.HandOver, bool) {
	if _, ok := st.Current(); !ok {
		return remote.HandOver{}, false
	}
	from := max(st.Index-remote.MaxHandOver/10, 0)
	to := min(from+remote.MaxHandOver, len(st.Queue))
	h := remote.HandOver{Index: st.Index - from, Playing: st.Playing || st.Loading || st.Waiting && st.WillPlay, Shuffle: st.Shuffle}
	if !st.Live {
		h.PositionMs = st.Position.Milliseconds()
	}
	switch st.Repeat {
	case player.RepeatAll:
		h.Repeat = "all"
	case player.RepeatOne:
		h.Repeat = "one"
	}
	for _, it := range st.Queue[from:to] {
		h.Queue = append(h.Queue, queueTrackOf(it.Track))
	}
	return h, true
}

// queueTrackOf is t as a hand-over tells it: a file of this computer by
// its tags alone, without its path, the IDs of its artist and album, which
// name what the index groups, or its cover, kept on this computer.
func queueTrackOf(t media.Track) remote.QueueTrack {
	file := t.Source == media.SourceFile
	q := remote.QueueTrack{Source: string(t.Source), Title: t.Title, DurationMs: int64(t.DurationMs), Explicit: t.Explicit,
		Video: t.Video, VideoType: t.VideoType}
	id := func(s string) string {
		if file {
			return ""
		}
		return s
	}
	q.ID = id(t.ID)
	for _, ar := range t.Artists {
		q.Artists = append(q.Artists, remote.Ref{ID: id(ar.ID), Name: ar.Name})
	}
	if t.Album != nil {
		q.Album = &remote.Ref{ID: id(t.Album.ID), Name: t.Album.Name}
	}
	for _, img := range t.Artwork {
		if strings.HasPrefix(img.URL, "https://") {
			q.Artwork = append(q.Artwork, remote.Image{URL: img.URL, Width: img.Width, Height: img.Height})
		}
	}
	if t.Episode != nil {
		q.Show = &remote.Ref{ID: id(t.Episode.Show.ID), Name: t.Episode.Show.Name}
	}
	return q
}

// TakeOver plays what the computer of fp hands over, in place of what
// plays: its tracks of a service as its own, its files as the same songs
// of this computer's files, where it has them, and the rest left out.
// What played there plays here from where it was; when it was left out,
// the next track kept plays from its start.
func (h remoteHost) TakeOver(fp remote.Fingerprint, ho remote.HandOver) remote.Taken {
	a := h.a
	taken := remote.Taken{Left: len(ho.Queue)}
	p := h.player()
	if p == nil {
		return taken
	}
	var mine *localSongs
	find := func(q remote.QueueTrack) (media.Track, bool) {
		if mine == nil {
			mine = a.localSongs()
		}
		return mine.find(q)
	}
	var tracks []media.Track
	at, current := -1, false
	for i, q := range ho.Queue {
		t, ok := trackOfHanded(q, find)
		switch {
		case !ok:
			taken.CurrentLeft = taken.CurrentLeft || i == ho.Index
			continue
		case i == ho.Index:
			at, current = len(tracks), true
		case i > ho.Index && at < 0:
			at = len(tracks)
		}
		tracks = append(tracks, t)
	}
	if len(tracks) == 0 {
		return taken
	}
	if at < 0 {
		// Only tracks before the one that played were kept.
		at = len(tracks) - 1
	}
	pos := time.Duration(0)
	if current {
		pos = time.Duration(ho.PositionMs) * time.Millisecond
	}
	repeat := player.RepeatOff
	switch ho.Repeat {
	case "all":
		repeat = player.RepeatAll
	case "one":
		repeat = player.RepeatOne
	}
	p.TakeOver(tracks, at, pos, ho.Playing, ho.Shuffle, repeat)
	taken.Kept, taken.Left = len(tracks), len(ho.Queue)-len(tracks)
	name := a.pairedName(fp)
	log.Printf("mixmusic: remote control: %s handed %d tracks over, of which %d were left out", name, len(ho.Queue), taken.Left)
	a.notifyRemote(name + " moved what it played to this computer")
	return taken
}

// pairedName is the name of the computer paired of the key fp.
func (a *App) pairedName(fp remote.Fingerprint) string {
	for _, c := range a.remoteKept().Computers {
		if c.Key == fp.String() {
			return c.Name
		}
	}
	return "Another computer"
}

// spotifyIDRe matches the ID of a Spotify track or episode, and refIDRe
// that of an artist, an album or a podcast of a service.
var (
	spotifyIDRe = regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`)
	refIDRe     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// trackOfHanded is the track of q, handed over, as this computer plays it:
// of a service, by its ID, when it is one of that service's; a file of the
// other computer, as find finds it among this one's. It is false for the
// rest, which can't play here.
func trackOfHanded(q remote.QueueTrack, find func(remote.QueueTrack) (media.Track, bool)) (media.Track, bool) {
	src := media.Source(q.Source)
	switch src {
	case media.SourceFile:
		return find(q)
	case media.SourceYouTube, media.SourceSpotify:
	default:
		return media.Track{}, false
	}
	s, native, err := media.ParseID(q.ID)
	if err != nil || s != src || src == media.SourceYouTube && !stream.ValidVideoID(native) ||
		src == media.SourceSpotify && !spotifyIDRe.MatchString(native) {
		return media.Track{}, false
	}
	t := media.Track{ID: q.ID, Source: src, Title: q.Title, DurationMs: int(min(q.DurationMs, 24*60*60*1000)),
		Explicit: q.Explicit, Video: q.Video, VideoType: q.VideoType}
	ref := func(r remote.Ref) media.Ref {
		out := media.Ref{Name: r.Name}
		if s, native, err := media.ParseID(r.ID); err == nil && s == src && refIDRe.MatchString(native) {
			out.ID = r.ID
		}
		return out
	}
	for _, ar := range q.Artists {
		t.Artists = append(t.Artists, ref(ar))
	}
	if q.Album != nil {
		al := ref(*q.Album)
		t.Album = &al
	}
	for _, img := range q.Artwork {
		t.Artwork = append(t.Artwork, media.Image{URL: img.URL, Width: img.Width, Height: img.Height})
	}
	if q.Show != nil {
		t.Episode = &media.Episode{Show: ref(*q.Show)}
	}
	return t, true
}

// localSongs are the files of this computer by their titles, folded, to
// find the songs of another computer's files among them.
type localSongs struct{ byTitle map[string][]media.Track }

// localSongs indexes the files of this computer.
func (a *App) localSongs() *localSongs {
	m := &localSongs{byTitle: map[string][]media.Track{}}
	if a.localFiles == nil {
		return m
	}
	for _, t := range a.localFiles.Tracks() {
		k := foldTag(t.Title)
		m.byTitle[k] = append(m.byTitle[k], t)
	}
	return m
}

// find finds the file of the song of q, another computer's file: of the
// same title, by an artist of the same name, as their tags tell, of the
// same length give or take 3 seconds, where both know it; of the same
// album first.
func (m *localSongs) find(q remote.QueueTrack) (media.Track, bool) {
	var found []media.Track
	for _, t := range m.byTitle[foldTag(q.Title)] {
		d := q.DurationMs - int64(t.DurationMs)
		if sameArtist(q.Artists, t.Artists) && (q.DurationMs == 0 || t.DurationMs == 0 || d >= -3000 && d <= 3000) {
			found = append(found, t)
		}
	}
	if len(found) == 0 {
		return media.Track{}, false
	}
	for _, t := range found {
		if q.Album != nil && t.Album != nil && foldTag(q.Album.Name) == foldTag(t.Album.Name) {
			return t, true
		}
	}
	return found[0], true
}

// sameArtist reports whether one of theirs is one of mine, by name.
func sameArtist(theirs []remote.Ref, mine []media.Ref) bool {
	for _, x := range theirs {
		for _, y := range mine {
			if foldTag(x.Name) == foldTag(y.Name) {
				return true
			}
		}
	}
	return false
}

// foldTag is s as tags compare: in lower case, with its spaces as one.
func foldTag(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
