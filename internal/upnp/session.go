package upnp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
)

// These times are the session's; tests shorten them.
var (
	// pollPlaying is how often the renderer is asked where it is while it
	// plays, and pollOther while it doesn't, to hear soon that it does.
	pollPlaying = time.Second
	pollOther   = 600 * time.Millisecond
	// volumeEvery is how often its volume is read.
	volumeEvery = 5 * time.Second
	// loadPoll is how often a load asks whether the renderer plays yet.
	loadPoll = 250 * time.Millisecond
	// actionTimeout bounds an action the session sends by itself.
	actionTimeout = 5 * time.Second
)

// maxFailedPolls is how many polls in a row fail before the renderer is
// taken as gone, as a lost Google Cast session is.
const maxFailedPolls = 10

// endNear is how near the end of the media a renderer that stops played
// it to its end: the next track loads then.
const endNear = 4 * time.Second

// ErrLost is why a session ends whose renderer stopped answering.
var ErrLost = errors.New("the renderer stopped answering")

// ErrNoSpeed is why the renderer doesn't play at another speed: it turned
// the speed down.
var ErrNoSpeed = errors.New("the renderer plays only at its own speed")

// errNoVolume is why the volume of a renderer without RenderingControl
// doesn't change.
var errNoVolume = errors.New("the renderer has no volume MixMusic sets")

// SessionOptions configure a session. Their zero value hears nothing of
// what the renderer does.
type SessionOptions struct {
	// OnMedia is told the status of the media loaded, each time the
	// renderer is asked, as cast.Session tells it: on the session's
	// goroutine.
	OnMedia func(cast.MediaStatus)
	// OnVolume is told the renderer's volume, as it changes.
	OnVolume func(cast.Volume)
	// OnEnd is told, once, that the session ended without Close: another
	// app took the renderer over (ErrTaken, which wraps cast.ErrStopped),
	// or it stopped answering (ErrLost).
	OnEnd func(err error)
	// HTTP sends the actions: a client of its own when nil.
	HTTP *http.Client
}

// Session is MixMusic's session on a renderer: the media loaded in its
// AVTransport, which it is asked about every pollPlaying while it plays.
// Its methods are safe for concurrent use; those that take a context wait
// for the renderer's answer.
type Session struct {
	r      Renderer
	opts   SessionOptions
	client *http.Client
	local  net.Addr
	sink   Sink

	ctx    context.Context
	cancel context.CancelFunc
	poke   chan struct{}

	mu sync.Mutex
	// ours reports whether an address the renderer plays is MixMusic's.
	ours func(string) bool
	// media counts the media loaded, as cast's media sessions: the
	// renderer moving on to the next by itself starts one too. loading is
	// set while a load waits for the renderer.
	media   int
	loading bool
	// uri is the address of the media, and next the one queued after it;
	// advanced is set once the renderer moved on to next by itself, which
	// the next Load of it takes in place of loading it.
	uri, next string
	advanced  bool
	// holding is set while media loaded to wait paused waits, stopped,
	// for Play, which plays it from start.
	holding bool
	start   time.Duration
	// played is set once the media played; idle once the renderer was
	// told idle, which it is told once.
	played, idle bool
	state        cast.PlayerState
	pos          time.Duration
	posAt        time.Time
	duration     time.Duration
	// rate is the speed asked, and noSpeed set once the renderer turned
	// one down.
	rate    float64
	noSpeed bool
	volume  cast.Volume
	ended   bool
	fails   int
}

