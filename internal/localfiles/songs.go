package localfiles

// The songs with copies of the index (#340), as copies.go groups them:
// kept in local-copies.json, beside the index, and in memory as of the
// index's entries, which the views show once each, and which play their
// best copy.

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
)

// songPrefix starts the native part of a song's ID: "file:song:<key>",
// whose key is its first best copy's path in base64, so that the same
// copies make a song of the same ID again; ".<n>" follows when another
// song had that one.
const songPrefix = "song:"

// SongID returns the ID a song whose best copy is the file at path takes,
// unless another song has it.
func SongID(path string) string {
	return songID(b64.EncodeToString([]byte(filepath.Clean(path))))
}

func songID(key string) string { return media.ID(media.SourceFile, songPrefix+key) }

// IsSong reports whether id is a song's with copies.
func IsSong(id string) bool {
	source, native, err := media.ParseID(id)
	return err == nil && source == media.SourceFile && strings.HasPrefix(native, songPrefix)
}

// anchorOf returns the path a song's ID names: of the copy it was made of.
func anchorOf(id string) (string, bool) {
	_, native, err := media.ParseID(id)
	if err != nil {
		return "", false
	}
	key, _, _ := strings.Cut(strings.TrimPrefix(native, songPrefix), ".")
	b, err := b64.DecodeString(key)
	if err != nil || len(b) == 0 || !filepath.IsAbs(string(b)) {
		return "", false
	}
	return string(b), true
}

// Song is a song with copies: its track, which plays its best copy, and
// its copies of the index, best first.
type Song struct {
	Track  media.Track
	Copies []Copy
}

// copyLinks are the songs with copies of an index: kept in file, and as
// of the index's entries in state, under the index's lock; mu has one
// change of them at a time. inspect reads what a copy's audio is, and
// linked hears of the tracks that became songs (OnLinked); told is set
// once it heard of them all.
type copyLinks struct {
	file    *store.File[savedLinks]
	state   linkState
	mu      sync.Mutex
	inspect func(io.ReadSeeker) (Facts, error)
	linked  func(map[string]media.Track)
	told    bool
}

// linkState is what the songs with copies are of the entries: each song
// by its ID, with the paths of its copies the index holds, best first, and
// the song of each of those paths.
type linkState struct {
	songs  map[string]*linkedSong
	ofPath map[string]*linkedSong
	saved  savedLinks
}

type linkedSong struct {
	id     string
	copies []string
	linked map[string]bool
}

// openLinks opens the songs with copies kept beside the index at path.
func (x *Index) openLinks(path string) error {
	f, err := store.Open(filepath.Join(filepath.Dir(path), "local-copies.json"), savedLinks{Version: linksVersion})
	if err != nil {
		return err
	}
	x.copies.file = f
	x.copies.state = stateOf(decodePaths(f.Get()), x.entries)
	return nil
}

// stateOf is what the songs kept are of entries.
func stateOf(saved savedLinks, entries map[string]Entry) linkState {
	st := linkState{songs: map[string]*linkedSong{}, ofPath: map[string]*linkedSong{}, saved: saved}
	for _, s := range saved.Songs {
		ls := &linkedSong{id: s.ID, linked: map[string]bool{}}
		for _, p := range s.Linked {
			ls.linked[p] = true
		}
		for _, p := range s.Copies {
			if e, ok := entries[p]; ok && e.Skipped == "" && st.ofPath[p] == nil {
				ls.copies = append(ls.copies, p)
				st.ofPath[p] = ls
			}
		}
		if len(ls.copies) > 0 {
			st.songs[s.ID] = ls
		}
	}
	return st
}

// decodePaths and encodePaths turn the paths of the songs kept from and
// to how the file holds them: one that isn't UTF-8, in base64.
func decodePaths(s savedLinks) savedLinks { return mapPaths(s, store.DecodePath) }
func encodePaths(s savedLinks) savedLinks { return mapPaths(s, store.EncodePath) }

func mapPaths(s savedLinks, fn func(string) string) savedLinks {
	each := func(ps []string) []string {
		if ps == nil {
			return nil
		}
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = fn(p)
		}
		return out
	}
	out := savedLinks{Version: s.Version, Apart: each(s.Apart)}
	for _, sg := range s.Songs {
		sg.Copies, sg.Linked = each(sg.Copies), each(sg.Linked)
		out.Songs = append(out.Songs, sg)
	}
	for _, f := range s.Facts {
		f.Path = fn(f.Path)
		out.Facts = append(out.Facts, f)
	}
	return out
}

