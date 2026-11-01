// Package downloads keeps the tracks the user chooses to play without the
// network, apart from the offline cache, which keeps what played lately
// and drops it as it fills: a folder of their audio and covers, with an
// index of what it holds. Two workers download what waits, in the order
// it was asked for, going on where a download stopped, after a restart
// too, and check each file before keeping it. Downloads wait while the
// network is gone, on a metered connection unless allowed, while YouTube
// turns requests down, and on a full disk.
package downloads

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// Fetcher downloads the audio of videos: *stream.Proxy.
type Fetcher interface {
	Download(ctx context.Context, videoID string, resume func(s stream.Stream) (int64, error), w io.Writer, progress func(done, size int64)) (stream.Stream, error)
}

// Matcher finds the recording of a Spotify track on YouTube Music:
// *match.Matcher.
type Matcher interface {
	Match(ctx context.Context, t media.Track) (media.Track, error)
}

// Cache is the offline cache, which hands over the tracks it holds:
// *stream.Cache.
type Cache interface {
	Find(videoID string) (path, mime string, ok bool)
	Take(videoID, dst string) (bool, error)
}

// Network tells whether the network answers: *online.Watch.
type Network interface {
	Check(ctx context.Context) bool
	Wait(ctx context.Context) error
}

// Options are what a Manager downloads with. Each may be nil, but Fetcher.
type Options struct {
	Fetcher Fetcher
	Matcher Matcher
	Cache   Cache
	Network Network
	// Art returns the bytes of a cover, as its host sent them.
	Art func(ctx context.Context, url string) ([]byte, error)
	// Avoid are folders the downloads never move into, as the cache's.
	Avoid []string
}

// These are the times the manager keeps; tests shorten them.
var (
	// retryWaits are the waits before each try again of a download that
	// failed for a reason that may pass: once they ran out, it failed.
	retryWaits = []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute}
	// throttleFor is how long downloads wait once YouTube turned one down.
	throttleFor = 15 * time.Minute
	// diskFullFor is how long downloads wait on a full disk before trying
	// again; failingFor how long after failingAfter failures in a row.
	diskFullFor  = 10 * time.Minute
	failingFor   = 10 * time.Minute
	failingAfter = 5
	// startGap spaces out the downloads asking YouTube for their streams,
	// so that a batch of hundreds doesn't look like a bot to it, nor take
	// the streams playback needs.
	startGap = 3 * time.Second
	// saveAfter is how long the index waits to be written after a change.
	saveAfter = 2 * time.Second
	// tellEvery is how often watchers are told of progress, at most.
	tellEvery = 250 * time.Millisecond
	// busyWait is how long a track waits whose audio another worker
	// downloads.
	busyWait = 2 * time.Second
)

// workers is how many tracks download at once.
const workers = 2

// The holds, which keep downloads from starting; those of holdCancels stop
// the downloads under way too, whose parts stay.
const (
	holdPaused    = "paused"
	holdOffline   = "offline"
	holdMetered   = "metered"
	holdThrottled = "throttled"
	holdDisk      = "disk"
	holdFolder    = "folder"
	holdFailing   = "failing"
	holdMoving    = "moving"
)

// holdOrder is the order in which holds tell why downloads wait: the first
// that holds.
var holdOrder = []string{holdFolder, holdMoving, holdPaused, holdOffline, holdMetered, holdDisk, holdThrottled, holdFailing}

func holdCancels(kind string) bool {
	return kind == holdPaused || kind == holdOffline || kind == holdFolder || kind == holdMoving || kind == holdMetered
}

// hold keeps downloads from starting, until until, if set.
type hold struct {
	why   string
	until time.Time
}

