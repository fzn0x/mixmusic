package cast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// PlayerState is what the receiver's player does.
type PlayerState string

const (
	Idle      PlayerState = "IDLE"
	Playing   PlayerState = "PLAYING"
	Paused    PlayerState = "PAUSED"
	Buffering PlayerState = "BUFFERING"
)

// Why a player is Idle, as MediaStatus.IdleReason tells it.
const (
	// Finished: the media played to its end.
	Finished = "FINISHED"
	// Failed: the media couldn't play on.
	Failed = "ERROR"
	// Cancelled: a sender stopped it.
	Cancelled = "CANCELLED"
	// Interrupted: another load took its place.
	Interrupted = "INTERRUPTED"
)

// MediaStatus is what the device's receiver plays, as it tells it.
type MediaStatus struct {
	// Session is the media session the status is of: each Load starts one.
	Session int
	State   PlayerState
	// IdleReason tells why an Idle player is, as Finished or Failed, or is
	// "" for one that never played.
	IdleReason string
	// Position is where the media was At, which moves on at Rate while it
	// plays.
	Position time.Duration
	Rate     float64
	At       time.Time
	// Duration is the media's length, 0 until the device tells it.
	Duration time.Duration
	// ContentID is the URL of the media, when the status tells it.
	ContentID string
}

// Volume is a device's volume.
type Volume struct {
	// Level is from 0 to 1.
	Level float64
	Muted bool
	// Fixed is set for a device that keeps its volume as it is, as a TV
	// whose speakers its own remote sets.
	Fixed bool
}

// Media is what Load has the device play.
type Media struct {
	// URL is where the device fetches the media, and ContentType its MIME
	// type, as "audio/webm".
	URL, ContentType string
	// Title, Artist, Album and Image show on the device's screen, Image
	// as the URL of the cover.
	Title, Artist, Album, Image string
	// Duration is the media's length, when known.
	Duration time.Duration
	// Start is where it starts; Autoplay has it play from there, rather
	// than wait there paused.
	Start    time.Duration
	Autoplay bool
}

// SessionOptions configure a session. Their zero value hears nothing of
// what the device does.
type SessionOptions struct {
	// OnMedia is told each status of the media of the session, as it
	// loads, plays, pauses, ends or fails, by the session's commands or by
	// the device's own controls. Statuses of media a later Load took over
	// from don't come.
	OnMedia func(MediaStatus)
	// OnVolume is told the device's volume, as it changes.
	OnVolume func(Volume)
	// OnEnd is told, once, that the session ended without Close: the
	// receiver stopped, another app took the device over, or the
	// connection was lost. err tells why.
	OnEnd func(err error)
}

// Session is MixMusic's session on a cast device: its default media
// receiver, which Open launches, or joins where it runs already, and the
// media loaded in it. Its methods are safe for concurrent use; those that
// take a context wait for the device's answer.
type Session struct {
	device Device
	c      *conn
	opts   SessionOptions

	mu sync.Mutex
	// app and transport are the receiver's session, and the destination of
	// its messages.
	app, transport string
	// media is the media session of the last load, and loading the
	// requestId of a load under way, whose answer tells its media session.
	media   int
	loading int
	volume  Volume
	ended   bool
}

// ErrStopped is why a session ends that the device ended: its receiver
// stopped, as from another sender or by its own timeout.
var ErrStopped = errors.New("the cast device stopped playing from MixMusic")

// errNoMedia is why a command of the media fails before anything loaded.
var errNoMedia = errors.New("cast: nothing loaded")

// launchWait bounds how long the device takes to start its receiver;
// tests shorten launchPoll.
var (
	launchWait = 15 * time.Second
	launchPoll = 500 * time.Millisecond
)

// Open starts a session on d: it connects, and launches the default media
// receiver, or joins it where it runs.
func Open(ctx context.Context, d Device, opts SessionOptions) (*Session, error) {
	s := &Session{device: d, opts: opts}
	c, err := dial(ctx, d.Addr(), s.onMessage)
	if err != nil {
		return nil, err
	}
	s.c = c
	ok := false
	defer func() {
		if !ok {
			c.close(ErrClosed)
		}
	}()
	if err := c.connect(receiverID); err != nil {
		return nil, err
	}
	st, err := s.launch(ctx)
	if err != nil {
		return nil, err
	}
	app := st.app(DefaultMediaReceiver)
	if err := c.connect(app.TransportID); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.app, s.transport = app.SessionID, app.TransportID
	s.volume = st.Volume.volume()
	s.mu.Unlock()
	ok = true
	go func() {
		<-c.closed
		s.end(c.reason())
	}()
	return s, nil
}

