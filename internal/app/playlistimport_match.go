package app

// Looking for the songs of an import (#613): their IDs first, then the
// library, then YouTube Music, 4 searches at once at most, each starting
// 100 ms after the one before at least, so that playback, which asks the
// same services, isn't held back. The looking stops when the service
// refuses for asking too often, or after 3 failures in a row. What it
// finds by searching is kept 90 days, for the next import of the same
// songs.

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/playlistio"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

const (
	// importSearches is how many searches an import makes at once.
	importSearches = 4
	// importGap is the least time between the starts of two searches.
	importGap = 100 * time.Millisecond
	// importFailures is how many failures in a row stop the looking.
	importFailures = 3
	// importSaveEvery is how often the looking saves what it found.
	importSaveEvery = 2 * time.Second
	// importCacheName is the file of what searches found, in the cache
	// folder, which keeps importCacheKept for importCacheAge.
	importCacheName = "import-matches.json"
	importCacheKept = 20000
	importCacheAge  = 90 * 24 * time.Hour
)

// ImportServices look for the songs of an import. A nil one is the
// app's own.
type ImportServices struct {
	// Search searches YouTube Music.
	Search match.Searcher
	// Song reads a YouTube video, by its ID.
	Song func(ctx context.Context, videoID string) (media.Track, error)
	// SpotifyTrack reads a Spotify track, by its ID, while the user is
	// signed in to Spotify.
	SpotifyTrack func(ctx context.Context, id string) (media.Track, error)
}

// UseServices has the imports look for songs with s, in place of the
// app's services: tests, which use no network, give theirs.
func (i *Imports) UseServices(s ImportServices) {
	im := &i.a.imports
	im.mu.Lock()
	im.services = &s
	im.mu.Unlock()
}

// importServices are the services the looking uses: the tests', else the
// app's, which read a Spotify track only while signed in. It holds im.mu.
func (a *App) importServices() ImportServices {
	if s := a.imports.services; s != nil {
		return *s
	}
	s := ImportServices{Search: a.youtube, Song: a.youtube.Song}
	if a.spotify.SignedIn() {
		s.SpotifyTrack = a.spotify.Track
	}
	return s
}

// pacer paces the requests of an import: importSearches at once, each
// starting importGap after the one before at least.
type pacer struct {
	slots chan struct{}
	mu    sync.Mutex
	next  time.Time
}

func newPacer() *pacer { return &pacer{slots: make(chan struct{}, importSearches)} }

// do runs fn in its turn, unless ctx ends first.
func (p *pacer) do(ctx context.Context, fn func() error) error {
	select {
	case p.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-p.slots }()
	p.mu.Lock()
	now := time.Now()
	start := now
	if p.next.After(now) {
		start = p.next
	}
	p.next = start.Add(importGap)
	p.mu.Unlock()
	if wait := start.Sub(now); wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fn()
}

// pacedSearch searches in its turn.
type pacedSearch struct {
	p *pacer
	s match.Searcher
}

func (ps pacedSearch) Search(ctx context.Context, query string, filter youtube.Filter) ([]media.Item, error) {
	var items []media.Item
	err := ps.p.do(ctx, func() error {
		var err error
		items, err = ps.s.Search(ctx, query, filter)
		return err
	})
	return items, err
}

// cachedMatch is what a search found of a song, and when.
type cachedMatch struct {
	Track   media.Track   `json:"track"`
	Verdict match.Verdict `json:"verdict"`
	At      time.Time     `json:"at"`
}

// importCache is what searches found, by the songs' keys (importKey).
type importCache struct {
	path    string
	mu      sync.Mutex
	matches map[string]cachedMatch
	dirty   bool
}

// importCacheOf returns the cache of the searches' finds, read at its
// first use. It holds im.mu.
func (a *App) importCacheOf() *importCache {
	im := &a.imports
	if im.cache != nil {
		return im.cache
	}
	c := &importCache{path: filepath.Join(a.opts.CacheDir, importCacheName), matches: map[string]cachedMatch{}}
	if data, err := store.ReadFile(c.path); err == nil {
		if err := json.Unmarshal(data, &c.matches); err != nil {
			c.matches = map[string]cachedMatch{}
		}
	}
	im.cache = c
	return c
}

func (c *importCache) get(key string, now time.Time) (cachedMatch, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.matches[key]
	if !ok || now.Sub(m.At) > importCacheAge {
		return cachedMatch{}, false
	}
	return m, true
}

func (c *importCache) put(key string, m cachedMatch) {
	c.mu.Lock()
	c.matches[key] = m
	c.dirty = true
	c.mu.Unlock()
}

