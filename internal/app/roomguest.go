package app

// The guest's side of Listen with friends: knocking at a room found on the
// network, by its code or its invite link, seeing who hosts and what
// plays, joining it, coming back after dropping off, and leaving, which
// gives the user their own queue back. roomfollow.go plays what the host
// plays meanwhile.

import (
	"context"
	"errors"
	"log"
	"net"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
)

// These times are the guest's; tests shorten them.
var (
	// roomRetry is how long after dropping off a guest tries to come
	// back, at first, and roomRetryMost at most, doubling between, for
	// roomTries tries: about 15 minutes, as long as the host keeps its
	// place, after a sleep or a change of network on either side.
	roomRetry     = time.Second
	roomRetryMost = 32 * time.Second
	roomTries     = 32
	// roomKnockFor bounds a knock, and joining after it.
	roomKnockFor = 30 * time.Second
)

// RoomStep is where a guest of a room is.
type RoomStep int

const (
	// RoomKnocking: it proves the code, or the link.
	RoomKnocking RoomStep = iota
	// RoomPreview: it proved it, and shows who hosts, what plays and how
	// many listen, to join.
	RoomPreview
	// RoomJoining: it asked to join.
	RoomJoining
	// RoomIn: it is in the room, and plays what the host plays.
	RoomIn
	// RoomAway: it dropped off, and comes back.
	RoomAway
	// RoomEnded: it isn't in the room, as Problem tells.
	RoomEnded
)

// guestRoom is the room this computer is a guest of, or knocks at. a.room.mu
// guards it.
type guestRoom struct {
	// id is the host's ID, name its name, and key its key, once known.
	id, name string
	key      remote.Fingerprint
	hasKey   bool
	step     RoomStep
	problem  string
	session  *room.Session
	cancel   context.CancelFunc
	// own is the user's own queue, which comes back as the guest leaves.
	own *ownQueue
	// follow follows the host while in the room.
	follow *follower
	// tries counts the tries to come back, and retry waits for the next.
	tries int
	retry *time.Timer
	// hint is where the room moved, as its old host reached it; pending
	// the room opened for it to move here, once the user agreed; ask the
	// host's ask that waits for the user, and askedAt when it last asked
	// (roommove.go).
	hint    string
	pending *hostedRoom
	ask     *hostAsk
	askedAt time.Time
	// declined counts the host's asks the user declined, or left
	// unanswered: past maxDeclines, it asks no more (roommove.go).
	declined int
}

// GuestView is the room this computer is a guest of, for its page: the
// host, where the guest is, why it isn't in the room, if it isn't, what
// the room shows before joining, and, in it, what plays and who listens.
type GuestView struct {
	ID, Host string
	Step     RoomStep
	Problem  string
	Preview  room.Preview
	State    room.State
	HasState bool
	Queue    room.Queue
	Members  []string
	// Note tells why the host's track doesn't play here, as "Can't play
	// here", while it doesn't.
	Note string
}

// Guest returns the room this computer is a guest of, or knocks at, and
// whether it is one.
func (rs *Rooms) Guest() (GuestView, bool) {
	r := &rs.a.room
	r.mu.Lock()
	g := r.guest
	if g == nil {
		r.mu.Unlock()
		return GuestView{}, false
	}
	v := GuestView{ID: g.id, Host: g.name, Step: g.step, Problem: g.problem}
	s, f := g.session, g.follow
	r.mu.Unlock()
	if s != nil {
		v.Preview = s.Preview()
		v.State, v.HasState = s.State()
		v.Queue, v.Members = s.Queue(), s.Members()
	}
	if f != nil {
		v.Note = f.noteNow()
	}
	return v, true
}

// guest is the room this computer is a guest of, or nil.
func (a *App) guestRoom() *guestRoom {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.guest
}

// roomGuest is this computer as a guest of rooms.
func (a *App) roomGuest(id *remote.Identity) *room.Guest {
	return &room.Guest{Identity: id, Name: a.roomName(), Clock: a.roomClock(), Timing: a.roomOptions().Timing, Elsewhere: a.roomElsewhere}
}

// Knock knocks at the room of the host id, found on the network, with the
// code the user typed.
func (rs *Rooms) Knock(id, code string) {
	parsed, ok := room.ParseCode(code)
	if !ok {
		rs.a.startKnock(id, nil, knockBy{}, errors.New("A room's code is 6 letters and digits."))
		return
	}
	rs.a.startKnock(id, nil, knockBy{password: []byte(parsed)}, nil)
}

// OpenLink knocks at the room of an invite link: its host's key must
// answer, and its secret proves the guest has the link.
func (rs *Rooms) OpenLink(link string) error {
	in, err := room.ParseLink(link)
	if err != nil {
		return err
	}
	rs.a.openInvite(in)
	return nil
}