// Inspect has fn read what the audio of a copy is, as audio.Inspect does,
// which tells the best copy of a song: until it did, a copy's extension
// tells its codec.
func (x *Index) Inspect(fn func(io.ReadSeeker) (Facts, error)) {
	x.copies.mu.Lock()
	defer x.copies.mu.Unlock()
	x.copies.inspect = fn
}

// OnLinked has fn told, after a change to the songs with copies, and once
// after Open, the ID of each track that is a song's now, by its ID: each
// copy's file, and each song that became one with another. What the user
// keeps of those, as the liked songs, goes to the song.
func (x *Index) OnLinked(fn func(map[string]media.Track)) {
	x.copies.mu.Lock()
	defer x.copies.mu.Unlock()
	x.copies.linked = fn
}

// relink groups the copies of the songs again, as the entries changed,
// keeps what changed, and tells OnLinked's function.
func (x *Index) relink() {
	c := &x.copies
	c.mu.Lock()
	defer c.mu.Unlock()
	x.relinkHeld(nil)
}

// relinkHeld is relink, while mu is held; merged are songs that a link
// by hand made one with another, by their IDs.
func (x *Index) relinkHeld(merged map[string]string) (changed bool) {
	c := &x.copies
	x.mu.Lock()
	entries := x.entries
	saved := c.state.saved
	x.mu.Unlock()
	known := map[string]savedFacts{}
	for _, f := range saved.Facts {
		known[f.Path] = f
	}
	factsOf := func(e Entry) Facts {
		if f, ok := known[e.Path]; ok && f.Size == e.Size && f.Modified == e.Modified {
			return f.Facts
		}
		return guessFacts(e.Path)
	}
	g := group(entries, saved, factsOf)
	// What the copies' audio is, read once of each, may change which is
	// the best, and which a new song's ID names.
	if c.inspect != nil {
		read := false
		for _, s := range g.saved.Songs {
			for _, p := range s.Copies {
				e := entries[p]
				if f, ok := known[p]; ok && f.Size == e.Size && f.Modified == e.Modified {
					continue
				}
				known[p] = savedFacts{Path: p, Size: e.Size, Modified: e.Modified, Facts: x.readFacts(p)}
				read = true
			}
		}
		if read {
			g = group(entries, saved, factsOf)
		}
	}
	for _, s := range g.saved.Songs {
		for _, p := range s.Copies {
			if f, ok := known[p]; ok && f.Size == entries[p].Size && f.Modified == entries[p].Modified {
				g.saved.Facts = append(g.saved.Facts, f)
			}
		}
	}
	slices.SortFunc(g.saved.Facts, func(a, b savedFacts) int { return strings.Compare(a.Path, b.Path) })
	maps.Copy(g.merged, merged)
	changed = !sameLinks(g.saved, decodePaths(c.file.Get()))
	if changed {
		if err := c.file.Set(encodePaths(g.saved)); err != nil {
			log.Printf("mixmusic: keeping the copies of the songs on this computer: %v", err)
		}
	}
	st := stateOf(g.saved, entries)
	x.mu.Lock()
	shown := !sameState(st, c.state)
	c.state = st
	if shown {
		x.version++
	}
	x.mu.Unlock()
	if c.linked != nil && (changed || !c.told || len(g.merged) > 0) {
		c.told = true
		c.linked(x.retargets(st, entries, g.merged))
	}
	return changed || shown
}

// readFacts reads what the audio of the file at path is: else what its
// extension tells.
func (x *Index) readFacts(path string) Facts {
	f, err := os.Open(path)
	if err != nil {
		return guessFacts(path)
	}
	defer f.Close()
	facts, err := x.copies.inspect(f)
	if err != nil || facts.Codec == "" {
		return guessFacts(path)
	}
	return facts
}

// sameLinks reports whether a and b keep the same.
func sameLinks(a, b savedLinks) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ja) == string(jb)
}

// sameState reports whether a and b show the same songs.
func sameState(a, b linkState) bool {
	if len(a.songs) != len(b.songs) {
		return false
	}
	for id, s := range a.songs {
		t := b.songs[id]
		if t == nil || !slices.Equal(s.copies, t.copies) || !maps.Equal(s.linked, t.linked) {
			return false
		}
	}
	return true
}

// retargets are the tracks that are songs' now, by their IDs: each copy's
// file, and each song merged into another.
func (x *Index) retargets(st linkState, entries map[string]Entry, merged map[string]string) map[string]media.Track {
	out := map[string]media.Track{}
	for _, s := range st.songs {
		t := songTrack(s, entries)
		for _, p := range s.copies {
			out[TrackID(p)] = t
		}
	}
	for from, to := range merged {
		if s := st.songs[to]; s != nil {
			out[from] = songTrack(s, entries)
		}
	}
	return out
}

