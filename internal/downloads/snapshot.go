package downloads

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"sync/atomic"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Kind is where the download of a track is.
type Kind int

const (
	// None: the track isn't downloaded.
	None Kind = iota
	// Waiting: it waits its turn, or for what holds the downloads.
	Waiting
	// Downloading: it downloads now.
	Downloading
	// Done: it plays without the network.
	Done
	// Failed: it couldn't download, for Why.
	Failed
)

// TrackState is where the download of a track is.
type TrackState struct {
	Kind Kind
	// Progress is how far a download under way came, from 0 to 1, or -1
	// before its size is known.
	Progress float64
	// Why is why it failed, in plain words.
	Why string
}

// Kept is a track downloaded: the video whose audio plays for it, the
// track that plays, which is a Spotify track's recording on YouTube Music,
// and its file.
type Kept struct {
	Video string
	Track media.Track
	Path  string
}

// Job is a track that downloads, waits to, or failed, as the downloads
// page lists it.
type Job struct {
	Track media.Track
	State TrackState
	// Phase tells what a download under way does, when it isn't fetching
	// the audio: "finding its recording on YouTube Music".
	Phase string
}

// CollectionView is a collection downloaded, and how many of its tracks
// play without the network.
type CollectionView struct {
	Collection
	Kept, Waiting, Failed int
}

// Status is what the downloads hold and do, for the downloads page and the
// settings.
type Status struct {
	// Tracks are the tracks downloaded, in Bytes.
	Tracks int
	Bytes  int64
	// Waiting counts the tracks that download or wait to, and Failed those
	// that failed.
	Waiting, Failed int
	// BatchDone counts the tracks downloaded since the downloads last had
	// nothing to do: "12 of 57", of BatchDone+Waiting.
	BatchDone int
	Paused    bool
	// Hold is why the downloads wait, or "" when they don't.
	Hold string
	// Folder is the folder the downloads are in, and Problem why it can't
	// be used, or "".
	Folder, Problem string
	// Jobs are the downloads under way, then the next ones that wait, up
	// to maxJobs; FailedJobs those that failed, up to maxJobs.
	Jobs, FailedJobs []Job
}

// maxJobs bounds the downloads a Status lists of each kind.
const maxJobs = 50

// Snapshot is what the downloads hold and do at one moment: the windows
// read it in each frame, which costs a map lookup a track. A nil Snapshot
// holds nothing.
type Snapshot struct {
	// version changes with what the downloads hold, and their state.
	version uint64
	states  map[string]TrackState
	running map[string]*job
	kept    map[string]Kept
	// originals are the tracks downloaded, as they were asked for.
	originals map[string]media.Track
	// files are the downloaded audio by video ID, and art the covers kept
	// by their addresses: absolute paths.
	files       map[string]string
	art         map[string]string
	status      Status
	tracks      []media.Track
	collections []CollectionView
	byID        map[string]int
}

// Snapshot returns what the downloads hold and do now.
func (m *Manager) Snapshot() *Snapshot {
	if m == nil {
		return nil
	}
	return m.snap.Load()
}

// Version changes with what the downloads hold, and their state, not with
// the progress of those under way.
func (s *Snapshot) Version() uint64 {
	if s == nil {
		return 0
	}
	return s.version
}

// Track returns a track downloaded, as it was asked for: a Spotify track
// as Spotify's.
func (s *Snapshot) Track(id string) (media.Track, bool) {
	if s == nil {
		return media.Track{}, false
	}
	t, ok := s.originals[id]
	return t, ok
}

// Any reports whether the downloads hold or wait for anything.
func (s *Snapshot) Any() bool { return s != nil && len(s.states) > 0 }

// State returns where the download of a track is.
func (s *Snapshot) State(trackID string) TrackState {
	if s == nil {
		return TrackState{}
	}
	if j, ok := s.running[trackID]; ok {
		return TrackState{Kind: Downloading, Progress: j.progress()}
	}
	return s.states[trackID]
}

// Lookup returns the download of a track, once it plays without the
// network.
func (s *Snapshot) Lookup(trackID string) (Kept, bool) {
	if s == nil {
		return Kept{}, false
	}
	k, ok := s.kept[trackID]
	return k, ok
}

// Status returns what the downloads hold and do.
func (s *Snapshot) Status() Status {
	if s == nil {
		return Status{}
	}
	return s.status
}

// Tracks returns the tracks downloaded, the last downloaded first. Don't
// change it.
func (s *Snapshot) Tracks() []media.Track {
	if s == nil {
		return nil
	}
	return s.tracks
}

// Collections returns the collections downloaded, the last first.
func (s *Snapshot) Collections() []CollectionView {
	if s == nil {
		return nil
	}
	return s.collections
}

// Collection returns the collection downloaded of id, if it is.
func (s *Snapshot) Collection(id string) (CollectionView, bool) {
	if s == nil {
		return CollectionView{}, false
	}
	i, ok := s.byID[id]
	if !ok {
		return CollectionView{}, false
	}
	return s.collections[i], true
}

// Missing counts the tracks, of those a collection holds now, that its
// download doesn't hold: those added to it since.
func (s *Snapshot) Missing(id string, tracks []media.Track) int {
	if _, ok := s.Collection(id); !ok {
		return len(tracks)
	}
	n := 0
	for _, t := range tracks {
		if st, ok := s.states[t.ID]; !ok || st.Kind == None {
			if _, running := s.running[t.ID]; !running {
				n++
			}
		}
	}
	return n
}

// HasVideo reports whether the downloads hold the audio of a video.
func (s *Snapshot) HasVideo(video string) bool {
	if s == nil {
		return false
	}
	_, ok := s.files[video]
	return ok
}

