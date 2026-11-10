// Package overlay serves what plays to the programs on the same computer,
// for the overlays of streams (#344): JSON, the cover, a stream of events
// and a page for a browser source. It listens on 127.0.0.1 only, and takes
// no command: every route reads.
package overlay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// DefaultPort is the port the server listens on unless the user chose
// another.
const DefaultPort = 8284

// seekSkew is how far the position may drift from where it should be, at
// the speed the track plays, before the change counts as a seek.
const seekSkew = 2 * time.Second

// writeWait is how long a client of the event stream may take to read an
// event before the server lets it go.
const writeWait = 5 * time.Second

// keepAlive is how often the event stream sends a comment, which keeps the
// connection open through programs that close an idle one, and finds a
// client that went away.
const keepAlive = 15 * time.Second

// Info is what plays, as the server tells it.
type Info struct {
	// ID tells one track from another: the track's ID.
	ID      string
	Title   string
	Artists []string
	Album   string
	// Source is the service the track comes from: "youtube", "spotify" or
	// "file".
	Source     string
	PositionMs int
	DurationMs int
	// Playing is set while the track sounds or loads to sound.
	Playing bool
	// Live is set for a live stream, which has no length.
	Live bool
	// Speed is the speed the track plays at, 1 when zero.
	Speed float64
	// CoverURL is where the cover is, which the server fetches on /cover
	// through Server.Cover; empty for none.
	CoverURL string
}

func (i Info) speed() float64 {
	if i.Speed > 0 {
		return i.Speed
	}
	return 1
}

// Status is what the server does, for the settings page.
type Status struct {
	// On is set while the server listens.
	On bool
	// Addr is the address it listens on, as "127.0.0.1:8284".
	Addr string
	// Err is why it can't listen, as a port that another program holds.
	Err string
}

// Server serves what plays. The zero value isn't ready: use New.
type Server struct {
	// Cover returns the bytes of the cover at an address. Without it, /cover
	// has none.
	Cover func(ctx context.Context, url string) ([]byte, error)
	// Listen opens the listener: net.Listen when nil. Tests give their own.
	Listen func(network, addr string) (net.Listener, error)

	mu     sync.Mutex
	info   Info
	at     time.Time // when info's position was told
	cover  int       // changes with each track, to tell the browser to load the cover again
	subs   map[*subscriber]struct{}
	ln     net.Listener
	srv    *http.Server
	port   int
	status Status
	done   chan struct{} // closed as the server stops listening
}

// subscriber is a client of the event stream, which holds the latest event
// it wasn't sent yet: a client that reads slowly skips the events between,
// and never holds the server back.
type subscriber struct {
	wake chan struct{}
}

// New returns a server that listens nowhere yet.
func New() *Server {
	return &Server{subs: map[*subscriber]struct{}{}}
}

// Status returns what the server does now.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Apply has the server listen on 127.0.0.1 at port while on, or stop. It
// changes nothing while the server already listens as asked. A port that
// another program holds is returned, and is the status's Err, until Apply
// finds it free.
func (s *Server) Apply(on bool, port int) error {
	s.mu.Lock()
	if !on {
		s.stopLocked()
		s.status = Status{}
		s.mu.Unlock()
		return nil
	}
	if s.ln != nil && s.port == port {
		s.mu.Unlock()
		return nil
	}
	s.stopLocked()
	listen := s.Listen
	if listen == nil {
		listen = net.Listen
	}
	ln, err := listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		s.status = Status{Err: listenProblem(port, err)}
		s.mu.Unlock()
		return errors.New(s.status.Err)
	}
	s.ln, s.port = ln, port
	s.done = make(chan struct{})
	s.srv = &http.Server{Handler: s.mux(), ReadHeaderTimeout: 10 * time.Second}
	s.status = Status{On: true, Addr: ln.Addr().String()}
	go s.srv.Serve(ln)
	s.mu.Unlock()
	return nil
}

// listenProblem says why port can't be used.
func listenProblem(port int, err error) string {
	return fmt.Sprintf("Port %d can't be used, perhaps another program holds it: %v", port, err)
}

// stopLocked stops listening, and ends the event streams. s.mu is held.
func (s *Server) stopLocked() {
	if s.ln == nil {
		return
	}
	close(s.done)
	_ = s.srv.Close()
	s.ln, s.srv = nil, nil
}

// Close stops the server, and ends the event streams.
func (s *Server) Close() {
	s.mu.Lock()
	s.stopLocked()
	s.status = Status{}
	s.mu.Unlock()
}

// Streams returns how many clients of the event stream are connected.
func (s *Server) Streams() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subs)
}

