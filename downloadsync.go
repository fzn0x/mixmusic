package main

// Downloads kept up to date: the tracks added to a playlist, an album, a
// mix or the liked songs downloaded download on their own as its page
// shows them, unless Settings › Playback › Downloads says not to.

import (
	"fmt"

	"github.com/fzn0x/mixmusic/internal/media"
)

// syncMemo is the tracks of a page that its collection's download took
// last, as missingMemo tells them apart: by the first and how many.
type syncMemo struct {
	first *media.Track
	of    int
}

// syncDownload downloads the tracks added to d's collection since it was
// downloaded, once for each answer of its page: as the page first shows,
// once refreshed, once a long playlist is read whole, and as the liked
// songs change. Nothing is asked of the services, and the tracks are gone
// through once, not in each frame. The tracks the download held stay as
// they are, so that one that failed, or whose download was removed, isn't
// asked for again; and as many as confirmDownload asks before downloading
// wait for the head's button, which a notice tells.
func (s *shell) syncDownload(d *downloadHead) {
	if !s.set.DownloadAdded || len(d.tracks) == 0 {
		return
	}
	v, held := s.kept.Collection(d.c.ID)
	if !held {
		return
	}
	memo := syncMemo{first: &d.tracks[0], of: len(d.tracks)}
	if s.syncMemos[d.c.ID] == memo {
		return
	}
	if s.syncMemos == nil {
		s.syncMemos = map[string]syncMemo{}
	}
	s.syncMemos[d.c.ID] = memo
	had := make(map[string]bool, len(v.Tracks))
	for _, id := range v.Tracks {
		had[id] = true
	}
	tracks := s.downloadableOf(d.tracks)
	var added []media.Track
	for _, t := range tracks {
		if t.ID != "" && !had[t.ID] {
			had[t.ID] = true
			added = append(added, t)
		}
	}
	if len(added) == 0 {
		return
	}
	if size, big := s.downloadSize(added); big {
		were := "were"
		if len(added) == 1 {
			were = "was"
		}
		s.notes = append(s.notes, notice(fmt.Sprintf("%s %s added to %s since you downloaded it. They take about %s, so they download only once you ask.",
			plural(len(added), "track"), were, d.c.Title, byteSize(size))))
		return
	}
	if n := s.svc.Downloads.SyncCollection(d.c, tracks); n > 0 {
		s.notes = append(s.notes, notice(fmt.Sprintf("Downloading %s added to %s", plural(n, "track"), d.c.Title)))
	}
}
