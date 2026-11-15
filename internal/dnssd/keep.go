package dnssd

import (
	"log"
	"net"
	"sync"
	"time"
)

// Keep has r answer on the socket open makes until stop, and keeps that
// socket working (#725). It checks the socket now and then with a probe,
// a question to mDNS's group that no one answers, which the socket hears
// back. A socket that can't send it, or hears nothing, is checked against
// a fresh one, which takes its place if it passes. So does a socket whose
// Serve stopped on an error.
//
// macOS lets an app use the local network only once the user allows it,
// and a socket opened before that stayed deaf after it: a computer that
// turned remote control on before the answer wasn't found until it
// restarted. A fresh socket works.
func (r *Responder) Keep(open func() (net.PacketConn, error)) (stop func(), err error) {
	k := &keeper{r: r, open: open, group: mdnsGroup, first: firstCheck, wait: probeWait, every: checkEvery, after: time.After}
	return k.start()
}

// firstCheck is how long after it starts a keeper checks its socket the
// first time: soon, as the user may be answering macOS's question about
// the local network just then. probeWait is how long a probe waits to be
// heard back.
const (
	firstCheck = 5 * time.Second
	probeWait  = 2 * time.Second
)

// checkEvery is how long a keeper waits before its next check, after
// failed checks in a row: a few seconds at first, as when the user is
// about to allow the local network, and half a minute at most; a minute
// while the socket works.
func checkEvery(failed int) time.Duration {
	if failed == 0 {
		return time.Minute
	}
	return min(time.Duration(failed)*time.Duration(failed)*2*time.Second, 30*time.Second)
}

// keeper keeps a Responder answering on a working socket.
type keeper struct {
	r     *Responder
	open  func() (net.PacketConn, error)
	group net.Addr
	first time.Duration
	wait  time.Duration
	every func(failed int) time.Duration
	after func(time.Duration) <-chan time.Time

	// cur is the socket answered on: run's alone, once it started.
	cur  *served
	quit chan struct{}
	done chan struct{}
	once sync.Once
}

// served is a socket a Responder serves.
type served struct {
	conn net.PacketConn
	// read is told of each packet the socket reads.
	read chan struct{}
	// ended is closed once Serve returned.
	ended chan struct{}
}

// heard is a socket that tells served of each packet it reads.
type heard struct {
	net.PacketConn
	read chan struct{}
}

func (h heard) ReadFrom(b []byte) (int, net.Addr, error) {
	n, from, err := h.PacketConn.ReadFrom(b)
	if err == nil {
		select {
		case h.read <- struct{}{}:
		default:
		}
	}
	return n, from, err
}

func (k *keeper) start() (func(), error) {
	conn, err := k.open()
	if err != nil {
		return nil, err
	}
	k.cur = k.serve(conn)
	k.quit, k.done = make(chan struct{}), make(chan struct{})
	go k.run()
	return func() {
		k.once.Do(func() { close(k.quit) })
		<-k.done
	}, nil
}

// serve has the Responder answer on conn.
func (k *keeper) serve(conn net.PacketConn) *served {
	s := &served{conn: conn, read: make(chan struct{}, 1), ended: make(chan struct{})}
	go func() {
		defer close(s.ended)
		if err := k.r.Serve(heard{conn, s.read}); err != nil {
			log.Printf("mixmusic: answering mDNS: %v", err)
		}
	}()
	return s
}

// close closes the socket s serves and waits for its Serve to return.
func (s *served) close() {
	s.conn.Close()
	<-s.ended
}

// run checks the socket answered on until stop.
func (k *keeper) run() {
	defer close(k.done)
	defer func() { k.cur.close() }()
	failed, next := 0, k.first
	for {
		// A Serve that stopped is noted once: a fresh socket that does
		// no better leaves the next check to the clock.
		ended := k.cur.ended
		if failed > 0 {
			ended = nil
		}
		select {
		case <-k.quit:
			return
		case <-ended:
		case <-k.after(next):
		}
		failed = k.renew(failed)
		next = k.every(failed)
	}
}

// renew checks the socket answered on, after failed checks in a row, and
// puts a fresh socket in its place when it fails and the fresh one
// passes. It returns the failed checks in a row then.
func (k *keeper) renew(failed int) int {
	if k.check(k.cur) {
		return 0
	}
	conn, err := k.open()
	if err != nil {
		return failed + 1
	}
	fresh := k.serve(conn)
	if !k.check(fresh) {
		fresh.close()
		return failed + 1
	}
	log.Printf("mixmusic: mDNS answers on a new socket: the last one stopped hearing the network")
	k.cur.close()
	k.cur = fresh
	return 0
}

// check reports whether s works: its Serve runs, and it hears the probe it
// sends to the group, or another packet meanwhile, within the keeper's
// wait.
func (k *keeper) check(s *served) bool {
	select {
	case <-s.ended:
		return false
	case <-s.read:
	default:
	}
	q, err := k.r.probe()
	if err != nil {
		return false
	}
	if _, err := s.conn.WriteTo(q, k.group); err != nil {
		return false
	}
	select {
	case <-s.read:
		return true
	case <-s.ended:
	case <-k.after(k.wait):
	}
	return false
}

// probe is a question no one answers, the Responder included: of a name
// under its host's, which it has no records of.
func (r *Responder) probe() ([]byte, error) {
	return appendQuery(nil, 0, []question{{name: "probe." + r.hostName(), qtype: typeTXT}})
}
