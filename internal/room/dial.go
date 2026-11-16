package room

// A room that friends outside the network join is reached at several
// addresses (#562): the host's on the network, as discovery found it, and
// those of its invite link, its router's internet address, its public
// IPv6 address, or one the host typed. A guest that knows the host's key
// connects to them all, each a moment after the one before, TLS and the
// key's pin too, and keeps the first that is the host: an address that
// answers first but isn't, as one gone stale, or another's, wins nothing.
// One that knows no key, as with a code, connects where it was told alone.

import (
	"context"
	"crypto/tls"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// dialStagger is how long a connection has before the next address is
// tried beside it.
const dialStagger = 250 * time.Millisecond

// UnreachableError: none of Addrs answered as the room.
type UnreachableError struct {
	Addrs []string
	Err   error
}

func (e *UnreachableError) Error() string {
	return "couldn't reach " + strings.Join(e.Addrs, " or ") + ": " + e.Err.Error()
}

func (e *UnreachableError) Unwrap() error { return e.Err }

// addrsOf are the addresses g connects to for addr: addr, then those
// Elsewhere tells, each once, when the host's key is pinned.
func (g *Guest) addrsOf(addr string, pinned *remote.Fingerprint) []string {
	addrs := []string{addr}
	if pinned == nil || g.Elsewhere == nil {
		return addrs
	}
	for _, a := range g.Elsewhere() {
		if a != "" && !slices.Contains(addrs, a) {
			addrs = append(addrs, a)
		}
	}
	return addrs
}

// connectFirst connects to each of addrs, dialStagger after the one before
// while none is the host, as connect does, and returns the first that
// passes TLS and the pin, closing those that pass after it.
func (g *Guest) connectFirst(ctx context.Context, addrs []string, pinned *remote.Fingerprint) (*tls.Conn, remote.Fingerprint, error) {
	one := *g
	one.Elsewhere = nil
	type result struct {
		tc   *tls.Conn
		host remote.Fingerprint
		err  error
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan result, len(addrs))
	next := time.NewTimer(0)
	defer next.Stop()
	started, failed := 0, 0
	var errs []error
	// drain closes what the n tries under way still connect.
	drain := func(n int) {
		go func() {
			for range n {
				if r := <-results; r.tc != nil {
					r.tc.Close()
				}
			}
		}()
	}
	for {
		select {
		case <-next.C:
			addr := addrs[started]
			started++
			go func() {
				tc, host, err := one.connect(ctx, addr, pinned)
				results <- result{tc, host, err}
			}()
			if started < len(addrs) {
				next.Reset(dialStagger)
			}
		case r := <-results:
			if r.err == nil {
				cancel()
				drain(started - failed - 1)
				return r.tc, r.host, nil
			}
			failed++
			errs = append(errs, r.err)
			if failed == len(addrs) {
				return nil, remote.Fingerprint{}, &UnreachableError{Addrs: addrs, Err: errors.Join(errs...)}
			}
			if started == failed && started < len(addrs) {
				// Every try so far failed: the next goes at once.
				next.Reset(0)
			}
		case <-ctx.Done():
			drain(started - failed)
			// The addresses that never answered, as a port no router
			// forwards drops what comes.
			return nil, remote.Fingerprint{}, &UnreachableError{Addrs: addrs, Err: errors.Join(append(errs, ctx.Err())...)}
		}
	}
}
