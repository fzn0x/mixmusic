package app

// The side of remote control that controls: the computers with remote
// control on that this one finds on the network, pairing with one, and a
// session with the one whose page shows, which ends once the page has
// been gone a while.

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// These times are remote control's; tests shorten them.
var (
	// remoteIdle is how long a session lasts once its page no longer
	// shows.
	remoteIdle = 30 * time.Second
	// remoteRetry is how long after a session ended the page connects
	// again, at first, and remoteRetryMost at most, doubling between.
	remoteRetry     = time.Second
	remoteRetryMost = 10 * time.Second
	// remotePairFor bounds a pairing, the user's typing of the code too.
	remotePairFor = 3 * time.Minute
)

// RemoteComputer is a computer with remote control on, found on the
// network: its ID, its name as it tells it, and whether this computer
// controls it, paired.
type RemoteComputer struct {
	ID, Name string
	Paired   bool
}

type remoteFound struct {
	found remote.Found
	seen  time.Time
}

// Computers returns the computers with remote control on found on the
// network, but this one, sorted by name, for a frame: don't change it.
func (rc *Remote) Computers() []RemoteComputer {
	if l := rc.a.remote.shown.Load(); l != nil {
		return *l
	}
	return nil
}

// Find looks for the computers at once, as the devices menu opens, unless
// it looked a moment ago.
func (rc *Remote) Find() {
	r := &rc.a.remote
	r.mu.Lock()
	recent := time.Since(r.browsed) < castBrowseAgain
	r.mu.Unlock()
	if !recent {
		go rc.a.browseRemote()
	}
}

// browseRemote looks for the computers once, unless a look is under way,
// and tells the windows when the list changes.
func (a *App) browseRemote() {
	r := &a.remote
	r.mu.Lock()
	if r.browsing {
		r.mu.Unlock()
		return
	}
	r.browsing = true
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	found, err := a.remoteOptions().Discovery.Browse(ctx)
	cancel()
	if err != nil {
		log.Printf("mixmusic: looking for computers with remote control on: %v", err)
	}
	now := time.Now()
	r.mu.Lock()
	r.browsing, r.browsed = false, now
	if r.found == nil {
		r.found = map[string]remoteFound{}
	}
	for _, f := range found {
		r.found[f.ID] = remoteFound{found: f, seen: now}
	}
	for id, f := range r.found {
		if now.Sub(f.seen) > castForget && (r.control == nil || r.control.id != id) {
			delete(r.found, id)
		}
	}
	r.mu.Unlock()
	if a.relistRemote() {
		a.notifyRemote("")
	}
}

// relistRemote makes the list the windows show of the computers found, and
// reports whether it changed.
func (a *App) relistRemote() bool {
	kept := a.remoteKept()
	r := &a.remote
	r.mu.Lock()
	defer r.mu.Unlock()
	self := ""
	if id := r.id.Load(); id != nil {
		self = id.Fingerprint().ID()
	}
	shown := make([]RemoteComputer, 0, len(r.found))
	for id, f := range r.found {
		if id == self {
			continue
		}
		paired := slices.ContainsFunc(kept.Computers, func(c PairedComputer) bool { return c.ID == id && c.Controlled })
		shown = append(shown, RemoteComputer{ID: id, Name: f.found.Name, Paired: paired})
	}
	slices.SortFunc(shown, func(x, y RemoteComputer) int {
		if n := strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)); n != 0 {
			return n
		}
		return strings.Compare(x.ID, y.ID)
	})
	if old := r.shown.Load(); old != nil && slices.Equal(*old, shown) {
		return false
	}
	r.shown.Store(&shown)
	return true
}

// RemoteStep is where the page of a computer is.
type RemoteStep int

const (
	// RemoteAway: the computer isn't on the network, or not yet found.
	RemoteAway RemoteStep = iota
	// RemoteUnpaired: it is, and this computer doesn't control it yet.
	RemoteUnpaired
	// RemotePairing: this computer asks to pair; RemoteCodeAsked: the
	// other shows its code, which the user types; RemoteChecking: the two
	// prove it.
	RemotePairing
	RemoteCodeAsked
	RemoteChecking
	// RemoteConnecting: paired, it connects; RemoteConnected: it shows
	// what plays there.
	RemoteConnecting
	RemoteConnected
)