// Open starts a session on r: it asks what it plays and its volume, and
// starts asking where it is.
func Open(ctx context.Context, r Renderer, opts SessionOptions) (*Session, error) {
	client := opts.HTTP
	if client == nil {
		client = &http.Client{}
	}
	local, err := localAddrFor(r)
	if err != nil {
		return nil, err
	}
	s := &Session{r: r, opts: opts, client: client, local: local, poke: make(chan struct{}, 1), rate: 1,
		volume: cast.Volume{Level: 1, Fixed: r.RenderingControl.Control == ""}}
	// It answers, or isn't there.
	if _, err := call(ctx, client, r.AVTransport, "GetTransportInfo", arg{"InstanceID", "0"}); err != nil {
		return nil, err
	}
	if r.ConnectionManager.Control != "" {
		if a, err := call(ctx, client, r.ConnectionManager, "GetProtocolInfo"); err == nil {
			s.sink = parseSink(a["Sink"])
		}
	}
	if v, ok := s.readVolume(ctx); ok {
		s.volume = v
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.run()
	return s, nil
}

// localAddrFor returns the computer's address that reaches r: the system
// picks it as it routes there, sending nothing.
func localAddrFor(r Renderer) (net.Addr, error) {
	port := "80"
	if u, err := url.Parse(r.AVTransport.Control); err == nil && u.Port() != "" {
		port = u.Port()
	}
	c, err := net.Dial("udp4", net.JoinHostPort(r.Host, port))
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return c.LocalAddr(), nil
}

// Renderer returns the renderer the session is on.
func (s *Session) Renderer() Renderer { return s.r }

// LocalAddr returns the computer's address the renderer reaches it at.
func (s *Session) LocalAddr() net.Addr { return s.local }

// RemoteIP returns the renderer's address.
func (s *Session) RemoteIP() net.IP { return net.ParseIP(s.r.Host) }

// Sink returns what the renderer plays, as it told: nil when it didn't.
func (s *Session) Sink() Sink { return s.sink }

// SetOurs has the session tell which addresses are MixMusic's: once the
// renderer plays another, another app took it over, and the session ends.
func (s *Session) SetOurs(ours func(uri string) bool) {
	s.mu.Lock()
	s.ours = ours
	s.mu.Unlock()
}

// Volume returns the renderer's volume, as it last told it.
func (s *Session) Volume() cast.Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.volume
}

// SpeedRefused reports whether the renderer turned down a speed: it plays
// at its own then.
func (s *Session) SpeedRefused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.noSpeed
}

func (s *Session) action(ctx context.Context, name string, args ...arg) (map[string]string, error) {
	return call(ctx, s.client, s.r.AVTransport, name, append([]arg{{"InstanceID", "0"}}, args...)...)
}

// loadError is err, of an action of a load, as cast's: a renderer that
// turned the media down failed to load it.
func loadError(err error) error {
	var f *Fault
	if errors.As(err, &f) {
		return &cast.RequestError{Type: "LOAD_FAILED", Reason: f.Error()}
	}
	return err
}

// Load has the renderer play m, with its metadata, and returns its first
// status: once it plays, or, for m not to play at once, as it waits
// stopped. The media the renderer moved on to by itself, from the queue,
// isn't loaded again.
func (s *Session) Load(ctx context.Context, m cast.Media) (cast.MediaStatus, error) {
	s.mu.Lock()
	if s.advanced && m.URL == s.uri {
		s.advanced = false
		st := s.statusLocked()
		s.mu.Unlock()
		return st, nil
	}
	s.media++
	media := s.media
	s.loading = true
	s.uri, s.next, s.advanced = m.URL, "", false
	s.holding, s.played, s.idle = false, false, false
	s.pos, s.posAt, s.duration, s.state = m.Start, time.Now(), m.Duration, cast.Buffering
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.media == media {
			s.loading = false
		}
		s.mu.Unlock()
	}()
	if _, err := s.action(ctx, "SetAVTransportURI", arg{"CurrentURI", m.URL}, arg{"CurrentURIMetaData", didl(m)}); err != nil {
		return cast.MediaStatus{}, loadError(err)
	}
	if !m.Autoplay {
		s.mu.Lock()
		s.holding, s.start, s.state = true, m.Start, cast.Paused
		st := s.statusLocked()
		s.mu.Unlock()
		return st, nil
	}
	if err := s.begin(ctx, m.Start); err != nil {
		return cast.MediaStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked(), nil
}

