package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The timing of a track's synced lyrics (#610): how far the listener moved
// them, earlier above 0, as an LRC [offset:] tag moves them, or later. It
// holds for the lyrics of the source it was set for alone: as the sources
// fall back, or another is picked, the lyrics of another show as timed. It
// is kept in the data folder, a file each in lyrics-offsets/, as the
// lyrics picked are in lyrics-picks/, lyricsOffsetsKept of them, the least
// used going past that, and none of a timing of 0. The presses come in
// runs, so a change is written lyricsOffsetWait after the last, and as
// MixMusic quits. Settings' lyricsOffset moves every song's besides
// (settings.Lyrics.LyricsOffset); the windows add the two.

// lyricsOffsetsKept is how many tracks' timings the data folder keeps.
const lyricsOffsetsKept = 10_000

// lyricsOffsetWait is how long a change of a timing waits to be written,
// for the next press: tests shorten it.
var lyricsOffsetWait = 3 * time.Second

// LyricsOffset returns how far the listener moved the lyrics ly of a
// track, earlier above 0: 0 while they didn't, or moved those of another
// source.
func (c *Catalog) LyricsOffset(track media.Track, ly lyrics.Lyrics) time.Duration {
	k := c.a.lyricsOffset.get(track.ID)
	if k.Ms == 0 || k.Source != lyricsOffsetSource(ly) {
		return 0
	}
	return time.Duration(k.Ms) * time.Millisecond
}

// SetLyricsOffset moves the lyrics ly of a track by d, earlier above 0, to
// the millisecond and settings.MaxLyricsOffset at most either way: 0 shows
// them as timed. It holds for the lyrics of their source alone.
func (c *Catalog) SetLyricsOffset(track media.Track, ly lyrics.Lyrics, d time.Duration) {
	c.a.lyricsOffset.set(track.ID, lyricsOffsetSource(ly), settings.CleanLyricsOffset(int(d/time.Millisecond)))
}

// MovedLyrics are lyrics with how far they show moved, as plugins get
// them.
type MovedLyrics struct {
	lyrics.Lyrics
	// OffsetMs is how far they show moved, in milliseconds, earlier above
	// 0: the track's own timing of them, and Settings' timing of every
	// song, at the track's own speed.
	OffsetMs int `json:"offsetMs"`
}

// LyricsMoved returns the lyrics of a track, as Lyrics does, with how far
// they show moved.
func (c *Catalog) LyricsMoved(ctx context.Context, track media.Track) (MovedLyrics, error) {
	l, err := c.Lyrics(ctx, track)
	if err != nil {
		return MovedLyrics{}, err
	}
	every := c.a.settings.Get().LyricsOffset
	return MovedLyrics{Lyrics: l, OffsetMs: int(c.LyricsOffset(track, l)/time.Millisecond) + every}, nil
}

// lyricsOffsetSource names the lyrics a timing holds for: their source,
// and, of captions, their language, as each caption track of a video is
// lyrics of their own.
func lyricsOffsetSource(ly lyrics.Lyrics) string {
	if ly.Source == CaptionsSource && ly.Language != "" {
		return ly.Source + ":" + ly.Language
	}
	return ly.Source
}

// keptOffset is a track's timing, as its file keeps it: the lyrics it
// holds for, and how far it moves them.
type keptOffset struct {
	Track  string `json:"track"`
	Source string `json:"source"`
	Ms     int    `json:"offsetMs"`
}

// lyricsOffsets keeps the tracks' timings in a folder, a file each, named
// by a hash of the track's ID, keep of them at most, the least used going
// first. It knows those it read or set this run, and writes those set
// wait after the last, or at flush. Its methods are safe for concurrent
// use; a nil one keeps none.
type lyricsOffsets struct {
	dir  string
	keep int
	wait time.Duration

	mu    sync.Mutex
	known map[string]keptOffset
	// pending are those set and not written yet, and timer writes them.
	pending map[string]keptOffset
	timer   *time.Timer
}

func newLyricsOffsets(dir string) *lyricsOffsets {
	return &lyricsOffsets{dir: dir, keep: lyricsOffsetsKept, wait: lyricsOffsetWait,
		known: map[string]keptOffset{}, pending: map[string]keptOffset{}}
}

// path is the file of the timing of the track trackID.
func (o *lyricsOffsets) path(trackID string) string {
	h := sha256.Sum256([]byte(trackID))
	return filepath.Join(o.dir, hex.EncodeToString(h[:16])+".json")
}

// get returns the timing of the track trackID: the zero one for none.
func (o *lyricsOffsets) get(trackID string) keptOffset {
	if o == nil {
		return keptOffset{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if k, ok := o.pending[trackID]; ok {
		return k
	}
	k, ok := o.known[trackID]
	if !ok {
		k = o.read(trackID)
		if len(o.known) >= 4096 {
			clear(o.known)
		}
		o.known[trackID] = k
	}
	return k
}

// read reads the timing of the track trackID from its file: none when
// there is none, or it doesn't read. Its caller holds o.mu.
func (o *lyricsOffsets) read(trackID string) keptOffset {
	path := o.path(trackID)
	data, err := os.ReadFile(path)
	if err != nil {
		return keptOffset{}
	}
	var k keptOffset
	if json.Unmarshal(data, &k) != nil || k.Track != trackID {
		return keptOffset{}
	}
	// Used now, it is among the last to go.
	now := time.Now()
	os.Chtimes(path, now, now)
	k.Ms = settings.CleanLyricsOffset(k.Ms)
	return k
}

// set keeps the timing ms of the lyrics of source for the track trackID,
// which is written wait after the last change.
func (o *lyricsOffsets) set(trackID, source string, ms int) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	k := keptOffset{Track: trackID, Source: source, Ms: ms}
	o.pending[trackID], o.known[trackID] = k, k
	if o.timer == nil {
		o.timer = time.AfterFunc(o.wait, o.flush)
	} else {
		o.timer.Reset(o.wait)
	}
}

// flush writes the timings set and not written yet, at once, and lets the
// least used go past those kept.
func (o *lyricsOffsets) flush() {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.timer != nil {
		o.timer.Stop()
	}
	if len(o.pending) == 0 {
		return
	}
	for id, k := range o.pending {
		if err := o.write(k); err != nil {
			log.Printf("mixmusic: keeping the timing of the lyrics of %s: %v", id, err)
		}
	}
	clear(o.pending)
	o.prune()
}

// write writes the file of a timing, or removes it for a timing of 0. Its
// caller holds o.mu.
func (o *lyricsOffsets) write(k keptOffset) error {
	path := o.path(k.Track)
	if k.Ms == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(k)
	if err != nil {
		return err
	}
	return store.WriteFile(path, data)
}

// prune lets the least used timings go past those kept. Its caller holds
// o.mu.
func (o *lyricsOffsets) prune() {
	entries, err := os.ReadDir(o.dir)
	if err != nil || len(entries) <= o.keep {
		return
	}
	type file struct {
		name string
		used time.Time
	}
	var files []file
	for _, e := range entries {
		if info, err := e.Info(); err == nil && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, file{e.Name(), info.ModTime()})
		}
	}
	slices.SortFunc(files, func(a, b file) int { return a.used.Compare(b.used) })
	for _, f := range files[:max(len(files)-o.keep, 0)] {
		os.Remove(filepath.Join(o.dir, f.name))
	}
	// What it knows of those gone, it reads again.
	clear(o.known)
}
