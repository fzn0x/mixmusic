package library

// What a backup's restore asks of the library (#592): to add what a
// backup's library holds to the one here, rather than replace it, and to
// give the tracks of music files that moved, as to another computer, the
// IDs of where they are now.

import (
	"cmp"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Added counts what a merge added to the library here.
type Added struct {
	Liked, Disliked, Mixes, Plays, Artists, Podcasts int
}

// MergeData returns the library here with what file, a backup's, adds to
// it, and counts what it added. Likes, dislikes, mixes and follows are
// joined; a mix both hold keeps the copy changed last. A track liked or
// disliked here stays so, whatever file says of it. The history is merged
// by time, the latest first, each play once, and kept to its last 1,000
// plays; the searches are here's, then file's, up to 20.
func MergeData(here, file Data) (Data, Added) {
	var a Added
	out := Data{
		Liked:     slices.Clone(here.Liked),
		Disliked:  slices.Clone(here.Disliked),
		Playlists: slices.Clone(here.Playlists),
		Searches:  slices.Clone(here.Searches),
		Artists:   slices.Clone(here.Artists),
		Podcasts:  slices.Clone(here.Podcasts),
	}
	rated := map[string]bool{}
	for _, t := range here.Liked {
		rated[t.ID] = true
	}
	for _, t := range here.Disliked {
		rated[t.ID] = true
	}
	for _, t := range file.Liked {
		if !rated[t.ID] {
			rated[t.ID] = true
			out.Liked = append(out.Liked, t)
			a.Liked++
		}
	}
	for _, t := range file.Disliked {
		if !rated[t.ID] {
			rated[t.ID] = true
			out.Disliked = append(out.Disliked, t)
			a.Disliked++
		}
	}
	at := map[string]int{}
	for i, p := range out.Playlists {
		at[p.ID] = i
	}
	for _, p := range file.Playlists {
		i, ok := at[p.ID]
		switch {
		case !ok:
			at[p.ID] = len(out.Playlists)
			out.Playlists = append(out.Playlists, p)
			a.Mixes++
		case p.Updated.After(out.Playlists[i].Updated):
			out.Playlists[i] = p
		}
	}
	// The mixes stay the last changed first.
	slices.SortStableFunc(out.Playlists, func(x, y Playlist) int { return y.Updated.Compare(x.Updated) })
	out.History = mergeHistory(here.History, file.History)
	for _, q := range file.Searches {
		if len(out.Searches) >= maxSearches {
			break
		}
		if !slices.Contains(out.Searches, q) {
			out.Searches = append(out.Searches, q)
		}
	}
	for _, ar := range file.Artists {
		if !slices.ContainsFunc(out.Artists, func(x FollowedArtist) bool { return x.ID == ar.ID }) {
			out.Artists = append(out.Artists, ar)
			a.Artists++
		}
	}
	for _, p := range file.Podcasts {
		if !slices.ContainsFunc(out.Podcasts, func(x media.Podcast) bool { return x.ID == p.ID }) {
			out.Podcasts = append(out.Podcasts, p)
			a.Podcasts++
		}
	}
	return out, a
}

// mergeHistory merges two histories, each the latest play first: by time,
// a play both hold once, the last maxHistory kept.
func mergeHistory(here, file []Play) []Play {
	out := make([]Play, 0, len(here)+len(file))
	seen := map[playKey]bool{}
	for _, p := range slices.Concat(here, file) {
		k := playKey{p.Track.ID, p.PlayedAt.Unix()}
		if !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(x, y Play) int { return y.PlayedAt.Compare(x.PlayedAt) })
	if len(out) > maxHistory {
		out = out[:maxHistory]
	}
	return out
}

// playKey tells a play from another: its track, and when it played, to
// the second, as the log keeps it.
type playKey struct {
	id string
	at int64
}

// MergePlays writes to out the play logs kept at here and at file, a
// backup's, merged: every play of both, the oldest first, a play both hold
// once. It returns how many plays file added. A log that isn't there holds
// no play.
func MergePlays(here, file, out string) (added int, err error) {
	plays, tracks, added, err := mergeLogs(here, file)
	if err != nil {
		return 0, err
	}
	g := &playLog{path: out}
	return added, g.writeWhole(plays, tracks)
}

// PlaysAdded counts the plays of the log kept at file, a backup's, that
// the one kept at here doesn't hold, which MergePlays would add.
func PlaysAdded(here, file string) (int, error) {
	_, _, added, err := mergeLogs(here, file)
	return added, err
}

// CountPlays counts the plays of the log kept at path: 0 when it isn't
// there.
func CountPlays(path string) (int, error) {
	g, _, err := openPlayLog(path)
	if err != nil {
		return 0, err
	}
	return len(g.plays), nil
}

// mergeLogs reads the logs kept at here and at file, and returns their
// plays merged, of tracks, and how many file added.
func mergeLogs(here, file string) (plays []loggedPlay, tracks []loggedTrack, added int, err error) {
	a, _, err := openPlayLog(here)
	if err != nil {
		return nil, nil, 0, err
	}
	b, _, err := openPlayLog(file)
	if err != nil {
		return nil, nil, 0, err
	}
	seen := map[playKey]bool{}
	for _, p := range a.plays {
		seen[playKey{a.tracks[p.track].ID, p.at}] = true
	}
	// b's tracks join a's, each once, found by what its lines hold.
	index := make([]int32, len(b.tracks))
	for i, t := range b.tracks {
		index[i] = a.intern(lineOf(time.Time{}, 0, t).trackLine)
	}
	plays = slices.Clone(a.plays)
	for _, p := range b.plays {
		k := playKey{b.tracks[p.track].ID, p.at}
		if seen[k] {
			continue
		}
		seen[k] = true
		plays = append(plays, loggedPlay{at: p.at, heardMs: p.heardMs, track: index[p.track]})
		added++
	}
	slices.SortStableFunc(plays, func(x, y loggedPlay) int { return cmp.Compare(x.at, y.at) })
	return plays, a.tracks, added, nil
}

// Move gives the tracks the library keeps that move tells a new ID of,
// as of music files now elsewhere, that ID, keeping the rest of each
// track: in the liked and disliked songs, the mixes, the history and the
// play log, as Retarget does.
func (l *Library) Move(move func(id string) (string, bool)) error {
	to := map[string]media.Track{}
	add := func(t media.Track) {
		if _, done := to[t.ID]; done {
			return
		}
		if id, ok := move(t.ID); ok && id != t.ID {
			n := t
			n.ID = id
			to[t.ID] = n
		}
	}
	d := l.file.Get()
	for _, t := range slices.Concat(d.Liked, d.Disliked) {
		add(t)
	}
	for _, p := range d.Playlists {
		for _, t := range p.Tracks {
			add(t)
		}
	}
	for _, p := range d.History {
		add(p.Track)
	}
	for _, t := range l.plays.view().tracks {
		add(t.Track)
	}
	if len(to) == 0 {
		return nil
	}
	return l.Retarget(to)
}