// begin has the renderer play the media loaded, from from, and waits until
// it does. A renderer that answers but doesn't play by ctx's end failed
// to load it.
func (s *Session) begin(ctx context.Context, from time.Duration) error {
	if err := s.play(ctx); err != nil {
		return loadError(err)
	}
	for {
		a, err := s.action(ctx, "GetTransportInfo")
		switch {
		case ctx.Err() != nil:
			return &cast.RequestError{Type: "LOAD_FAILED", Reason: "the renderer didn't play the media in time"}
		case err != nil:
			return err
		}
		switch a["CurrentTransportState"] {
		case "PLAYING", "PAUSED_PLAYBACK":
			if from > time.Second {
				if err := s.seek(ctx, from); err != nil {
					from = 0
				}
			}
			s.mu.Lock()
			s.holding, s.played, s.idle = false, true, false
			s.state, s.pos, s.posAt = cast.Playing, from, time.Now()
			s.mu.Unlock()
			return nil
		case "STOPPED", "NO_MEDIA_PRESENT":
			if a["CurrentTransportStatus"] == "ERROR_OCCURRED" {
				return &cast.RequestError{Type: "LOAD_FAILED", Reason: "the renderer couldn't play the media"}
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(loadPoll):
		}
	}
}

// play sends Play, at the speed asked: a renderer that turns the speed
// down plays at its own.
func (s *Session) play(ctx context.Context) error {
	s.mu.Lock()
	rate, refused := s.rate, s.noSpeed
	s.mu.Unlock()
	if rate != 1 && !refused {
		_, err := s.action(ctx, "Play", arg{"Speed", speedText(rate)})
		var f *Fault
		if !errors.As(err, &f) {
			return err
		}
		s.mu.Lock()
		s.noSpeed = true
		s.mu.Unlock()
	}
	_, err := s.action(ctx, "Play", arg{"Speed", "1"})
	return err
}

// speedText writes a speed as TransportPlaySpeed has it.
func speedText(rate float64) string {
	return strconv.FormatFloat(math.Round(rate*100)/100, 'f', -1, 64)
}

// Play plays the media loaded; media waiting paused from where it loaded
// plays from there.
func (s *Session) Play(ctx context.Context) error {
	s.mu.Lock()
	holding, from := s.holding, s.start
	s.mu.Unlock()
	if !holding {
		return s.play(ctx)
	}
	err := s.begin(ctx, from)
	if err != nil {
		s.tell(cast.Idle, cast.Failed)
	}
	return err
}

// Pause pauses it.
func (s *Session) Pause(ctx context.Context) error {
	s.mu.Lock()
	holding := s.holding
	s.mu.Unlock()
	if holding {
		return nil
	}
	_, err := s.action(ctx, "Pause")
	return err
}

// Stop stops it.
func (s *Session) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.holding, s.next, s.advanced, s.idle = false, "", false, true
	s.mu.Unlock()
	_, err := s.action(ctx, "Stop")
	return err
}

// Seek moves it to t.
func (s *Session) Seek(ctx context.Context, t time.Duration) error {
	s.mu.Lock()
	holding := s.holding
	if holding {
		s.start = t
	}
	s.mu.Unlock()
	if holding {
		return nil
	}
	if err := s.seek(ctx, t); err != nil {
		return err
	}
	s.mu.Lock()
	s.pos, s.posAt = t, time.Now()
	s.mu.Unlock()
	return nil
}

// seek moves the media to t, by its relative time, or its absolute time
// on a renderer that seeks only so.
func (s *Session) seek(ctx context.Context, t time.Duration) error {
	_, err := s.action(ctx, "Seek", arg{"Unit", "REL_TIME"}, arg{"Target", clock(t)})
	var f *Fault
	if errors.As(err, &f) && f.Code == 710 { // seek mode not supported
		_, err = s.action(ctx, "Seek", arg{"Unit", "ABS_TIME"}, arg{"Target", clock(t)})
	}
	return err
}

// SetRate plays it at rate, 1 its own speed: ErrNoSpeed once the renderer
// turned another down.
func (s *Session) SetRate(ctx context.Context, rate float64) error {
	s.mu.Lock()
	if s.noSpeed && rate != 1 {
		s.mu.Unlock()
		return ErrNoSpeed
	}
	same := s.rate == rate
	s.rate = rate
	playing := s.state == cast.Playing && !s.holding && s.uri != ""
	s.mu.Unlock()
	if same || !playing {
		return nil
	}
	if err := s.play(ctx); err != nil {
		return err
	}
	if s.SpeedRefused() && rate != 1 {
		return ErrNoSpeed
	}
	return nil
}

// Status asks the renderer where it is at once, which OnMedia is told.
func (s *Session) Status(context.Context) error {
	select {
	case s.poke <- struct{}{}:
	default:
	}
	return nil
}

// SetVolume sets the renderer's volume, from 0 to 1.
func (s *Session) SetVolume(ctx context.Context, level float64) error {
	if s.r.RenderingControl.Control == "" {
		return errNoVolume
	}
	level = min(max(level, 0), 1)
	_, err := call(ctx, s.client, s.r.RenderingControl, "SetVolume", arg{"InstanceID", "0"}, arg{"Channel", "Master"},
		arg{"DesiredVolume", strconv.Itoa(int(math.Round(level * 100)))})
	if err == nil {
		s.mu.Lock()
		s.volume.Level = math.Round(level*100) / 100
		s.mu.Unlock()
	}
	return err
}

