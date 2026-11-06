package app

// New releases: what the artists the user follows put out lately, in
// MixMusic or on their services, for Home and Library › Artists. Each
// artist's releases are asked of their service at most every
// releasesFresh, a few artists at once and a bounded number in a look, and
// kept between runs in releases.json in the cache folder.

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
)

// These are how long what was read of an artist's releases holds, and how
// long a release stays new; tests change them.
var (
	// releasesFresh is how long an artist's releases hold, and
	// releasesRetry how long an artist whose releases failed waits.
	releasesFresh = 12 * time.Hour
	releasesRetry = time.Hour
	// releasesNewFor is how long a release is new after the last day it
	// may have come out on: a release known by its year alone is new all
	// that year.
	releasesNewFor = 8 * 7 * 24 * time.Hour
	// releasesKeptFor is how long after its last day a release is kept in
	// the file: one older is never new again.
	releasesKeptFor = 366 * 24 * time.Hour
	// releaseWait bounds the reading of one artist's releases.
	releaseWait = 20 * time.Second
)

const (
	// releasesPerLook is how many artists a look asks of their services
	// at most, the ones never asked first, then those asked longest ago:
	// the others wait for the next look. releasesAtOnce are asked at once.
	releasesPerLook = 30
	releasesAtOnce  = 3
	// releasesPerArtist is how many of an artist's new releases show at
	// most, their newest, and releasesShown how many in all.
	releasesPerArtist = 4
	releasesShown     = 30
)

// artistReleases are an artist's releases as their service last listed
// them, those of the last year.
type artistReleases struct {
	Artist media.Ref `json:"artist"`
	// Asked is when the service was last asked, and Read when it last
	// answered: before Asked, the ask after it failed.
	Asked    time.Time       `json:"asked"`
	Read     time.Time       `json:"read,omitzero"`
	Releases []media.Release `json:"releases"`
}

// releaseBook keeps the releases of the artists the user follows, by
// artist ID, in releases.json in the cache folder, which it opens at its
// first use. Its methods are safe for concurrent use.
type releaseBook struct {
	once sync.Once
	file *store.File[map[string]artistReleases]
	// looking orders the looks: two at once would ask twice.
	looking sync.Mutex
	// read reads an artist's releases off their service, and now tells the
	// time: App's, unless tests set them.
	read func(ctx context.Context, artistID string) ([]media.Release, error)
	now  func() time.Time
}

// open returns the file of releases, nil when it can't be read, or an app
// without a cache folder has none.
func (b *releaseBook) open(a *App) *store.File[map[string]artistReleases] {
	b.once.Do(func() {
		if a.opts.CacheDir == "" {
			return
		}
		f, err := store.Open(filepath.Join(a.opts.CacheDir, "releases.json"), map[string]artistReleases{})
		if err != nil {
			log.Printf("mixmusic: opening the releases of the artists followed: %v", err)
			return
		}
		b.file = f
		if b.read == nil {
			b.read = a.artistReleases
		}
		if b.now == nil {
			b.now = time.Now
		}
	})
	return b.file
}

// NewReleases returns what the artists the user follows put out lately,
// the newest first: those they follow in MixMusic, and on Spotify and
// YouTube Music where they signed in. It asks the services for the
// releases of the artists not asked for a while (look); the others, and
// those that fail, show as last read.
func (c *Catalog) NewReleases(ctx context.Context) ([]media.Release, error) {
	followed, lost := c.a.followedArtists(ctx)
	return c.a.releases.look(ctx, c.a, followed, lost), nil
}

// KnownReleases returns NewReleases as last read, without asking anything:
// of the artists followed at the last look.
func (c *Catalog) KnownReleases() []media.Release {
	f := c.a.releases.open(c.a)
	if f == nil {
		return []media.Release{}
	}
	entries := f.Get()
	return newReleases(entries, slices.Sorted(maps.Keys(entries)), c.a.releases.now())
}