// Manager keeps the downloads of a folder. Its methods are safe for
// concurrent use, and those of a nil Manager tell that it holds nothing.
type Manager struct {
	opts Options

	mu  sync.Mutex
	dir string
	idx *index
	// err is why the folder can't be used, or nil.
	err   error
	holds map[string]hold
	jobs  map[string]*job // the downloads under way, by track ID
	// busy are the videos being downloaded, by ID.
	busy map[string]bool
	// tries counts the tries of each track since it last downloaded, and
	// nextAt is when it may try again.
	tries  map[string]int
	nextAt map[string]time.Time
	// failing counts the failures in a row, of any track.
	failing int
	// batchDone counts the tracks done since the queue was last empty.
	batchDone int
	// kick is closed, and replaced, to have the workers look again.
	kick chan struct{}
	// startAt is when the next download may ask YouTube for its stream.
	startAt time.Time
	// saveTimer writes the index a moment after a change.
	saveTimer *time.Timer
	dirty     bool
	started   bool
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	snap atomic.Pointer[Snapshot]
	seq  uint64 // the snapshots' versions

	watchMu  sync.Mutex
	watchers []*func()
	toldAt   time.Time
	telling  bool
}

// Open opens the downloads kept in dir, making it when it doesn't exist. A
// folder that can't be used, as on a drive unplugged, holds the downloads,
// and Status tells why; tracks stream as before meanwhile.
func Open(dir string, opts Options) *Manager {
	m := &Manager{opts: opts, dir: dir, holds: map[string]hold{}, jobs: map[string]*job{}, busy: map[string]bool{},
		tries: map[string]int{}, nextAt: map[string]time.Time{}, kick: make(chan struct{})}
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.idx = newIndex()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.err = err
		m.holds[holdFolder] = hold{why: "The folder of your downloads can't be reached: " + dir}
	} else if idx, _, err := readIndex(dir); err != nil {
		m.err = err
		m.holds[holdFolder] = hold{why: "The downloads in " + dir + " can't be read: " + err.Error()}
	} else {
		m.idx = idx
		if idx.Paused {
			m.holds[holdPaused] = hold{why: "Paused"}
		}
	}
	m.mu.Lock()
	m.publish()
	m.mu.Unlock()
	return m
}

// Start starts the workers, once the folder was checked: files that went
// missing download again, and what couldn't be deleted before goes.
func (m *Manager) Start() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.check()
		for range workers {
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				m.work()
			}()
		}
	}()
}

// Close stops the downloads under way, which go on from their parts at the
// next start, and writes the index.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.cancel()
	m.wg.Wait()
	m.Flush()
}

// Flush writes the index now, if it changed.
func (m *Manager) Flush() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeNow()
}

// check deletes what couldn't be deleted before, and the parts no track
// waits for, and has the tracks whose files went missing download again.
func (m *Manager) check() {
	m.mu.Lock()
	if m.err != nil {
		m.mu.Unlock()
		return
	}
	dir := m.dir
	pending := m.idx.Pending
	files := map[string]string{}
	for video, f := range m.idx.Files {
		files[video] = f.Path
	}
	wanted := map[string]bool{}
	for _, e := range m.idx.Tracks {
		if v := e.video(); v != "" {
			wanted[v] = true
		}
	}
	m.mu.Unlock()

	var left []string
	for _, p := range pending {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(p))); err != nil && !errors.Is(err, os.ErrNotExist) {
			left = append(left, p)
		}
	}
	var gone []string
	for video, p := range files {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); errors.Is(err, os.ErrNotExist) {
			gone = append(gone, video)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "audio")); err == nil {
		for _, e := range entries {
			name := e.Name()
			if video, ok := strings.CutSuffix(strings.TrimPrefix(name, "."), ".part"); ok && strings.HasPrefix(name, ".") && !wanted[video] {
				os.Remove(filepath.Join(dir, "audio", name))
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dir != dir {
		return // moved meanwhile
	}
	m.idx.Pending = left
	for _, video := range gone {
		log.Printf("mixmusic: the download of %s went missing; downloading it again", video)
		delete(m.idx.Files, video)
	}
	if len(gone) > 0 || len(left) != len(pending) {
		m.changed()
	}
}

// Folder returns the folder the downloads are in.
func (m *Manager) Folder() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dir
}

// Watch has fn told, from any goroutine, when what the downloads hold or
// do changes, and of their progress a few times a second, until cancel.
func (m *Manager) Watch(fn func()) (cancel func()) {
	if m == nil {
		return func() {}
	}
	f := &fn
	m.watchMu.Lock()
	m.watchers = append(slices.Clone(m.watchers), f)
	m.watchMu.Unlock()
	return func() {
		m.watchMu.Lock()
		m.watchers = slices.DeleteFunc(slices.Clone(m.watchers), func(g *func()) bool { return g == f })
		m.watchMu.Unlock()
	}
}

// tell tells the watchers at once.
func (m *Manager) tell() {
	m.watchMu.Lock()
	m.toldAt = time.Now()
	fns := m.watchers
	m.watchMu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
}

// tellSoon tells the watchers, tellEvery at most, beside its caller, which
// may hold mu: a watcher may ask the manager what changed.
func (m *Manager) tellSoon() {
	m.watchMu.Lock()
	if m.telling {
		m.watchMu.Unlock()
		return
	}
	wait := max(tellEvery-time.Since(m.toldAt), 0)
	m.telling = true
	m.watchMu.Unlock()
	time.AfterFunc(wait, func() {
		m.watchMu.Lock()
		m.telling = false
		m.watchMu.Unlock()
		m.tell()
	})
}

// changed takes in a change of the index: it is written soon, the
// snapshot made again, and the watchers told. Its caller holds mu.
func (m *Manager) changed() {
	m.dirty = true
	if m.saveTimer == nil && m.err == nil {
		m.saveTimer = time.AfterFunc(saveAfter, func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			m.saveTimer = nil
			m.writeNow()
		})
	}
	m.publish()
}