// SetMuted mutes the renderer, or unmutes it.
func (s *Session) SetMuted(ctx context.Context, muted bool) error {
	if s.r.RenderingControl.Control == "" {
		return errNoVolume
	}
	v := "0"
	if muted {
		v = "1"
	}
	_, err := call(ctx, s.client, s.r.RenderingControl, "SetMute", arg{"InstanceID", "0"}, arg{"Channel", "Master"},
		arg{"DesiredMute", v})
	if err == nil {
		s.mu.Lock()
		s.volume.Muted = muted
		s.mu.Unlock()
	}
	return err
}

// Next hands the renderer m to play once the media loaded ends, with no
// gap between them: SetNextAVTransportURI, which not every renderer has.
func (s *Session) Next(ctx context.Context, m cast.Media) error {
	if _, err := s.action(ctx, "SetNextAVTransportURI", arg{"NextURI", m.URL}, arg{"NextURIMetaData", didl(m)}); err != nil {
		return err
	}
	s.mu.Lock()
	s.next = m.URL
	s.mu.Unlock()
	return nil
}

// Close ends the session; stop also stops the renderer. OnEnd isn't told.
func (s *Session) Close(stop bool) {
	s.mu.Lock()
	ended := s.ended
	s.ended = true
	s.mu.Unlock()
	if ended {
		return
	}
	s.cancel()
	if stop {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = s.action(ctx, "Stop")
	}
}

// end ends the session for err, unless it ended, and tells OnEnd.
func (s *Session) end(err error) {
	s.mu.Lock()
	ended := s.ended
	s.ended = true
	s.mu.Unlock()
	if ended {
		return
	}
	s.cancel()
	if s.opts.OnEnd != nil {
		go s.opts.OnEnd(err)
	}
}

// run asks the renderer where it is, and its volume, until the session
// ends.
func (s *Session) run() {
	volumeAt := time.Now()
	for {
		s.mu.Lock()
		wait := pollOther
		if s.state == cast.Playing {
			wait = pollPlaying
		}
		s.mu.Unlock()
		t := time.NewTimer(wait)
		select {
		case <-s.ctx.Done():
			t.Stop()
			return
		case <-t.C:
		case <-s.poke:
			t.Stop()
		}
		s.poll()
		if time.Since(volumeAt) >= volumeEvery {
			volumeAt = time.Now()
			s.pollVolume()
		}
	}
}

// poll asks the renderer what it plays and where, and tells OnMedia.
func (s *Session) poll() {
	ctx, cancel := context.WithTimeout(s.ctx, actionTimeout)
	defer cancel()
	ti, err := s.action(ctx, "GetTransportInfo")
	var pi map[string]string
	if err == nil {
		pi, err = s.action(ctx, "GetPositionInfo")
	}
	if s.ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	if err != nil {
		s.fails++
		lost := s.fails >= maxFailedPolls
		s.mu.Unlock()
		if lost {
			s.end(fmt.Errorf("%w: %v", ErrLost, err))
		}
		return
	}
	s.fails = 0
	if s.media == 0 || s.loading {
		s.mu.Unlock()
		return
	}
	track := strings.TrimSpace(pi["TrackURI"])
	if track != "" && track != "NOT_IMPLEMENTED" && s.ours != nil && !s.ours(track) {
		s.mu.Unlock()
		s.end(fmt.Errorf("%w: %w", ErrTaken, cast.ErrStopped))
		return
	}
	var sts []cast.MediaStatus
	if s.next != "" && track == s.next && s.next != s.uri {
		// The renderer moved on to the media queued: what played ended.
		sts = append(sts, cast.MediaStatus{Session: s.media, State: cast.Idle, IdleReason: cast.Finished, Rate: 1,
			At: time.Now(), Position: s.duration, Duration: s.duration})
		s.media++
		s.uri, s.next, s.advanced = s.next, "", true
		s.played, s.idle, s.duration = true, false, 0
	}
	if st, ok := s.read(ti, pi); ok {
		sts = append(sts, st)
	}
	s.mu.Unlock()
	for _, st := range sts {
		if s.opts.OnMedia != nil {
			s.opts.OnMedia(st)
		}
	}
}

