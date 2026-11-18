package fcast

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
)

// These times are the session's; tests shorten them.
var (
	// pingAfter is how long the receiver says nothing before it's asked
	// whether it's there, and deadAfter how long before it's taken as
	// gone, as the protocol's reference does.
	pingAfter = 3 * time.Second
	deadAfter = 6 * time.Second
	// heartbeat is how often the session looks at how long it heard
	// nothing.
	heartbeat = 500 * time.Millisecond
)

// handshakeTimeout bounds starting a session, when its context doesn't.
const handshakeTimeout = 15 * time.Second

// endNear is how near the end of the media a v3 receiver that goes idle
// played it to its end.
const endNear = 4 * time.Second

var (
	// ErrNoKey is why MixMusic doesn't play on a receiver that announced
	// protocol v4 without the fingerprint of its key.
	ErrNoKey = errors.New("the receiver announced protocol v4 without the fingerprint of its key")
	// ErrKey is why it doesn't play on one whose key isn't the one it
	// announced: another device may stand in its place.
	ErrKey = errors.New("the receiver's key isn't the one it announced")
	// ErrDowngrade is why it doesn't play on one that announced v4 and
	// answered an older version, which would be unencrypted.
	ErrDowngrade = errors.New("the receiver announced protocol v4 but answered an older version")
	// ErrLost is why a session ends whose receiver stopped answering.
	ErrLost = errors.New("the receiver stopped answering")
	// ErrTaken is why a session ends that another app took over: it wraps
	// cast.ErrStopped.
	ErrTaken = fmt.Errorf("another app plays on the receiver now: %w", cast.ErrStopped)
)

// SessionOptions configure a session. Their zero value hears nothing of
// what the receiver does.
type SessionOptions struct {
	// OnMedia is told the status of the media loaded as it changes, as
	// cast.Session tells it, on the session's goroutine.
	OnMedia func(cast.MediaStatus)
	// OnVolume is told the receiver's volume, as it changes.
	OnVolume func(cast.Volume)
	// OnEnd is told, once, that the session ended without Close: another
	// app took the receiver over (ErrTaken), or it stopped answering.
	OnEnd func(err error)
	// App and Version name MixMusic to the receiver.
	App, Version string
}

// media is what a load asks the receiver to play.
type media struct {
	container, url              string
	start                       time.Duration
	speed                       float64
	title, image, artist, album string
}

// Session is MixMusic's session on a receiver. Its methods are safe for
// concurrent use; Load and Play wait for the receiver to play.
type Session struct {
	r       Receiver
	opts    SessionOptions
	conn    net.Conn
	version int
	limit   int
	// containers and audio are what a v4 receiver told it plays, as
	// tokens; known is set once it told.
	containers, audio []string
	known             bool

	wmu sync.Mutex

	mu   sync.Mutex
	ours func(string) bool
	// media counts the media loaded, as cast's media sessions; loaded,
	// while a load waits, is told how it went.
	media  int
	loaded chan error
	uri    string
	// holding is set while media loaded to wait paused waits for Play,
	// which loads it from start.
	holding  bool
	pending  media
	start    time.Duration
	played   bool
	idle     bool
	state    cast.PlayerState
	pos      time.Duration
	posAt    time.Time
	duration time.Duration
	rate     float64
	level    float64
	muted    bool
	sent     []sentVolume
	heard    time.Time
	pinged   bool
	ended    bool
	done     chan struct{}
}

