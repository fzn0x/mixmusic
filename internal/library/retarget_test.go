package library

import (
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// ids are the IDs of tracks.
func ids(tracks []media.Track) []string {
	out := []string{}
	for _, t := range tracks {
		out = append(out, t.ID)
	}
	return out
}

// TestRetarget moves what was kept of two files on this computer, before
// they were linked as copies of one song, to the song: its likes once,
// where the first was; its plays, which count together, each when it
// played; its places in a mix, once, but where the mix held it twice
// before; and a dislike of one, as the other is liked, goes.
func TestRetarget(t *testing.T) {
	l := open(t)
	mp3, flac := tr("file:bXAz", media.SourceFile, "Ada Field"), tr("file:ZmxhYw", media.SourceFile, "Ada Field")
	song := tr("file:song:ZmxhYw", media.SourceFile, "Ada Field")
	other := tr("yt:other", media.SourceYouTube, "Orin Vale")
	start := time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)
	at := start
	l.now = func() time.Time { at = at.Add(time.Minute); return at }
	for _, t := range []media.Track{mp3, other, flac, mp3} {
		l.RecordPlay(t)
	}
	l.SetLiked(flac, true)
	l.SetLiked(other, true)
	l.SetLiked(mp3, true)
	l.SetDisliked(tr("file:b3RoZXI", media.SourceFile, "Ada Field"), true)
	p, _ := l.CreatePlaylist("Kites and more", "", []media.Track{song, other, mp3, flac})
	_, mix, _ := media.ParseID(p.ID)
	l.AddToPlaylistAgain(mix, []media.Track{song})
	if err := l.Retarget(map[string]media.Track{mp3.ID: song, flac.ID: song, "file:b3RoZXI": song}); err != nil {
		t.Fatal(err)
	}
	if got := ids(l.Liked()); !slices.Equal(got, []string{song.ID, other.ID}) {
		t.Errorf("liked %q", got)
	}
	if got := ids(l.Disliked()); len(got) != 0 || l.IsDisliked(song.ID) {
		t.Errorf("disliked %q", got)
	}
	if got := l.PlayCounts(); got[song.ID] != 3 || got[mp3.ID] != 0 || got[other.ID] != 1 {
		t.Errorf("plays %v", got)
	}
	h := l.History(0)
	if len(h) != 4 || h[0].Track.ID != song.ID || !h[0].PlayedAt.Equal(start.Add(4*time.Minute)) {
		t.Errorf("history %+v", h)
	}
	got, _ := l.Playlist(mix)
	if want := []string{song.ID, other.ID, song.ID}; !slices.Equal(ids(got.Tracks), want) {
		t.Errorf("the mix holds %q, want %q", ids(got.Tracks), want)
	}
	if s := l.Stats(0); len(s.TopTracks) == 0 || s.TopTracks[0].Track.ID != song.ID || s.TopTracks[0].Plays != 3 {
		t.Errorf("the stats' top tracks %+v", s.TopTracks)
	}
	// Nothing to move writes nothing.
	v := l.Version()
	if err := l.Retarget(map[string]media.Track{mp3.ID: song}); err != nil || l.Version() != v {
		t.Errorf("a retarget of nothing held: %v, version %d, was %d", err, l.Version(), v)
	}
}