// openInvite knocks at the room of the invite in.
func (a *App) openInvite(in room.Invite) {
	key := in.Host
	if a.backByInvite(in) {
		return
	}
	a.startKnock(key.ID(), &key, knockBy{password: in.Secret, link: true, pinned: &key, addrs: in.Addrs}, nil)
}

// knockBy is how a knock proves the guest may join.
type knockBy struct {
	password []byte
	link     bool
	pinned   *remote.Fingerprint
	// addrs are where the room is reached from outside the network, as
	// its link, or the user, tells them (roomfar.go).
	addrs []string
}

// startKnock starts knocking at the room of the host id, of the key key
// when known, in place of a knock under way or a room that ended, unless
// failed tells why it can't.
func (a *App) startKnock(id string, key *remote.Fingerprint, by knockBy, failed error) {
	r := &a.room
	r.mu.Lock()
	if g := r.guest; g != nil && g.step != RoomEnded && g.step != RoomPreview && g.step != RoomKnocking {
		r.mu.Unlock()
		a.notifyRooms("Leave the room you're in to join another")
		return
	}
	if r.hosted != nil {
		r.mu.Unlock()
		a.notifyRooms("End the room you host to join another")
		return
	}
	old := r.guest
	f, found := r.found[id]
	name := f.found.Name
	if name == "" {
		name = "The room"
	}
	g := &guestRoom{id: id, name: name, step: RoomKnocking}
	if key != nil {
		g.key, g.hasKey = *key, true
	}
	if failed != nil {
		g.step, g.problem = RoomEnded, failed.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), roomKnockFor)
	g.cancel = cancel
	r.guest = g
	r.far.knock(id, by)
	r.mu.Unlock()
	if old != nil {
		old.drop()
	}
	a.notifyRooms("")
	if failed != nil {
		cancel()
		return
	}
	go a.knock(ctx, g, found, by)
}

// knock proves the code, or the link, at the room of g.
func (a *App) knock(ctx context.Context, g *guestRoom, wasFound bool, by knockBy) {
	fail := func(err error) {
		log.Printf("mixmusic: room: knocking at %s: %v", g.id, err)
		a.setGuest(g, func() { g.step, g.problem = RoomEnded, roomProblem(g.name, err) })
	}
	// On a goroutine of its own: it waits for the key another call reads.
	id, err := a.identity(true)
	if err != nil {
		fail(err)
		return
	}
	a.keepRooms()
	f, ok := a.foundRoom(g.id, !wasFound)
	if !ok {
		fail(errRoomAway)
		return
	}
	a.setGuest(g, func() {
		if g.name == "The room" {
			g.name = f.Name
		}
	})
	s, err := a.roomGuest(id).Knock(ctx, f.Addr, by.pinned, by.password, by.link)
	if err != nil {
		fail(err)
		return
	}
	if !a.setGuest(g, func() { g.session, g.key, g.hasKey, g.step = s, s.Host(), true, RoomPreview }) {
		s.Close()
		return
	}
	// Who hosts, as the host tells it, once it proved its code.
	a.setGuest(g, func() { g.name = s.Preview().Host })
	go func() {
		<-s.Done()
		a.setGuest(g, func() {
			if g.session == s && g.step == RoomPreview {
				g.step, g.problem, g.session = RoomEnded, roomProblem(g.name, s.Err()), nil
			}
		})
	}()
}

// errRoomAway: the room isn't on the network.
var errRoomAway = errors.New("the room isn't on the network")

// setGuest runs set on g while it is the room this computer is a guest of,
// tells the windows, and reports whether it was.
func (a *App) setGuest(g *guestRoom, set func()) bool {
	r := &a.room
	r.mu.Lock()
	mine := r.guest == g
	if mine {
		set()
	}
	r.mu.Unlock()
	if mine {
		a.notifyRooms("")
	}
	return mine
}

// Join joins the room this computer knocked at, once it showed who hosts
// and what plays: from then on, it plays what the host plays.
func (rs *Rooms) Join() {
	a := rs.a
	r := &a.room
	r.mu.Lock()
	g := r.guest
	if g == nil || g.step != RoomPreview || g.session == nil {
		r.mu.Unlock()
		return
	}
	g.step = RoomJoining
	s := g.session
	r.mu.Unlock()
	a.notifyRooms("")
	go func() {
		// The host may ask its user to let this computer in first.
		ctx, cancel := context.WithTimeout(context.Background(), roomKnockFor+room.AskFor)
		stop := s.Watch(func(e room.Event) {
			if e.Kind == room.Asking {
				a.notifyRooms("")
			}
		})
		err := s.Join(ctx)
		stop()
		cancel()
		if err != nil {
			a.setGuest(g, func() { g.step, g.problem, g.session = RoomEnded, roomProblem(g.name, err), nil })
			s.Close()
			return
		}
		a.entered(g, s, true)
	}()
}

