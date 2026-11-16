package room

// Make host (#734): the room moves to one of its guests, if the others can
// reach it. The host asks the guest to host (host), with the places of the
// others and of itself to keep, and the keys it blocked; the guest opens a
// room of its own, which keeps them, and says where (hosting). The host
// asks each other guest whether it reaches that room (reach), and tries it
// itself; once all did, it tells them all the room moved (moved), and they
// come back to the new host by their keys, as after a drop. The guest's
// user is asked first, AnswerFor at most; one that can't reach the room,
// or no answer in MoveFor, keeps the room where it is.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

func init() {
	features = append(features, featureMove)
	guestAsks["hosting"] = (*Server).hosting
	guestAsks["reach"] = func(s *Server, c *conn, m message) { s.reachedBy(c, m.OK) }
	hostTells["host"] = func(s *Session, m message) {
		s.tell(Event{Kind: HostAsked, Seats: Seats{Guests: membersOfSeats(m.Seats, MaxGuests+1), Blocked: membersOfSeats(m.Blocked, maxBlocked)}})
	}
	hostTells["unhost"] = func(s *Session, m message) { s.tell(Event{Kind: Unhosted}) }
	hostTells["reach"] = func(s *Session, m message) {
		if key, err := remote.ParseFingerprint(m.Key); err == nil {
			s.tell(Event{Kind: Reach, Key: key, Addr: cleanText(m.Addr, 300)})
		}
	}
	hostTells["moved"] = (*Session).moved
}

// featureMove: the guest can host the room it is in (MakeHost).
const featureMove = "move"

// AnswerFor bounds the wait for the guest asked to host, whose user says
// whether it does, and MoveFor the others reaching its room then. Tests
// shorten answerFor and moveFor.
const (
	AnswerFor = time.Minute
	MoveFor   = 10 * time.Second
)

var answerFor, moveFor = AnswerFor, MoveFor

var (
	// ErrCantHost: the guest is away, or its MixMusic can't host the room.
	ErrCantHost = errors.New("that guest can't host the room now")
	// ErrMoving: the room moves to another guest already.
	ErrMoving = errors.New("the room moves to another guest already")
	// ErrMoved: the room moved to another computer (Session.Moved).
	ErrMoved = errors.New("the room moved to another computer")
	// ErrRoomMoves: the room moves to another computer, or moved: it
	// lets no one in meanwhile, as their places wouldn't move with it.
	ErrRoomMoves = errors.New("the room is moving to another computer: try again in a moment")
)

// Seat is a place a room keeps, as the host that moves it tells: a key, in
// hex, and its name in rooms.
type Seat struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// seatsOf are the members ms as seats.
func seatsOf(ms []Member) []Seat {
	out := make([]Seat, len(ms))
	for i, m := range ms {
		out[i] = Seat{Key: m.Key.String(), Name: m.Name}
	}
	return out
}

// maxBlocked bounds the keys a room that moves tells blocked: more than
// its guests, as a host blocks those who left too, and few enough that
// their message, with the seats, takes less than the protocol's MiB.
const maxBlocked = 2048

// membersOfSeats are the seats a host told, n at most, of keys that are
// keys, once each, as members.
func membersOfSeats(seats []Seat, n int) []Member {
	if len(seats) > n {
		seats = seats[:n]
	}
	var out []Member
	seen := map[remote.Fingerprint]bool{}
	for _, st := range seats {
		key, err := remote.ParseFingerprint(st.Key)
		if err != nil || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Member{Key: key, Name: CleanName(st.Name)})
	}
	return out
}

// Seats are the places a room that moved here keeps as it opens: of the
// guests of the room it moved from, and its host, which come back by their
// keys, and the keys it blocked.
type Seats struct {
	Guests  []Member
	Blocked []Member
}

// seat keeps the places of the seats, as of guests that dropped off now,
// and blocks their blocked keys. s.mu is held.
func (s *Server) seat() {
	if s.Seats == nil {
		return
	}
	now := time.Now()
	for i, m := range s.Seats.Guests {
		if m.Key == s.Identity.Fingerprint() || len(s.guests) >= MaxGuests {
			continue
		}
		if s.guests == nil {
			s.guests = map[remote.Fingerprint]*guest{}
		}
		// In the order they joined the room they come from.
		s.guests[m.Key] = &guest{Member: Member{Key: m.Key, Name: m.Name, Joined: now.Add(time.Duration(i))}, away: now}
		if s.asks.seated == nil {
			s.asks.seated = map[remote.Fingerprint]bool{}
		}
		s.asks.seated[m.Key] = true
	}
	for _, m := range s.Seats.Blocked {
		if s.blocked == nil {
			s.blocked = map[remote.Fingerprint]string{}
		}
		s.blocked[m.Key] = m.Name
	}
}

// Move is where the room moved: the new host's key, its name in rooms,
// and the address of its room as the old host reached it.
type Move struct {
	Key  remote.Fingerprint
	Name string
	Addr string
}

