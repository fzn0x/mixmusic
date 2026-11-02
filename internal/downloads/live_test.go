package downloads

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestLiveDownload downloads a song of YouTube Music as the app does,
// through the proxy, and checks that its file plays, of its whole size:
// YouTube changes what it answers from time to time. It runs with
// MIXMUSIC_LIVE=1, finding streams through YouTube's player API, and
// yt-dlp where MIXMUSIC_YTDLP names one:
//
//	MIXMUSIC_LIVE=1 go test ./internal/downloads -run Live -v
func TestLiveDownload(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	songs, err := youtube.New().Search(ctx, "massive attack teardrop", youtube.FilterSongs)
	if err != nil || len(songs) == 0 || songs[0].Track == nil {
		t.Fatalf("search: %d songs, %v", len(songs), err)
	}
	song := *songs[0].Track
	tm := tools.New(t.TempDir())
	tm.Configure(os.Getenv("MIXMUSIC_YTDLP"), "")
	r := stream.NewResolver(tm, t.TempDir())
	r.Direct = stream.NewDirect()
	p := stream.NewProxy(r)
	dir := t.TempDir()
	m := Open(dir, Options{Fetcher: p})
	defer m.Close()
	m.Add([]media.Track{song}, "best")
	m.Start()
	start := time.Now()
	for kind(m, song.ID) != Done {
		if st := m.State(song.ID); st.Kind == Failed {
			t.Fatalf("%s: %s", song.ID, st.Why)
		}
		if time.Since(start) > 2*time.Minute {
			t.Fatalf("%s didn't download: %+v", song.ID, m.State(song.ID))
		}
		time.Sleep(100 * time.Millisecond)
	}
	k, _ := m.Lookup(song.ID)
	a, err := inspect(k.Path, -1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s (%s): %d bytes of %s in %s, %d kbit/s, in %s", song.ID, song.Title, a.bytes, a.codec, a.format, a.bitrate,
		time.Since(start).Round(time.Millisecond))
	if a.bitrate < minBest {
		t.Errorf("%d kbit/s at the best quality", a.bitrate)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "audio")); len(entries) != 1 {
		t.Errorf("the folder holds %v", entries)
	}
}