// Open starts a session on r: v4 over TLS where r announces it, its key
// checked against its fingerprint, else v3.
func Open(ctx context.Context, r Receiver, opts SessionOptions) (*Session, error) {
	want := 3
	var fp []byte
	if r.Version >= firstVersionOfTLS {
		want = firstVersionOfTLS
		b, err := base64.StdEncoding.DecodeString(r.Fingerprint)
		if err != nil || len(b) != sha256.Size {
			return nil, ErrNoKey
		}
		fp = b
	}
	var d net.Dialer
	raw, err := d.DialContext(ctx, "tcp", r.Addr())
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			raw.Close()
		}
	}()
	deadline, has := ctx.Deadline()
	if !has {
		deadline = time.Now().Add(handshakeTimeout)
	}
	raw.SetDeadline(deadline)
	s := &Session{r: r, opts: opts, conn: raw, limit: maxPacketV3, rate: 1, level: 1, done: make(chan struct{})}
	if err := s.writeJSON(opVersion, map[string]int{"version": want}); err != nil {
		return nil, err
	}
	theirs, err := s.readVersion()
	if err != nil {
		return nil, err
	}
	if want >= firstVersionOfTLS {
		if theirs < firstVersionOfTLS {
			return nil, ErrDowngrade
		}
		tc := tls.Client(raw, &tls.Config{
			MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			// The receiver's certificate is its own, signed by itself: its
			// key is checked against the fingerprint it announced, and
			// TLS 1.3 checks it holds the key.
			InsecureSkipVerify: true,
			VerifyConnection: func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return ErrKey
				}
				sum := sha256.Sum256(cs.PeerCertificates[0].RawSubjectPublicKeyInfo)
				if !bytes.Equal(sum[:], fp) {
					return ErrKey
				}
				return nil
			},
		})
		if err := tc.HandshakeContext(ctx); err != nil {
			if errors.Is(err, ErrKey) {
				return nil, ErrKey
			}
			return nil, err
		}
		s.conn, s.version, s.limit = tc, firstVersionOfTLS, maxPacketV4
		if err := s.write(opFlatbuf, v4Introduction(opts.App, opts.Version)); err != nil {
			return nil, err
		}
		if err := s.readIntroduction(); err != nil {
			return nil, err
		}
	} else {
		s.version = min(3, max(theirs, 1))
		if s.version >= 3 {
			if err := s.writeJSON(opInitial, map[string]string{"displayName": opts.App, "appName": opts.App, "appVersion": opts.Version}); err != nil {
				return nil, err
			}
			// The end of each media, which a v3 receiver tells as an
			// event.
			if err := s.writeJSON(opSubscribeEvent, map[string]any{"event": map[string]int{"type": 1}}); err != nil {
				return nil, err
			}
		}
	}
	raw.SetDeadline(time.Time{})
	ok = true
	s.heard = time.Now()
	go s.read()
	go s.beat()
	return s, nil
}

// readVersion reads the packets of the receiver until its Version, and
// returns it.
func (s *Session) readVersion() (int, error) {
	for {
		p, err := readPacket(s.conn, s.limit)
		if err != nil {
			return 0, err
		}
		switch p.op {
		case opVersion:
			var v struct {
				Version int `json:"version"`
			}
			if err := json.Unmarshal(p.body, &v); err != nil {
				return 0, fmt.Errorf("%w: %v", errPacket, err)
			}
			return v.Version, nil
		case opPing:
			if err := s.write(opPong, nil); err != nil {
				return 0, err
			}
		}
	}
}

// readIntroduction reads the packets of a v4 receiver until its
// introduction, which tells what it plays.
func (s *Session) readIntroduction() error {
	for {
		p, err := readPacket(s.conn, s.limit)
		if err != nil {
			return err
		}
		switch p.op {
		case opPing:
			if err := s.write(opPong, nil); err != nil {
				return err
			}
		case opFlatbuf:
			m, err := readV4(p.body)
			if err != nil {
				return err
			}
			if m.typ == msgReceiverIntroduction {
				s.containers, s.audio, s.known = m.containers, m.audio, m.containers != nil || m.audio != nil
				return nil
			}
		}
	}
}

// Receiver returns the receiver the session is on.
func (s *Session) Receiver() Receiver { return s.r }

// Version returns the version of the protocol the session speaks.
func (s *Session) Version() int { return s.version }

// Plays reports whether the receiver told it plays the container, as
// "webm", and the audio format, as "opus", as v4 receivers tell: false
// for a v3 receiver, which tells nothing.
func (s *Session) Plays(container, audio string) bool {
	return s.known && slices.Contains(s.containers, container) && slices.Contains(s.audio, audio)
}

// PlaysAudio reports whether the receiver told it plays the audio format,
// as "mp3", of any container.
func (s *Session) PlaysAudio(audio string) bool { return s.known && slices.Contains(s.audio, audio) }

// LocalAddr returns the computer's address the receiver reaches it at.
func (s *Session) LocalAddr() net.Addr { return s.conn.LocalAddr() }

// RemoteIP returns the receiver's address.
func (s *Session) RemoteIP() net.IP {
	if a, ok := s.conn.RemoteAddr().(*net.TCPAddr); ok {
		return a.IP
	}
	return net.ParseIP(s.r.Host)
}

