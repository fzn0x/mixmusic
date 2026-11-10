package localfiles

// Copies of one song (#340): a library often holds a song more than once,
// as an MP3 and a FLAC of it, or a single and its album's copy. Files of
// the same title and artists, whose lengths are a few seconds apart at
// most, are copies of one song, which the songs of this computer and its
// search show once, and which plays its best copy: lossless before lossy,
// then the higher bitrate, then the higher sample rate. The user links
// other files by hand, and takes a copy out of its song.
//
// A song with copies has an ID of its own, "file:song:<key>", which the
// liked songs, the mixes and the history keep, so that they follow the
// song, not one of its files: one copy removed, they stay. The links are
// kept in the data folder (local-copies.json), never in the files. group
// works out the songs from the files of the index and what was kept;
// songs.go keeps them, and tells the app which tracks became songs.

import (
	"cmp"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// copyTolerance is how far apart, in milliseconds, the lengths of two
// copies of a song may be: a single's copy and an album's often differ by
// a second or two of silence.
const copyTolerance = 3000

// Facts are what a copy's audio is, which tells its quality: its codec,
// as internal/audio names it ("FLAC", "PCM", "MP3", "AAC", "Opus",
// "Vorbis"), and its sample rate and bit depth, 0 when they weren't read.
type Facts struct {
	Codec      string `json:"codec,omitempty"`
	SampleRate int    `json:"sampleRate,omitempty"`
	BitDepth   int    `json:"bitDepth,omitempty"`
}

// Lossless reports whether the codec keeps all of the audio, as FLAC and
// the PCM of WAV do.
func (f Facts) Lossless() bool { return f.Codec == "FLAC" || f.Codec == "PCM" }

// guessFacts are the facts a file's extension tells, until its audio is
// read: its codec, as mostly held in that kind of file.
func guessFacts(path string) Facts {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".flac":
		return Facts{Codec: "FLAC"}
	case ".wav", ".wave":
		return Facts{Codec: "PCM"}
	case ".mp3":
		return Facts{Codec: "MP3"}
	case ".m4a", ".mp4":
		return Facts{Codec: "AAC"}
	case ".opus":
		return Facts{Codec: "Opus"}
	case ".ogg", ".oga":
		return Facts{Codec: "Vorbis"}
	}
	return Facts{}
}

// Copy is a file of a song with copies.
type Copy struct {
	// Track is the file's own track, which plays this copy alone.
	Track media.Track
	Path  string
	Facts
	// Bitrate is its average, in kbit/s, of its size and length.
	Bitrate int
	// Best is set for the copy the song plays.
	Best bool
	// Linked is set for a copy the user linked to the song by hand, which
	// stays whatever its tags say.
	Linked bool
}

// bitrate is the average bitrate of a file, in kbit/s: its bits over its
// milliseconds.
func bitrate(e Entry) int {
	if e.DurationMs <= 0 {
		return 0
	}
	return int(e.Size * 8 / int64(e.DurationMs))
}

// compareCopies orders copies best first: lossless before lossy, then the
// higher bitrate, then the higher sample rate, then by their paths, so
// that the order never changes by itself.
func compareCopies(a, b Copy) int {
	lossless := func(c Copy) int {
		if c.Lossless() {
			return 0
		}
		return 1
	}
	return cmp.Or(cmp.Compare(lossless(a), lossless(b)), cmp.Compare(b.Bitrate, a.Bitrate),
		cmp.Compare(b.SampleRate, a.SampleRate), strings.Compare(a.Path, b.Path))
}

// matchKey is what copies of a song share: their titles and artists, as
// a reader reads them, the artists in any order. A file without a title
// or an artist of its own in its tags, as a voice memo, or without a
// length, has none: only the user links it.
func matchKey(e Entry) string {
	t := squash(fold(e.Title))
	if t == "" || e.DurationMs <= 0 {
		return ""
	}
	var names []string
	for _, a := range e.Artists {
		if n := squash(fold(a)); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return ""
	}
	slices.Sort(names)
	return t + "\x00" + strings.Join(slices.Compact(names), "\x00")
}

// squash takes the spaces at the ends of s away, and makes those between
// its words one.
func squash(s string) string { return strings.Join(strings.Fields(s), " ") }

// savedLinks is what local-copies.json holds: the songs with copies, the
// files the user took out of a song, and the facts of the copies' audio.
type savedLinks struct {
	Version int         `json:"version"`
	Songs   []savedSong `json:"songs"`
	// Apart are the files the user took out of a song: only the user
	// links them again.
	Apart []string     `json:"apart,omitempty"`
	Facts []savedFacts `json:"facts,omitempty"`
}

// linksVersion is the version of local-copies.json.
const linksVersion = 1