// writeNow writes the index, if it changed. Its caller holds mu.
func (m *Manager) writeNow() {
	if !m.dirty || m.err != nil {
		return
	}
	if err := m.idx.write(m.dir); err != nil {
		log.Printf("mixmusic: keeping the index of the downloads: %v", err)
		return
	}
	m.dirty = false
}

// wake has the workers look for a download again. Its caller holds mu.
func (m *Manager) wake() {
	close(m.kick)
	m.kick = make(chan struct{})
}

// Add downloads tracks on their own, in quality: "best" or "saver". It
// returns how many it added; those it holds already stay as they are,
// but those that failed, which it tries again.
func (m *Manager) Add(tracks []media.Track, quality string) int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, t := range tracks {
		if m.pin(t, pinTrack, quality) {
			n++
		}
	}
	m.wake()
	m.changed()
	return n
}

// pin keeps t for pin, adding it in quality when it's new, and tells
// whether it was. A track that failed is tried again. Its caller holds mu.
func (m *Manager) pin(t media.Track, pin, quality string) bool {
	if t.ID == "" {
		return false
	}
	if e, ok := m.idx.Tracks[t.ID]; ok {
		if !e.pinned(pin) {
			e.Pins = append(e.Pins, pin)
		}
		if e.Failed != "" {
			e.Failed = ""
			delete(m.tries, t.ID)
			delete(m.nextAt, t.ID)
		}
		return false
	}
	m.idx.Seq++
	m.idx.Tracks[t.ID] = &entry{Track: t, Quality: cmp.Or(quality, "best"), Pins: []string{pin}, Seq: m.idx.Seq, Added: time.Now()}
	return true
}

// AddCollection downloads the tracks of an album, a playlist, a mix or the
// liked songs, as c. A collection downloaded before takes the tracks it
// holds now. It returns how many tracks it added.
func (m *Manager) AddCollection(c Collection, tracks []media.Track, quality string) int {
	if m == nil || c.ID == "" {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.idx.Collections[c.ID]; ok {
		c.Added, c.Art = old.Added, old.Art
	} else {
		c.Added = time.Now()
	}
	c.Tracks = make([]string, 0, len(tracks))
	n := 0
	for _, t := range tracks {
		if t.ID == "" {
			continue
		}
		c.Tracks = append(c.Tracks, t.ID)
		if m.pin(t, c.ID, quality) {
			n++
		}
	}
	if c.Total <= len(c.Tracks) {
		c.Total = 0
	}
	m.idx.Collections[c.ID] = &c
	m.wake()
	m.changed()
	return n
}

// Remove removes the downloads of tracks, whatever keeps them: their
// downloads under way stop, and the files no other track plays go.
func (m *Manager) Remove(trackIDs ...string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range trackIDs {
		m.drop(id)
	}
	m.changed()
}

// Cancel stops the download of tracks, and removes them: as Remove.
func (m *Manager) Cancel(trackIDs ...string) { m.Remove(trackIDs...) }

// CancelAll stops every download under way or waiting, and removes them;
// what downloaded stays.
func (m *Manager) CancelAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.idx.Tracks {
		if !m.done(e) {
			m.drop(id)
		}
	}
	m.changed()
}