// SetOurs has the session tell which addresses are MixMusic's: once the
// receiver loads another, another app took it over, and the session ends.
func (s *Session) SetOurs(ours func(uri string) bool) {
	s.mu.Lock()
	s.ours = ours
	s.mu.Unlock()
}

// Volume returns the receiver's volume, as it last told it.
func (s *Session) Volume() cast.Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cast.Volume{Level: s.level, Muted: s.muted}
}

func (s *Session) write(op byte, body []byte) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return writePacket(s.conn, op, body)
}

func (s *Session) writeJSON(op byte, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.write(op, b)
}

// mediaOf is m as a load asks for it.
func mediaOf(m cast.Media, rate float64) media {
	return media{container: m.ContentType, url: m.URL, start: m.Start, speed: rate, title: m.Title, image: m.Image,
		artist: m.Artist, album: m.Album}
}

// sendLoad asks the receiver to play m.
func (s *Session) sendLoad(m media) error {
	if s.version >= firstVersionOfTLS {
		return s.write(opFlatbuf, v4Load(m))
	}
	play := map[string]any{"container": m.container, "url": m.url, "time": m.start.Seconds()}
	if m.speed != 0 && m.speed != 1 {
		play["speed"] = m.speed
	}
	meta := map[string]any{"type": 0}
	if m.title != "" {
		meta["title"] = m.title
	}
	if m.image != "" {
		meta["thumbnailUrl"] = m.image
	}
	play["metadata"] = meta
	return s.writeJSON(opPlay, play)
}

// Load has the receiver play m, and returns its first status: once it
// plays, or, for m not to play at once, as it waits for Play.
func (s *Session) Load(ctx context.Context, m cast.Media) (cast.MediaStatus, error) {
	s.mu.Lock()
	s.media++
	s.uri, s.holding, s.played, s.idle = m.URL, false, false, false
	s.pos, s.posAt, s.duration, s.state = m.Start, time.Now(), m.Duration, cast.Buffering
	if !m.Autoplay {
		s.holding, s.pending, s.start, s.state = true, mediaOf(m, s.rate), m.Start, cast.Paused
		st := s.statusLocked()
		s.mu.Unlock()
		return st, nil
	}
	mm := mediaOf(m, s.rate)
	s.mu.Unlock()
	return s.begin(ctx, mm)
}

