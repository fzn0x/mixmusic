package downloads

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/mp4"
	"github.com/fzn0x/mixmusic/webm"
)

var (
	// errBusy is returned for a track whose audio another worker downloads:
	// it shares the file once done.
	errBusy = errors.New("downloads: another download fetches this audio")
	// errBadAudio is returned for audio that came whole, and doesn't play.
	errBadAudio = errors.New("the audio that came can't play")
	// errNotYouTube is returned for a track of no video.
	errNotYouTube = errors.New("only tracks of YouTube Music, and Spotify tracks through their recording there, download")
	// errNoMatcher is returned for a Spotify track where nothing finds its
	// recording.
	errNoMatcher = errors.New("nothing here finds the recordings of Spotify tracks")
)

// minBest is the bitrate a track the cache holds needs, in kbit/s, to be
// downloaded at the best quality from the cache: the cache keeps a track
// in the quality it played in, and data saver's are about 50 kbit/s.
const minBest = 96

// work downloads what waits, one track at a time, until the manager
// closes.
func (m *Manager) work() {
	for {
		j := m.next()
		if j == nil {
			return
		}
		err := m.fetch(j)
		m.finish(j, err)
	}
}

// next returns the next download, once one may start, or nil once the
// manager closed.
func (m *Manager) next() *job {
	for {
		m.mu.Lock()
		j, wait, kick := m.pick()
		m.mu.Unlock()
		if j != nil {
			return j
		}
		var timer *time.Timer
		var due <-chan time.Time
		if wait > 0 {
			timer = time.NewTimer(wait)
			due = timer.C
		}
		select {
		case <-m.ctx.Done():
		case <-kick:
		case <-due:
		}
		if timer != nil {
			timer.Stop()
		}
		if m.ctx.Err() != nil {
			return nil
		}
	}
}

// pick starts the download of the track asked for first that waits, or
// returns how long to wait for one, 0 until told, and what tells. Its
// caller holds mu.
func (m *Manager) pick() (*job, time.Duration, chan struct{}) {
	if m.ctx.Err() != nil {
		return nil, 0, m.kick
	}
	if why, until := m.held(); why != "" {
		var wait time.Duration
		if !until.IsZero() {
			wait = max(time.Until(until), time.Millisecond)
		}
		return nil, wait, m.kick
	}
	var best *entry
	var bestID string
	var soonest time.Time
	now := time.Now()
	for id, e := range m.idx.Tracks {
		if e.Failed != "" || m.jobs[id] != nil || m.done(e) {
			continue
		}
		if v := e.video(); v != "" && m.busy[v] {
			continue
		}
		if at, ok := m.nextAt[id]; ok && now.Before(at) {
			if soonest.IsZero() || at.Before(soonest) {
				soonest = at
			}
			continue
		}
		if best == nil || e.Seq < best.Seq {
			best, bestID = e, id
		}
	}
	if best == nil {
		var wait time.Duration
		if !soonest.IsZero() {
			wait = max(time.Until(soonest), time.Millisecond)
		}
		return nil, wait, m.kick
	}
	ctx, cancel := context.WithCancelCause(m.ctx)
	j := &job{id: bestID, seq: best.Seq, track: best.Track, recording: best.Recording, quality: best.Quality, cancel: cancel}
	j.size.Store(-1)
	j.ctx = ctx
	if v := best.video(); v != "" {
		m.busy[v] = true
		j.video = v
	}
	m.jobs[bestID] = j
	m.publish()
	return j, 0, nil
}

// setPhase tells what a job does.
func (m *Manager) setPhase(j *job, phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j.phase != phase {
		j.phase = phase
		m.publish()
	}
}