// RemoveCollection removes the download of a collection: its tracks no
// other download keeps go.
func (m *Manager) RemoveCollection(id string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.idx.Collections[id]
	if !ok {
		return
	}
	delete(m.idx.Collections, id)
	for _, tid := range c.Tracks {
		if e, ok := m.idx.Tracks[tid]; ok {
			e.unpin(id)
			if len(e.Pins) == 0 {
				m.drop(tid)
			}
		}
	}
	m.dropArt(c.Art)
	m.changed()
}

// RemoveAll removes every download.
func (m *Manager) RemoveAll() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.idx.Tracks {
		m.drop(id)
	}
	for id, c := range m.idx.Collections {
		delete(m.idx.Collections, id)
		m.dropArt(c.Art)
	}
	m.changed()
}

// drop removes the track id, stopping its download, and deletes the files
// no other track needs. Its caller holds mu.
func (m *Manager) drop(id string) {
	e, ok := m.idx.Tracks[id]
	if !ok {
		return
	}
	delete(m.idx.Tracks, id)
	delete(m.tries, id)
	delete(m.nextAt, id)
	if j := m.jobs[id]; j != nil {
		j.cancel(errRemoved)
	}
	if video := e.video(); video != "" && !m.needed(video) && !m.busy[video] {
		m.dropVideo(video)
	}
	m.dropArt(e.Art)
}

// needed reports whether a track keeps the audio of video. Its caller
// holds mu.
func (m *Manager) needed(video string) bool {
	for _, e := range m.idx.Tracks {
		if e.video() == video {
			return true
		}
	}
	return false
}

// dropVideo deletes the audio of video, and its part. Its caller holds mu.
func (m *Manager) dropVideo(video string) {
	if f, ok := m.idx.Files[video]; ok {
		delete(m.idx.Files, video)
		m.remove(f.Path)
	}
	delete(m.idx.Parts, video)
	os.Remove(m.partPath(video))
}

// dropArt deletes a cover kept, unless a track or a collection keeps it.
// Its caller holds mu.
func (m *Manager) dropArt(p string) {
	if p == "" {
		return
	}
	for _, e := range m.idx.Tracks {
		if e.Art == p {
			return
		}
	}
	for _, c := range m.idx.Collections {
		if c.Art == p {
			return
		}
	}
	for u, q := range m.idx.Art {
		if q == p {
			delete(m.idx.Art, u)
		}
	}
	m.remove(p)
}

// remove deletes a file of the folder, or keeps it to delete at the next
// start when it can't, as one open on Windows. Its caller holds mu.
func (m *Manager) remove(p string) {
	if err := os.Remove(filepath.Join(m.dir, filepath.FromSlash(p))); err != nil && !errors.Is(err, os.ErrNotExist) {
		m.idx.Pending = append(m.idx.Pending, p)
	}
}

// Retry tries the download of a track that failed again.
func (m *Manager) Retry(id string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.idx.Tracks[id]; ok {
		e.Failed = ""
		delete(m.tries, id)
		delete(m.nextAt, id)
	}
	m.wake()
	m.changed()
}

// RetryFailed tries every download that failed again, at once, and those
// that wait to: what kept them all waiting, as YouTube turning requests
// down, may have passed.
func (m *Manager) RetryFailed() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, e := range m.idx.Tracks {
		e.Failed = ""
		delete(m.tries, id)
		delete(m.nextAt, id)
	}
	for _, kind := range []string{holdThrottled, holdDisk, holdFailing} {
		delete(m.holds, kind)
	}
	m.failing = 0
	m.wake()
	m.changed()
}