// forget forgets the releases of an artist the user stopped following.
func (b *releaseBook) forget(a *App, artistID string) {
	f := b.open(a)
	if f == nil {
		return
	}
	if _, ok := f.Get()[artistID]; !ok {
		return
	}
	if err := f.Update(func(m *map[string]artistReleases) error { delete(*m, artistID); return nil }); err != nil {
		log.Printf("mixmusic: forgetting an artist's releases: %v", err)
	}
}

// look asks the services for the releases of the artists followed that
// weren't asked for a while, at most releasesPerLook, releasesAtOnce at
// a time, keeps what they answer with those of the other artists, and
// returns the new ones. An artist that fails keeps what was read before,
// and waits releasesRetry. The file forgets the artists no longer
// followed, but those of the services in lost, whose lists didn't read.
func (b *releaseBook) look(ctx context.Context, a *App, followed []media.Artist, lost map[media.Source]bool) []media.Release {
	f := b.open(a)
	if f == nil {
		return []media.Release{}
	}
	b.looking.Lock()
	defer b.looking.Unlock()
	now := b.now()
	entries := f.Get()
	var due []media.Artist
	for _, ar := range followed {
		e, ok := entries[ar.ID]
		switch {
		case !ok, e.Read.Before(e.Asked) && now.Sub(e.Asked) >= releasesRetry, !e.Read.Before(e.Asked) && now.Sub(e.Read) >= releasesFresh:
			due = append(due, ar)
		}
	}
	// Those never asked first, then those asked longest ago.
	slices.SortStableFunc(due, func(x, y media.Artist) int { return entries[x.ID].Asked.Compare(entries[y.ID].Asked) })
	due = due[:min(len(due), releasesPerLook)]

	read := make([]*artistReleases, len(due))
	slots := make(chan struct{}, releasesAtOnce)
	var wg sync.WaitGroup
	for i, ar := range due {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			if ctx.Err() != nil {
				return
			}
			actx, cancel := context.WithTimeout(ctx, releaseWait)
			defer cancel()
			releases, err := b.read(actx, ar.ID)
			if err != nil && ctx.Err() != nil {
				return // the look ended, not the artist
			}
			e := entries[ar.ID]
			e.Artist, e.Asked = media.Ref{ID: ar.ID, Name: ar.Name}, now
			if err != nil {
				log.Printf("mixmusic: reading the releases of %s: %v", ar.ID, err)
			} else {
				e.Read, e.Releases = now, recent(releases, now)
			}
			read[i] = &e
		})
	}
	wg.Wait()

	// The artists of a service whose list didn't read are taken as still
	// followed, after the others.
	var ids []string
	following := map[string]bool{}
	for _, ar := range followed {
		ids = append(ids, ar.ID)
		following[ar.ID] = true
	}
	changed := slices.ContainsFunc(read, func(e *artistReleases) bool { return e != nil })
	for _, id := range slices.Sorted(maps.Keys(entries)) {
		source, _, _ := media.ParseID(id)
		switch {
		case following[id]:
		case lost[source]:
			ids = append(ids, id)
		default:
			changed = true
		}
	}
	if changed {
		err := f.Update(func(m *map[string]artistReleases) error {
			for id := range *m {
				if source, _, _ := media.ParseID(id); !following[id] && !lost[source] {
					delete(*m, id)
				}
			}
			for _, e := range read {
				if e != nil {
					(*m)[e.Artist.ID] = *e
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("mixmusic: keeping the releases of the artists followed: %v", err)
		}
	}
	return newReleases(f.Get(), ids, now)
}

// recent returns the releases that came out in the last year, which may
// still be new, or later.
func recent(releases []media.Release, now time.Time) []media.Release {
	out := []media.Release{}
	for _, r := range releases {
		if _, last, ok := r.Span(); ok && now.Sub(last) < releasesKeptFor {
			r.Album.Tracks = []media.Track{}
			out = append(out, r)
		}
	}
	return out
}

// newReleases returns the releases of the artists ids, as entries hold
// them, that are new at now: out since the day releasesNewFor before, at
// most releasesPerArtist of an artist, releasesShown in all, each once.
// They come newest first, by the first day they may have come out on, as
// the first of its year for a release known by its year alone; those of a
// day come by their places among their artists' new releases, then by
// their artists' names.
func newReleases(entries map[string]artistReleases, ids []string, now time.Time) []media.Release {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	since := today.Add(-releasesNewFor)
	type ranked struct {
		r     media.Release
		first time.Time
		place int
	}
	var all []ranked
	seen := map[string]bool{}
	for _, id := range ids {
		e, ok := entries[id]
		if !ok {
			continue
		}
		var mine []ranked
		for _, r := range e.Releases {
			first, last, ok := r.Span()
			if ok && !last.Before(since) && !first.After(today) {
				r.Of = e.Artist
				mine = append(mine, ranked{r: r, first: first})
			}
		}
		slices.SortStableFunc(mine, func(x, y ranked) int { return y.first.Compare(x.first) })
		for i, x := range mine[:min(len(mine), releasesPerArtist)] {
			if !seen[x.r.Album.ID] {
				seen[x.r.Album.ID] = true
				x.place = i
				all = append(all, x)
			}
		}
	}
	slices.SortStableFunc(all, func(x, y ranked) int {
		return cmp.Or(y.first.Compare(x.first), cmp.Compare(x.place, y.place), cmp.Compare(x.r.Of.Name, y.r.Of.Name),
			cmp.Compare(x.r.Album.ID, y.r.Album.ID))
	})
	out := make([]media.Release, 0, min(len(all), releasesShown))
	for _, x := range all[:min(len(all), releasesShown)] {
		out = append(out, x.r)
	}
	return out
}

// followedArtists returns the artists the user follows, each once: in
// MixMusic, then on Spotify and on YouTube Music, where they signed in.
// lost holds the services whose lists failed.
func (a *App) followedArtists(ctx context.Context) (followed []media.Artist, lost map[media.Source]bool) {
	lists := []struct {
		source media.Source
		on     bool
		get    func(ctx context.Context) ([]media.Artist, error)
		got    []media.Artist
		err    error
	}{
		{source: media.SourceSpotify, on: a.spotify.SignedIn(), get: a.spotify.FollowedArtists},
		{source: media.SourceYouTube, on: a.youtube.SignedIn(), get: a.youtube.LibraryArtists},
	}
	var wg sync.WaitGroup
	for i := range lists {
		if l := &lists[i]; l.on {
			wg.Go(func() { l.got, l.err = l.get(ctx) })
		}
	}
	wg.Wait()
	followed, lost = a.library.Artists(), map[media.Source]bool{}
	for _, l := range lists {
		if l.err != nil {
			log.Printf("mixmusic: reading the artists followed on %s: %v", l.source, l.err)
			lost[l.source] = true
		}
		followed = append(followed, l.got...)
	}
	seen := map[string]bool{}
	return slices.DeleteFunc(followed, func(ar media.Artist) bool {
		dup := seen[ar.ID]
		seen[ar.ID] = true
		return dup
	}), lost
}

// artistReleases reads an artist's releases off their service: YouTube
// Music's page of the artist, which tells the year of each, or Spotify's
// list of their albums, which tells their days.
func (a *App) artistReleases(ctx context.Context, artistID string) ([]media.Release, error) {
	source, native, err := media.ParseID(artistID)
	if err != nil {
		return nil, err
	}
	switch source {
	case media.SourceYouTube:
		page, err := a.youtube.Artist(ctx, native)
		if err != nil {
			return nil, err
		}
		out := []media.Release{}
		for _, al := range slices.Concat(page.Albums, page.Singles) {
			out = append(out, media.Release{Album: al, Released: al.Year})
		}
		return out, nil
	case media.SourceSpotify:
		if !a.spotify.SignedIn() {
			return nil, errNotSignedIn
		}
		return a.spotify.ArtistReleases(ctx, native)
	}
	return nil, fmt.Errorf("artists of %s have no releases to read", source)
}
