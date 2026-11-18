package app

// The audio a cast device plays, which it fetches from MixMusic: the
// addresses the player plays are MixMusic's own, which no device can
// open, and YouTube's streams come through the proxy, which fetches them
// in chunks, waits for YouTube to serve them, and serves the cache and the
// downloads first. While casting, and only then, the proxy is served on
// the network the device is on, at an address of its own, to that device
// alone, under a path no one could guess.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/netutil"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/upnp"
)

// castServer serves the proxy's audio to a cast device, while casting.
type castServer struct {
	ln     net.Listener
	srv    *http.Server
	token  string
	device net.IP
	proxy  http.Handler
	// dlna is set for a DLNA renderer, which gets DLNA's headers.
	dlna bool
	// fetched counts the requests of audio the device made.
	fetched atomic.Int64

	mu sync.Mutex
	// served are the addresses handed to the device, past the token,
	// by what they serve (castFormats); refused is set once a request
	// from another address was turned away.
	served  map[string]castServed
	refused bool
}

// castServed is what an address handed to a device serves: a path of the
// proxy, or the file of a song on this computer, as contentType, or the
// cover of one, when cover is set. of is the stream whose audio or cover
// it is, as the player plays it: the address serves it only while that
// stream is cast or queued (keep).
type castServed struct {
	path, file  string
	contentType string
	cover       bool
	of          string
}

// castConns is how many connections at once a renderer's server takes:
// a renderer opens one for each request, and some never close them.
const castConns = 8

// serveToCast serves proxy to the device at device, on the computer's
// address local, which reaches it, at a port of its own.
func serveToCast(local, device net.IP, proxy http.Handler) (*castServer, error) {
	return serveCast(local, device, proxy, false)
}

// serveCast is serveToCast, of a DLNA renderer when dlna: with DLNA's
// headers, over castConns connections at once.
func serveCast(local, device net.IP, proxy http.Handler, dlna bool) (*castServer, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(local.String(), "0"))
	if err != nil {
		return nil, err
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		ln.Close()
		return nil, err
	}
	s := &castServer{ln: ln, token: hex.EncodeToString(b[:]), device: device, proxy: proxy, dlna: dlna,
		served: map[string]castServed{}}
	s.srv = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	if dlna {
		ln = netutil.LimitListener(ln, castConns)
	}
	go s.srv.Serve(ln)
	return s, nil
}

// serve hands the device the address at, past the token, which serves c.
func (s *castServer) serve(at string, c castServed) {
	c.of = streamKey(c.of)
	s.mu.Lock()
	s.served[at] = c
	s.mu.Unlock()
}

