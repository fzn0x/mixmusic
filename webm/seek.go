package webm

import (
	"cmp"
	"slices"
	"sort"
	"time"
)

// Seek moves to the last cue point at or before t, or to the start, so
// that the next ReadPacket returns the first packet of its cluster.
// Callers that need t exactly decode from there, dropping the packets
// before t; Opus also needs SeekPreRoll of decoding before t, so its
// callers seek to t - SeekPreRoll.
//
// The first Seek reads the Cues, when they come after the clusters. In a
// file without Cues, or with unusable ones, Seek reads the timestamps of
// the clusters instead, from the first: a read, or a seek, per cluster.
// It remembers them, so that it reads each cluster once.
func (r *Reader) Seek(t time.Duration) error {
	r.queue, r.qi, r.inCluster, r.err = r.queue[:0], 0, false, nil
	if err := r.seek(t); err != nil {
		r.err = err
		return err
	}
	return nil
}

func (r *Reader) seek(t time.Duration) error {
	if err := r.loadCues(); err != nil {
		return err
	}
	if len(r.cues) > 0 {
		pos := r.start
		if i := sort.Search(len(r.cues), func(i int) bool { return r.cues[i].time > t }); i > 0 {
			pos = r.cues[i-1].pos
		}
		ok, err := r.atCluster(pos)
		if err != nil || ok {
			return err
		}
		r.cues = nil // they point elsewhere than at clusters: do without
	}
	for !r.scanned && (len(r.clusters) == 0 || r.clusters[len(r.clusters)-1].time <= t) {
		if err := r.scanCluster(); err != nil {
			return err
		}
	}
	pos := r.start
	if i := sort.Search(len(r.clusters), func(i int) bool { return r.clusters[i].time > t }); i > 0 {
		pos = r.clusters[i-1].pos
	}
	r.src.seekTo(pos)
	return nil
}

// loadCues reads the Cues the SeekHeads point to, once. Cues that cannot
// be read are done without.
func (r *Reader) loadCues() error {
	if r.cuesRead {
		return nil
	}
	b, err := r.loadIndexed(idCues)
	if err != nil && !isDataError(err) {
		return err // the reading failed: try again at the next Seek
	}
	r.setCues(b)
	return nil
}

// setCues keeps the cue points of the track in Cues of data b, or those
// of any track when it has none: as the clusters they point to start at
// their time, they are places to read the track from too. Corrupt Cues
// give the cue points before the corruption.
func (r *Reader) setCues(b []byte) {
	r.cuesRead = true
	var mine, all []cuePoint
	eachChild(b, func(id uint32, data []byte) error {
		if id != idCuePoint {
			return nil
		}
		var ticks uint64
		var points []cuePoint
		var tracks []uint64
		err := eachChild(data, func(id uint32, data []byte) error {
			var err error
			switch id {
			case idCueTime:
				ticks, err = readUint(data)
			case idCueTrackPositions:
				var track, pos uint64
				found := false
				err = eachChild(data, func(id uint32, data []byte) error {
					var err error
					switch id {
					case idCueTrack:
						track, err = readUint(data)
					case idCueClusterPosition:
						pos, err = readUint(data)
						found = true
					}
					return err
				})
				if off, ok := r.segmentOffset(pos); found && ok {
					points = append(points, cuePoint{pos: off})
					tracks = append(tracks, track)
				}
			}
			return err
		})
		t, ok := r.ticks(ticks)
		if err != nil || !ok {
			return err
		}
		for i, p := range points {
			p.time = t
			all = append(all, p)
			if tracks[i] == r.track.Number {
				mine = append(mine, p)
			}
		}
		return nil
	})
	if len(mine) == 0 {
		mine = all
	}
	slices.SortStableFunc(mine, func(a, b cuePoint) int { return cmp.Compare(a.time, b.time) })
	r.cues = mine
}

// atCluster moves to offset pos, and reports whether a Cluster starts
// there.
func (r *Reader) atCluster(pos int64) (bool, error) {
	r.src.seekTo(pos)
	h, err := r.src.readHeader()
	if err != nil {
		if isDataError(err) {
			err = nil
		}
		return false, err
	}
	r.src.seekTo(pos)
	return h.id == idCluster, nil
}

// scanCluster reads the timestamp of the cluster after those scanned, and
// remembers where it starts, or sets r.scanned at the end of the Segment.
// A file that ends early, or with corrupt data, ends the scan there.
func (r *Reader) scanCluster() error {
	s := r.src
	s.seekTo(r.scanAt)
	for {
		if r.segEnd >= 0 && s.offset() >= r.segEnd {
			r.scanned = true
			return nil
		}
		h, err := s.readHeader()
		switch {
		case err != nil:
		case h.id == idEBML || h.id == idSegment:
			r.scanned = true
			return nil
		case h.id != idCluster:
			if err = checkChild(h, r.segEnd); err == nil {
				s.skip(h.size)
				continue
			}
		case h.size != unknownSize:
			err = checkChild(h, r.segEnd)
		}
		if err == nil {
			err = r.scanTimestamp(h)
		}
		if err != nil {
			if !isDataError(err) {
				return err
			}
			r.scanned = true // seek within what comes before
			return nil
		}
		r.scanAt = s.offset()
		return nil
	}
}

// scanTimestamp reads the Timestamp of the cluster of header h, and
// remembers it, leaving the source after the cluster.
func (r *Reader) scanTimestamp(h header) error {
	r.inCluster, r.clusterEnd, r.clusterTime = true, h.end(), 0
	defer func() { r.inCluster = false }()
	found := false
	for {
		c, ok, err := r.clusterChild()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if c.id != idTimestamp || found {
			r.src.skip(c.size)
			continue
		}
		if err := r.readTimestamp(c); err != nil {
			return err
		}
		found = true
		if r.clusterEnd >= 0 {
			r.src.seekTo(r.clusterEnd)
			break
		}
	}
	t, _ := r.ticks(uint64(r.clusterTime))
	if n := len(r.clusters); n > 0 && t < r.clusters[n-1].time {
		// Out of order: keep the times sorted for the search.
		t = r.clusters[n-1].time
	}
	r.clusters = append(r.clusters, cuePoint{time: t, pos: h.start})
	return nil
}
