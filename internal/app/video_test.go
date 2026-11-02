package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// The music video of the stream package's test data, and the size of its
// stream 243, VP9 in WebM, whose first bytes are there.
const (
	musicVideoID = "Kq-7hp9i4lE"
	size243      = 21115748
)

var musicVideo = media.Track{ID: "yt:" + musicVideoID, Source: media.SourceYouTube, Title: "Ajen ajen", Video: true, VideoType: media.VideoOfficial}

// headStream is a stream of size bytes whose first bytes are head.
type headStream struct {
	head []byte
	size int64
	at   int64
}

func (s *headStream) Read(p []byte) (int, error) {
	if s.at >= s.size {
		return 0, io.EOF
	}
	n := min(int64(len(p)), s.size-s.at)
	for i := range n {
		if off := s.at + i; off < int64(len(s.head)) {
			p[i] = s.head[off]
		} else {
			p[i] = 0
		}
	}
	s.at += n
	return int(n), nil
}

func (s *headStream) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		s.at = off
	case io.SeekCurrent:
		s.at += off
	case io.SeekEnd:
		s.at = s.size + off
	}
	return s.at, nil
}

// videoApp is an app whose yt-dlp finds the audio of musicVideoID and two
// streams of its picture in VP9, 243 and a smaller 278, served by a
// server of its own, with settings s.
func videoApp(t *testing.T, s settings.Settings) *App {
	t.Helper()
	head, err := os.ReadFile("../stream/testdata/video-243-head.webm")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "", time.Time{}, &headStream{head: head, size: size243})
	}))
	t.Cleanup(srv.Close)
	r := stream.NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		u := func(itag string) string {
			return fmt.Sprintf("%s/v?itag=%s&expire=%d", srv.URL, itag, time.Now().Add(6*time.Hour).Unix())
		}
		video := func(itag string, w, h int) string {
			return fmt.Sprintf(`{"format_id": %q, "url": %q, "ext": "webm", "protocol": "https", "vcodec": "vp9", "acodec": "none",
				"width": %d, "height": %d, "fps": 30, "filesize": %d}`, itag, u(itag), w, h, size243)
		}
		return fmt.Appendf(nil, `{"url": %q, "ext": "webm", "acodec": "opus", "filesize": 5, "formats": [%s, %s]}`,
			u("251"), video("243", 640, 360), video("278", 256, 144)), nil
	}
	f, err := store.Open(filepath.Join(t.TempDir(), "settings.json"), s)
	if err != nil {
		t.Fatal(err)
	}
	return &App{proxy: stream.NewProxy(r), settings: f}
}

func TestPlayerVideo(t *testing.T) {
	p := &Player{videoApp(t, settings.Default())}
	src, err := p.Video(context.Background(), musicVideo, 630, 354, []string{"avc1", "vp9"})
	if err != nil {
		t.Fatal(err)
	}
	if want := schemeURL(streamScheme) + "video/" + musicVideoID + "?itag=243"; src.Src != want {
		t.Errorf("src %q, want %q", src.Src, want)
	}
	if src.MimeType != `video/webm; codecs="vp9"` || src.Width != 640 || src.Height != 360 || src.Init != 220 {
		t.Errorf("%+v", src)
	}
	if len(src.Segments) != 97 || src.Segments[0][0] != 0 || src.Segments[0][2] != 1876 || src.Segments[96][3] != size243-1 {
		t.Errorf("%d segments, from %v to %v", len(src.Segments), src.Segments[0], src.Segments[len(src.Segments)-1])
	}
	if src.Duration < 474 || src.Duration > 475 {
		t.Errorf("lasts %.1fs", src.Duration)
	}

	saver := settings.Default()
	saver.Quality = settings.QualitySaver
	p = &Player{videoApp(t, saver)}
	if src, err := p.Video(context.Background(), musicVideo, 630, 354, []string{"vp9"}); err != nil || src.Width != 256 {
		t.Errorf("data saver: %d × %d, %v", src.Width, src.Height, err)
	}
	for _, other := range []media.Track{
		{ID: "yt:w3XzgjlKFjc", Source: media.SourceYouTube, Title: "a song"},
		{ID: "yt:w3XzgjlKFjc", Source: media.SourceYouTube, Title: "an episode", Video: true, VideoType: "MUSIC_VIDEO_TYPE_PODCAST_EPISODE"},
		{ID: media.ID(media.SourceSpotify, "abc"), Source: media.SourceSpotify, Title: "a Spotify track", Video: true},
	} {
		if _, err := p.Video(context.Background(), other, 630, 354, []string{"vp9"}); !errors.Is(err, errNotMusicVideo) {
			t.Errorf("%s: %v", other.Title, err)
		}
	}
}

// TestVideoHandlerServesItsPageOnly serves the picture of music videos to
// their page only, while it is open.
func TestVideoHandlerServesItsPageOnly(t *testing.T) {
	a := videoApp(t, settings.Default())
	var caller *mygo.Page
	saved := callerPage
	callerPage = func(context.Context) *mygo.Page { return caller }
	t.Cleanup(func() { callerPage = saved })
	h := a.videoHandler()
	ask := func(path string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "http://stream.localhost"+path, nil))
		return rec.Code
	}
	page, other := new(mygo.Page), new(mygo.Page)
	part := "/video/" + musicVideoID + "?itag=243&range=0-99"
	for _, c := range []struct {
		name         string
		open, caller *mygo.Page
		path         string
		want         int
	}{
		{"no page open", nil, nil, part, http.StatusForbidden},
		{"no page asks", page, nil, part, http.StatusForbidden},
		{"another page", page, other, part, http.StatusForbidden},
		{"its page", page, page, part, http.StatusPartialContent},
		{"its page, the audio", page, page, "/yt/" + musicVideoID, http.StatusNotFound},
	} {
		a.SetVideoPage(c.open)
		caller = c.caller
		if got := ask(c.path); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}
