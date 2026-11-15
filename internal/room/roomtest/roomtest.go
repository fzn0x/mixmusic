// Package roomtest is a network of rooms in memory, for tests: what each
// host announces, the others find, at 127.0.0.1, where tests listen.
package roomtest

import (
	"context"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/fzn0x/mixmusic/internal/room"
)

// Network is a network the computers of a test share.
type Network struct {
	mu        sync.Mutex
	announced []*room.Announcement
}

// New returns an empty network.
func New() *Network { return &Network{} }

// Announce adds a room to the network until stop.
func (n *Network) Announce(a room.Announcement) (func(), error) {
	p := &a
	n.mu.Lock()
	n.announced = append(n.announced, p)
	n.mu.Unlock()
	return func() {
		n.mu.Lock()
		n.announced = slices.DeleteFunc(n.announced, func(o *room.Announcement) bool { return o == p })
		n.mu.Unlock()
	}, nil
}

// Browse returns the rooms on the network, sorted by name.
func (n *Network) Browse(context.Context) ([]room.Found, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []room.Found
	for _, a := range n.announced {
		out = append(out, room.Found{ID: a.ID, Name: a.Name, Addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(a.Port))})
	}
	slices.SortFunc(out, func(a, b room.Found) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Announced returns how many rooms are announced.
func (n *Network) Announced() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.announced)
}

// Listen listens on a port of 127.0.0.1, as tests do: never on the
// network.
func Listen() (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