// move is a move under way, to the guest of to, named name, whose
// connection conn asked it to host; addr is where its room listens once it
// told, and waiting the guests yet to say they reach it, and self whether
// the host is.
type move struct {
	to      remote.Fingerprint
	name    string
	conn    *conn
	addr    string
	waiting map[remote.Fingerprint]bool
	self    bool
	timer   *time.Timer
	// done, when set, is told once the move ends: where the room moved,
	// or why it didn't.
	done func(Move, error)
}

// MakeHost moves the room to the guest of key, if the others can reach it:
// done is told, without the server's lock, where it moved, or why it
// didn't. Once it moved, the server lets no one in, and Close ends it.
func (s *Server) MakeHost(key remote.Fingerprint, done func(Move, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.guests[key]
	switch {
	case s.closed:
		return ErrEnded
	case s.asks.move != nil:
		return ErrMoving
	case g == nil || g.conn == nil || !slices.Contains(g.conn.features, featureMove):
		return ErrCantHost
	}
	seats := []Member{{Key: s.Identity.Fingerprint(), Name: CleanName(s.Name)}}
	for _, m := range s.members() {
		if m.Key != key {
			seats = append(seats, m)
		}
	}
	var blocked []Member
	for k, name := range s.blocked {
		blocked = append(blocked, Member{Key: k, Name: name})
	}
	slices.SortFunc(blocked, func(a, b Member) int { return compareKeys(a.Key, b.Key) })
	blocked = blocked[:min(len(blocked), maxBlocked)]
	mv := &move{to: key, name: g.Name, conn: g.conn, done: done}
	mv.timer = time.AfterFunc(answerFor, func() { s.notMoved(mv, fmt.Errorf("%s didn't answer in time", mv.name)) })
	s.asks.move = mv
	g.conn.send(message{Type: "host", Seats: seatsOf(seats), Blocked: seatsOf(blocked)})
	return nil
}

// Moving returns the name of the guest the room moves to, while it does.
func (s *Server) Moving() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mv := s.asks.move; mv != nil {
		return mv.name, true
	}
	return "", false
}

// CanHost reports whether the guest of key can host the room: it is here,
// and its MixMusic can.
func (s *Server) CanHost(key remote.Fingerprint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.guests[key]
	return g != nil && g.conn != nil && slices.Contains(g.conn.features, featureMove)
}

// hosting takes in where the guest the room moves to opened its room, and
// has the others, and the host, try to reach it.
func (s *Server) hosting(c *conn, m message) {
	s.mu.Lock()
	mv := s.asks.move
	if mv == nil || mv.conn != c || mv.addr != "" || s.guestOf(c) == nil {
		s.mu.Unlock()
		return
	}
	if m.Reason == reasonDeclined {
		s.mu.Unlock()
		s.notMoved(mv, fmt.Errorf("%s doesn't host the room", mv.name))
		return
	}
	if m.Reason != "" || m.Port <= 0 || m.Port > 65535 {
		s.mu.Unlock()
		s.notMoved(mv, fmt.Errorf("%s's computer couldn't open the room", mv.name))
		return
	}
	host, _, err := net.SplitHostPort(c.raw.RemoteAddr().String())
	if err != nil {
		host = c.raw.RemoteAddr().String()
	}
	mv.addr = net.JoinHostPort(host, strconv.Itoa(m.Port))
	mv.waiting, mv.self = map[remote.Fingerprint]bool{}, true
	mv.timer.Stop()
	mv.timer = time.AfterFunc(moveFor, func() { s.notMoved(mv, fmt.Errorf("not everyone reached %s's computer in time", mv.name)) })
	for k, g := range s.guests {
		if k != mv.to && g.conn != nil {
			mv.waiting[k] = true
			g.conn.send(message{Type: "reach", Key: mv.to.String(), Addr: mv.addr})
		}
	}
	addr, to := mv.addr, mv.to
	s.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), moveFor)
		defer cancel()
		err := (&Guest{Identity: s.Identity, Timing: s.Timing}).Reach(ctx, addr, to)
		s.reached(mv, nil, err == nil)
	}()
}

// reachedBy takes in whether the guest of c reaches the room it moves to.
func (s *Server) reachedBy(c *conn, ok bool) {
	s.mu.Lock()
	mv := s.asks.move
	g := s.guestOf(c)
	s.mu.Unlock()
	if mv != nil && g != nil {
		s.reached(mv, &g.Member, ok)
	}
}