// Device returns the device the session is on.
func (s *Session) Device() Device { return s.device }

// LocalAddr returns the computer's address the device reaches it at.
func (s *Session) LocalAddr() net.Addr { return s.c.localAddr() }

// RemoteIP returns the device's address.
func (s *Session) RemoteIP() net.IP { return s.c.remoteIP() }

// Volume returns the device's volume, as it last told it.
func (s *Session) Volume() Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.volume
}

// launch launches the default media receiver, unless it runs, and returns
// the device's status once it does.
func (s *Session) launch(ctx context.Context) (receiverStatus, error) {
	st, err := s.receiverStatus(ctx)
	if err != nil {
		return st, err
	}
	if st.app(DefaultMediaReceiver) != nil {
		return st, nil
	}
	r, err := s.c.request(ctx, nsReceiver, receiverID, map[string]any{"type": "LAUNCH", "appId": DefaultMediaReceiver})
	if err != nil {
		return st, fmt.Errorf("the device didn't start its media player: %w", err)
	}
	if st, err = parseReceiverStatus(r.msg.Payload); err == nil && st.app(DefaultMediaReceiver) != nil {
		return st, nil
	}
	// The answer may come before the receiver runs: it is asked until it
	// does.
	ctx, cancel := context.WithTimeout(ctx, launchWait)
	defer cancel()
	t := time.NewTicker(launchPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return st, errors.New("the device didn't start its media player in time")
		case <-t.C:
		}
		if st, err = s.receiverStatus(ctx); err == nil && st.app(DefaultMediaReceiver) != nil {
			return st, nil
		}
	}
}

func (s *Session) receiverStatus(ctx context.Context) (receiverStatus, error) {
	r, err := s.c.request(ctx, nsReceiver, receiverID, map[string]any{"type": "GET_STATUS"})
	if err != nil {
		return receiverStatus{}, err
	}
	return parseReceiverStatus(r.msg.Payload)
}