// RemoteView is what the page of a computer shows: the computer, where
// the page is, why the last try failed, if it did, and what plays there,
// as the state that came At tells, once one came.
type RemoteView struct {
	Computer RemoteComputer
	Step     RemoteStep
	Problem  string
	State    remote.State
	At       time.Time
	HasState bool
	// CanQueue is set when the computer takes the commands on its queue,
	// and CanHandOver when it takes what plays here: an older MixMusic
	// doesn't.
	CanQueue, CanHandOver bool
}

// remoteControl is the session with the computer whose page shows.
type remoteControl struct {
	id      string
	session *remote.Session
	// connecting is set while it connects; problem is why it last
	// failed, and retryAt when it tries again, after tries tries.
	connecting bool
	problem    string
	retryAt    time.Time
	tries      int
	touched    time.Time
	idle       *time.Timer
	stopWatch  func()
	closed     bool
	mu         sync.Mutex
}

func (c *remoteControl) close() {
	c.mu.Lock()
	c.closed = true
	s, idle := c.session, c.idle
	c.mu.Unlock()
	if idle != nil {
		idle.Stop()
	}
	if s != nil {
		s.Close()
	}
}

// remotePairing is a pairing this computer runs with another.
type remotePairing struct {
	id      string
	name    string
	step    RemoteStep
	problem string
	codes   chan string
	cancel  context.CancelFunc
}

// paired returns the computer id that this computer controls, as kept.
func (a *App) paired(id string) (PairedComputer, bool) {
	for _, c := range a.remoteKept().Computers {
		if c.ID == id && c.Controlled {
			return c, true
		}
	}
	return PairedComputer{}, false
}

// dialer is how this computer pairs and connects. r.mu must not be held;
// wait is identity's.
func (a *App) dialer(wait bool) (*remote.Dialer, error) {
	id, err := a.identity(wait)
	if err != nil {
		return nil, err
	}
	o := a.remoteOptions()
	return &remote.Dialer{Identity: id, Name: o.Name, Timing: o.Timing}, nil
}

// View returns what the page of the computer id shows, in a frame that
// shows it: it keeps a session with it while paired, connecting again after
// a while when one ends, until the page has been gone remoteIdle. Only
// one computer has a session at once.
func (rc *Remote) View(id string) RemoteView {
	a := rc.a
	r := &a.remote
	kept, isPaired := a.paired(id)
	r.mu.Lock()
	f, isFound := r.found[id]
	v := RemoteView{Computer: RemoteComputer{ID: id, Name: f.found.Name, Paired: isPaired}}
	if !isFound {
		v.Computer.Name = kept.Name
	}
	if v.Computer.Name == "" {
		v.Computer.Name = "That computer"
	}
	if p := r.pairing; p != nil && p.id == id {
		v.Step, v.Problem = p.step, p.problem
		r.mu.Unlock()
		return v
	}
	if !isPaired {
		v.Step = RemoteAway
		if isFound {
			v.Step = RemoteUnpaired
		}
		// Why the session ended, as the other computer removed this one.
		ctl := r.control
		r.mu.Unlock()
		if ctl != nil && ctl.id == id {
			ctl.mu.Lock()
			v.Problem = ctl.problem
			ctl.mu.Unlock()
		}
		return v
	}
	ctl := r.control
	if ctl == nil || ctl.id != id {
		if ctl != nil {
			go ctl.close()
		}
		ctl = &remoteControl{id: id}
		r.control = ctl
	}
	r.mu.Unlock()

	now := time.Now()
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	ctl.touched = now
	if ctl.idle == nil {
		ctl.idle = time.AfterFunc(remoteIdle, func() { a.idleControl(ctl) })
	}
	v.Problem = ctl.problem
	switch {
	case ctl.session != nil:
		v.Step = RemoteConnected
		v.State, v.At, v.HasState = ctl.session.State()
		v.CanQueue, v.CanHandOver = ctl.session.Can(remote.FeatureQueue), ctl.session.Can(remote.FeatureHandOver)
		return v
	case !isFound:
		v.Step = RemoteAway
		return v
	}
	v.Step = RemoteConnecting
	if !ctl.connecting && now.After(ctl.retryAt) {
		ctl.connecting = true
		go a.connectRemote(ctl, f.found.Addr, kept)
	}
	return v
}