// reached takes in whether who, a guest, or the host when nil, reaches
// the room of the move mv: it moves once all do.
func (s *Server) reached(mv *move, who *Member, ok bool) {
	s.mu.Lock()
	if s.asks.move != mv || mv.addr == "" {
		s.mu.Unlock()
		return
	}
	switch {
	case who == nil:
		mv.self = false
	case mv.waiting[who.Key]:
		delete(mv.waiting, who.Key)
	default:
		s.mu.Unlock()
		return
	}
	if !ok {
		s.mu.Unlock()
		if who == nil {
			s.notMoved(mv, fmt.Errorf("this computer can't reach %s's", mv.name))
		} else {
			s.notMoved(mv, fmt.Errorf("%s can't reach %s's computer", who.Name, mv.name))
		}
		return
	}
	if len(mv.waiting) > 0 || mv.self {
		s.mu.Unlock()
		return
	}
	if s.guestOf(mv.conn) == nil {
		// The guest it moves to went meanwhile.
		s.mu.Unlock()
		s.notMoved(mv, fmt.Errorf("%s left the room", mv.name))
		return
	}
	s.asks.move = nil
	s.asks.moved = true
	mv.timer.Stop()
	var conns []*conn
	for _, g := range s.guests {
		if g.conn != nil {
			conns = append(conns, g.conn)
		}
	}
	done := Move{Key: mv.to, Name: mv.name, Addr: mv.addr}
	s.mu.Unlock()
	// Each is told before anything else ends its connection.
	var wg sync.WaitGroup
	for _, c := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.f.Send(message{Type: "moved", Key: done.Key.String(), Name: done.Name, Addr: done.Addr}, time.Second)
		}()
	}
	wg.Wait()
	if mv.done != nil {
		mv.done(done, nil)
	}
}

// reasonMoving: the room moves, or moved, and lets no one in.
const reasonMoving = "moving"

// moves reports whether a move is under way, or the room moved, or this
// room, of seats a room that moves keeps, isn't open yet (Open). s.mu is
// held.
func (s *Server) moves() bool {
	return s.asks.move != nil || s.asks.moved || s.Seats != nil && !s.asks.open
}

// Open lets in the computers of the seats of a room that moved here, and
// the others, once it moved: until then, they're told to try again in a
// moment, though another tells them where it listens.
func (s *Server) Open() {
	s.mu.Lock()
	s.asks.open = true
	s.mu.Unlock()
}

// errDropped: the host removed or blocked someone as the room moved, whose
// place the room it moved to would keep.
var errDropped = errors.New("you removed or blocked someone while it moved")

// errLeft: a guest left as the room moved, whose place the room it moved
// to would keep.
var errLeft = errors.New("someone left while it moved")

// stopMove ends the move under way, if any, for why.
func (s *Server) stopMove(why error) {
	s.mu.Lock()
	mv := s.asks.move
	s.mu.Unlock()
	if mv != nil {
		s.notMoved(mv, why)
	}
}

// notMoved ends the move mv, which didn't: the guest it was to closes its
// room, and done is told why.
func (s *Server) notMoved(mv *move, why error) {
	s.mu.Lock()
	if s.asks.move != mv {
		s.mu.Unlock()
		return
	}
	s.asks.move = nil
	mv.timer.Stop()
	s.mu.Unlock()
	mv.conn.send(message{Type: "unhost"})
	if mv.done != nil {
		mv.done(Move{}, why)
	}
}

// Reach reports, with nil, whether the guest reaches the room of the host
// of key at addr: TLS, with that key, then nothing; at addr alone, not
// where Elsewhere tells the room the guest is in is reached.
func (g *Guest) Reach(ctx context.Context, addr string, key remote.Fingerprint) error {
	one := *g
	one.Elsewhere = nil
	tc, _, err := one.connect(ctx, addr, &key)
	if err != nil {
		return err
	}
	return tc.Close()
}

// What changed in a session as the room moves.
const (
	// HostAsked: the host asks this guest to host the room, which keeps
	// Seats (Hosting).
	HostAsked EventKind = 68
	// Unhosted: the room stays where it is: this guest closes the room it
	// opened for it.
	Unhosted EventKind = 69
	// Reach: the host asks whether this guest reaches the room of Key, at
	// Addr or where discovery finds it (Reached).
	Reach EventKind = 70
)

// Hosting tells the host that this guest opened a room for it to move to,
// at port, or couldn't, at 0.
func (s *Session) Hosting(port int) error {
	m := message{Type: "hosting", Port: port}
	if port <= 0 {
		m.Port, m.Reason = 0, "can't"
	}
	return s.f.Send(m, s.timing.silence())
}

// DeclineHost tells the host that this guest's user doesn't host the
// room.
func (s *Session) DeclineHost() error {
	return s.f.Send(message{Type: "hosting", Reason: reasonDeclined}, s.timing.silence())
}

// Reached tells the host whether this guest reaches the room it moves to.
func (s *Session) Reached(ok bool) error {
	return s.f.Send(message{Type: "reach", OK: ok}, s.timing.silence())
}

// Moved returns where the room moved, once the session ended for it
// (ErrMoved).
func (s *Session) Moved() (Move, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asks.moved == nil {
		return Move{}, false
	}
	return *s.asks.moved, true
}

// moved takes in that the room moved: the session ends, and its guest
// comes back to the new host by its key.
func (s *Session) moved(m message) {
	key, err := remote.ParseFingerprint(m.Key)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.asks.moved = &Move{Key: key, Name: CleanName(m.Name), Addr: cleanText(m.Addr, 300)}
	s.mu.Unlock()
	s.end(ErrMoved)
}
