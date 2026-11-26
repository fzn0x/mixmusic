package overlay

// Programs on this computer controlling MixMusic (#591): the routes of
// /control, which take commands, guarded by a token that only this
// computer's user may read, and tell what plays beside the queue.

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// controlBody is how much a command may carry: a track's ID and a few
// numbers.
const controlBody = 1 << 10

// controlBurst is how many commands may come at once, and controlPace how
// many a second come after, so that a volume dial keeps up.
const (
	controlBurst = 20
	controlPace  = 20
)

// ErrNothingLoaded is why a command can't run: no track is loaded.
var ErrNothingLoaded = errors.New("nothing is loaded")

// ErrNoPlayer is why a command can't run: the player isn't playing here.
var ErrNoPlayer = errors.New("the player isn't here")

// ErrNoTrack is why a command can't run: the track it names is in no
// queue.
var ErrNoTrack = errors.New("no such track in the queue")

// Control is the player, as the routes of /control run commands on it and
// tell of it: what /now-playing leaves out, and the queue.
type Control interface {
	// Extra is what the state adds to /now-playing.
	Extra() Extra
	// Do runs c, or says why it can't: ErrNothingLoaded, ErrNoPlayer,
	// ErrNoTrack, or an error for fields it can't use.
	Do(c Command) error
	// Watch has fn told as what Extra tells changes, and returns a func
	// that stops it.
	Watch(fn func()) func()
}

// Extra is what /control/state adds to /now-playing.
type Extra struct {
	// Volume is from 0 to 100, without Muted.
	Volume  int  `json:"volume"`
	Muted   bool `json:"muted"`
	Shuffle bool `json:"shuffle"`
	// Repeat is "off", "all" or "one".
	Repeat string `json:"repeat"`
	// Liked is set while the track is among the liked songs.
	Liked bool `json:"liked"`
	// CanNext and CanPrevious are whether the player has somewhere to go.
	CanNext     bool `json:"canNext"`
	CanPrevious bool `json:"canPrevious"`
	// Queue is the next tracks.
	Queue []Track `json:"queue"`
}

// Track is one of the queue's tracks, as the state tells it.
type Track struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Artists []string `json:"artists"`
}

// Command is what a route of /control asks.
type Command struct {
	// Op is the route's name after /control/: "toggle", "play", "pause",
	// "next", "previous", "mute", "shuffle", "repeat", "like", "dislike",
	// "track", "seek" or "volume".
	Op string
	// ID is the track "track" plays.
	ID string
	// Position and By are the seconds "seek" goes to, or moves by.
	Position *float64
	By       *float64
	// Percent and ByPercent are where "volume" goes, from 0 to 100, or
	// moves by.
	Percent   *float64
	ByPercent *float64
	// Repeat is what "repeat" sets: "off", "all", "one", or empty to
	// cycle.
	Repeat string
}

// statePayload is what /control/state and /control/events tell: what
// /now-playing tells, and the player's own state beside it.
type statePayload struct {
	payload
	Extra
}

// SetToken has /control take only the requests that carry token, as
// "Authorization: Bearer <token>". An empty token refuses every request,
// which is how the server starts.
func (s *Server) SetToken(token string) {
	s.mu.Lock()
	s.token = token
	s.mu.Unlock()
}

// serving reports whether the reads answer.
func (s *Server) serving() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serve
}