// ArtworkFile returns the file that keeps the cover at url, if a download
// keeps it.
func (s *Snapshot) ArtworkFile(url string) (string, bool) {
	if s == nil {
		return "", false
	}
	p, ok := s.art[url]
	return p, ok
}

// Lookup returns the download of a track, once it plays without the
// network.
func (m *Manager) Lookup(trackID string) (Kept, bool) { return m.Snapshot().Lookup(trackID) }

// State returns where the download of a track is.
func (m *Manager) State(trackID string) TrackState { return m.Snapshot().State(trackID) }

// Status returns what the downloads hold and do.
func (m *Manager) Status() Status { return m.Snapshot().Status() }

// publish makes the snapshot again, and tells the watchers soon. Its
// caller holds mu.
func (m *Manager) publish() {
	m.seq++
	s := &Snapshot{version: m.seq, originals: map[string]media.Track{}, states: make(map[string]TrackState, len(m.idx.Tracks)), running: make(map[string]*job, len(m.jobs)),
		kept: map[string]Kept{}, files: make(map[string]string, len(m.idx.Files)), art: make(map[string]string, len(m.idx.Art)),
		byID: map[string]int{}}
	for video, f := range m.idx.Files {
		s.files[video] = filepath.Join(m.dir, filepath.FromSlash(f.Path))
	}
	for u, p := range m.idx.Art {
		s.art[u] = filepath.Join(m.dir, filepath.FromSlash(p))
	}
	st := &s.status
	st.Folder, st.Paused = m.dir, m.idx.Paused
	if m.err != nil {
		st.Problem, _ = m.held()
	}
	st.Hold, _ = m.held()
	type queued struct {
		seq int64
		job Job
	}
	var done []*entry
	var waiting, failed []queued
	for id, e := range m.idx.Tracks {
		if j := m.jobs[id]; j != nil {
			s.running[id] = j
			st.Waiting++
			continue
		}
		switch {
		case m.done(e):
			s.states[id] = TrackState{Kind: Done}
			t := e.Track
			if e.Recording != nil {
				t = *e.Recording
			}
			s.kept[id] = Kept{Video: e.video(), Track: t, Path: s.files[e.video()]}
			s.originals[id] = e.Track
			st.Tracks++
			done = append(done, e)
		case e.Failed != "":
			s.states[id] = TrackState{Kind: Failed, Why: e.Failed}
			st.Failed++
			failed = append(failed, queued{e.Seq, Job{Track: e.Track, State: s.states[id]}})
		default:
			s.states[id] = TrackState{Kind: Waiting, Progress: -1}
			st.Waiting++
			waiting = append(waiting, queued{e.Seq, Job{Track: e.Track, State: s.states[id]}})
		}
	}
	for id := range s.running {
		s.states[id] = TrackState{Kind: Downloading, Progress: -1}
	}
	for _, f := range m.idx.Files {
		st.Bytes += f.Bytes
	}
	st.BatchDone = m.batchDone
	// The downloads under way, then those that wait, in their order.
	var running []*job
	for _, j := range m.jobs {
		running = append(running, j)
	}
	slices.SortFunc(running, func(a, b *job) int { return cmp.Compare(a.seq, b.seq) })
	for _, j := range running {
		st.Jobs = append(st.Jobs, Job{Track: j.track, State: TrackState{Kind: Downloading, Progress: -1}, Phase: j.phaseNow()})
	}
	bySeq := func(a, b queued) int { return cmp.Compare(a.seq, b.seq) }
	slices.SortFunc(waiting, bySeq)
	for _, q := range waiting[:min(len(waiting), max(maxJobs-len(st.Jobs), 0))] {
		st.Jobs = append(st.Jobs, q.job)
	}
	slices.SortFunc(failed, bySeq)
	for _, q := range failed[:min(len(failed), maxJobs)] {
		st.FailedJobs = append(st.FailedJobs, q.job)
	}
	// The tracks downloaded, the last first.
	slices.SortFunc(done, func(a, b *entry) int { return cmp.Compare(b.Seq, a.Seq) })
	s.tracks = make([]media.Track, len(done))
	for i, e := range done {
		s.tracks[i] = e.Track
	}
	cols := make([]*Collection, 0, len(m.idx.Collections))
	for _, c := range m.idx.Collections {
		cols = append(cols, c)
	}
	slices.SortFunc(cols, func(a, b *Collection) int { return cmp.Or(b.Added.Compare(a.Added), cmp.Compare(a.ID, b.ID)) })
	for _, c := range cols {
		v := CollectionView{Collection: *c}
		for _, id := range c.Tracks {
			switch s.State(id).Kind {
			case Done:
				v.Kept++
			case Waiting, Downloading:
				v.Waiting++
			case Failed:
				v.Failed++
			}
		}
		s.byID[c.ID] = len(s.collections)
		s.collections = append(s.collections, v)
	}
	m.snap.Store(s)
	m.tellSoon()
}

// job is a download under way.
type job struct {
	id        string
	seq       int64
	track     media.Track
	recording *media.Track
	quality   string
	video     string
	ctx       context.Context
	cancel    context.CancelCauseFunc
	// done and size are how far it came, size -1 until known; phase is
	// what it does, which mu guards.
	done, size atomic.Int64
	phase      string
	// from is where the download went on from, in its part.
	from atomic.Int64
}

// progressed reports whether bytes came in the job.
func (j *job) progressed() bool { return j.done.Load() > j.from.Load() }

// progress is how far the job came, from 0 to 1, or -1 before its size is
// known.
func (j *job) progress() float64 {
	size := j.size.Load()
	if size <= 0 {
		return -1
	}
	return min(float64(j.done.Load())/float64(size), 1)
}

func (j *job) phaseNow() string { return j.phase }