// savedSong is a song with copies, by its ID: its copies as they were last
// grouped, best first, those the user linked by hand, which it keeps
// while their files are away, and the key and the length of its best copy,
// which a file of the song finds it by once its copies all went, as when
// they moved.
type savedSong struct {
	ID         string   `json:"id"`
	Copies     []string `json:"copies,omitempty"`
	Linked     []string `json:"linked,omitempty"`
	Key        string   `json:"key,omitempty"`
	DurationMs int      `json:"durationMs,omitempty"`
}

// savedFacts are the facts read of a copy's audio, while its file has the
// size and the time of change it had then.
type savedFacts struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Facts
}

// grouping is what group works out: the songs to keep, and the songs
// that became one with another, by their IDs, whose liked songs, mixes
// and history go to the other's.
type grouping struct {
	saved  savedLinks
	merged map[string]string
}

// group works out the songs with copies of the files of the index, from
// what was kept of them, factsOf telling what each file's audio is:
//
//   - Files of the same match key whose lengths are within copyTolerance
//     of the shortest are copies, but those the user took apart; and
//     those the user linked to a song are its copies whatever their tags.
//   - Each group of copies is the song of its files' past: the one whose
//     copies it holds most of, those linked by hand first, then those of
//     the song's key; else, if it lost all its files, as when they moved,
//     the song of its key and length; else, of two files or more, a new
//     song, whose ID names its best copy.
//   - A song whose files all went to another's group became that one; one
//     without files is kept, for them to come back, with its key.
func group(entries map[string]Entry, saved savedLinks, factsOf func(Entry) Facts) grouping {
	present := map[string]Entry{}
	for p, e := range entries {
		if e.Skipped == "" {
			present[p] = e
		}
	}
	apart := map[string]bool{}
	for _, p := range saved.Apart {
		if _, ok := present[p]; ok {
			apart[p] = true
		}
	}
	keys := map[string]string{}
	for p, e := range present {
		if !apart[p] {
			keys[p] = matchKey(e)
		}
	}

	// The files that make one song, as a union of sets.
	parent := map[string]string{}
	var find func(p string) string
	find = func(p string) string {
		q, ok := parent[p]
		if !ok {
			parent[p] = p
			return p
		}
		if q == p {
			return p
		}
		r := find(q)
		parent[p] = r
		return r
	}
	union := func(a, b string) {
		if ra, rb := find(a), find(b); ra != rb {
			parent[rb] = ra
		}
	}
	byKey := map[string][]Entry{}
	for p, k := range keys {
		if k != "" {
			byKey[k] = append(byKey[k], present[p])
		}
	}
	for _, es := range byKey {
		if len(es) < 2 {
			continue
		}
		slices.SortFunc(es, func(a, b Entry) int {
			return cmp.Or(cmp.Compare(a.DurationMs, b.DurationMs), strings.Compare(a.Path, b.Path))
		})
		start := 0
		for i := 1; i < len(es); i++ {
			if es[i].DurationMs-es[start].DurationMs > copyTolerance {
				start = i
				continue
			}
			union(es[start].Path, es[i].Path)
		}
	}
	// What the user linked stays linked; and the files a song held, and
	// those of the keys of songs that lost theirs, may be its own.
	held := func(s savedSong) bool {
		return slices.ContainsFunc(append(slices.Clone(s.Copies), s.Linked...), func(p string) bool {
			_, ok := present[p]
			return ok
		})
	}
	dormant := map[string]bool{}
	for _, s := range saved.Songs {
		first := ""
		for _, p := range s.Linked {
			if _, ok := present[p]; !ok {
				continue
			}
			if first == "" {
				first = find(p)
			} else {
				union(first, p)
			}
		}
		for _, p := range s.Copies {
			if _, ok := present[p]; ok {
				find(p)
			}
		}
		if !held(s) && s.Key != "" {
			dormant[s.Key] = true
		}
	}
	for p, k := range keys {
		if k != "" && dormant[k] {
			find(p)
		}
	}

	// The groups, each best first, in the order of their best copies.
	copyOf := func(p string) Copy {
		e := present[p]
		return Copy{Path: p, Facts: factsOf(e), Bitrate: bitrate(e)}
	}
	members := map[string][]Copy{}
	for p := range parent {
		r := find(p)
		members[r] = append(members[r], copyOf(p))
	}
	var groups [][]Copy
	for _, m := range members {
		slices.SortFunc(m, compareCopies)
		groups = append(groups, m)
	}
	slices.SortFunc(groups, func(a, b []Copy) int { return strings.Compare(a[0].Path, b[0].Path) })
	groupAt := map[string]int{}
	for gi, g := range groups {
		for _, c := range g {
			groupAt[c.Path] = gi
		}
	}

	// Which song each group is: the claims of the songs kept, the
	// strongest first.
	type claim struct{ group, song, score int }
	var claims []claim
	for gi, g := range groups {
		for si, s := range saved.Songs {
			score := 0
			for _, c := range g {
				switch {
				case slices.Contains(s.Linked, c.Path):
					score += 4
				case slices.Contains(s.Copies, c.Path) && keys[c.Path] == s.Key:
					score += 2
				case slices.Contains(s.Copies, c.Path):
					score++
				}
			}
			if score > 0 {
				claims = append(claims, claim{gi, si, score})
			}
		}
	}
	slices.SortFunc(claims, func(a, b claim) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.song, b.song), cmp.Compare(a.group, b.group))
	})
	songOf := make([]int, len(groups)) // the index of the song kept, -1 for none
	for i := range songOf {
		songOf[i] = -1
	}
	groupOf := make([]int, len(saved.Songs))
	for i := range groupOf {
		groupOf[i] = -1
	}
	for _, c := range claims {
		if songOf[c.group] < 0 && groupOf[c.song] < 0 {
			songOf[c.group], groupOf[c.song] = c.song, c.group
		}
	}
	// A song that lost all its files is the one of a group of its key
	// and length, as when its files moved.
	for gi, g := range groups {
		if songOf[gi] >= 0 || keys[g[0].Path] == "" {
			continue
		}
		best := present[g[0].Path]
		for si, s := range saved.Songs {
			if groupOf[si] < 0 && !held(s) && s.Key == keys[g[0].Path] && abs(s.DurationMs-best.DurationMs) <= copyTolerance {
				songOf[gi], groupOf[si] = si, gi
				break
			}
		}
	}

	ids := map[string]bool{}
	for _, s := range saved.Songs {
		ids[s.ID] = true
	}
	next := savedLinks{Version: linksVersion}
	idOf := make([]string, len(groups))
	for gi, g := range groups {
		if si := songOf[gi]; si >= 0 {
			idOf[gi] = saved.Songs[si].ID
		} else if len(g) > 1 {
			idOf[gi] = newSongID(g, ids)
			ids[idOf[gi]] = true
		}
	}
	songAt := func(gi int, s savedSong) savedSong {
		g := groups[gi]
		best := present[g[0].Path]
		out := savedSong{ID: idOf[gi], Key: keys[g[0].Path], DurationMs: best.DurationMs}
		if out.Key == "" {
			out.Key = s.Key
		}
		for _, c := range g {
			out.Copies = append(out.Copies, c.Path)
		}
		for _, p := range s.Linked {
			_, there := present[p]
			if !there || slices.Contains(out.Copies, p) {
				out.Linked = append(out.Linked, p)
			}
		}
		return out
	}
	merged := map[string]string{}
	for si, s := range saved.Songs {
		if gi := groupOf[si]; gi >= 0 {
			next.Songs = append(next.Songs, songAt(gi, s))
			continue
		}
		// The files it held went to one other song: it became that one.
		into := ""
		one := true
		for _, p := range append(slices.Clone(s.Copies), s.Linked...) {
			if _, ok := present[p]; !ok {
				continue
			}
			id := ""
			if gi, ok := groupAt[p]; ok {
				id = idOf[gi]
			}
			switch {
			case id == "":
				one = false
			case into == "":
				into = id
			case into != id:
				one = false
			}
		}
		if into != "" && one {
			merged[s.ID] = into
			continue
		}
		// Kept without files, for them to come back.
		kept := savedSong{ID: s.ID, Key: s.Key, DurationMs: s.DurationMs}
		for _, p := range s.Linked {
			if _, ok := present[p]; !ok {
				kept.Linked = append(kept.Linked, p)
			}
		}
		if kept.Key != "" || len(kept.Linked) > 0 {
			next.Songs = append(next.Songs, kept)
		}
	}
	var fresh []savedSong
	for gi := range groups {
		if songOf[gi] < 0 && idOf[gi] != "" {
			fresh = append(fresh, songAt(gi, savedSong{}))
		}
	}
	slices.SortFunc(fresh, func(a, b savedSong) int { return strings.Compare(a.ID, b.ID) })
	next.Songs = append(next.Songs, fresh...)
	for p := range apart {
		next.Apart = append(next.Apart, p)
	}
	slices.Sort(next.Apart)
	return grouping{saved: next, merged: merged}
}

// newSongID is the ID of a new song of copies g, best first: of its best
// copy's path, or of another's when a song has that one, or of the best's
// with a number.
func newSongID(g []Copy, taken map[string]bool) string {
	for _, c := range g {
		if id := SongID(c.Path); !taken[id] {
			return id
		}
	}
	for n := 2; ; n++ {
		if id := songID(b64.EncodeToString([]byte(g[0].Path)) + "." + strconv.Itoa(n)); !taken[id] {
			return id
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