// fetch downloads the track of j: it finds a Spotify track's recording,
// shares the file of a track of the same recording, takes it from the
// cache, or downloads it.
func (m *Manager) fetch(j *job) error {
	rec := j.recording
	if rec == nil && j.track.Source == media.SourceYouTube {
		t := j.track
		rec = &t
	}
	if rec == nil {
		if m.opts.Matcher == nil {
			return errNoMatcher
		}
		m.setPhase(j, "finding its recording on YouTube Music")
		found, err := m.opts.Matcher.Match(j.ctx, j.track)
		if err != nil {
			return err
		}
		rec = &found
		m.mu.Lock()
		if e := m.idx.Tracks[j.id]; e != nil {
			e.Recording = rec
			m.changed()
		}
		m.mu.Unlock()
		m.setPhase(j, "")
	}
	_, video, err := media.ParseID(rec.ID)
	if err != nil || rec.Source != media.SourceYouTube || !stream.ValidVideoID(video) {
		return errNotYouTube
	}
	m.mu.Lock()
	if j.video != video {
		if m.busy[video] {
			m.mu.Unlock()
			return errBusy
		}
		m.busy[video] = true
		j.video = video
	}
	shared := m.idx.Files[video] != nil
	dir := m.dir
	m.mu.Unlock()
	if shared || m.fromCache(j, dir, video) {
		return nil
	}
	return m.download(j, dir, video)
}

// fromCache takes the audio of video from the offline cache, and reports
// whether it did: at data saver whatever it holds, at the best quality
// only what isn't data saver's.
func (m *Manager) fromCache(j *job, dir, video string) bool {
	if m.opts.Cache == nil {
		return false
	}
	path, _, ok := m.opts.Cache.Find(video)
	if !ok {
		return false
	}
	a, err := inspect(path, -1)
	if err != nil || j.quality != "saver" && a.bitrate > 0 && a.bitrate < minBest {
		return false
	}
	name := "audio/" + video + extension(a.format)
	final := filepath.Join(dir, filepath.FromSlash(name))
	if ok, err := m.opts.Cache.Take(video, final); !ok || err != nil {
		return false
	}
	if a, err = inspect(final, a.bytes); err != nil {
		os.Remove(final)
		return false
	}
	m.keep(video, &file{Path: name, Bytes: a.bytes, Format: a.format, Codec: a.codec, Bitrate: a.bitrate, Quality: j.quality,
		Cached: true, Done: time.Now()})
	return true
}