// entered follows the host of g in the session s, which joined, or came
// back: own keeps the user's own queue first.
func (a *App) entered(g *guestRoom, s *room.Session, own bool) {
	var f *follower
	var mine *ownQueue
	if own {
		mine = a.keepOwnQueue()
	}
	ok := a.setGuest(g, func() {
		g.step, g.problem, g.session, g.tries = RoomIn, "", s, 0
		if mine != nil && g.own == nil {
			g.own = mine
		}
		if g.follow == nil {
			g.follow = newFollower(a)
		}
		f = g.follow
	})
	if !ok {
		s.Leave()
		return
	}
	log.Printf("mixmusic: room: in %s's room", g.name)
	if own {
		a.notifyRooms("You joined " + g.name + "'s room")
	}
	f.follow(s)
	a.watchRoomAsks(s, g.name)
	a.watchMove(g, s)
	go func() {
		<-s.Done()
		a.dropped(g, s)
	}()
}

// dropped takes in that the session s with the host of g ended: the room
// ended, or removed this computer, or the guest drops off, and comes back.
func (a *App) dropped(g *guestRoom, s *room.Session) {
	err := s.Err()
	r := &a.room
	r.mu.Lock()
	mine := r.guest == g && g.session == s
	r.mu.Unlock()
	if !mine || errors.Is(err, net.ErrClosed) {
		return
	}
	if m, ok := s.Moved(); ok && errors.Is(err, room.ErrMoved) {
		a.moved(g, s, m)
		return
	}
	if !errors.Is(err, room.ErrLost) {
		log.Printf("mixmusic: room: %s's room: %v", g.name, err)
		g.end(a, err)
		return
	}
	log.Printf("mixmusic: room: %s stopped answering: coming back", g.name)
	a.setGuest(g, func() { g.step, g.session = RoomAway, nil })
	a.comeBackLater(g)
}

// comeBackLater tries to come back to the room of g after a while, longer
// each time, until it tried roomTries times: the room ended then.
func (a *App) comeBackLater(g *guestRoom) {
	r := &a.room
	r.mu.Lock()
	if r.guest != g || g.step != RoomAway {
		r.mu.Unlock()
		return
	}
	if g.tries >= roomTries {
		r.mu.Unlock()
		g.end(a, room.ErrEnded)
		return
	}
	wait := min(roomRetry<<min(g.tries, 8), roomRetryMost)
	g.tries++
	if g.retry != nil {
		g.retry.Stop()
	}
	g.retry = time.AfterFunc(wait, func() { a.comeBack(g) })
	r.mu.Unlock()
}