// songTrack is a song's track: its first copy's of entries, under the
// song's ID.
func songTrack(s *linkedSong, entries map[string]Entry) media.Track {
	e := entries[s.copies[0]]
	for _, p := range s.copies {
		if f, ok := entries[p]; ok && f.Skipped == "" {
			e = f
			break
		}
	}
	t := e.track()
	t.ID = s.id
	return t
}

// leadOf is the path of the first copy of a song the index holds, which
// the lists show, while the lock is held.
func (x *Index) leadOf(s *linkedSong) string {
	for _, p := range s.copies {
		if e, ok := x.entries[p]; ok && e.Skipped == "" {
			return p
		}
	}
	return ""
}

// songOf returns the song of a song's ID or of a copy's, while the lock
// is held.
func (x *Index) songOf(id string) *linkedSong {
	st := &x.copies.state
	if IsSong(id) {
		return st.songs[id]
	}
	if p, ok := PathOf(id); ok {
		return st.ofPath[p]
	}
	return nil
}

// Song returns the song of a song's ID or of the ID of one of its copies,
// with its copies the index holds, best first: the one it plays marked,
// the first whose file is there.
func (x *Index) Song(id string) (Song, bool) {
	x.mu.Lock()
	s := x.songOf(id)
	if s == nil {
		x.mu.Unlock()
		return Song{}, false
	}
	song := Song{Track: songTrack(s, x.entries)}
	known := map[string]savedFacts{}
	for _, f := range x.copies.state.saved.Facts {
		known[f.Path] = f
	}
	for _, p := range s.copies {
		e := x.entries[p]
		cp := Copy{Track: e.track(), Path: p, Facts: guessFacts(p), Bitrate: bitrate(e), Linked: s.linked[p]}
		if f, ok := known[p]; ok && f.Size == e.Size && f.Modified == e.Modified {
			cp.Facts = f.Facts
		}
		song.Copies = append(song.Copies, cp)
	}
	x.mu.Unlock()
	for i := range song.Copies {
		if !x.Missing(song.Copies[i].Track.ID) {
			song.Copies[i].Best = true
			break
		}
	}
	return song, true
}

// FileOf returns the path of the file that plays a local track: a file's
// own, and of a song with copies, its best copy whose file is there, else
// the one its ID names, if that's there.
func (x *Index) FileOf(id string) (string, bool) {
	if !IsSong(id) {
		return PathOf(id)
	}
	x.mu.Lock()
	var copies []string
	if s := x.songOf(id); s != nil {
		copies = slices.Clone(s.copies)
	}
	x.mu.Unlock()
	for _, p := range copies {
		if !x.Missing(TrackID(p)) {
			return p, true
		}
	}
	if len(copies) == 0 {
		// A song without copies here, as when its links were lost, plays
		// the file its ID names, while it's there.
		if p, ok := anchorOf(id); ok && !x.Missing(TrackID(p)) {
			return p, true
		}
	}
	return "", false
}