// keep serves, of the addresses handed to the device, only those of the
// streams of: the track cast, and the one queued on the device after it.
// The device can fetch no other file of this computer, nor its cover.
func (s *castServer) keep(of ...string) {
	for i := range of {
		of[i] = streamKey(of[i])
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for at, c := range s.served {
		if c.of == "" || !slices.Contains(of, c.of) {
			delete(s.served, at)
		}
	}
}

// streamKey is the address of a stream the player plays, without its
// query, which changes as it tries again.
func streamKey(rawURL string) string {
	key, _, _ := strings.Cut(rawURL, "?")
	return key
}

// ours reports whether an address a device plays is one this server
// handed it: its token is in no other.
func (s *castServer) ours(uri string) bool { return strings.Contains(uri, "/"+s.token+"/") }

// url returns the address the device fetches the audio of path at, a path
// of the proxy, as /yt/<video id>.
func (s *castServer) url(path string) string {
	return "http://" + s.ln.Addr().String() + "/" + s.token + path
}

// close stops serving: the device can fetch nothing more.
func (s *castServer) close() {
	s.srv.Close()
}

var errNotCast = errors.New("not found")

func (s *castServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip == nil || !ip.Equal(s.device) {
		s.turnedAway()
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	path, ok := strings.CutPrefix(r.URL.Path, "/"+s.token+"/")
	s.mu.Lock()
	c, handed := s.served["/"+path]
	s.mu.Unlock()
	if !ok || !handed && !strings.HasPrefix(path, "yt/") {
		http.Error(w, errNotCast.Error(), http.StatusNotFound)
		return
	}
	if c.cover {
		// A cover is no audio: it comes whole, as its file's kind.
		serveCastFile(w, r, c.file)
		return
	}
	s.fetched.Add(1)
	if s.dlna {
		// The audio comes as it plays, from its start or a byte range
		// (DLNA.ORG_OP=01).
		w.Header().Set("transferMode.dlna.org", "Streaming")
		w.Header().Set("contentFeatures.dlna.org", upnp.Features)
	}
	if handed {
		w = &typedWriter{ResponseWriter: w, contentType: c.contentType}
		if c.file != "" {
			serveCastFile(w, r, c.file)
			return
		}
		path = strings.TrimPrefix(c.path, "/")
	}
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + path
	r2.URL.RawPath = ""
	s.proxy.ServeHTTP(w, r2)
}

// turnedAway logs, once, that a request from an address other than the
// device's was turned away: a member of a group of speakers that fetches
// in place of the one cast to would be.
func (s *castServer) turnedAway() {
	s.mu.Lock()
	first := !s.refused
	s.refused = true
	s.mu.Unlock()
	if first {
		log.Printf("mixmusic: the cast server turned away a request from an address other than the device's")
	}
}

// serveCastFile serves the file of a song on this computer, or its cover,
// by ranges, as the type of its extension.
func serveCastFile(w http.ResponseWriter, r *http.Request, file string) {
	f, err := os.Open(file)
	if err != nil {
		http.Error(w, errNotCast.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, errNotCast.Error(), http.StatusNotFound)
		return
	}
	http.ServeContent(w, r, file, info.ModTime(), f)
}

// typedWriter answers with the content type the device was told of, in
// place of the proxy's, which names the codecs too.
type typedWriter struct {
	http.ResponseWriter
	contentType string
	wrote       bool
}

func (w *typedWriter) WriteHeader(code int) {
	if !w.wrote && code < 300 {
		w.Header().Set("Content-Type", w.contentType)
	}
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *typedWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *typedWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// castArt serves server's device the cover the index keeps of t, a song
// on this computer, while its stream (of) is cast or queued, and returns
// its address: "" for a track with none.
func (a *App) castArt(server *castServer) func(t media.Track, of string) string {
	return func(t media.Track, of string) string {
		if a.localFiles == nil {
			return ""
		}
		for _, img := range t.Artwork {
			if file, ok := a.localFiles.ArtFile(img.URL); ok {
				at := "/art/" + filepath.Base(file)
				server.serve(at, castServed{file: file, cover: true, of: of})
				return server.url(at)
			}
		}
		return ""
	}
}

// castContentType returns the MIME type of the audio of a video the cast
// device fetches, as far as MixMusic knows it without fetching it: of its
// download, its cached audio, or its stream found, else the type of most
// of YouTube's audio. A device picks its player by it.
func (a *App) castContentType(videoID string, surround bool) string {
	if surround {
		return "audio/mp4" // AC-3 or E-AC-3 in MP4
	}
	if a.downloads.Snapshot().HasVideo(videoID) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodHead, stream.Path(videoID), nil)
		if a.downloads.Serve(rec, req, videoID) {
			return baseType(rec.Header().Get("Content-Type"))
		}
	}
	if _, mt, ok := a.cache.Find(videoID); ok {
		return baseType(mt)
	}
	if a.resolver.Fresh(videoID) {
		// A stream found comes at once.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if s, err := a.resolver.Resolve(ctx, videoID); err == nil && s.MimeType != "" {
			return baseType(s.MimeType)
		}
	}
	return "audio/webm"
}

// baseType returns a MIME type without its parameters, as "audio/webm" of
// `audio/webm; codecs="opus"`.
func baseType(t string) string {
	if mt, _, err := mime.ParseMediaType(t); err == nil {
		return mt
	}
	return "audio/webm"
}

// localIP returns the IP of a connection's local address.
func localIP(addr net.Addr) net.IP {
	if a, ok := addr.(*net.TCPAddr); ok {
		return a.IP
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}
