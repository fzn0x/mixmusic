package app

// The clips of moving covers (#569): an album's short video, which loops
// muted in place of its cover on the now playing page, in the page of
// the picture of music videos (public/video.html). Where clips come from
// waits for a choice of their source, which plugs in as a ClipSource of
// Options: until then there is none, and only the GIF covers move. A clip
// is one rendition of those its source gives, fetched whole into a store
// of its own beside the covers' (package clips), which keeps each album's
// match, a miss too, and which the stream scheme serves to the page.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/fzn0x/mixmusic/internal/clips"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

const (
	// clipBitrate bounds the renditions of a clip taken, in bits a second.
	clipBitrate = 3_000_000
	// clipFetch bounds the fetch of a clip.
	clipFetch = time.Minute
	// clipPath starts the paths of the clips the stream scheme serves.
	clipPath = "/moving-cover/"
)

// A ClipSource finds the clip of the moving cover of an album: none is
// chosen yet (#569).
type ClipSource interface {
	// Clips returns the renditions of the clip of the album q asks for, of
	// a match of q's title, its every artist, its album when it has one,
	// and its length within 3 seconds when the source tells one: none
	// for a compilation, nor a clip cut from a music video. ErrNoClip
	// tells the album has none.
	Clips(ctx context.Context, q ClipQuery) ([]ClipRendition, error)
}

// ClipQuery is the track whose album's clip a ClipSource looks for.
type ClipQuery struct {
	Title   string
	Artists []string
	Album   string
	Length  time.Duration
}

// ClipRendition is a rendition of a clip, a file fetched whole.
type ClipRendition struct {
	URL string
	// Mime is its type, as video/mp4, and Codec the family of its codec:
	// "avc1" (H.264) or "hvc1" (HEVC).
	Mime  string
	Codec string
	// Width and Height are in pixels, Bitrate in bits a second.
	Width, Height int
	Bitrate       int
}

// ErrNoClip tells an album has no clip, or none plays here now.
var ErrNoClip = errors.New("no moving cover")

// CoverClipOptions are how MovingCover looks for a clip.
type CoverClipOptions struct {
	// Px is the width of the sleeve in pixels, which the rendition is
	// chosen for.
	Px int
	// Failed is the clip that didn't play, as the page told: a kept one
	// is dropped, and another rendition is tried, once.
	Failed string
}

// HasClips reports whether a source of clips is set: until one is, no
// album's cover moves, and the settings don't offer their store.
func (p *Player) HasClips() bool { return p.a.opts.Clips != nil }

// MovingCover returns the clip of the moving cover of t's album: the one
// kept, else the one its source gives, fetched whole and kept, unless
// data saver is on, the connection is metered or the network is gone, as
// only kept clips play then. ErrNoClip tells there is none.
func (p *Player) MovingCover(ctx context.Context, t media.Track, o CoverClipOptions) (VideoSource, error) {
	a := p.a
	key := clipKey(t)
	if key == "" || a.opts.Clips == nil {
		return VideoSource{}, ErrNoClip
	}
	var tried []string
	if o.Failed != "" {
		if m, ok := a.clips.Match(key); ok && !m.Miss() {
			tried = append(tried, m.Source)
			a.clips.Forget(key)
		}
	}
	if m, ok := a.clips.Match(key); ok {
		if m.Miss() {
			return VideoSource{}, ErrNoClip
		}
		a.clips.Played(key)
		return coverClip(m), nil
	}
	if a.settings.Get().Quality == settings.QualitySaver || a.metered.Load() || a.online != nil && a.online.Offline() {
		return VideoSource{}, ErrNoClip
	}
	rs, err := a.opts.Clips.Clips(ctx, clipQuery(t))
	switch {
	case errors.Is(err, ErrNoClip):
		a.clips.Miss(key)
		return VideoSource{}, ErrNoClip
	case err != nil:
		return VideoSource{}, err
	}
	r, ok := pickRendition(rs, o.Px, tried)
	if !ok {
		if len(tried) == 0 {
			a.clips.Miss(key)
		}
		return VideoSource{}, ErrNoClip
	}
	m, err := a.fetchClip(ctx, key, r)
	if err != nil {
		return VideoSource{}, err
	}
	return coverClip(m), nil
}

// ForgetMovingCover forgets what was found of the clip of t's album, for
// Find the moving cover again: it is looked for anew.
func (p *Player) ForgetMovingCover(t media.Track) {
	if key := clipKey(t); key != "" {
		p.a.clips.Forget(key)
	}
}