// controlGuard refuses a request of /control that mayn't come: while
// control is off, from a web page, without the token, or too fast.
func (s *Server) controlGuard(w http.ResponseWriter, r *http.Request) bool {
	s.mu.Lock()
	token, control := s.token, s.controlling
	s.mu.Unlock()
	if !control {
		http.Error(w, "not found", http.StatusNotFound)
		return false
	}
	if r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
		http.Error(w, "no web pages", http.StatusForbidden)
		return false
	}
	if !sameToken(bearer(r), token) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="MixMusic"`)
		http.Error(w, "a token is needed", http.StatusUnauthorized)
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if p := r.URL.Path; p != "/control/state" && p != "/control/events" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "this one takes commands", http.StatusMethodNotAllowed)
			return false
		}
	case http.MethodPost:
		if r.URL.Path == "/control/state" || r.URL.Path == "/control/events" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "this one only tells", http.StatusMethodNotAllowed)
			return false
		}
		if !s.pace.take(time.Now()) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "too many commands at once", http.StatusTooManyRequests)
			return false
		}
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "this takes commands and reads", http.StatusMethodNotAllowed)
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}

// bearer is the token a request carries.
func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}

// sameToken reports whether two tokens are equal, in a time that doesn't
// tell where they differ.
func sameToken(given, token string) bool {
	if token == "" {
		return false
	}
	g, t := sha256.Sum256([]byte(given)), sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(g[:], t[:]) == 1
}

// pace is how many commands may come: a burst at once, then a pace.
type pace struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// take reports whether another command may come now.
func (p *pace) take(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.last.IsZero() {
		p.tokens, p.last = controlBurst, now
	}
	p.tokens = min(controlBurst, p.tokens+now.Sub(p.last).Seconds()*controlPace)
	p.last = now
	if p.tokens < 1 {
		return false
	}
	p.tokens--
	return true
}

// controlState is what plays, with what the Control tells.
func (s *Server) controlState(host string) statePayload {
	out := statePayload{payload: s.snapshot(host)}
	if c := s.control(); c != nil {
		out.Extra = c.Extra()
	}
	if out.Queue == nil {
		out.Queue = []Track{}
	}
	for i, t := range out.Queue {
		if t.Artists == nil {
			out.Queue[i].Artists = []string{}
		}
	}
	return out
}

// control is the player, if the app gave one.
func (s *Server) control() Control {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Control
}

func (s *Server) controlStateRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.controlState(r.Host))
}

// controlBodyFields is what a command's JSON body may carry.
type controlBodyFields struct {
	ID              string   `json:"id"`
	PositionSeconds *float64 `json:"position_seconds"`
	BySeconds       *float64 `json:"by_seconds"`
	Percent         *float64 `json:"percent"`
	ByPercent       *float64 `json:"by_percent"`
	Repeat          string   `json:"repeat"`
}

// controlCommands are the routes of /control, after /control/.
var controlCommands = map[string]bool{
	"toggle": true, "play": true, "pause": true, "next": true, "previous": true,
	"mute": true, "shuffle": true, "repeat": true, "like": true, "dislike": true,
	"track": true, "seek": true, "volume": true,
}

func (s *Server) controlCommand(w http.ResponseWriter, r *http.Request) {
	op := strings.TrimPrefix(r.URL.Path, "/control/")
	if !controlCommands[op] {
		http.Error(w, "no such command", http.StatusNotFound)
		return
	}
	var f controlBodyFields
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, controlBody)).Decode(&f)
	var tooBig *http.MaxBytesError
	switch {
	case err == nil, errors.Is(err, io.EOF):
	case errors.As(err, &tooBig):
		http.Error(w, "a command carries 1 KiB at most", http.StatusRequestEntityTooLarge)
		return
	default:
		http.Error(w, "the body must be JSON", http.StatusBadRequest)
		return
	}
	c := Command{Op: op, ID: f.ID, Position: f.PositionSeconds, By: f.BySeconds, Percent: f.Percent,
		ByPercent: f.ByPercent, Repeat: f.Repeat}
	switch {
	case op == "track" && c.ID == "":
		http.Error(w, "id names the track to play", http.StatusBadRequest)
		return
	case op == "seek" && c.Position == nil && c.By == nil:
		http.Error(w, "position_seconds or by_seconds says where to go", http.StatusBadRequest)
		return
	case op == "volume" && c.Percent == nil && c.ByPercent == nil:
		http.Error(w, "percent or by_percent says how loud", http.StatusBadRequest)
		return
	case op == "repeat" && c.Repeat != "" && c.Repeat != "off" && c.Repeat != "all" && c.Repeat != "one":
		http.Error(w, `repeat is "off", "all" or "one"`, http.StatusBadRequest)
		return
	}
	ctl := s.control()
	if ctl == nil {
		writeControlError(w, ErrNoPlayer)
		return
	}
	if err := ctl.Do(c); err != nil {
		if errors.Is(err, ErrNothingLoaded) || errors.Is(err, ErrNoPlayer) || errors.Is(err, ErrNoTrack) {
			writeControlError(w, err)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Clients of the stream hear of it, whether the player told them or not.
	s.wake()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.controlState(r.Host))
}

// writeControlError answers a command that can't run, with the status
// that says whether to try again.
func writeControlError(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	if errors.Is(err, ErrNoPlayer) {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// controlEvents streams the state as Server-Sent Events: at once, then at
// each change of what plays, of the volume, of the queue and of the rest.
func (s *Server) controlEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	s.mu.Lock()
	done, cdone := s.done, s.ctlDone
	sub := &subscriber{wake: make(chan struct{}, 1)}
	if done != nil && cdone != nil {
		s.subs[sub] = struct{}{}
	}
	s.mu.Unlock()
	if done == nil || cdone == nil {
		http.Error(w, "stopped", http.StatusServiceUnavailable)
		return
	}
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
		data, _ := json.Marshal(s.controlState(r.Host))
		return send("event: state\ndata: %s\n\n", data)
	}
	if !send("retry: 3000\n\n") || !sendState() {
		return
	}
	beat := time.NewTicker(keepAlive)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-done:
			return
		case <-cdone:
			return
		case <-sub.wake:
			if !sendState() {
				return
			}
		case <-beat.C:
			if !send(": keep-alive\n\n") {
				return
			}
		}
	}
}
