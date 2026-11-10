package localfiles

import (
	"cmp"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/media"
)

// UnknownArtist is the artist of a track whose file names none.
const UnknownArtist = "Unknown artist"

// NoArtist reports whether t is a track on this computer whose file names
// no artist, as a voice memo: its title is its file's name, which may be
// anything, and no service hears of it.
func NoArtist(t media.Track) bool {
	return t.Source == media.SourceFile && len(t.Artists) == 1 && t.Artists[0].ID == "" && t.Artists[0].Name == UnknownArtist
}

// VariousArtists is the artist of an album of several artists that names
// no album artist.
const VariousArtists = "Various artists"

// views are the tracks, albums and artists of the index, as of a version
// of it, which the windows read many times between changes.
type views struct {
	version uint64
	tracks  []media.Track
	albums  []media.Album
	artists []media.Artist
	// byAlbum and byArtist index albums and artists by their IDs, and the
	// tracks of each; text is each track's text that a search searches.
	byAlbum  map[string]int
	byArtist map[string]int
	text     []string
}

// view returns the views of the index as it is.
func (x *Index) view() *views {
	x.mu.Lock()
	defer x.mu.Unlock()
	if x.views != nil && x.views.version == x.version {
		return x.views
	}
	v := &views{version: x.version, byAlbum: map[string]int{}, byArtist: map[string]int{}}
	var entries []Entry
	for _, e := range x.entries {
		if e.Skipped == "" {
			entries = append(entries, e)
		}
	}
	// Tracks in the order of their albums, then of their numbers and
	// titles, as a shelf of records.
	keys := make(map[string][2]string, len(entries))
	for _, e := range entries {
		keys[e.Path] = [2]string{fold(e.Album), fold(title(e))}
	}
	slices.SortFunc(entries, func(a, b Entry) int {
		ka, kb := keys[a.Path], keys[b.Path]
		return cmp.Or(strings.Compare(ka[0], kb[0]), cmp.Compare(a.Disc, b.Disc), cmp.Compare(a.Track, b.Track),
			strings.Compare(ka[1], kb[1]), cmp.Compare(a.Path, b.Path))
	})
	albumOf := map[string]*media.Album{}
	albumArtistsOf := map[string][]string{}
	var albumOrder []string
	artistOf := map[string]*media.Artist{}
	var artistOrder []string
	for _, e := range entries {
		t := e.track()
		v.tracks = append(v.tracks, t)
		v.text = append(v.text, fold(t.Title+" "+t.ArtistNames()+" "+e.Album+" "+strings.Join(e.AlbumArtists, " ")))
		if t.Album != nil {
			a, ok := albumOf[t.Album.ID]
			if !ok {
				a = &media.Album{ID: t.Album.ID, Source: media.SourceFile, Title: e.Album, Artwork: t.Artwork}
				albumOf[t.Album.ID] = a
				albumOrder = append(albumOrder, t.Album.ID)
			}
			a.Tracks = append(a.Tracks, t)
			if len(e.AlbumArtists) > 0 && albumArtistsOf[t.Album.ID] == nil {
				albumArtistsOf[t.Album.ID] = e.AlbumArtists
			}
			if e.Year > 0 {
				if y, _ := strconv.Atoi(a.Year); e.Year > y {
					a.Year = strconv.Itoa(e.Year)
				}
			}
			if len(a.Artwork) == 0 {
				a.Artwork = t.Artwork
			}
		}
		names := append(slices.Clone(e.Artists), e.AlbumArtists...)
		for _, name := range names {
			id := artistID(name)
			r, ok := artistOf[id]
			if !ok {
				r = &media.Artist{ID: id, Source: media.SourceFile, Name: name}
				artistOf[id] = r
				artistOrder = append(artistOrder, id)
			}
			if slices.ContainsFunc(e.Artists, func(n string) bool { return fold(n) == fold(name) }) &&
				!slices.ContainsFunc(r.TopTracks, func(t2 media.Track) bool { return t2.ID == t.ID }) {
				r.TopTracks = append(r.TopTracks, t)
			}
			if len(r.Artwork) == 0 {
				r.Artwork = t.Artwork
			}
		}
	}
	for _, id := range albumOrder {
		a := albumOf[id]
		a.Artists = albumArtists(albumArtistsOf[id], a.Tracks)
		a.TrackCount = len(a.Tracks)
		v.byAlbum[id] = len(v.albums)
		v.albums = append(v.albums, *a)
		for _, r := range a.Artists {
			if artist, ok := artistOf[r.ID]; ok {
				artist.Albums = append(artist.Albums, *a)
			}
		}
	}
	for _, id := range artistOrder {
		r := artistOf[id]
		// An artist's albums are also those of their tracks.
		for _, t := range r.TopTracks {
			if t.Album != nil && !slices.ContainsFunc(r.Albums, func(a media.Album) bool { return a.ID == t.Album.ID }) {
				r.Albums = append(r.Albums, v.albums[v.byAlbum[t.Album.ID]])
			}
		}
		for i := range r.Albums {
			r.Albums[i].Tracks = nil
		}
		v.byArtist[id] = len(v.artists)
		v.artists = append(v.artists, *r)
	}
	x.views = v
	return v
}