// read takes in what the renderer told of its transport and position, and
// returns the status of the media, unless there is none to tell. s.mu is
// held.
func (s *Session) read(ti, pi map[string]string) (cast.MediaStatus, bool) {
	if d, ok := parseClock(pi["TrackDuration"]); ok && d > 0 {
		s.duration = d
	}
	pos, posOK := parseClock(pi["RelTime"])
	now := time.Now()
	switch ti["CurrentTransportState"] {
	case "PLAYING":
		s.state, s.holding, s.played, s.idle = cast.Playing, false, true, false
	case "PAUSED_PLAYBACK", "PAUSED_RECORDING":
		s.state = cast.Paused
	case "TRANSITIONING":
		s.state = cast.Buffering
	case "STOPPED", "NO_MEDIA_PRESENT":
		switch {
		case s.holding:
			s.state, s.pos, s.posAt = cast.Paused, s.start, now
			return s.statusLocked(), true
		case !s.played || s.idle:
			return cast.MediaStatus{}, false
		}
		// It stopped by itself: at the end, when it was near it, else as
		// its own remote stopped it, or as it failed.
		reach := s.pos
		if s.state == cast.Playing {
			reach += time.Duration(float64(now.Sub(s.posAt)) * s.rateLocked())
		}
		reason := cast.Cancelled
		switch {
		case ti["CurrentTransportStatus"] == "ERROR_OCCURRED":
			reason = cast.Failed
		case s.duration > 0 && reach >= s.duration-endNear:
			reason = cast.Finished
		}
		if s.duration > 0 {
			reach = min(reach, s.duration)
		}
		s.state, s.idle, s.pos, s.posAt = cast.Idle, true, reach, now
		st := s.statusLocked()
		st.IdleReason = reason
		return st, true
	default:
		return cast.MediaStatus{}, false
	}
	if posOK {
		s.pos, s.posAt = pos, now
	}
	return s.statusLocked(), true
}

// rateLocked is the speed the media plays at. s.mu is held.
func (s *Session) rateLocked() float64 {
	if s.noSpeed || s.rate <= 0 {
		return 1
	}
	return s.rate
}

// statusLocked is the media's status as the session knows it. s.mu is
// held.
func (s *Session) statusLocked() cast.MediaStatus {
	return cast.MediaStatus{Session: s.media, State: s.state, Position: s.pos, Rate: s.rateLocked(), At: s.posAt,
		Duration: s.duration, ContentID: s.uri}
}

// tell tells OnMedia the media is in state, for reason.
func (s *Session) tell(state cast.PlayerState, reason string) {
	s.mu.Lock()
	s.state, s.idle = state, state == cast.Idle
	st := s.statusLocked()
	st.IdleReason = reason
	s.mu.Unlock()
	if s.opts.OnMedia != nil {
		s.opts.OnMedia(st)
	}
}

// pollVolume reads the renderer's volume, and tells OnVolume when it
// changed.
func (s *Session) pollVolume() {
	ctx, cancel := context.WithTimeout(s.ctx, actionTimeout)
	defer cancel()
	v, ok := s.readVolume(ctx)
	if !ok {
		return
	}
	s.mu.Lock()
	changed := v != s.volume
	s.volume = v
	s.mu.Unlock()
	if changed && s.opts.OnVolume != nil {
		s.opts.OnVolume(v)
	}
}

// readVolume reads the renderer's volume and whether it's muted, from its
// RenderingControl.
func (s *Session) readVolume(ctx context.Context) (cast.Volume, bool) {
	rc := s.r.RenderingControl
	if rc.Control == "" {
		return cast.Volume{}, false
	}
	a, err := call(ctx, s.client, rc, "GetVolume", arg{"InstanceID", "0"}, arg{"Channel", "Master"})
	if err != nil {
		return cast.Volume{}, false
	}
	level, err := strconv.Atoi(strings.TrimSpace(a["CurrentVolume"]))
	if err != nil {
		return cast.Volume{}, false
	}
	v := cast.Volume{Level: min(max(float64(level)/100, 0), 1)}
	if m, err := call(ctx, s.client, rc, "GetMute", arg{"InstanceID", "0"}, arg{"Channel", "Master"}); err == nil {
		mute := strings.TrimSpace(m["CurrentMute"])
		v.Muted = mute == "1" || strings.EqualFold(mute, "true")
	}
	return v, true
}
