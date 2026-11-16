package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/align"
	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestLiveLyricsTiming finds where the songs of a few music videos start
// in them, from their audio on YouTube, signed out, as the app does: the
// video YouTube Music finds first for each search, its song as the
// matcher finds it. It logs each shift and how each window matched,
// rather than expecting shifts YouTube's copies may change. It runs with
// MIXMUSIC_LIVE=1, more videos with MIXMUSIC_LIVE_VIDEOS naming searches
// apart by commas:
//
//	MIXMUSIC_LIVE=1 go test ./internal/app -run LiveLyricsTiming -v
func TestLiveLyricsTiming(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	searches := []string{"rick astley never gonna give you up", "daft punk instant crush"}
	if more := os.Getenv("MIXMUSIC_LIVE_VIDEOS"); more != "" {
		searches = strings.Split(more, ",")
	}
	yt := youtube.New()
	known, err := store.Open(filepath.Join(t.TempDir(), "matches.json"), map[string]media.Track{})
	if err != nil {
		t.Fatal(err)
	}
	matcher := match.New(yt, known)
	r := stream.NewResolver(tools.New(t.TempDir()), t.TempDir())
	r.Direct = stream.NewDirect()
	p := stream.NewProxy(r)
	open := func(ctx context.Context, videoID string) (io.ReadSeekCloser, error) {
		return p.OpenRanges(ctx, videoID)
	}
	matched := 0
	for _, q := range searches {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		videos, err := yt.Search(ctx, q, youtube.FilterVideos)
		if err != nil || len(videos) == 0 || videos[0].Track == nil {
			cancel()
			t.Errorf("%q: %d videos, %v", q, len(videos), err)
			continue
		}
		video := *videos[0].Track
		song, err := matcher.Song(ctx, video)
		if err != nil {
			cancel()
			t.Logf("%q: %s has no song: %v", q, video.Title, err)
			continue
		}
		_, videoID, _ := media.ParseID(video.ID)
		_, songID, _ := media.ParseID(song.ID)
		start := time.Now()
		res, err := measureTiming(ctx, open, videoID, songID)
		cancel()
		took := time.Since(start).Round(time.Millisecond)
		switch {
		case err == nil:
			matched++
			t.Logf("%s (%s), song %s: %+.3fs, in %v: %s", video.Title, videoID, songID, res.Shift.Seconds(), took, res.Windows)
		case errors.Is(err, align.ErrNoMatch):
			t.Logf("%s (%s), song %s: no clear match, in %v: %v", video.Title, videoID, songID, took, err)
		default:
			t.Errorf("%s (%s), song %s: %v", video.Title, videoID, songID, err)
		}
	}
	if matched == 0 {
		t.Error("no song found in its video")
	}
}
