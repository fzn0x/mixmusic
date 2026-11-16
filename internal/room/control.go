package room

// Guests that control playback (#734): while the host's rules let them, a
// guest has the host's player play, pause, or move to the next or the
// last track; controls at once at most, then one each controlEvery.

import "time"

func init() {
	guestAsks["control"] = (*Server).control
	hostTells["control"] = func(s *Session, m message) {
		s.tell(Event{Kind: ControlRefused, Reason: cleanText(m.Reason, 32)})
	}
}

// What a guest may have the host's player do.
const (
	Play     = "play"
	Pause    = "pause"
	Next     = "next"
	Previous = "previous"
)

// The bounds of a guest's controls.
const (
	controls     = 5
	controlEvery = time.Second
)

// What the activity of a room tells of a guest's control.
const (
	Played   = "played"
	Paused   = "paused"
	Skipped  = "skipped to the next track"
	WentBack = "went back a track"
)

// NoControl is why the host refused a control, beside TooMany: its rules
// don't let guests control its player.
const NoControl = "no control"

// doneOf is what the activity tells of a guest that asked do.
func doneOf(do string) string {
	switch do {
	case Play:
		return Played
	case Pause:
		return Paused
	case Next:
		return Skipped
	}
	return WentBack
}

// control has the host's player do what the guest of c asks, when the
// rules let guests control it.
func (s *Server) control(c *conn, m message) {
	switch m.Do {
	case Play, Pause, Next, Previous:
	default:
		return
	}
	rules := s.rules()
	s.mu.Lock()
	g := s.guestOf(c)
	if g == nil {
		s.mu.Unlock()
		return
	}
	from := g.Member
	gs := s.Guests
	reason := ""
	switch {
	case !rules.Control || gs == nil || gs.Control == nil:
		reason = NoControl
	case !s.takes(c, func(l *limits) *bucket { return &l.controls }, controls, controlEvery):
		reason = TooMany
	default:
		s.note(from.Name, doneOf(m.Do))
	}
	s.mu.Unlock()
	if reason == "" && gs.Control(from, m.Do) != nil {
		reason = NoControl
	}
	if reason != "" {
		c.send(message{Type: "control", Do: m.Do, Reason: reason})
		return
	}
	s.changed(false)
}

// ControlRefused: the host refused a control (Reason).
const ControlRefused EventKind = 66

// Control asks the host's player to do do: Play, Pause, Next or Previous.
func (s *Session) Control(do string) error {
	return s.f.Send(message{Type: "control", Do: do}, s.timing.silence())
}