// comeBack tries once to come back to the room of g, by this computer's
// key: where the host was, or is found now.
func (a *App) comeBack(g *guestRoom) {
	r := &a.room
	r.mu.Lock()
	ok := r.guest == g && g.step == RoomAway && g.hasKey
	r.mu.Unlock()
	if !ok {
		return
	}
	id, err := a.identity(true)
	if err != nil {
		a.comeBackLater(g)
		return
	}
	f, found := a.foundRoom(g.id, true)
	if !found && g.hint != "" {
		// Where the room moved, as its old host reached it.
		f, found = room.Found{ID: g.id, Addr: g.hint}, true
	}
	if !found {
		a.comeBackLater(g)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	s, err := a.roomGuest(id).Rejoin(ctx, f.Addr, g.key)
	cancel()
	switch {
	case err == nil:
		a.entered(g, s, false)
	case errors.Is(err, room.ErrNotGuest), errors.Is(err, room.ErrBlocked), errors.Is(err, room.ErrVersion),
		errors.Is(err, room.ErrDeclined):
		g.end(a, err)
	default:
		a.comeBackLater(g)
	}
}

// roomsWoke has a guest that dropped off try to come back at once, as the
// computer woke.
func (a *App) roomsWoke() {
	g := a.guestRoom()
	if g == nil {
		return
	}
	r := &a.room
	r.mu.Lock()
	away := g.step == RoomAway
	if away && g.retry != nil {
		g.retry.Stop()
	}
	r.mu.Unlock()
	if away {
		go a.comeBack(g)
	}
}

// Leave leaves the room this computer is a guest of, or stops knocking:
// the user's own queue comes back.
func (rs *Rooms) Leave() {
	a := rs.a
	r := &a.room
	r.mu.Lock()
	g := r.guest
	r.guest = nil
	r.mu.Unlock()
	if g == nil {
		return
	}
	if s := g.sessionNow(a); s != nil {
		s.Leave()
	}
	g.end(a, nil)
	a.notifyRooms("")
}

func (g *guestRoom) sessionNow(a *App) *room.Session {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	return g.session
}

// end ends this computer's stay in the room of g, for err: it no longer
// follows the host, and the user's own queue comes back. The page shows
// why, unless err is nil, as the user left.
func (g *guestRoom) end(a *App, err error) {
	r := &a.room
	r.mu.Lock()
	s, f, own, retry, cancel := g.session, g.follow, g.own, g.retry, g.cancel
	g.session, g.follow, g.own, g.retry = nil, nil, nil, nil
	if err != nil {
		g.step, g.problem = RoomEnded, roomProblem(g.name, err)
	}
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if retry != nil {
		retry.Stop()
	}
	if f != nil {
		f.stop()
	}
	if s != nil {
		s.Close()
	}
	if own != nil {
		a.giveOwnQueueBack(own)
	}
	if err != nil {
		a.notifyRooms(roomProblem(g.name, err))
	}
}

// quit ends this computer's stay in the room of g as MixMusic quits: the
// host hears it left, and the player still keeps no queue, so that
// queue.json keeps the user's own, which the next start plays.
func (g *guestRoom) quit(a *App) {
	r := &a.room
	r.mu.Lock()
	s, f, retry, cancel := g.session, g.follow, g.retry, g.cancel
	g.session, g.follow, g.retry = nil, nil, nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if retry != nil {
		retry.Stop()
	}
	if f != nil {
		f.halt()
	}
	if s != nil {
		s.Leave()
	}
}

// drop ends a knock that gave way to another, or a room that ended.
func (g *guestRoom) drop() {
	if g.cancel != nil {
		g.cancel()
	}
	if g.session != nil {
		g.session.Close()
	}
}

// Dismiss forgets a room that ended, once its page closes.
func (rs *Rooms) Dismiss() {
	r := &rs.a.room
	r.mu.Lock()
	g := r.guest
	if g != nil && g.step == RoomEnded {
		r.guest = nil
	} else {
		g = nil
	}
	r.mu.Unlock()
	if g != nil {
		g.drop()
		rs.a.notifyRooms("")
	}
}

// roomProblem tells why a stay in the room of the host named name ended,
// or didn't start, in the interface's words.
func roomProblem(name string, err error) string {
	switch {
	case errors.Is(err, room.ErrWrongCode):
		return "That isn't the code of " + name + "'s room."
	case errors.Is(err, room.ErrWait):
		return "Too many wrong codes from this computer: try again in a minute."
	case errors.Is(err, room.ErrLocked):
		return name + "'s room isn't taking codes for a minute: ask " + name + " for the invite link."
	case errors.Is(err, room.ErrNewCode):
		return name + "'s room has a new code: ask " + name + " for it, or for the new invite link."
	case errors.Is(err, room.ErrPreviewEnded):
		return "You waited long to join " + name + "'s room, or many others knocked at once: open its link, or type its code, again."
	case errors.Is(err, room.ErrBusy):
		return name + "'s room checks other codes: try again in a moment."
	case errors.Is(err, room.ErrBlocked):
		return name + " blocked this computer from the room."
	case errors.Is(err, room.ErrRemoved):
		return name + " removed you from the room."
	case errors.Is(err, room.ErrDeclined):
		return name + " didn't let you in."
	case errors.Is(err, room.ErrRoomMoves):
		return name + "'s room is moving to another computer: try again in a moment."
	case errors.Is(err, room.ErrManyAsk):
		return "So many wait to join " + name + "'s room that it takes no more for now: try again later."
	case errors.Is(err, room.ErrFull):
		return name + "'s room is full."
	case errors.Is(err, room.ErrNotGuest):
		return name + "'s room no longer keeps your place: join again with its code."
	case errors.Is(err, room.ErrEnded):
		return "The room ended."
	case errors.Is(err, room.ErrVersion):
		return room.ErrVersion.Error() + "."
	case errors.Is(err, room.ErrNotSame):
		return "Another computer answered in place of " + name + ": the link isn't of its room."
	case errors.Is(err, errRoomAway):
		return name + "'s room isn't on the network now."
	case farUnreachable(err):
		return farProblem(name)
	case errors.Is(err, room.ErrNotLink):
		return "That isn't a link to a room."
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "Joining took too long: try again."
	}
	if err != nil && !errors.Is(err, room.ErrLost) && !isNetErr(err) {
		return err.Error()
	}
	return "Couldn't reach " + name + ": it may be asleep, or a firewall may turn MixMusic away."
}

// isNetErr reports whether err is the network's.
func isNetErr(err error) bool {
	var ne net.Error
	var oe *net.OpError
	return errors.As(err, &ne) || errors.As(err, &oe)
}