// idleControl ends the session of ctl once its page has been gone
// remoteIdle.
func (a *App) idleControl(ctl *remoteControl) {
	ctl.mu.Lock()
	left := remoteIdle - time.Since(ctl.touched)
	if left > 0 && !ctl.closed {
		ctl.idle.Reset(left)
		ctl.mu.Unlock()
		return
	}
	ctl.mu.Unlock()
	r := &a.remote
	r.mu.Lock()
	if r.control == ctl {
		r.control = nil
	}
	r.mu.Unlock()
	ctl.close()
}

// endControl ends the session with the computer id, if one runs.
func (a *App) endControl(id string) {
	r := &a.remote
	r.mu.Lock()
	ctl := r.control
	if ctl != nil && ctl.id == id {
		r.control = nil
	} else {
		ctl = nil
	}
	r.mu.Unlock()
	if ctl != nil {
		ctl.close()
	}
}

// connectRemote connects to the computer paired at addr, for ctl.
func (a *App) connectRemote(ctl *remoteControl, addr string, kept PairedComputer) {
	fail := func(err error) {
		ctl.mu.Lock()
		ctl.connecting = false
		ctl.problem = remoteProblem(kept.Name, err)
		ctl.retryAt = time.Now().Add(min(remoteRetry<<min(ctl.tries, 8), remoteRetryMost))
		ctl.tries++
		ctl.mu.Unlock()
		if errors.Is(err, remote.ErrNotPaired) {
			if key, kerr := remote.ParseFingerprint(kept.Key); kerr == nil {
				a.dropControlled(key)
			}
			a.relistRemote()
			a.notifyRemote(kept.Name + " removed this computer: pair again to control it")
			return
		}
		// The computer may listen elsewhere since it started again: it is
		// looked for anew, unless it was a moment ago.
		a.Remote().Find()
		a.notifyRemote("")
	}
	key, err := remote.ParseFingerprint(kept.Key)
	if err != nil {
		fail(err)
		return
	}
	// On a goroutine of its own: it waits for the key another call reads.
	d, err := a.dialer(true)
	if err != nil {
		fail(err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	s, err := d.Connect(ctx, addr, key)
	cancel()
	if err != nil {
		fail(err)
		return
	}
	ctl.mu.Lock()
	if ctl.closed {
		ctl.mu.Unlock()
		s.Close()
		return
	}
	ctl.session, ctl.connecting, ctl.problem, ctl.tries = s, false, "", 0
	ctl.mu.Unlock()
	stop := s.Watch(func() { a.notifyRemote("") })
	a.notifyRemote("")
	go func() {
		<-s.Done()
		stop()
		ctl.mu.Lock()
		if ctl.session == s {
			ctl.session = nil
		}
		closed := ctl.closed
		ctl.mu.Unlock()
		if closed {
			return
		}
		log.Printf("mixmusic: remote control: the session with %s ended: %v", kept.Name, s.Err())
		fail(s.Err())
	}()
}

// Do sends a command to the computer id, whose page shows.
func (rc *Remote) Do(id string, c remote.Command) error {
	s, err := rc.session(id)
	if err != nil {
		return err
	}
	return s.Do(c)
}

// Pair starts pairing with the computer id: once it shows its code,
// the page asks for it (EnterCode).
func (rc *Remote) Pair(id string) {
	a := rc.a
	r := &a.remote
	// The key first, with r.mu free: see identity.
	d, err := a.dialer(false)
	r.mu.Lock()
	f, ok := r.found[id]
	if !ok || r.pairing != nil && r.pairing.id == id && r.pairing.step != RemoteUnpaired {
		r.mu.Unlock()
		return
	}
	if r.pairing != nil {
		r.pairing.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), remotePairFor)
	p := &remotePairing{id: id, name: f.found.Name, step: RemotePairing, codes: make(chan string, 1), cancel: cancel}
	if err != nil {
		p.step, p.problem = RemoteUnpaired, err.Error()
	}
	r.pairing = p
	r.mu.Unlock()
	a.notifyRemote("")
	if err != nil {
		cancel()
		return
	}
	go a.pair(ctx, d, p, f.found)
}

// pair runs p with the computer found at f.
func (a *App) pair(ctx context.Context, d *remote.Dialer, p *remotePairing, f remote.Found) {
	defer p.cancel()
	r := &a.remote
	step := func(s RemoteStep) {
		r.mu.Lock()
		p.step = s
		r.mu.Unlock()
		a.notifyRemote("")
	}
	peer, err := d.Pair(ctx, f.Addr, func(ctx context.Context) (string, error) {
		step(RemoteCodeAsked)
		select {
		case code := <-p.codes:
			step(RemoteChecking)
			return code, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	if err == nil && peer.Key.ID() != f.ID {
		// The ID it told isn't its key's.
		err = remote.ErrNotSame
	}
	if err == nil {
		err = a.keepPaired(peer.Key, peer.Name, false)
	}
	r.mu.Lock()
	mine := r.pairing == p
	if mine && err == nil {
		r.pairing = nil
	}
	if mine && err != nil {
		p.step, p.problem = RemoteUnpaired, remoteProblem(f.Name, err)
	}
	r.mu.Unlock()
	if err != nil {
		log.Printf("mixmusic: remote control: pairing with %s: %v", f.Name, err)
		a.notifyRemote("")
		return
	}
	log.Printf("mixmusic: remote control: paired with %s, which this computer controls", peer.Name)
	// A session that ended as the computer removed this one gives way to
	// a new one, at once.
	a.endControl(f.ID)
	a.relistRemote()
	a.notifyRemote("Paired with " + peer.Name)
}

// EnterCode gives the pairing with the computer id the code the user
// typed.
func (rc *Remote) EnterCode(id, code string) {
	r := &rc.a.remote
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.pairing; p != nil && p.id == id && p.step == RemoteCodeAsked {
		select {
		case p.codes <- code:
		default:
		}
	}
}

// CancelPairing ends the pairing with the computer id, and forgets why one
// failed.
func (rc *Remote) CancelPairing(id string) {
	r := &rc.a.remote
	r.mu.Lock()
	p := r.pairing
	if p != nil && p.id == id {
		r.pairing = nil
	} else {
		p = nil
	}
	r.mu.Unlock()
	if p != nil {
		p.cancel()
		rc.a.notifyRemote("")
	}
}

// remoteProblem tells why remote control with the computer named name
// failed, in the interface's words.
func remoteProblem(name string, err error) string {
	switch {
	case errors.Is(err, remote.ErrWrongCode):
		return "That isn't the code " + name + " showed. Pair again for a new code."
	case errors.Is(err, remote.ErrOff):
		return "Remote control is off on " + name + "."
	case errors.Is(err, remote.ErrBusy):
		return name + " is pairing with another computer: try again in a moment."
	case errors.Is(err, remote.ErrCanceled):
		return "The pairing was canceled on " + name + "."
	case errors.Is(err, remote.ErrNotPaired):
		return name + " removed this computer: pair again to control it."
	case errors.Is(err, remote.ErrNotSame):
		return "Another computer answered in place of " + name + ": it isn't the one paired."
	case errors.Is(err, remote.ErrVersion):
		return remote.ErrVersion.Error() + "."
	case errors.Is(err, remote.ErrOlder):
		return "MixMusic on " + name + " is older, and can't do that: update it there."
	case errors.Is(err, remote.ErrLost):
		return name + " stopped answering: connecting again…"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "The pairing took too long. Pair again for a new code."
	}
	return "Couldn't reach " + name + ": it may be asleep, or a firewall may turn MixMusic away."
}