// download downloads the audio of video into its part, going on from what
// the part holds of the same stream, checks it, and keeps it.
func (m *Manager) download(j *job, dir, video string) error {
	if err := m.waitTurn(j.ctx); err != nil {
		return err
	}
	part := filepath.Join(dir, "audio", "."+video+".part")
	if err := os.MkdirAll(filepath.Dir(part), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(part, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	resume := func(s stream.Stream) (int64, error) {
		info, err := f.Stat()
		if err != nil {
			return 0, err
		}
		m.mu.Lock()
		was := m.idx.Parts[video]
		m.mu.Unlock()
		if n := info.Size(); n > 0 && n <= s.Size && was == s.Version() {
			_, err := f.Seek(0, io.SeekEnd)
			j.from.Store(n)
			j.done.Store(n)
			return n, err
		}
		if err := f.Truncate(0); err != nil {
			return 0, err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return 0, err
		}
		m.mu.Lock()
		m.idx.Parts[video] = s.Version()
		m.changed()
		m.mu.Unlock()
		return 0, nil
	}
	s, err := m.opts.Fetcher.Download(j.ctx, video, resume, f, func(done, size int64) {
		j.done.Store(done)
		j.size.Store(size)
		m.tellSoon()
	})
	// What came stays, whole or not, for the next try to go on from.
	if serr := f.Sync(); err == nil {
		err = serr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	a, err := inspect(part, s.Size)
	if err != nil {
		log.Printf("mixmusic: the download of %s doesn't play: %v", video, err)
		os.Remove(part)
		m.mu.Lock()
		delete(m.idx.Parts, video)
		m.changed()
		m.mu.Unlock()
		return errBadAudio
	}
	name := "audio/" + video + extension(a.format)
	if err := os.Rename(part, filepath.Join(dir, filepath.FromSlash(name))); err != nil {
		return err
	}
	m.keep(video, &file{Path: name, Bytes: a.bytes, Format: a.format, Codec: a.codec, Itag: s.Itag(), Bitrate: a.bitrate,
		Quality: j.quality, Done: time.Now()})
	return nil
}

// waitTurn waits until the download may ask YouTube for its stream:
// startGap after the one before.
func (m *Manager) waitTurn(ctx context.Context) error {
	m.mu.Lock()
	at := time.Now()
	if m.startAt.After(at) {
		at = m.startAt
	}
	m.startAt = at.Add(startGap)
	m.mu.Unlock()
	wait := time.Until(at)
	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// keep keeps the audio of video.
func (m *Manager) keep(video string, f *file) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idx.Files[video] = f
	delete(m.idx.Parts, video)
	m.changed()
}

// finish takes in how the download of j went.
func (m *Manager) finish(j *job, err error) {
	cause := context.Cause(j.ctx)
	j.cancel(nil)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, j.id)
	if j.video != "" {
		delete(m.busy, j.video)
	}
	e := m.idx.Tracks[j.id]
	switch {
	case e == nil:
		// Removed meanwhile: its audio goes, unless another track plays it.
		if j.video != "" && !m.needed(j.video) {
			m.dropVideo(j.video)
		}
	case err == nil:
		delete(m.tries, j.id)
		delete(m.nextAt, j.id)
		m.failing = 0
		m.batchDone++
		m.keepArtSoon(j.id)
	case errors.Is(cause, errHeld), errors.Is(cause, errRemoved), m.ctx.Err() != nil:
		// It waits, and goes on from its part.
	case errors.Is(err, errBusy):
		m.nextAt[j.id] = time.Now().Add(busyWait)
	default:
		if j.progressed() {
			// It came on before it failed, as on a slow, flaky network: its
			// tries start over, from what came.
			delete(m.tries, j.id)
			m.failing = 0
		}
		m.failed(j.id, e, err)
	}
	if !m.waiting() {
		m.batchDone = 0
	}
	m.wake()
	m.changed()
}

// waiting reports whether a track waits to download. Its caller holds mu.
func (m *Manager) waiting() bool {
	for _, e := range m.idx.Tracks {
		if e.Failed == "" && !m.done(e) {
			return true
		}
	}
	return false
}

// failed takes in a download that failed with err: it waits for what holds
// every download, fails for good when it can't download, else tries again
// later. Its caller holds mu.
func (m *Manager) failed(id string, e *entry, err error) {
	why := plain(err)
	now := time.Now()
	log.Printf("mixmusic: downloading %s: %s", id, why)
	switch {
	case errors.Is(err, stream.ErrThrottled):
		m.setHold(holdThrottled, true, "YouTube is turning down requests from this network for now; downloads go on in a while", now.Add(throttleFor))
	case diskFull(err):
		m.setHold(holdDisk, true, "The disk of your downloads is full: they go on once it has room", now.Add(diskFullFor))
	case errors.Is(err, tools.ErrNoYTDLP):
		e.Failed = "it needs yt-dlp, which Settings › Playback installs"
	case errors.Is(err, stream.ErrUnavailable), errors.Is(err, stream.ErrNoAudio), errors.Is(err, match.ErrNoMatch),
		errors.Is(err, errBadAudio), errors.Is(err, errNotYouTube), errors.Is(err, errNoMatcher):
		e.Failed = why
	case stream.IsNetwork(err) && m.opts.Network != nil:
		// Gone with the network, it waits for it, without spending a try.
		m.nextAt[id] = now.Add(busyWait)
		go m.checkNetwork(id, why)
	default:
		m.retryLater(id, e, why)
	}
}

// retryLater has a download that failed for why try again after the next
// of retryWaits, or fail once they ran out. Failures in a row, of any
// track, hold every download a while. Its caller holds mu.
func (m *Manager) retryLater(id string, e *entry, why string) {
	n := m.tries[id]
	if n >= len(retryWaits) {
		e.Failed = why
		delete(m.tries, id)
		delete(m.nextAt, id)
	} else {
		m.tries[id] = n + 1
		m.nextAt[id] = time.Now().Add(retryWaits[n])
	}
	m.failing++
	if m.failing >= failingAfter {
		m.setHold(holdFailing, true, "Downloads keep failing ("+why+"); they go on in a while", time.Now().Add(failingFor))
	}
}

// checkNetwork asks whether the network answers, after a download failed
// as when it went: while it doesn't, every download waits for it.
func (m *Manager) checkNetwork(id, why string) {
	if m.opts.Network.Check(m.ctx) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if e := m.idx.Tracks[id]; e != nil && m.ctx.Err() == nil {
			m.retryLater(id, e, why)
			m.changed()
		}
		return
	}
	if m.ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	_, waits := m.holds[holdOffline]
	m.setHold(holdOffline, true, "Waiting for the network", time.Time{})
	m.publish()
	m.mu.Unlock()
	if waits {
		return
	}
	if err := m.opts.Network.Wait(m.ctx); err != nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setHold(holdOffline, false, "", time.Time{})
	m.publish()
}

// plain says why a download failed, in plain words.
func plain(err error) string {
	switch {
	case errors.Is(err, match.ErrNoMatch), errors.Is(err, errBadAudio), errors.Is(err, errNotYouTube), errors.Is(err, errNoMatcher):
		return err.Error()
	case diskFull(err):
		return "the disk is full"
	}
	return stream.PlainFailure(err)
}

// audio is what a file of audio holds.
type audio struct {
	format, codec string
	bytes         int64
	bitrate       int // kbit/s, 0 when its length is unknown
}

// ebml is the magic number WebM files start with.
var ebml = []byte{0x1a, 0x45, 0xdf, 0xa3}

// inspect checks that the file at path is audio the engine plays, of size
// bytes unless size is below 0: Opus in WebM, or AAC-LC in MP4.
func inspect(path string, size int64) (audio, error) {
	f, err := os.Open(path)
	if err != nil {
		return audio{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return audio{}, err
	}
	a := audio{bytes: info.Size()}
	if size >= 0 && a.bytes != size {
		return a, fmt.Errorf("%d bytes came of %d", a.bytes, size)
	}
	var head [8]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return a, fmt.Errorf("reading its start: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return a, err
	}
	var length time.Duration
	switch {
	case bytes.Equal(head[:4], ebml):
		r, err := webm.NewReader(f)
		if err != nil {
			return a, err
		}
		if c := r.Track().Codec; c != "A_OPUS" {
			return a, fmt.Errorf("%s in WebM", c)
		}
		a.format, a.codec, length = "webm", "opus", r.Duration()
	case mp4Box(string(head[4:])):
		r, err := mp4.NewReader(f)
		if err != nil {
			return a, err
		}
		if c := r.Track().Codec; c != "mp4a.40.2" {
			return a, fmt.Errorf("%s in MP4", c)
		}
		a.format, a.codec, length = "mp4", "mp4a.40.2", r.Duration()
	default:
		return a, errors.New("neither WebM nor MP4")
	}
	if length > 0 {
		a.bitrate = int(float64(a.bytes) * 8 / length.Seconds() / 1000)
	}
	return a, nil
}

// mp4Box reports whether name is the type of a box an MP4 file starts
// with, as the engine tells them.
func mp4Box(name string) bool {
	switch name {
	case "ftyp", "styp", "moov", "mdat", "free", "skip", "wide", "pdin", "uuid":
		return true
	}
	return false
}

// extension is the extension of the files of a container.
func extension(format string) string {
	if format == "mp4" {
		return ".m4a"
	}
	return ".webm"
}
