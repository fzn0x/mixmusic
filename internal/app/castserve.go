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
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/stream"
)

// castServer serves the proxy's audio to a cast device, while casting.
type castServer struct {
	ln     net.Listener
	srv    *http.Server
	token  string
	device net.IP
	proxy  http.Handler
	// fetched counts the requests of audio the device made.
	fetched atomic.Int64
}

// serveToCast serves proxy to the device at device, on the computer's
// address local, which reaches it, at a port of its own.
func serveToCast(local, device net.IP, proxy http.Handler) (*castServer, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(local.String(), "0"))
	if err != nil {
		return nil, err
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		ln.Close()
		return nil, err
	}
	s := &castServer{ln: ln, token: hex.EncodeToString(b[:]), device: device, proxy: proxy}
	s.srv = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second}
	go s.srv.Serve(ln)
	return s, nil
}

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
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	path, ok := strings.CutPrefix(r.URL.Path, "/"+s.token+"/")
	if !ok || !strings.HasPrefix(path, "yt/") {
		http.Error(w, errNotCast.Error(), http.StatusNotFound)
		return
	}
	s.fetched.Add(1)
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + path
	r2.URL.RawPath = ""
	s.proxy.ServeHTTP(w, r2)
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