// begin loads m on the receiver and waits until it plays.
func (s *Session) begin(ctx context.Context, m media) (cast.MediaStatus, error) {
	ch := make(chan error, 1)
	s.mu.Lock()
	s.loaded, s.holding = ch, false
	s.pos, s.posAt = m.start, time.Now()
	s.mu.Unlock()
	if err := s.sendLoad(m); err != nil {
		return cast.MediaStatus{}, err
	}
	var err error
	select {
	case err = <-ch:
	case <-ctx.Done():
		err = &cast.RequestError{Type: "LOAD_FAILED", Reason: "the receiver didn't play the media in time"}
	case <-s.done:
		err = ErrLost
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded == ch {
		s.loaded = nil
	}
	if err != nil {
		return cast.MediaStatus{}, err
	}
	return s.statusLocked(), nil
}

// Play plays the media loaded; media waiting for Play loads from where it
// was asked to start.
func (s *Session) Play(ctx context.Context) error {
	s.mu.Lock()
	holding, m := s.holding, s.pending
	m.start, m.speed = s.start, s.rate
	s.mu.Unlock()
	if holding {
		_, err := s.begin(ctx, m)
		if err != nil {
			s.tell(cast.Idle, cast.Failed)
		}
		return err
	}
	if s.version >= firstVersionOfTLS {
		return s.write(opFlatbuf, v4State(statePlaying))
	}
	return s.write(opResume, nil)
}

// Pause pauses it.
func (s *Session) Pause(context.Context) error {
	s.mu.Lock()
	holding := s.holding
	s.mu.Unlock()
	if holding {
		return nil
	}
	if s.version >= firstVersionOfTLS {
		return s.write(opFlatbuf, v4State(statePaused))
	}
	return s.write(opPause, nil)
}

// Stop stops it.
func (s *Session) Stop(context.Context) error {
	s.mu.Lock()
	s.holding, s.idle = false, true
	s.mu.Unlock()
	if s.version >= firstVersionOfTLS {
		return s.write(opFlatbuf, v4Stop())
	}
	return s.write(opStop, nil)
}

// Seek moves it to t.
func (s *Session) Seek(_ context.Context, t time.Duration) error {
	s.mu.Lock()
	holding, dur := s.holding, s.duration
	if holding {
		s.start = t
	} else {
		s.pos, s.posAt = t, time.Now()
	}
	s.mu.Unlock()
	switch {
	case holding:
		return nil
	case s.version >= firstVersionOfTLS:
		return s.write(opFlatbuf, v4Seek(t, dur))
	}
	return s.writeJSON(opSeek, map[string]float64{"time": t.Seconds()})
}

// SetRate plays it at rate, 1 its own speed.
func (s *Session) SetRate(_ context.Context, rate float64) error {
	s.mu.Lock()
	s.rate = rate
	holding := s.holding
	s.mu.Unlock()
	switch {
	case holding:
		return nil
	case s.version >= firstVersionOfTLS:
		return s.write(opFlatbuf, v4Speed(rate))
	}
	return s.writeJSON(opSetSpeed, map[string]float64{"speed": rate})
}

// Status does nothing: the receiver tells what it plays as it changes.
func (s *Session) Status(context.Context) error { return nil }

// echoFor is how long after MixMusic set the receiver's volume the
// receiver's word of that volume is taken as its echo.
const echoFor = 2 * time.Second

// sentVolume is a volume MixMusic set, whose echo is due.
type sentVolume struct {
	v  float32
	at time.Time
}

// sendVolume sets the receiver's volume to v, from 0 to 1.
func (s *Session) sendVolume(v float64) error {
	s.mu.Lock()
	s.sent = append(slices.DeleteFunc(s.sent, func(e sentVolume) bool { return time.Since(e.at) > echoFor }),
		sentVolume{float32(v), time.Now()})
	s.mu.Unlock()
	if s.version >= firstVersionOfTLS {
		return s.write(opFlatbuf, v4Volume(v))
	}
	return s.writeJSON(opSetVolume, map[string]float64{"volume": v})
}

// SetVolume sets the receiver's volume, from 0 to 1: while it's muted,
// the volume it gets back once unmuted.
func (s *Session) SetVolume(_ context.Context, level float64) error {
	level = min(max(level, 0), 1)
	s.mu.Lock()
	s.level = level
	muted := s.muted
	s.mu.Unlock()
	if muted {
		return nil
	}
	return s.sendVolume(level)
}

// SetMuted mutes the receiver, or unmutes it: FCast has no muting, so a
// muted receiver's volume is 0, and its own comes back as it's unmuted.
func (s *Session) SetMuted(_ context.Context, muted bool) error {
	s.mu.Lock()
	s.muted = muted
	level := s.level
	s.mu.Unlock()
	if muted {
		level = 0
	}
	return s.sendVolume(level)
}

// Close ends the session; stop also stops what plays. OnEnd isn't told.
func (s *Session) Close(stop bool) {
	s.mu.Lock()
	ended := s.ended
	s.ended = true
	s.mu.Unlock()
	if ended {
		return
	}
	if stop {
		s.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_ = s.Stop(context.Background())
	}
	s.conn.Close()
	close(s.done)
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
	s.conn.Close()
	close(s.done)
	if s.opts.OnEnd != nil {
		go s.opts.OnEnd(err)
	}
}

// beat asks the receiver whether it's there once it said nothing for
// pingAfter, and ends the session once it said nothing for deadAfter.
func (s *Session) beat() {
	t := time.NewTicker(heartbeat)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
		}
		s.mu.Lock()
		quiet := time.Since(s.heard)
		ping := quiet >= pingAfter && !s.pinged
		if ping {
			s.pinged = true
		}
		s.mu.Unlock()
		switch {
		case quiet >= deadAfter:
			s.end(ErrLost)
			return
		case ping:
			_ = s.write(opPing, nil)
		}
	}
}