// Load has the device play m, in a media session of its own, and returns
// its first status.
func (s *Session) Load(ctx context.Context, m Media) (MediaStatus, error) {
	media := map[string]any{"contentId": m.URL, "contentUrl": m.URL, "streamType": "BUFFERED", "contentType": m.ContentType}
	if m.Duration > 0 {
		media["duration"] = m.Duration.Seconds()
	}
	meta := map[string]any{"metadataType": 3} // a music track
	put := func(k, v string) {
		if v != "" {
			meta[k] = v
		}
	}
	put("title", m.Title)
	put("artist", m.Artist)
	put("albumName", m.Album)
	if m.Image != "" {
		meta["images"] = []map[string]string{{"url": m.Image}}
	}
	media["metadata"] = meta
	id := s.c.newID()
	s.mu.Lock()
	app, transport := s.app, s.transport
	s.loading = id
	s.mu.Unlock()
	r, err := s.c.requestAs(ctx, id, nsMedia, transport, map[string]any{"type": "LOAD", "sessionId": app, "media": media,
		"autoplay": m.Autoplay, "currentTime": m.Start.Seconds()})
	var sts []MediaStatus
	if err == nil {
		if sts, err = parseMediaStatus(r.msg.Payload, time.Now()); err == nil && len(sts) == 0 {
			err = errors.New("the device answered the load without a status")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loading == id {
		s.loading = 0
	}
	if err != nil {
		return MediaStatus{}, err
	}
	// The answer names the load's media session, whose statuses come from
	// now on, as the answer itself does (onMessage).
	s.media = sts[0].Session
	return sts[0], nil
}

// mediaCommand sends a command of the media loaded, with fields, and waits
// for the device to take it.
func (s *Session) mediaCommand(ctx context.Context, kind string, fields map[string]any) error {
	s.mu.Lock()
	media, transport := s.media, s.transport
	s.mu.Unlock()
	if media == 0 {
		return errNoMedia
	}
	payload := map[string]any{"type": kind, "mediaSessionId": media}
	for k, v := range fields {
		payload[k] = v
	}
	_, err := s.c.request(ctx, nsMedia, transport, payload)
	return err
}

// Play plays the media loaded.
func (s *Session) Play(ctx context.Context) error { return s.mediaCommand(ctx, "PLAY", nil) }

// Pause pauses it.
func (s *Session) Pause(ctx context.Context) error { return s.mediaCommand(ctx, "PAUSE", nil) }

// Stop stops it: the receiver goes on running, without media.
func (s *Session) Stop(ctx context.Context) error { return s.mediaCommand(ctx, "STOP", nil) }

// Seek moves it to t.
func (s *Session) Seek(ctx context.Context, t time.Duration) error {
	return s.mediaCommand(ctx, "SEEK", map[string]any{"currentTime": t.Seconds()})
}

// SetRate plays it at rate, 1 its own speed.
func (s *Session) SetRate(ctx context.Context, rate float64) error {
	return s.mediaCommand(ctx, "SET_PLAYBACK_RATE", map[string]any{"playbackRate": rate})
}

// Status asks the device what the media session plays, which OnMedia is
// told too.
func (s *Session) Status(ctx context.Context) error {
	s.mu.Lock()
	transport := s.transport
	s.mu.Unlock()
	_, err := s.c.request(ctx, nsMedia, transport, map[string]any{"type": "GET_STATUS"})
	return err
}

// SetVolume sets the device's volume, from 0 to 1.
func (s *Session) SetVolume(ctx context.Context, level float64) error {
	level = min(max(level, 0), 1)
	_, err := s.c.request(ctx, nsReceiver, receiverID, map[string]any{"type": "SET_VOLUME", "volume": map[string]any{"level": level}})
	return err
}

// SetMuted mutes the device, or unmutes it.
func (s *Session) SetMuted(ctx context.Context, muted bool) error {
	_, err := s.c.request(ctx, nsReceiver, receiverID, map[string]any{"type": "SET_VOLUME", "volume": map[string]any{"muted": muted}})
	return err
}

// Close ends the session; stop also stops the receiver on the device, as
// casting ends, which shows its idle screen again. OnEnd isn't told.
func (s *Session) Close(stop bool) {
	s.mu.Lock()
	ended := s.ended
	s.ended = true
	app, transport := s.app, s.transport
	s.mu.Unlock()
	if ended {
		return
	}
	if stop && app != "" {
		_ = s.c.send(nsReceiver, receiverID, map[string]any{"type": "STOP", "sessionId": app, "requestId": s.c.newID()})
	}
	if transport != "" {
		_ = s.c.send(nsConnection, transport, map[string]string{"type": "CLOSE"})
	}
	_ = s.c.send(nsConnection, receiverID, map[string]string{"type": "CLOSE"})
	s.c.shutdown()
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
	s.c.close(err)
	if s.opts.OnEnd != nil {
		go s.opts.OnEnd(err)
	}
}

// onMessage takes in what the device tells, on the connection's
// goroutine: statuses of the media, of the receiver and of the volume.
func (s *Session) onMessage(m message, h header) {
	s.mu.Lock()
	ended := s.ended
	s.mu.Unlock()
	if ended {
		return
	}
	switch {
	case m.Namespace == nsMedia && h.Type == "MEDIA_STATUS":
		sts, err := parseMediaStatus(m.Payload, time.Now())
		if err != nil {
			return
		}
		s.mu.Lock()
		if len(sts) > 0 && h.RequestID != 0 && h.RequestID == s.loading {
			// The answer to the load under way names its media session.
			s.media, s.loading = sts[0].Session, 0
		}
		media, loading := s.media, s.loading != 0
		s.mu.Unlock()
		switch {
		case len(sts) == 0 && media != 0:
			// The receiver holds no media any more.
			sts = []MediaStatus{{Session: media, State: Idle, At: time.Now()}}
		case len(sts) > 0 && media != 0 && !loading && sts[0].Session > media:
			// Another sender loaded media of its own in the place of the
			// session's.
			sts = []MediaStatus{{Session: media, State: Idle, IdleReason: Interrupted, At: time.Now()}}
		}
		for _, st := range sts {
			if st.Session == media && media != 0 && s.opts.OnMedia != nil {
				s.opts.OnMedia(st)
			}
		}
	case m.Namespace == nsReceiver && h.Type == "RECEIVER_STATUS":
		st, err := parseReceiverStatus(m.Payload)
		if err != nil {
			return
		}
		v := st.Volume.volume()
		s.mu.Lock()
		app := s.app
		changed := st.Volume.known() && v != s.volume
		if changed {
			s.volume = v
		}
		s.mu.Unlock()
		if changed && app != "" && s.opts.OnVolume != nil {
			s.opts.OnVolume(v)
		}
		if app != "" && !st.runs(app) {
			why := ErrStopped
			if other := st.other(app); other != "" {
				why = fmt.Errorf("%s plays on the cast device now", other)
			}
			s.end(why)
		}
	case m.Namespace == nsConnection && h.Type == "CLOSE":
		s.mu.Lock()
		transport := s.transport
		s.mu.Unlock()
		if m.Source == receiverID || (transport != "" && m.Source == transport) {
			s.end(ErrStopped)
		}
	}
}

// receiverStatus is a RECEIVER_STATUS of the device.
type receiverStatus struct {
	Applications []application `json:"applications"`
	Volume       volumeJSON    `json:"volume"`
}

type application struct {
	AppID       string `json:"appId"`
	DisplayName string `json:"displayName"`
	SessionID   string `json:"sessionId"`
	TransportID string `json:"transportId"`
	IsIdle      bool   `json:"isIdleScreen"`
}

type volumeJSON struct {
	Level       *float64 `json:"level"`
	Muted       *bool    `json:"muted"`
	ControlType string   `json:"controlType"`
}

func (v volumeJSON) known() bool { return v.Level != nil || v.Muted != nil }

func (v volumeJSON) volume() Volume {
	var out Volume
	if v.Level != nil {
		out.Level = min(max(*v.Level, 0), 1)
	}
	if v.Muted != nil {
		out.Muted = *v.Muted
	}
	out.Fixed = v.ControlType == "fixed"
	return out
}

// app returns the running app of appID with a transport, or nil.
func (st receiverStatus) app(appID string) *application {
	for i, a := range st.Applications {
		if a.AppID == appID && a.TransportID != "" {
			return &st.Applications[i]
		}
	}
	return nil
}

// runs reports whether the receiver's session sessionID still runs.
func (st receiverStatus) runs(sessionID string) bool {
	for _, a := range st.Applications {
		if a.SessionID == sessionID {
			return true
		}
	}
	return false
}

// other names the app that runs in the place of the session sessionID, or
// "": none, or the idle screen.
func (st receiverStatus) other(sessionID string) string {
	for _, a := range st.Applications {
		if a.SessionID != sessionID && !a.IsIdle && a.DisplayName != "" && a.AppID != "E8C28D3C" { // the backdrop
			return a.DisplayName
		}
	}
	return ""
}

func parseReceiverStatus(payload []byte) (receiverStatus, error) {
	var msg struct {
		Status receiverStatus `json:"status"`
	}
	err := json.Unmarshal(payload, &msg)
	return msg.Status, err
}

// parseMediaStatus reads the statuses of a MEDIA_STATUS, told at.
func parseMediaStatus(payload []byte, at time.Time) ([]MediaStatus, error) {
	var msg struct {
		Status []struct {
			MediaSessionID int     `json:"mediaSessionId"`
			PlayerState    string  `json:"playerState"`
			IdleReason     string  `json:"idleReason"`
			CurrentTime    float64 `json:"currentTime"`
			PlaybackRate   float64 `json:"playbackRate"`
			Media          *struct {
				ContentID string  `json:"contentId"`
				Duration  float64 `json:"duration"`
			} `json:"media"`
		} `json:"status"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	out := make([]MediaStatus, 0, len(msg.Status))
	for _, st := range msg.Status {
		ms := MediaStatus{Session: st.MediaSessionID, State: PlayerState(st.PlayerState), IdleReason: st.IdleReason,
			Position: seconds(st.CurrentTime), Rate: st.PlaybackRate, At: at}
		if ms.Rate <= 0 {
			ms.Rate = 1
		}
		if st.Media != nil {
			ms.ContentID = st.Media.ContentID
			ms.Duration = seconds(st.Media.Duration)
		}
		out = append(out, ms)
	}
	return out, nil
}

func seconds(s float64) time.Duration {
	if s <= 0 {
		return 0
	}
	return time.Duration(s * float64(time.Second))
}
