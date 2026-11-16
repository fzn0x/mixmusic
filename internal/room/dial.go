package room

// A room that friends outside the network join is reached at several
// addresses (#562): the host's on the network, as discovery found it, and
// those of its invite link, its router's internet address, its public
// IPv6 address, or one the host typed. A guest dials them all, each a
// moment after the one before, and keeps the first that answers.

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

// dialStagger is how long a dial has before the next address is dialed
// beside it.
const dialStagger = 250 * time.Millisecond

// UnreachableError: none of Addrs answered.
type UnreachableError struct {
	Addrs []string
	Err   error
}

func (e *UnreachableError) Error() string {
	return "couldn't reach " + strings.Join(e.Addrs, " or ") + ": " + e.Err.Error()
}

func (e *UnreachableError) Unwrap() error { return e.Err }

// DialFirst connects to the first of addrs that answers, by TCP: it dials
// each, dialStagger after the one before while none answered, and closes
// those that answer after the first. dial is a net.Dialer's when nil.
func DialFirst(ctx context.Context, dial func(ctx context.Context, network, addr string) (net.Conn, error), addrs []string) (net.Conn, error) {
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	if len(addrs) == 0 {
		return nil, &UnreachableError{Err: errors.New("no address")}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		c   net.Conn
		err error
	}
	results := make(chan result, len(addrs))
	next := time.NewTimer(0)
	defer next.Stop()
	started, failed := 0, 0
	var errs []error
	for {
		select {
		case <-next.C:
			addr := addrs[started]
			started++
			go func() {
				c, err := dial(ctx, "tcp", addr)
				results <- result{c, err}
			}()
			if started < len(addrs) {
				next.Reset(dialStagger)
			}
		case r := <-results:
			if r.err == nil {
				// The others give up, and those that answered meanwhile
				// close.
				cancel()
				go func(n int) {
					for range n {
						if o := <-results; o.c != nil {
							o.c.Close()
						}
					}
				}(started - failed - 1)
				return r.c, nil
			}
			failed++
			errs = append(errs, r.err)
			if failed == len(addrs) {
				return nil, &UnreachableError{Addrs: addrs, Err: errors.Join(errs...)}
			}
			if started == failed && started < len(addrs) {
				// Every dial so far failed: the next goes at once.
				next.Reset(0)
			}
		case <-ctx.Done():
			go func(n int) {
				for range n {
					if o := <-results; o.c != nil {
						o.c.Close()
					}
				}
			}(started - failed)
			return nil, ctx.Err()
		}
	}
}

// Over returns g, connecting to the first of addrs that answers
// (DialFirst), in place of the address its calls are given.
func (g *Guest) Over(addrs []string) *Guest {
	over := *g
	dial := g.Dial
	over.Dial = func(ctx context.Context, _, _ string) (net.Conn, error) { return DialFirst(ctx, dial, addrs) }
	return &over
}
