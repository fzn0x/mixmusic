package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// errNoVideo is returned for a video YouTube gives no stream of its
// picture of, in a codec the page plays.
var errNoVideo = errors.New("stream: YouTube gave no stream of the video's picture that plays here")

// errNoSuchVideo is returned for a stream of a video's picture that its
// video doesn't have.
var errNoSuchVideo = errors.New("stream: the video has no such stream")

// Picture is the stream of a video's picture, as a page plays it, and
// where its parts are.
type Picture struct {
	Video
	// Duration is how long the picture lasts, as its index tells.
	Duration time.Duration
	Index    Index
}

// Picture finds the stream of a video's picture that pick chooses of
// those YouTube serves, and reads where its parts are from its first
// bytes. Finding it runs yt-dlp for a track whose audio came from the
// cache; for others, the stream was found with the audio. Its errors
// quote no stream URL, which carries the user's IP address.
func (p *Proxy) Picture(ctx context.Context, videoID string, pick func([]Video) (Video, bool)) (Picture, error) {
	pic, err := p.picture(ctx, videoID, pick)
	if err != nil {
		err = loggable(err)
	}
	return pic, err
}

func (p *Proxy) picture(ctx context.Context, videoID string, pick func([]Video) (Video, bool)) (Picture, error) {
	if !ValidVideoID(videoID) {
		return Picture{}, ErrBadVideoID
	}
	s, err := p.resolve(ctx, videoID)
	if err != nil {
		return Picture{}, err
	}
	v, ok := pick(s.Videos)
	if !ok {
		return Picture{}, errNoVideo
	}
	if v.Size == 0 {
		if v.Size, err = p.probeSize(ctx, Stream{URL: v.URL, Header: v.Header}); err != nil {
			return Picture{}, err
		}
	}
	need := min(int64(headSize), v.Size)
	for range 3 {
		head, err := p.readVideo(ctx, videoID, v.Itag, 0, need-1)
		if err != nil {
			return Picture{}, err
		}
		ix, err := parseIndex(head, v.MimeType, v.Size)
		var short shortHead
		switch {
		case errors.As(err, &short) && short.need > need && short.need <= maxHead:
			need = min(short.need, v.Size)
			continue
		case errors.As(err, &short):
			return Picture{}, errNoIndex
		case err != nil:
			return Picture{}, err
		}
		last := ix.Segments[len(ix.Segments)-1]
		return Picture{Video: v, Duration: last.Time + last.Duration, Index: ix}, nil
	}
	return Picture{}, errNoIndex
}

// readVideo reads bytes start to end of the stream itag of a video's
// picture.
func (p *Proxy) readVideo(ctx context.Context, videoID, itag string, start, end int64) ([]byte, error) {
	body, _, err := p.videoRange(ctx, videoID, itag, start, end)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	b, err := io.ReadAll(body)
	if err == nil && int64(len(b)) != end-start+1 {
		err = io.ErrUnexpectedEOF
	}
	return b, err
}

// videoOf returns the stream itag of a video's picture, once YouTube
// serves it.
func (p *Proxy) videoOf(ctx context.Context, videoID, itag string) (Video, error) {
	s, err := p.resolve(ctx, videoID)
	if err != nil {
		return Video{}, err
	}
	for _, v := range s.Videos {
		if v.Itag == itag {
			return v, nil
		}
	}
	return Video{}, errNoSuchVideo
}

// videoRange requests bytes start to end of the stream itag of a video's
// picture. When YouTube refuses its URL, as it does once it expired, it
// finds the video's streams again, once.
func (p *Proxy) videoRange(ctx context.Context, videoID, itag string, start, end int64) (*stallingBody, Video, error) {
	v, err := p.videoOf(ctx, videoID, itag)
	if err != nil {
		return nil, v, err
	}
	body, err := p.fetch(ctx, Stream{URL: v.URL, Header: v.Header}, start, end)
	if errors.Is(err, errExpired) {
		p.Resolver.Forget(videoID)
		if v, err = p.videoOf(ctx, videoID, itag); err == nil {
			body, err = p.fetch(ctx, Stream{URL: v.URL, Header: v.Header}, start, end)
		}
	}
	return body, v, err
}

// VideoPath returns the path the proxy serves the stream itag of a
// video's picture at, to which a page adds the bytes it asks for:
// &range=<first>-<last>.
func VideoPath(videoID, itag string) string {
	return "/video/" + videoID + "?itag=" + url.QueryEscape(itag)
}

// ServeVideo serves bytes of the stream of a video's picture, at
// VideoPath, for a page that plays it through Media Source Extensions:
// those its range parameter asks for, at most chunkSize, fetched from
// YouTube as they are asked for. A page asks for a part of the stream at
// a time, which it appends whole, so nothing is fetched ahead of what it
// asks for, though a web view may hand it a response only once it is
// whole, as WebView2 does. The picture isn't kept in the cache, and its
// failures aren't the audio's.
func (p *Proxy) ServeVideo(w http.ResponseWriter, r *http.Request) {
	videoID, ok := strings.CutPrefix(r.URL.Path, "/video/")
	if !ok || !ValidVideoID(videoID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	itag := q.Get("itag")
	start, end, ok := queryRange(q.Get("range"))
	if itag == "" || !ok {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	end = min(end, start+chunkSize-1)
	v, err := p.videoOf(r.Context(), videoID, itag)
	if errors.Is(err, errNoSuchVideo) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		p.failVideo(w, videoID, err)
		return
	}
	if v.Size > 0 {
		if start >= v.Size {
			w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(v.Size, 10))
			http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		end = min(end, v.Size-1)
	}
	var body *stallingBody
	if r.Method == http.MethodGet {
		if body, v, err = p.videoRange(r.Context(), videoID, itag, start, end); err != nil {
			p.failVideo(w, videoID, err)
			return
		}
		defer body.Close()
	}
	size := "*"
	if v.Size > 0 {
		size = strconv.FormatInt(v.Size, 10)
	}
	h := w.Header()
	h.Set("Content-Type", v.MimeType)
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%s", start, end, size))
	w.WriteHeader(http.StatusPartialContent)
	if body == nil {
		return
	}
	if _, err := io.Copy(w, body); err != nil && body.err != nil && r.Context().Err() == nil {
		log.Printf("mixmusic: the picture of %s broke off: %v", videoID, loggable(body.err))
	}
}

// queryRange reads a range parameter, <first>-<last>, both bytes given.
func queryRange(s string) (start, end int64, ok bool) {
	from, to, found := strings.Cut(s, "-")
	start, err1 := strconv.ParseInt(from, 10, 64)
	end, err2 := strconv.ParseInt(to, 10, 64)
	return start, end, found && err1 == nil && err2 == nil && start >= 0 && end >= start
}

// failVideo answers a request of a video's picture that could not be
// served with err, and logs why, unless the page gave it up. It leaves the
// audio's failures alone, which the player reads.
func (p *Proxy) failVideo(w http.ResponseWriter, videoID string, err error) {
	http.Error(w, "the picture couldn't load", http.StatusBadGateway)
	if !errors.Is(err, context.Canceled) {
		log.Printf("mixmusic: the picture of %s: %v", videoID, loggable(err))
	}
}
