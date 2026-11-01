// Package library keeps what the user collects in MixMusic itself, across
// services: liked and disliked tracks, playlists that mix tracks of every
// service, the history of what they played and their recent searches.
package library

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
)

const (
	maxHistory  = 1000
	maxSearches = 20
)

// ErrNoPlaylist is returned for playlists that do not exist.
var ErrNoPlaylist = errors.New("library: no such playlist")

// Playlist is a playlist made in MixMusic, which may hold tracks of every
// service.
type Playlist struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Tracks      []media.Track `json:"tracks"`
	Created     time.Time     `json:"created"`
	Updated     time.Time     `json:"updated"`
}

// Play is a track the user played, and when.
type Play struct {
	Track    media.Track `json:"track"`
	PlayedAt time.Time   `json:"playedAt"`
}

// Data is everything the library keeps.
type Data struct {
	Liked []media.Track `json:"liked"`
	// Disliked are the tracks the user dislikes, the last disliked first:
	// a track is liked or disliked, not both.
	Disliked  []media.Track `json:"disliked,omitempty"`
	Playlists []Playlist    `json:"playlists"`
	History   []Play        `json:"history"`
	Searches  []string      `json:"searches"`
}

// Library is the user's library, kept in a file.
type Library struct {
	file *store.File[Data]
	now  func() time.Time
	// disliked holds the IDs of the disliked tracks, for lists to ask of
	// each of their rows as they draw; indexing orders its changes.
	disliked atomic.Pointer[map[string]bool]
	indexing sync.Mutex
}

// Open opens the library kept at path.
func Open(path string) (*Library, error) {
	f, err := store.Open(path, Data{})
	if err != nil {
		return nil, err
	}
	l := &Library{file: f, now: time.Now}
	l.indexDisliked()
	return l, nil
}

// Liked returns the liked tracks, the last liked first.
func (l *Library) Liked() []media.Track {
	return orEmpty(l.file.Get().Liked)
}

// IsLiked reports whether a track is liked.
func (l *Library) IsLiked(id string) bool {
	return slices.ContainsFunc(l.file.Get().Liked, func(t media.Track) bool { return t.ID == id })
}

// SetLiked likes or unlikes a track. Liking a disliked track takes the
// dislike back.
func (l *Library) SetLiked(t media.Track, liked bool) error {
	err := l.file.Update(func(d *Data) error {
		d.Liked = slices.DeleteFunc(d.Liked, func(x media.Track) bool { return x.ID == t.ID })
		if liked {
			d.Liked = slices.Insert(d.Liked, 0, t)
			d.Disliked = slices.DeleteFunc(d.Disliked, func(x media.Track) bool { return x.ID == t.ID })
		}
		return nil
	})
	l.indexDisliked()
	return err
}

// Disliked returns the disliked tracks, the last disliked first.
func (l *Library) Disliked() []media.Track {
	return orEmpty(l.file.Get().Disliked)
}

// IsDisliked reports whether a track is disliked.
func (l *Library) IsDisliked(id string) bool {
	if ids := l.disliked.Load(); ids != nil {
		return (*ids)[id]
	}
	return false
}

// SetDisliked dislikes a track, or takes the dislike back. Disliking a
// liked track takes the like back, which unliked tells.
func (l *Library) SetDisliked(t media.Track, disliked bool) (unliked bool, err error) {
	err = l.file.Update(func(d *Data) error {
		unliked = false
		d.Disliked = slices.DeleteFunc(d.Disliked, func(x media.Track) bool { return x.ID == t.ID })
		if disliked {
			d.Disliked = slices.Insert(d.Disliked, 0, t)
			n := len(d.Liked)
			d.Liked = slices.DeleteFunc(d.Liked, func(x media.Track) bool { return x.ID == t.ID })
			unliked = len(d.Liked) < n
		}
		return nil
	})
	l.indexDisliked()
	return unliked && err == nil, err
}

// indexDisliked notes the IDs of the disliked tracks, as the file has
// them: after a change, the latest of them, also when another change ends
// meanwhile.
func (l *Library) indexDisliked() {
	l.indexing.Lock()
	defer l.indexing.Unlock()
	ids := map[string]bool{}
	for _, t := range l.file.Get().Disliked {
		ids[t.ID] = true
	}
	l.disliked.Store(&ids)
}

// Playlists returns the playlists, without their tracks, the last changed
// first.
func (l *Library) Playlists() []media.Playlist {
	ps := l.file.Get().Playlists
	out := make([]media.Playlist, 0, len(ps))
	for _, p := range ps {
		mp := p.media()
		mp.Tracks = []media.Track{}
		out = append(out, mp)
	}
	return out
}

// media converts a playlist, taking its artwork from its first tracks.
func (p Playlist) media() media.Playlist {
	mp := media.Playlist{
		ID:          media.ID(media.SourceLocal, p.ID),
		Source:      media.SourceLocal,
		Title:       p.Title,
		Description: p.Description,
		Owner:       "You",
		Artwork:     []media.Image{},
		TrackCount:  len(p.Tracks),
		Tracks:      orEmpty(p.Tracks),
		Editable:    true,
	}
	for _, t := range p.Tracks {
		if len(t.Artwork) > 0 {
			mp.Artwork = t.Artwork
			break
		}
	}
	return mp
}