// fetchClip fetches the rendition r of the clip of the album of key,
// whole, into the store.
func (a *App) fetchClip(ctx context.Context, key string, r ClipRendition) (clips.Match, error) {
	ctx, cancel := context.WithTimeout(ctx, clipFetch)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return clips.Match{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return clips.Match{}, fmt.Errorf("the moving cover didn't come: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return clips.Match{}, fmt.Errorf("the moving cover's host answered HTTP %d", res.StatusCode)
	}
	if res.ContentLength > clips.MaxClip {
		return clips.Match{}, clips.ErrTooLarge
	}
	mime := r.Mime
	if mime == "" {
		mime = "video/mp4"
	}
	return a.clips.Keep(key, clips.Match{Mime: mime, Width: r.Width, Height: r.Height, Source: r.URL}, res.Body)
}

// coverClip is the clip of m, for the page of the picture to loop: it
// fetches it whole from the stream scheme.
func coverClip(m clips.Match) VideoSource {
	return VideoSource{Src: schemeURL(streamScheme) + strings.TrimPrefix(clipPath, "/") + m.File, MimeType: m.Mime,
		Width: m.Width, Height: m.Height, Loop: true}
}

// pickRendition chooses the rendition of a clip to fetch for a sleeve px
// pixels wide, of those its source gives but those tried: H.264, the
// narrowest at least px wide of at most clipBitrate, else the widest of
// those, and HEVC alone when there is no H.264.
func pickRendition(rs []ClipRendition, px int, tried []string) (ClipRendition, bool) {
	for _, codec := range []string{"avc1", "hvc1"} {
		var fit []ClipRendition
		for _, r := range rs {
			if r.URL != "" && r.Codec == codec && r.Bitrate <= clipBitrate && !slices.Contains(tried, r.URL) {
				fit = append(fit, r)
			}
		}
		if len(fit) == 0 {
			continue
		}
		slices.SortStableFunc(fit, func(a, b ClipRendition) int { return a.Width - b.Width })
		for _, r := range fit {
			if r.Width >= px {
				return r, true
			}
		}
		return fit[len(fit)-1], true
	}
	return ClipRendition{}, false
}

// clipQuery is what a ClipSource is asked of t.
func clipQuery(t media.Track) ClipQuery {
	q := ClipQuery{Title: t.Title, Length: time.Duration(t.DurationMs) * time.Millisecond}
	for _, a := range t.Artists {
		q.Artists = append(q.Artists, a.Name)
	}
	if t.Album != nil {
		q.Album = t.Album.Name
	}
	return q
}

// clipKey names the album of t in the store, in any script: its artists
// and its album, folded and trimmed, or its title for a single without
// one; "" for a track of no artist or no title, which has no clip.
func clipKey(t media.Track) string {
	var artists []string
	for _, a := range t.Artists {
		if n := fold(a.Name); n != "" {
			artists = append(artists, n)
		}
	}
	album := ""
	if t.Album != nil {
		album = fold(t.Album.Name)
	}
	if album == "" {
		album = "single:" + fold(t.Title)
	}
	if len(artists) == 0 || album == "single:" {
		return ""
	}
	slices.Sort(artists)
	return strings.Join(artists, ",") + "|" + album
}

// fold is s in lower case, its spaces as one: names as services write
// them differently.
func fold(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), unicode.IsSpace), " ")
}

// clipName is the name of the file of a clip the stream scheme asks for,
// by the path of its address: "" when it names none.
func clipName(p string) string {
	name, ok := strings.CutPrefix(p, clipPath)
	if !ok || name == "" || name != path.Base(name) || strings.ContainsAny(name, `\:`) {
		return ""
	}
	return name
}

// serveClip serves a clip kept to the page that loops it.
func (a *App) serveClip(w http.ResponseWriter, r *http.Request) {
	name := clipName(r.URL.Path)
	file, ok := a.clips.Path(name)
	if name == "" || !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, file)
}

// MovingCoverUsage is what the store of the clips of moving covers holds.
type MovingCoverUsage struct {
	Bytes int64
	Clips int
}

// MovingCoverUsage tells what the store of the clips of moving covers
// holds.
func (s *System) MovingCoverUsage() MovingCoverUsage {
	b, n := s.a.clips.Usage()
	return MovingCoverUsage{Bytes: b, Clips: n}
}

// ClearMovingCovers removes the clips of moving covers kept, and forgets
// which albums have one.
func (s *System) ClearMovingCovers() error { return s.a.clips.Clear() }

// clipLimit is the bytes of clips kept of the size the settings give.
func clipLimit(mb int) int64 {
	if mb < 0 {
		return -1
	}
	return int64(mb) << 20
}