// SongTrack returns the track of the song t is a copy of, else t.
func (x *Index) SongTrack(t media.Track) media.Track {
	if t.Source != media.SourceFile || IsSong(t.ID) {
		return t
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if p, ok := PathOf(t.ID); ok {
		if s := x.copies.state.ofPath[p]; s != nil {
			return songTrack(s, x.entries)
		}
	}
	return t
}

// SongIDOf returns the ID of the song of a copy's ID, else id.
func (x *Index) SongIDOf(id string) string {
	p, ok := PathOf(id)
	if !ok {
		return id
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if s := x.copies.state.ofPath[p]; s != nil {
		return s.id
	}
	return id
}

// SongIDs returns the ID of the song of each copy the index holds, by the
// copy's ID.
func (x *Index) SongIDs() map[string]string {
	x.mu.Lock()
	defer x.mu.Unlock()
	out := map[string]string{}
	for p, s := range x.copies.state.ofPath {
		out[TrackID(p)] = s.id
	}
	return out
}

// errOneFile is a link of fewer than two files, and errOutside one of a
// track that isn't a song of the folders of music, as a file opened.
var (
	errOneFile = errors.New("a song needs two files or more to link")
	errOutside = errors.New("only the songs of your folders of music can be linked as copies")
)

// Link makes the tracks of ids copies of one song, by hand: files of the
// index, and songs with copies, which become one. They stay linked
// whatever their tags say, until the user takes one out.
func (x *Index) Link(ids []string) error {
	c := &x.copies
	c.mu.Lock()
	defer c.mu.Unlock()
	x.mu.Lock()
	var paths, songs []string
	outside := false
	for _, id := range ids {
		if s := x.songOf(id); s != nil {
			if !slices.Contains(songs, s.id) {
				songs = append(songs, s.id)
			}
			paths = append(paths, s.copies...)
			continue
		}
		p, ok := PathOf(id)
		if e, in := x.entries[p]; ok && in && e.Skipped == "" {
			paths = append(paths, p)
		} else {
			outside = true
		}
	}
	saved := c.state.saved
	x.mu.Unlock()
	slices.Sort(paths)
	paths = slices.Compact(paths)
	switch {
	case outside:
		return errOutside
	case len(paths) < 2:
		return errOneFile
	}
	next := cloneLinks(saved)
	target := ""
	if len(songs) > 0 {
		target = songs[0]
	}
	merged := map[string]string{}
	for i := range next.Songs {
		s := &next.Songs[i]
		switch {
		case s.ID == target:
			s.Copies = union(s.Copies, paths)
			s.Linked = union(s.Linked, paths)
		case slices.Contains(songs, s.ID):
			merged[s.ID] = target
			s.Copies, s.Linked = nil, nil
		default:
			s.Copies = slices.DeleteFunc(s.Copies, func(p string) bool { return slices.Contains(paths, p) })
			s.Linked = slices.DeleteFunc(s.Linked, func(p string) bool { return slices.Contains(paths, p) })
		}
	}
	next.Songs = slices.DeleteFunc(next.Songs, func(s savedSong) bool { return merged[s.ID] != "" })
	if target == "" {
		ids := map[string]bool{}
		for _, s := range next.Songs {
			ids[s.ID] = true
		}
		var cs []Copy
		x.mu.Lock()
		for _, p := range paths {
			e := x.entries[p]
			cs = append(cs, Copy{Path: p, Facts: guessFacts(p), Bitrate: bitrate(e)})
		}
		slices.SortFunc(cs, compareCopies)
		best := x.entries[cs[0].Path]
		x.mu.Unlock()
		next.Songs = append(next.Songs, savedSong{ID: newSongID(cs, ids), Copies: paths, Linked: paths,
			Key: matchKey(best), DurationMs: best.DurationMs})
	}
	next.Apart = slices.DeleteFunc(next.Apart, func(p string) bool { return slices.Contains(paths, p) })
	x.mu.Lock()
	c.state.saved = next
	x.mu.Unlock()
	if x.relinkHeld(merged) {
		x.changed()
	}
	return nil
}

// Unlink takes the copy of id, a file's ID, out of its song: the song
// keeps what was played and liked of it. Only the user links it again.
func (x *Index) Unlink(id string) error {
	path, ok := PathOf(id)
	if !ok {
		return errors.New("it isn't a file on this computer")
	}
	c := &x.copies
	c.mu.Lock()
	defer c.mu.Unlock()
	x.mu.Lock()
	s := c.state.ofPath[path]
	saved := c.state.saved
	x.mu.Unlock()
	if s == nil || len(s.copies) < 2 {
		return errors.New("it's the only copy of its song")
	}
	next := cloneLinks(saved)
	for i := range next.Songs {
		sg := &next.Songs[i]
		sg.Copies = slices.DeleteFunc(sg.Copies, func(p string) bool { return p == path })
		sg.Linked = slices.DeleteFunc(sg.Linked, func(p string) bool { return p == path })
	}
	if !slices.Contains(next.Apart, path) {
		next.Apart = append(next.Apart, path)
		slices.Sort(next.Apart)
	}
	x.mu.Lock()
	c.state.saved = next
	x.mu.Unlock()
	if x.relinkHeld(nil) {
		x.changed()
	}
	return nil
}

// cloneLinks returns a copy of s that changes apart from it.
func cloneLinks(s savedLinks) savedLinks {
	out := savedLinks{Version: s.Version, Apart: slices.Clone(s.Apart), Facts: slices.Clone(s.Facts)}
	for _, sg := range s.Songs {
		sg.Copies, sg.Linked = slices.Clone(sg.Copies), slices.Clone(sg.Linked)
		out.Songs = append(out.Songs, sg)
	}
	return out
}

// union returns a with the paths of b it hasn't, after them.
func union(a, b []string) []string {
	out := slices.Clone(a)
	for _, p := range b {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}