// Playlist returns a playlist with its tracks.
func (l *Library) Playlist(id string) (media.Playlist, error) {
	for _, p := range l.file.Get().Playlists {
		if p.ID == id {
			return p.media(), nil
		}
	}
	return media.Playlist{}, ErrNoPlaylist
}

// CreatePlaylist makes a playlist, holding tracks.
func (l *Library) CreatePlaylist(title, description string, tracks []media.Track) (media.Playlist, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New playlist"
	}
	now := l.now()
	p := Playlist{ID: newID(), Title: title, Description: strings.TrimSpace(description), Tracks: orEmpty(tracks), Created: now, Updated: now}
	err := l.file.Update(func(d *Data) error {
		d.Playlists = slices.Insert(d.Playlists, 0, p)
		return nil
	})
	return p.media(), err
}

// editPlaylist changes a playlist with fn, and moves it first.
func (l *Library) editPlaylist(id string, fn func(p *Playlist)) error {
	return l.file.Update(func(d *Data) error {
		i := slices.IndexFunc(d.Playlists, func(p Playlist) bool { return p.ID == id })
		if i < 0 {
			return ErrNoPlaylist
		}
		p := d.Playlists[i]
		fn(&p)
		p.Updated = l.now()
		d.Playlists = slices.Insert(slices.Delete(d.Playlists, i, i+1), 0, p)
		return nil
	})
}

// UpdatePlaylist changes the title and the description of a playlist.
func (l *Library) UpdatePlaylist(id, title, description string) error {
	return l.editPlaylist(id, func(p *Playlist) {
		if t := strings.TrimSpace(title); t != "" {
			p.Title = t
		}
		p.Description = strings.TrimSpace(description)
	})
}

// DeletePlaylist deletes a playlist.
func (l *Library) DeletePlaylist(id string) error {
	return l.file.Update(func(d *Data) error {
		n := len(d.Playlists)
		d.Playlists = slices.DeleteFunc(d.Playlists, func(p Playlist) bool { return p.ID == id })
		if len(d.Playlists) == n {
			return ErrNoPlaylist
		}
		return nil
	})
}

// AddToPlaylist appends tracks to a playlist, leaving out those it has.
func (l *Library) AddToPlaylist(id string, tracks []media.Track) error {
	return l.editPlaylist(id, func(p *Playlist) {
		for _, t := range tracks {
			if !slices.ContainsFunc(p.Tracks, func(x media.Track) bool { return x.ID == t.ID }) {
				p.Tracks = append(p.Tracks, t)
			}
		}
	})
}

// RemoveFromPlaylist removes the track at index from a playlist.
func (l *Library) RemoveFromPlaylist(id string, index int) error {
	return l.editPlaylist(id, func(p *Playlist) {
		if index >= 0 && index < len(p.Tracks) {
			p.Tracks = slices.Delete(p.Tracks, index, index+1)
		}
	})
}

// MoveInPlaylist moves the track at from to the position to.
func (l *Library) MoveInPlaylist(id string, from, to int) error {
	return l.editPlaylist(id, func(p *Playlist) {
		if from < 0 || from >= len(p.Tracks) || to < 0 || to >= len(p.Tracks) || from == to {
			return
		}
		t := p.Tracks[from]
		p.Tracks = slices.Insert(slices.Delete(p.Tracks, from, from+1), to, t)
	})
}

// RecordPlay adds a track to the history.
func (l *Library) RecordPlay(t media.Track) error {
	return l.file.Update(func(d *Data) error {
		d.History = slices.Insert(d.History, 0, Play{Track: t, PlayedAt: l.now()})
		if len(d.History) > maxHistory {
			d.History = d.History[:maxHistory]
		}
		return nil
	})
}

// History returns the last plays, the latest first, at most limit.
func (l *Library) History(limit int) []Play {
	h := l.file.Get().History
	if limit > 0 && len(h) > limit {
		h = h[:limit]
	}
	return orEmpty(h)
}

// RecentTracks returns the tracks played last, each once, at most limit.
func (l *Library) RecentTracks(limit int) []media.Track {
	out := []media.Track{}
	seen := map[string]bool{}
	for _, p := range l.file.Get().History {
		if !seen[p.Track.ID] {
			seen[p.Track.ID] = true
			out = append(out, p.Track)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

// ClearHistory forgets what the user played.
func (l *Library) ClearHistory() error {
	return l.file.Update(func(d *Data) error { d.History = nil; return nil })
}

// AddSearch records a search, moving it first if it was there.
func (l *Library) AddSearch(query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	return l.file.Update(func(d *Data) error {
		d.Searches = slices.DeleteFunc(d.Searches, func(s string) bool { return strings.EqualFold(s, query) })
		d.Searches = slices.Insert(d.Searches, 0, query)
		if len(d.Searches) > maxSearches {
			d.Searches = d.Searches[:maxSearches]
		}
		return nil
	})
}

// Searches returns the recent searches, the latest first.
func (l *Library) Searches() []string { return orEmpty(l.file.Get().Searches) }

// ClearSearches forgets the recent searches.
func (l *Library) ClearSearches() error {
	return l.file.Update(func(d *Data) error { d.Searches = nil; return nil })
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