// read reads what the receiver tells until the session ends.
func (s *Session) read() {
	for {
		p, err := readPacket(s.conn, s.limit)
		if err != nil {
			s.end(fmt.Errorf("%w: %v", ErrLost, err))
			return
		}
		s.mu.Lock()
		s.heard, s.pinged = time.Now(), false
		s.mu.Unlock()
		switch p.op {
		case opPing:
			_ = s.write(opPong, nil)
		case opFlatbuf:
			if s.version >= firstVersionOfTLS {
				if m, err := readV4(p.body); err == nil {
					s.onV4(m)
				}
			}
		default:
			if s.version < firstVersionOfTLS {
				s.onV3(p)
			}
		}
	}
}

// onV4 takes in a v4 message.
func (s *Session) onV4(m v4Message) {
	switch m.typ {
	case msgPlaybackStateChanged:
		switch m.state {
		case stateIdle:
			s.stopped(false)
		case stateBuffering:
			s.playerState(cast.Buffering)
		case statePlaying:
			s.playerState(cast.Playing)
		case statePaused:
			s.playerState(cast.Paused)
		case stateEnded:
			s.stopped(true)
		}
	case msgProgressChanged:
		s.progress(m.position, m.duration, m.hasDuration)
	case msgVolumeChanged:
		s.volumeTold(m.volume)
	case msgSpeedChanged:
		s.mu.Lock()
		if m.speed > 0 {
			s.rate = m.speed
		}
		s.mu.Unlock()
	case msgLoad:
		s.loadTold(m.url)
	case msgStopPlayback:
		s.stopped(false)
	case msgError:
		s.failed(fmt.Sprintf("error %d", m.errorKind), m.errorKind == errorUnsupportedFormat || m.errorKind == errorResourceNotFound)
	}
}

// onV3 takes in a v3 packet.
func (s *Session) onV3(p packet) {
	switch p.op {
	case opPlaybackUpdate:
		var u struct {
			State    int      `json:"state"`
			Time     *float64 `json:"time"`
			Duration *float64 `json:"duration"`
			Speed    *float64 `json:"speed"`
		}
		if json.Unmarshal(p.body, &u) != nil {
			return
		}
		if u.Speed != nil && *u.Speed > 0 {
			s.mu.Lock()
			s.rate = *u.Speed
			s.mu.Unlock()
		}
		if u.Time != nil {
			var d float64
			if u.Duration != nil {
				d = *u.Duration
			}
			s.progress(seconds(*u.Time), seconds(d), u.Duration != nil && d > 0)
		}
		switch u.State {
		case 0:
			s.mu.Lock()
			end := s.duration > 0 && s.reachLocked() >= s.duration-endNear
			s.mu.Unlock()
			s.stopped(end)
		case 1:
			s.playerState(cast.Playing)
		case 2:
			s.playerState(cast.Paused)
		}
	case opVolumeUpdate:
		var u struct {
			Volume float64 `json:"volume"`
		}
		if json.Unmarshal(p.body, &u) == nil {
			s.volumeTold(u.Volume)
		}
	case opPlaybackError:
		var u struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(p.body, &u)
		s.failed(u.Message, true)
	case opPlayUpdate:
		var u struct {
			PlayData *struct {
				URL string `json:"url"`
			} `json:"playData"`
		}
		if json.Unmarshal(p.body, &u) == nil && u.PlayData != nil {
			s.loadTold(u.PlayData.URL)
		}
	case opEvent:
		var u struct {
			Event struct {
				Type int `json:"type"`
			} `json:"event"`
		}
		if json.Unmarshal(p.body, &u) == nil && u.Event.Type == 1 { // MediaItemEnd
			s.stopped(true)
		}
	}
}

func seconds(v float64) time.Duration {
	if v <= 0 {
		return 0
	}
	return time.Duration(v * float64(time.Second))
}

// reachLocked is where the media is now, moved on at its speed while it
// plays. s.mu is held.
func (s *Session) reachLocked() time.Duration {
	pos := s.pos
	if s.state == cast.Playing {
		pos += time.Duration(float64(time.Since(s.posAt)) * max(s.rate, 0))
	}
	return pos
}