// Rematch drops the recording found for a Spotify track, and its audio
// when no other track plays it, and downloads the track again, as its new
// match: the old one was wrong.
func (m *Manager) Rematch(id string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.idx.Tracks[id]
	if !ok || e.Recording == nil {
		return
	}
	video := e.video()
	e.Recording, e.Failed = nil, ""
	if j := m.jobs[id]; j != nil {
		j.cancel(errHeld)
	}
	if video != "" && !m.needed(video) && !m.busy[video] {
		m.dropVideo(video)
	}
	m.wake()
	m.changed()
}

// SetPaused pauses the downloads, the user's way: those under way stop,
// and go on from where they were once resumed.
func (m *Manager) SetPaused(paused bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Paused = paused
	m.setHold(holdPaused, paused, "Paused", time.Time{})
	m.changed()
}

// SetMetered tells whether the connection is metered, and whether the user
// lets downloads go on over one: they wait otherwise.
func (m *Manager) SetMetered(metered, allowed bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setHold(holdMetered, metered && !allowed, "Waiting for a connection that isn't metered, as Settings › Playback asks", time.Time{})
	m.publish()
}

// setHold holds or releases the downloads for kind, until until when it's
// set; a hold that stops downloads under way stops them. Its caller holds
// mu, and publishes.
func (m *Manager) setHold(kind string, on bool, why string, until time.Time) {
	if !on {
		if _, ok := m.holds[kind]; ok {
			delete(m.holds, kind)
			m.wake()
		}
		return
	}
	m.holds[kind] = hold{why: why, until: until}
	if holdCancels(kind) {
		for _, j := range m.jobs {
			j.cancel(errHeld)
		}
	}
}

// held returns why the downloads wait, or "" when they don't, and until
// when, if known; holds that ran out go. Its caller holds mu.
func (m *Manager) held() (string, time.Time) {
	now := time.Now()
	for _, kind := range holdOrder {
		h, ok := m.holds[kind]
		if !ok {
			continue
		}
		if !h.until.IsZero() && !now.Before(h.until) {
			delete(m.holds, kind)
			if kind == holdFailing {
				m.failing = 0
			}
			continue
		}
		return h.why, h.until
	}
	return "", time.Time{}
}

// done reports whether the audio of e is downloaded. Its caller holds mu.
func (m *Manager) done(e *entry) bool {
	v := e.video()
	return v != "" && m.idx.Files[v] != nil
}

// partPath is where the audio of video downloads to.
func (m *Manager) partPath(video string) string {
	return filepath.Join(m.dir, "audio", "."+video+".part")
}

// Serve answers r with the downloaded audio of a video, and reports
// whether it had it: as stream.Store, for the proxy. A file gone missing,
// as on a drive unplugged, streams instead.
func (m *Manager) Serve(w http.ResponseWriter, r *http.Request, videoID string) bool {
	if m == nil {
		return false
	}
	path, ok := m.Snapshot().files[videoID]
	if !ok {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	mime := "audio/webm"
	if strings.HasSuffix(path, ".m4a") {
		mime = "audio/mp4"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", time.Time{}, f)
	return true
}

// Estimate is about how many bytes the audio of tracks takes, downloaded
// in quality: YouTube's streams take about 17,000 bytes a second at their
// best, and 6,500 at data saver, a track of no known length 3.5 MB or 1.3.
func Estimate(tracks []media.Track, quality string) int64 {
	rate, unknown := int64(17000), int64(3_500_000)
	if quality == "saver" {
		rate, unknown = 6500, 1_300_000
	}
	var n int64
	for _, t := range tracks {
		if t.DurationMs > 0 {
			n += int64(t.DurationMs) * rate / 1000
		} else {
			n += unknown
		}
	}
	return n
}

var (
	// errRemoved stops a download whose track was removed, and errHeld one
	// that waits, which goes on from its part.
	errRemoved = errors.New("downloads: removed")
	errHeld    = errors.New("downloads: waiting")
)