// save writes the cache, without what's older than importCacheAge, and
// the newest importCacheKept at most.
func (c *importCache) save(now time.Time) {
	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()
		return
	}
	c.dirty = false
	type kept struct {
		key string
		m   cachedMatch
	}
	var all []kept
	for k, m := range c.matches {
		if now.Sub(m.At) <= importCacheAge {
			all = append(all, kept{k, m})
		}
	}
	if len(all) > importCacheKept {
		slices.SortFunc(all, func(x, y kept) int { return y.m.At.Compare(x.m.At) })
		all = all[:importCacheKept]
	}
	c.matches = make(map[string]cachedMatch, len(all))
	for _, k := range all {
		c.matches[k.key] = k.m
	}
	data, err := json.Marshal(c.matches)
	c.mu.Unlock()
	if err == nil {
		err = store.WriteFile(c.path, data)
	}
	if err != nil {
		log.Printf("mixmusic: saving %s: %v", importCacheName, err)
	}
}

// importKey is what the cache knows a song by: its title, its artists and
// its length, to the second.
func importKey(e playlistio.Entry) string {
	return match.TitleKey(e.Title) + "\x00" + strings.ToLower(strings.Join(e.Artists, ", ")) + "\x00" +
		e.Duration.Round(time.Second).String()
}

// importLibrary is the library as the looking reads it: the tracks by
// their titles' keys and their ISRCs, the files on this computer by their
// paths and names, and the IDs of what the library holds.
type importLibrary struct {
	byTitle map[string][]media.Track
	byISRC  map[string]media.Track
	files   map[string]media.Track
	names   map[string][]media.Track
	ids     map[string]bool
}

// importLibraryOf reads the library for the looking.
func (a *App) importLibraryOf() *importLibrary {
	lib := &importLibrary{byTitle: map[string][]media.Track{}, byISRC: map[string]media.Track{},
		files: map[string]media.Track{}, names: map[string][]media.Track{}, ids: a.library.TrackIDs()}
	add := func(t media.Track) {
		key := match.TitleKey(t.Title)
		lib.byTitle[key] = append(lib.byTitle[key], t)
		if t.ISRC != "" {
			lib.byISRC[t.ISRC] = t
		}
	}
	for _, t := range a.library.Tracks() {
		add(t)
	}
	if a.localFiles != nil {
		for _, t := range a.localFiles.Tracks() {
			add(t)
			lib.ids[t.ID] = true
			if p, ok := localfiles.PathOf(t.ID); ok {
				lib.files[pathKey(p)] = t
				name := pathKey(playlistio.PathName(p))
				lib.names[name] = append(lib.names[name], t)
			}
		}
	}
	return lib
}