// playerState takes in the receiver's player playing, paused or
// buffering: a load waiting is done once it plays.
func (s *Session) playerState(state cast.PlayerState) {
	s.mu.Lock()
	if s.holding || s.media == 0 || s.idle && s.loaded == nil {
		s.mu.Unlock()
		return
	}
	if s.state == cast.Playing && state != cast.Playing {
		s.pos, s.posAt = s.reachLocked(), time.Now()
	} else if state == cast.Playing && s.state != cast.Playing {
		s.posAt = time.Now()
	}
	s.state = state
	if state == cast.Playing || state == cast.Paused && s.played {
		s.played = true
		if s.loaded != nil {
			s.loaded <- nil
			s.loaded = nil
		}
	}
	waiting := s.loaded != nil
	st := s.statusLocked()
	s.mu.Unlock()
	if !waiting {
		s.emit(st)
	}
}

// progress takes in where the media is, and its length when told.
func (s *Session) progress(pos, duration time.Duration, hasDuration bool) {
	s.mu.Lock()
	if s.holding || s.media == 0 || s.idle {
		s.mu.Unlock()
		return
	}
	s.pos, s.posAt = pos, time.Now()
	if hasDuration && duration > 0 {
		s.duration = duration
	}
	waiting := s.loaded != nil
	st := s.statusLocked()
	s.mu.Unlock()
	if !waiting && st.State != cast.Buffering {
		s.emit(st)
	}
}

// stopped takes in the receiver going idle: at the media's end, or as it
// stopped otherwise. A load waiting isn't done by it.
func (s *Session) stopped(atEnd bool) {
	s.mu.Lock()
	if s.holding || s.media == 0 || s.idle || s.loaded != nil || !s.played {
		s.mu.Unlock()
		return
	}
	reason := cast.Cancelled
	if atEnd {
		reason = cast.Finished
		if s.duration > 0 {
			s.pos = s.duration
		}
	} else {
		s.pos = s.reachLocked()
	}
	s.posAt, s.state, s.idle = time.Now(), cast.Idle, true
	st := s.statusLocked()
	st.IdleReason = reason
	s.mu.Unlock()
	s.emit(st)
}

// failed takes in the receiver's error: a load waiting fails of it, and
// media playing stops, when it is of the media.
func (s *Session) failed(why string, ofMedia bool) {
	s.mu.Lock()
	if ch := s.loaded; ch != nil {
		s.loaded = nil
		s.mu.Unlock()
		ch <- &cast.RequestError{Type: "LOAD_FAILED", Reason: why}
		return
	}
	s.mu.Unlock()
	if ofMedia {
		s.tell(cast.Idle, cast.Failed)
	}
}

// loadTold takes in media loaded on the receiver, by MixMusic or another
// app: once it isn't MixMusic's, the session ends.
func (s *Session) loadTold(url string) {
	s.mu.Lock()
	ours := url == "" || url == s.uri || s.ours == nil || s.ours(url)
	s.mu.Unlock()
	if !ours {
		s.end(ErrTaken)
	}
}

// volumeTold takes in the receiver's volume: the echo of one MixMusic
// set, or its own, as from its remote, which unmutes it.
func (s *Session) volumeTold(v float64) {
	v = min(max(v, 0), 1)
	s.mu.Lock()
	if i := slices.IndexFunc(s.sent, func(e sentVolume) bool { return e.v == float32(v) && time.Since(e.at) <= echoFor }); i >= 0 {
		s.sent = slices.Delete(s.sent, 0, i+1)
		s.mu.Unlock()
		return
	}
	before := cast.Volume{Level: s.level, Muted: s.muted}
	s.level, s.muted = v, false
	after := cast.Volume{Level: v}
	s.mu.Unlock()
	if after != before && s.opts.OnVolume != nil {
		s.opts.OnVolume(after)
	}
}

// statusLocked is the media's status as the session knows it. s.mu is
// held.
func (s *Session) statusLocked() cast.MediaStatus {
	rate := s.rate
	if rate <= 0 {
		rate = 1
	}
	return cast.MediaStatus{Session: s.media, State: s.state, Position: s.pos, Rate: rate, At: s.posAt,
		Duration: s.duration, ContentID: s.uri}
}

// tell tells OnMedia the media is in state, for reason.
func (s *Session) tell(state cast.PlayerState, reason string) {
	s.mu.Lock()
	s.state, s.idle = state, state == cast.Idle
	st := s.statusLocked()
	st.IdleReason = reason
	s.mu.Unlock()
	s.emit(st)
}

func (s *Session) emit(st cast.MediaStatus) {
	if s.opts.OnMedia != nil {
		s.opts.OnMedia(st)
	}
}
