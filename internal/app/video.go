package app

// The picture of music videos, which the native interface shows on the
// now playing page: MyGo draws no video, so a page of MixMusic's own,
// public/video.html, plays it in a web view there. The page fetches the
// picture's stream from the proxy at the stream scheme, one part at a
// time, through Media Source Extensions, and follows the engine's clock,
// which the interface sends it.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// VideoSource is the stream of the picture of a music video, for the page
// that plays it.
type VideoSource struct {
	// Src is the address of the stream, to which the page adds the bytes
	// it asks for: &range=<first>-<last>.
	Src      string `json:"src"`
	MimeType string `json:"mime"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	// Duration is in seconds.
	Duration float64 `json:"duration"`
	// Init is the first byte after the stream's initialization segment.
	Init int64 `json:"init"`
	// Segments are the parts of the stream the page appends whole: their
	// time and duration in seconds, and their first and last bytes.
	Segments [][4]float64 `json:"segments"`
}

// errNotMusicVideo is returned for the picture of a track that is no music
// video.
var errNotMusicVideo = errors.New("only music videos show their picture")

// Video finds the stream of the picture of a music video, the recording
// PrepareStream returned, to show in a frame of w × h pixels, in the first
// of families, the codecs the page plays in the order it prefers them,
// that YouTube serves it in. With data saver, the stream is the smallest.
func (p *Player) Video(ctx context.Context, t media.Track, w, h int, families []string) (VideoSource, error) {
	source, videoID, err := media.ParseID(t.ID)
	switch {
	case err != nil:
		return VideoSource{}, err
	case source != media.SourceYouTube || !t.MusicVideo():
		return VideoSource{}, errNotMusicVideo
	case p.a.online != nil && p.a.online.Offline():
		return VideoSource{}, player.ErrOffline
	}
	saver := p.a.settings.Get().Quality == settings.QualitySaver
	pic, err := p.a.proxy.Picture(ctx, videoID, func(vs []stream.Video) (stream.Video, bool) {
		return stream.PickVideo(vs, families, w, h, saver)
	})
	if err != nil {
		return VideoSource{}, err
	}
	src := VideoSource{
		Src:      schemeURL(streamScheme) + strings.TrimPrefix(stream.VideoPath(videoID, pic.Itag), "/"),
		MimeType: pic.MimeType,
		Width:    pic.Width,
		Height:   pic.Height,
		Duration: pic.Duration.Seconds(),
		Init:     pic.Index.Init,
		Segments: make([][4]float64, len(pic.Index.Segments)),
	}
	for i, s := range pic.Index.Segments {
		src.Segments[i] = [4]float64{s.Time.Seconds(), s.Duration.Seconds(), float64(s.Start), float64(s.End)}
	}
	return src, nil
}

// Clock returns the time of the track the engine plays that is heard now,
// which the picture of a music video follows.
func (p *Player) Clock() time.Duration {
	if e := p.a.nativeEngine(); e != nil {
		return e.Position()
	}
	return 0
}

// SetVideoPage tells the app the page that plays the picture of music
// videos, the only one the stream scheme serves; nil once it closed.
func (a *App) SetVideoPage(page *mygo.Page) { a.videoPage.Store(page) }

// callerPage tells the page that made a request of a custom scheme; tests
// replace it.
var callerPage = mygo.CallerPage

// videoHandler serves the picture of music videos to their page, and to no
// other: Google's sign-in, or the page of MixMusic's own Spotify device,
// could otherwise make MixMusic find streams, and fetch them.
func (a *App) videoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if page := a.videoPage.Load(); page == nil || callerPage(r.Context()) != page {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		a.proxy.ServeVideo(w, r)
	})
}

// handleVideo serves the stream scheme to the page of the picture of music
// videos. Schemes must be handled before the windows that use them open.
func (a *App) handleVideo() {
	if err := mygo.Protocol.Handle(streamScheme, a.videoHandler()); err != nil {
		log.Printf("mixmusic: the picture of music videos can't load: %v", err)
	}
}

// WebViewProblem tells why music videos can't show their picture here, in
// plain words, or "" when they can: their page needs the system's web view.
func (s *System) WebViewProblem() string {
	if err := needWebView("Showing music videos"); err != nil {
		return err.Error()
	}
	return ""
}

// MiniPlayerOpen reports whether the mini player is open: the main window
// shows no music video meanwhile. Frames ask it.
func (s *System) MiniPlayerOpen() bool { return s.a.miniOpen.Load() }