// pathKey is a path as paths are compared: the same after Unicode's
// normalization, as macOS keeps names decomposed, and of either case.
func pathKey(p string) string {
	if !strings.Contains(p, "/") && !strings.Contains(p, `\`) {
		return strings.ToLower(norm.NFC.String(p))
	}
	return strings.ToLower(norm.NFC.String(filepath.Clean(p)))
}

// startImportRun starts looking for the songs not looked for yet. It
// holds im.mu.
func (a *App) startImportRun() {
	im := &a.imports
	if im.run != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &importRun{cancel: cancel, done: make(chan struct{})}
	im.run, im.stopping = run, ""
	svc := a.importServices()
	cache := a.importCacheOf()
	go a.runImport(ctx, run, svc, cache)
}

// stopImportRun stops the looking, if any, and waits for it to end: why
// is "stop", for the review, "cancel", or "quit", which keeps it for the
// next start.
func (a *App) stopImportRun(why string) {
	im := &a.imports
	im.mu.Lock()
	run := im.run
	if run != nil {
		im.stopping = why
		run.cancel()
	}
	im.mu.Unlock()
	if run != nil {
		<-run.done
	}
}

// stopImport stops the looking as the app closes: it goes on at the next
// start.
func (a *App) stopImport() { a.stopImportRun("quit") }

// resumeImport goes on looking for the songs of the import the last run
// left, as MixMusic starts.
func (a *App) resumeImport() {
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	if j := im.data.Job; j != nil && j.Step == ImportMatching {
		a.startImportRun()
		a.importChanged()
	}
}

// importTask is a song to look for: where it is in the import, what its
// file says of it, and whether its list is MixMusic's own.
type importTask struct {
	list, row int
	entry     playlistio.Entry
	own       bool
}

// runImport looks for the songs of the lists picked that aren't looked
// for yet, importSearches at once, until all are, or the looking stops.
func (a *App) runImport(ctx context.Context, run *importRun, svc ImportServices, cache *importCache) {
	im := &a.imports
	defer close(run.done)
	lib := a.importLibraryOf()
	p := newPacer()
	finder := match.New(pacedSearch{p, svc.Search}, nil)

	im.mu.Lock()
	j := im.data.Job
	var tasks []importTask
	if j != nil {
		for k, l := range j.Lists {
			if !l.Picked {
				continue
			}
			for n, r := range l.Rows {
				if r.State == ImportPending {
					tasks = append(tasks, importTask{k, n, r.Entry, l.Own})
				}
			}
		}
	}
	im.mu.Unlock()

	queue := make(chan importTask)
	var wg sync.WaitGroup
	failures := 0
	for range importSearches {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range queue {
				row, err := a.findImportRow(ctx, svc, p, finder, cache, lib, t.own, t.entry)
				im.mu.Lock()
				switch {
				case ctx.Err() != nil:
				case err != nil:
					failures++
					log.Printf("mixmusic: looking for %s of an import: %v", t.entry, err)
					if halt := haltOf(err, failures); halt != "" && im.stopping == "" {
						if j := im.data.Job; j != nil {
							j.Halt, j.Cause = halt, err.Error()
						}
						im.stopping = "halt"
						run.cancel()
					}
				default:
					failures = 0
					if j := im.data.Job; j != nil && t.list < len(j.Lists) && t.row < len(j.Lists[t.list].Rows) {
						j.Lists[t.list].Rows[t.row] = row
						if time.Since(im.saved) >= importSaveEvery {
							a.importSave()
							go cache.save(time.Now())
						}
						a.importChanged()
					}
				}
				im.mu.Unlock()
			}
		}()
	}
feed:
	for _, t := range tasks {
		select {
		case queue <- t:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()
	cache.save(time.Now())

	im.mu.Lock()
	defer im.mu.Unlock()
	im.run = nil
	j = im.data.Job
	switch {
	case j == nil || im.stopping == "cancel":
	case im.stopping == "quit":
		j.Step = ImportMatching
		a.importSave()
	case im.stopping == "halt":
		j.Step = ImportHalted
		a.importSave()
	default:
		j.Step = ImportReview
		a.importSave()
	}
	im.stopping = ""
	a.importChanged()
}

// haltOf is why a failure stops the looking: the service refused for
// asking too often, or failures in a row; "" when it goes on.
func haltOf(err error, failures int) string {
	var yt *youtube.APIError
	var sp *spotify.APIError
	switch {
	case errors.As(err, &yt) && yt.Status == http.StatusTooManyRequests,
		errors.As(err, &sp) && sp.Status == http.StatusTooManyRequests:
		return haltRate
	case failures >= importFailures:
		return haltErrors
	}
	return ""
}

// findImportRow finds the track of a song an import names: a track of
// MixMusic's own lists as it is; a file on this computer; a YouTube video
// or a Spotify track by its ID; else the song by its title and artists,
// in the library, then on YouTube Music. Of own lists, nothing is asked
// of the network.
func (a *App) findImportRow(ctx context.Context, svc ImportServices, p *pacer, finder *match.Matcher, cache *importCache,
	lib *importLibrary, own bool, e playlistio.Entry) (ImportRow, error) {
	row := ImportRow{Entry: e}
	found := func(t media.Track, v match.Verdict) (ImportRow, error) {
		t = a.asSong(t)
		row.Track = &t
		row.InLibrary = lib.ids[t.ID] || t.Source == media.SourceFile
		switch v {
		case match.Sure:
			row.State, row.Ticked = ImportSure, true
		case match.ToCheck:
			row.State = ImportCheck
		default:
			row.State, row.Track, row.InLibrary = ImportMissing, nil, false
		}
		return row, nil
	}
	if own && e.MixMusicID != "" {
		if t, ok := a.ownImportTrack(lib, e); ok {
			return found(t, match.Sure)
		}
	}
	if e.Path != "" {
		if t, ok := a.importPath(lib, e.Path); ok {
			return found(t, match.Sure)
		}
	}
	switch {
	case e.VideoID != "" && own && e.Title != "":
		return found(entryTrack(e, media.ID(media.SourceYouTube, e.VideoID)), match.Sure)
	case e.VideoID != "" && svc.Song == nil:
		return found(media.Track{}, match.NotFound)
	case e.VideoID != "":
		var t media.Track
		err := p.do(ctx, func() error {
			var err error
			t, err = svc.Song(ctx, e.VideoID)
			return err
		})
		switch {
		case errors.Is(err, youtube.ErrNotFound):
			return found(media.Track{}, match.NotFound)
		case err != nil:
			return row, err
		}
		return found(t, match.Sure)
	case e.SpotifyID != "" && e.Title != "":
		return found(entryTrack(e, media.ID(media.SourceSpotify, e.SpotifyID)), match.Sure)
	case e.SpotifyID != "" && svc.SpotifyTrack != nil:
		var t media.Track
		err := p.do(ctx, func() error {
			var err error
			t, err = svc.SpotifyTrack(ctx, e.SpotifyID)
			return err
		})
		if err != nil {
			return row, err
		}
		return found(t, match.Sure)
	}
	if e.Title == "" {
		return found(media.Track{}, match.NotFound)
	}
	if e.ISRC != "" {
		if t, ok := lib.byISRC[e.ISRC]; ok {
			return found(t, match.Sure)
		}
	}
	key := importKey(e)
	if c, ok := cache.get(key, time.Now()); ok {
		return found(c.Track, c.Verdict)
	}
	f, err := finder.Find(ctx, entryTrack(e, ""), match.FindOptions{Library: lib.byTitle[match.TitleKey(e.Title)]})
	if err != nil {
		return row, err
	}
	if f.Verdict != match.NotFound && !f.Library {
		cache.put(key, cachedMatch{Track: f.Track, Verdict: f.Verdict, At: time.Now()})
	}
	return found(f.Track, f.Verdict)
}

// entryTrack is the track an entry names, of the ID id, as its file
// tells it: a song to look for when id is "".
func entryTrack(e playlistio.Entry, id string) media.Track {
	t := media.Track{ID: id, Title: e.Title, DurationMs: int(e.Duration.Milliseconds()), ISRC: e.ISRC, Artwork: []media.Image{}}
	if source, native, err := media.ParseID(id); err == nil {
		t.Source = source
		switch source {
		case media.SourceYouTube:
			t.URL = "https://music.youtube.com/watch?v=" + native
			t.Artwork = []media.Image{{URL: "https://i.ytimg.com/vi/" + native + "/hqdefault.jpg", Width: 480, Height: 360}}
		case media.SourceSpotify:
			t.URL = "https://open.spotify.com/track/" + native
		}
	}
	for _, name := range e.Artists {
		t.Artists = append(t.Artists, media.Ref{Name: name})
	}
	if t.Artists == nil {
		t.Artists = []media.Ref{}
	}
	if e.Album != "" {
		t.Album = &media.Ref{Name: e.Album}
	}
	return t
}

// ownImportTrack is the track of an entry of MixMusic's own lists, by its
// ID: a file on this computer while it's there.
func (a *App) ownImportTrack(lib *importLibrary, e playlistio.Entry) (media.Track, bool) {
	source, _, err := media.ParseID(e.MixMusicID)
	if err != nil {
		return media.Track{}, false
	}
	if source == media.SourceFile {
		p, ok := localfiles.PathOf(e.MixMusicID)
		if !ok {
			return media.Track{}, false
		}
		return a.importPath(lib, p)
	}
	return entryTrack(e, e.MixMusicID), true
}

// importPath is the track of a file a list names: as the music on this
// computer has it, by its path or, failing that, by its name, of one file
// of that name alone; else the file, read, while it's there. A path of
// another computer, or one the sandbox of the Flatpak can't read, is
// found by its words instead.
func (a *App) importPath(lib *importLibrary, p string) (media.Track, bool) {
	if strings.HasPrefix(strings.ToLower(p), "file:") {
		p = playlistio.ResolvePath("", p)
	}
	if t, ok := lib.files[pathKey(p)]; ok {
		return t, true
	}
	if a.localFiles != nil && filepath.IsAbs(p) && localfiles.Plays(p) {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			if e, err := a.localFiles.ReadFile(p); err == nil {
				return localfiles.TrackOf(e), true
			}
		}
	}
	if ts := lib.names[pathKey(playlistio.PathName(p))]; len(ts) == 1 {
		return ts[0], true
	}
	return media.Track{}, false
}

// Search looks for a track to replace the one an import found: the
// library's, then YouTube Music's songs and videos.
func (i *Imports) Search(ctx context.Context, query string) ([]media.Track, error) {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	svc := a.importServices()
	im.mu.Unlock()
	var out []media.Track
	seen := map[string]bool{}
	add := func(t media.Track) {
		if !seen[t.ID] {
			seen[t.ID] = true
			out = append(out, t)
		}
	}
	words := strings.Fields(strings.ToLower(norm.NFC.String(query)))
	for _, t := range a.library.Tracks() {
		text := strings.ToLower(norm.NFC.String(t.Title + " " + t.ArtistNames()))
		if len(words) > 0 && !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(text, w) }) {
			add(t)
		}
		if len(out) >= 5 {
			break
		}
	}
	if a.localFiles != nil {
		for _, t := range a.localFiles.Search(query, 5).Tracks {
			add(t)
		}
	}
	var errs []error
	for _, filter := range []youtube.Filter{youtube.FilterSongs, youtube.FilterVideos} {
		items, err := svc.Search.Search(ctx, query, filter)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, it := range items {
			if it.Kind == media.ItemTrack && it.Track != nil {
				add(*it.Track)
			}
		}
	}
	if len(out) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return out, nil
}
