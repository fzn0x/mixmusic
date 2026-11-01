package stream

// Downloads: the whole audio of a track, fetched to keep on disk, as the
// downloads the user chooses are.

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// The kinds of the errors finding or fetching a stream ends with, for a
// caller that does something of each, as downloads do: errors.Is tells
// them. The errors' own texts are what the player shows.
var (
	// ErrThrottled: YouTube turns down requests from this network for now,
	// or asks to confirm this is not a bot.
	ErrThrottled = errors.New("stream: YouTube is turning down requests from this network for now")
	// ErrUnavailable: YouTube won't send this track's audio, to anyone or
	// to MixMusic: age-restricted, members only, private, removed, or not in
	// this country.
	ErrUnavailable = errors.New("stream: YouTube won't send this track")
	// ErrNetwork: the network failed while the stream was found or fetched.
	ErrNetwork = errors.New("stream: the network failed")
	// ErrNoAudio: YouTube gave a stream of a format the engine doesn't
	// play.
	ErrNoAudio = errors.New("YouTube has no audio of this track that MixMusic plays")
)

// IsNetwork reports whether err tells that the network failed, rather than
// YouTube turning the stream down: a request that couldn't connect, a
// connection that dropped or stalled, or yt-dlp that couldn't reach
// YouTube.
func IsNetwork(err error) bool {
	var ue *url.Error
	var ne net.Error
	return errors.Is(err, ErrNetwork) || errors.Is(err, errStalled) || errors.As(err, &ue) || errors.As(err, &ne) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

// Is makes YouTube's answer of HTTP 429 an ErrThrottled.
func (e statusError) Is(target error) bool {
	return target == ErrThrottled && e == http.StatusTooManyRequests
}

// PlainFailure says in plain words why the audio of a video could not be
// served or fetched, without the stream's URL.
func PlainFailure(err error) string { return plainFailure(err) }

// Store is audio kept apart from the cache, as the downloads the user
// chose: the proxy serves a video's audio from it before anything else.
type Store interface {
	// Serve answers r with the audio of a video, and reports whether it
	// had it.
	Serve(w http.ResponseWriter, r *http.Request, videoID string) bool
}

// Version names the file a stream is: its format, its length and when
// YouTube last changed it, as its URL tells. The streams of a video of one
// version hold the same bytes, so a download that stopped can go on from
// one to the other. Without what the URL tells, its type and size stand
// in. The URL itself, which carries the user's IP address, is no part of
// it.
func (s Stream) Version() string {
	q := s.query()
	if q.Get("itag") != "" && q.Get("lmt") != "" {
		return q.Get("itag") + "/" + cmp.Or(q.Get("clen"), strconv.FormatInt(s.Size, 10)) + "/" + q.Get("lmt")
	}
	return s.MimeType + "/" + strconv.FormatInt(s.Size, 10)
}

// Itag is the number of the stream's format on YouTube, as its URL tells,
// or 0.
func (s Stream) Itag() int {
	n, _ := strconv.Atoi(s.query().Get("itag"))
	return n
}

func (s Stream) query() url.Values {
	if u, err := url.Parse(s.URL); err == nil {
		return u.Query()
	}
	return url.Values{}
}

// Container is the stream's container, as its type tells: "webm" or
// "mp4", the audio the engine plays, or "".
func (s Stream) Container() string {
	base, _, _ := mime.ParseMediaType(s.MimeType)
	switch base {
	case "audio/webm":
		return "webm"
	case "audio/mp4":
		return "mp4"
	}
	return ""
}

// Download fetches the whole audio of a video into w, to keep on disk: in
// chunks, as the proxy serves it, once YouTube serves a stream yt-dlp just
// found. It finds the stream again when YouTube refuses its URL, and
// connects again from the byte it reached when the connection drops or
// stalls, for as long as the proxy does for the player. resume is given
// the stream found, and returns how many of its bytes w holds already,
// from an earlier download that stopped: Download goes on from there. The
// stream must be of a container the engine plays. progress, if set, is
// told how many bytes of the stream w holds as they come.
//
// Its errors never quote the stream's URL, which carries the user's IP
// address. A writer that fails ends it with the writer's error, as of a
// full disk.
func (p *Proxy) Download(ctx context.Context, videoID string, resume func(s Stream) (int64, error), w io.Writer, progress func(done, size int64)) (Stream, error) {
	s, err := p.resolve(ctx, videoID)
	if err != nil {
		return s, err
	}
	if s.Container() == "" {
		return s, ErrNoAudio
	}
	if s.Size == 0 {
		if s.Size, err = p.probeSize(ctx, s); err != nil {
			return s, network(err)
		}
	}
	off, err := resume(s)
	if err != nil {
		return s, err
	}
	if off < 0 || off > s.Size {
		return s, fmt.Errorf("stream: can't go on from byte %d of %d", off, s.Size)
	}
	out := &counter{w: w, n: off, size: s.Size, progress: progress}
	stall, reconnect, backoff := p.timing()
	wait := backoff
	var broke time.Time // when the download broke, while it stays broken
	again := false      // set once the stream was found again for this chunk
	for out.n < s.Size {
		from := out.n
		end := min(from+chunkSize, s.Size) - 1
		body, err := p.fetch(ctx, s, from, end)
		// lost is set when the network lost the download, rather than YouTube
		// turning it down: it connects again then.
		lost := false
		if err == nil {
			_, cerr := io.Copy(out, body)
			body.Close()
			rerr := body.err
			if out.n > from {
				broke, wait, again = time.Time{}, backoff, false
			}
			switch {
			case out.n > end:
				continue
			case out.werr != nil:
				return s, out.werr
			case rerr != nil:
				err, lost = rerr, true
			case cerr != nil:
				err, lost = cerr, true
			default:
				err, lost = io.ErrUnexpectedEOF, true // the answer ended early
			}
		} else {
			lost = IsNetwork(err)
		}
		switch {
		case ctx.Err() != nil:
			return s, ctx.Err()
		case errors.Is(err, errExpired) && !again:
			// YouTube refuses the URL: the stream is found again, once for
			// each chunk, and must be of the same version to go on.
			again = true
			p.Resolver.Forget(videoID)
			found, err := p.resolve(ctx, videoID)
			if err != nil {
				return s, err
			}
			if found.Size == 0 {
				found.Size = s.Size
			}
			if found.Version() != s.Version() {
				return s, errChanged
			}
			s = found
			continue
		case !lost:
			return s, err
		case broke.IsZero():
			broke = time.Now()
		case time.Since(broke)+wait > reconnect:
			return s, errStalled
		}
		// The connections kept for reuse may have died with this one.
		p.HTTP.CloseIdleConnections()
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return s, ctx.Err()
		}
		wait = min(2*wait, stall)
	}
	return s, nil
}

// errChanged is returned when the stream of a video found again is another
// file than the one a download had started.
var errChanged = errors.New("stream: YouTube now sends another file of this track")

// network returns err, a request's, as an ErrNetwork without the URL it
// quotes, when it tells that the network failed.
func network(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%w: %v", ErrNetwork, loggable(err))
	}
	return err
}

// counter counts the bytes written to w from n, and tells progress. A
// write that fails keeps its error.
type counter struct {
	w        io.Writer
	n, size  int64
	progress func(done, size int64)
	werr     error
}

func (c *counter) Write(b []byte) (int, error) {
	n, err := c.w.Write(b)
	c.n += int64(n)
	if err != nil {
		c.werr = err
	}
	if c.progress != nil && n > 0 {
		c.progress(c.n, c.size)
	}
	return n, err
}