// albumArtists are the artists of an album: its album artists, else the
// one artist of its tracks, else various artists.
func albumArtists(tagged []string, tracks []media.Track) []media.Ref {
	if len(tagged) > 0 {
		return refs(tagged)
	}
	first := ""
	for _, t := range tracks {
		names := t.ArtistNames()
		switch {
		case first == "":
			first = names
		case fold(names) != fold(first):
			return []media.Ref{{Name: VariousArtists}}
		}
	}
	if len(tracks) > 0 {
		return tracks[0].Artists
	}
	return nil
}

// refs are links to the artists of names.
func refs(names []string) []media.Ref {
	out := make([]media.Ref, len(names))
	for i, n := range names {
		out[i] = media.Ref{ID: artistID(n), Name: n}
	}
	return out
}

// title is an entry's title: its tag's, else its file's name.
func title(e Entry) string {
	if e.Title != "" {
		return e.Title
	}
	base := filepath.Base(e.Path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// albumKey is what an entry's album is grouped by: its title, and its
// album artist, else the folder the file is in, as an album's files
// mostly are, and a compilation's artists differ.
func (e Entry) albumKey() string {
	by := "dir:" + filepath.Dir(e.Path)
	if len(e.AlbumArtists) > 0 {
		by = "artist:" + fold(e.AlbumArtists[0])
	}
	return fold(e.Album) + "\x00" + by
}

// track is the entry as a track.
func (e Entry) track() media.Track {
	t := media.Track{ID: TrackID(e.Path), Source: media.SourceFile, Title: title(e), DurationMs: e.DurationMs, Explicit: e.Explicit}
	t.Artists = refs(e.Artists)
	if len(t.Artists) == 0 {
		t.Artists = []media.Ref{{Name: UnknownArtist}}
	}
	if e.Album != "" {
		t.Album = &media.Ref{ID: albumID(e.albumKey()), Name: e.Album}
	}
	if e.Art != "" {
		t.Artwork = []media.Image{{URL: artScheme + e.Art}}
	}
	return t
}

// Tracks returns the tracks of the index, in the order of their albums.
func (x *Index) Tracks() []media.Track { return x.view().tracks }

// Albums returns the albums of the index, in the order of their titles,
// without their tracks.
func (x *Index) Albums() []media.Album {
	v := x.view()
	out := make([]media.Album, len(v.albums))
	for i, a := range v.albums {
		a.Tracks = nil
		out[i] = a
	}
	return out
}

// Artists returns the artists of the index, in the order of their names,
// without their tracks and albums.
func (x *Index) Artists() []media.Artist {
	v := x.view()
	out := make([]media.Artist, len(v.artists))
	for i, a := range v.artists {
		a.TopTracks, a.Albums = nil, nil
		out[i] = a
	}
	slices.SortStableFunc(out, func(a, b media.Artist) int { return compareFold(a.Name, b.Name) })
	return out
}

// Album returns an album of the index, with its tracks.
func (x *Index) Album(id string) (media.Album, bool) {
	v := x.view()
	i, ok := v.byAlbum[id]
	if !ok {
		return media.Album{}, false
	}
	return v.albums[i], true
}

// Artist returns an artist of the index, with their tracks and albums.
func (x *Index) Artist(id string) (media.Artist, bool) {
	v := x.view()
	i, ok := v.byArtist[id]
	if !ok {
		return media.Artist{}, false
	}
	return v.artists[i], true
}

// Track returns the track of the file at path, as the index has it.
func (x *Index) Track(path string) (media.Track, bool) {
	e, ok := x.Entry(path)
	if !ok || e.Skipped != "" {
		return media.Track{}, false
	}
	return e.track(), true
}

// TrackOf returns the entry as a track, as the index would have it.
func TrackOf(e Entry) media.Track { return e.track() }

// Search returns what the index holds of q: the tracks whose titles,
// artists or albums hold each of its words, regardless of case and
// accents, up to limit, and the albums and artists whose names do.
func (x *Index) Search(q string, limit int) media.SearchResults {
	words := strings.Fields(fold(q))
	var res media.SearchResults
	if len(words) == 0 {
		return res
	}
	v := x.view()
	for i, t := range v.tracks {
		if len(res.Tracks) < limit && allIn(words, v.text[i]) {
			res.Tracks = append(res.Tracks, t)
		}
	}
	for _, a := range v.albums {
		if len(res.Albums) < limit && allIn(words, fold(a.Title+" "+names(a.Artists))) {
			a.Tracks = nil
			res.Albums = append(res.Albums, a)
		}
	}
	for _, a := range v.artists {
		if len(res.Artists) < limit && allIn(words, fold(a.Name)) {
			a.TopTracks, a.Albums = nil, nil
			res.Artists = append(res.Artists, a)
		}
	}
	return res
}

func names(rs []media.Ref) string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return strings.Join(out, " ")
}

func allIn(words []string, text string) bool {
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}

// fold makes text compare as a reader reads it: lowercase, and without
// its accents.
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func compareFold(a, b string) int { return strings.Compare(fold(a), fold(b)) }