// Update tells the server what plays. Clients of the event stream hear of
// a change of track, of play and pause, of a seek, and of a new cover; a
// position that moves on as it should tells them nothing.
func (s *Server) Update(next Info) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.info
	changed := next.ID != prev.ID || next.Playing != prev.Playing || next.Title != prev.Title ||
		next.CoverURL != prev.CoverURL || next.DurationMs != prev.DurationMs ||
		next.Live != prev.Live || next.Speed != prev.Speed || next.Source != prev.Source
	if !changed && next.Playing {
		drift := time.Duration(next.PositionMs-s.positionLocked(now)) * time.Millisecond
		changed = drift > seekSkew || drift < -seekSkew
	}
	if !changed && !next.Playing {
		changed = next.PositionMs != prev.PositionMs
	}
	if next.ID != prev.ID || next.CoverURL != prev.CoverURL {
		s.cover++
	}
	if !changed {
		return
	}
	s.info, s.at = next, now
	for sub := range s.subs {
		select {
		case sub.wake <- struct{}{}:
		default: // it will read the latest when it wakes
		}
	}
}

// positionLocked is where the track is at now, in milliseconds. s.mu is
// held.
func (s *Server) positionLocked(now time.Time) int {
	pos := s.info.PositionMs
	if s.info.Playing && !s.info.Live {
		pos += int(float64(now.Sub(s.at).Milliseconds()) * s.info.speed())
	}
	if d := s.info.DurationMs; d > 0 && pos > d {
		pos = d
	}
	return pos
}

// payload is what /now-playing and the events tell.
type payload struct {
	Active     bool     `json:"active"`
	Playing    bool     `json:"playing"`
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Artists    []string `json:"artists"`
	Artist     string   `json:"artist"`
	Album      string   `json:"album"`
	Source     string   `json:"source"`
	PositionMs int      `json:"positionMs"`
	DurationMs int      `json:"durationMs"`
	Live       bool     `json:"live"`
	Speed      float64  `json:"speed"`
	Cover      string   `json:"cover"`
}

// snapshot is what plays now, for the address the client asked.
func (s *Server) snapshot(host string) payload {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.info
	p := payload{Active: i.ID != "", Playing: i.Playing, ID: i.ID, Title: i.Title, Artists: i.Artists,
		Album: i.Album, Source: i.Source, PositionMs: s.positionLocked(now), DurationMs: i.DurationMs,
		Live: i.Live, Speed: i.speed()}
	if p.Artists == nil {
		p.Artists = []string{}
	}
	for n, a := range p.Artists {
		if n > 0 {
			p.Artist += ", "
		}
		p.Artist += a
	}
	if i.CoverURL != "" {
		p.Cover = "http://" + host + "/cover?v=" + strconv.Itoa(s.cover)
	}
	return p
}

// mux has the routes, which only read.
func (s *Server) mux() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/now-playing", s.nowPlaying)
	m.HandleFunc("/cover", s.coverImage)
	m.HandleFunc("/events", s.events)
	m.HandleFunc("/overlay", s.page)
	return s.guard(m)
}

// guard lets only reads through, and only to the names of this computer: a
// web page can't reach the server through a name of its own that points
// at 127.0.0.1.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localHost(r.Host) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "this only tells what plays", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// localHost reports whether host, as a request has it, names this computer
// as an address of its loopback.
func localHost(host string) bool {
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
	}
	if name == "localhost" {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) nowPlaying(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.snapshot(r.Host))
}

func (s *Server) coverImage(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	u := s.info.CoverURL
	s.mu.Unlock()
	if u == "" || s.Cover == nil {
		http.Error(w, "no cover", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	data, err := s.Cover(ctx, u)
	if err != nil || len(data) == 0 {
		http.Error(w, "no cover", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", sniffImage(data))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// sniffImage is the media type of an image, which a program that shows it
// takes as it is.
func sniffImage(data []byte) string {
	t := http.DetectContentType(data)
	switch t {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
		return t
	}
	return "application/octet-stream"
}

// events streams what plays, as Server-Sent Events: its state at once, then
// each change of track, play or pause, seek or cover.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	s.mu.Lock()
	done := s.done
	if done == nil {
		s.mu.Unlock()
		http.Error(w, "stopped", http.StatusServiceUnavailable)
		return
	}
	sub := &subscriber{wake: make(chan struct{}, 1)}
	s.subs[sub] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, sub)
		s.mu.Unlock()
	}()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("X-Accel-Buffering", "no")
	send := func(format string, args ...any) bool {
		_ = rc.SetWriteDeadline(time.Now().Add(writeWait))
		if _, err := fmt.Fprintf(w, format, args...); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	sendState := func() bool {
		data, _ := json.Marshal(s.snapshot(r.Host))
		return send("event: now-playing\ndata: %s\n\n", data)
	}
	if !send("retry: 3000\n\n") || !sendState() {
		return
	}
	tick := time.NewTicker(keepAlive)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-done:
			return
		case <-sub.wake:
			if !sendState() {
				return
			}
		case <-tick.C:
			if !send(": keep-alive\n\n") {
				return
			}
		}
	}
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(overlayPage))
}
