package app

// Listen with friends (#561): this computer hosts a room on the network,
// which the other MixMusic apps join by its code or its invite link, and
// whose guests play what this one plays, in step (roomhost.go); or joins
// another's room, and follows what its host plays (roomguest.go,
// roomfollow.go). internal/room is the protocol;
// docs/architecture/rooms.md tells why it is so.
//
// A room keeps nothing on the disk: it lives while its host keeps it open.
// It speaks with the key of remote control, which the credential store
// keeps.

import (
	"context"
	"log"
	"net"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/portmap"
	"github.com/fzn0x/mixmusic/internal/room"
)

// RoomOptions configure rooms in place of the network and the engine:
// tests set them, so that nothing listens but on 127.0.0.1.
type RoomOptions struct {
	// Discovery finds and announces the rooms in place of mDNS.
	Discovery room.Discovery
	// Listen listens for the guests' connections in place of every
	// network.
	Listen func() (net.Listener, error)
	// Router asks the routers for the forward of the room's port, in place
	// of the network's, and IPv6 tells this computer's public IPv6
	// addresses (roomreach.go): without Router, nothing is asked.
	Router *portmap.Client
	IPv6   func() []netip.Addr
	// Timing is the protocol's.
	Timing room.Timing
	// Song finds a song a guest adds by its ID, in place of its service
	// (roomrules.go).
	Song func(ctx context.Context, t media.Track) (media.Track, error)
	// Heard tells where the player's track is heard now, in place of the
	// engine; SetSpeed plays it faster or slower in place of the engine.
	Heard    func(player.State) time.Duration
	SetSpeed func(speed float64)
}

// roomState is Listen with friends', on both sides.
type roomState struct {
	// clock is this computer's, which a room's positions are at.
	clockOnce sync.Once
	clock     room.SystemClock
	flushing  sync.Once

	mu sync.Mutex
	// hosted is the room this computer hosts, and guest the one it is a
	// guest of: one or the other at once.
	hosted *hostedRoom
	guest  *guestRoom
	// The rooms found, and when a look last ran.
	found    map[string]roomFound
	browsing bool
	browsed  time.Time
	shown    atomic.Pointer[[]FoundRoom]
	watchers []*func(string)
	// far is what rooms keep of friends outside the network (roomreach.go,
	// roomfar.go).
	far roomFar
	// held is the start of the host's track that waits for its guests
	// (roomhost.go), and following is set while the player follows a host
	// (roomfollow.go).
	held      heldStart
	following atomic.Bool
	// fade is the crossfade that waits for the guests (roomfade.go).
	fade heldFade
}

// Rooms is Listen with friends, for the windows.
type Rooms struct{ a *App }

// Rooms returns Listen with friends.
func (a *App) Rooms() *Rooms { return &Rooms{a} }

// roomOptions are the options of rooms, the network's when the app's have
// none.
func (a *App) roomOptions() RoomOptions {
	var o RoomOptions
	if a.opts.Room != nil {
		o = *a.opts.Room
	}
	if o.Discovery == nil {
		o.Discovery = room.MDNS{}
	}
	if o.Listen == nil {
		// Every network of the computer, at the port kept for rooms: the
		// guests on the network learn it from the announcement, those
		// outside from the invite link.
		o.Listen = a.listenRoom
	}
	o.Listen = a.reachable(o.Listen)
	return o
}

// roomClock is this computer's clock, for rooms.
func (a *App) roomClock() room.SystemClock {
	r := &a.room
	r.clockOnce.Do(func() { r.clock = room.NewSystemClock() })
	return r.clock
}

// roomName is the user's name in rooms: the settings', or the computer's.
func (a *App) roomName() string {
	if name := a.settings.Get().Clean().RoomName; name != "" {
		return name
	}
	return room.CleanName(a.remoteOptions().Name)
}

// Name returns the user's name in rooms.
func (rs *Rooms) Name() string { return rs.a.roomName() }

// keepRooms has what runs of rooms end as MixMusic quits.
func (a *App) keepRooms() {
	a.room.flushing.Do(func() { a.OnFlush(a.closeRooms) })
}

// closeRooms ends the room this computer hosts, and leaves the one it is a
// guest of, as MixMusic quits.
func (a *App) closeRooms() {
	r := &a.room
	r.mu.Lock()
	h, g := r.hosted, r.guest
	r.hosted, r.guest = nil, nil
	r.mu.Unlock()
	if h != nil {
		h.close()
	}
	if g != nil {
		g.quit(a)
	}
}

// watchRooms looks for the rooms on the network a moment after start, then
// every castBrowseEvery, as for cast devices, and has a guest that dropped
// off try again at once as the computer wakes.
func (a *App) watchRooms() {
	go func() {
		time.Sleep(castFirstBrowse)
		for {
			a.browseRooms()
			time.Sleep(castBrowseEvery)
		}
	}()
	mygo.Power.OnResume(a.roomsWoke)
}

// FoundRoom is a room on the network: its host's ID and name, as they tell
// them.
type FoundRoom struct {
	ID, Name string
}

type roomFound struct {
	found room.Found
	seen  time.Time
}

// Found returns the rooms on the network, but this computer's, sorted by
// name, for a frame: don't change it.
func (rs *Rooms) Found() []FoundRoom {
	if l := rs.a.room.shown.Load(); l != nil {
		return *l
	}
	return nil
}

// Find looks for the rooms at once, as the devices menu opens, unless it
// looked a moment ago.
func (rs *Rooms) Find() {
	r := &rs.a.room
	r.mu.Lock()
	recent := time.Since(r.browsed) < castBrowseAgain
	r.mu.Unlock()
	if !recent {
		go rs.a.browseRooms()
	}
}

// browseRooms looks for the rooms once, unless a look is under way, and
// tells the windows when the list changes.
func (a *App) browseRooms() {
	r := &a.room
	r.mu.Lock()
	if r.browsing {
		r.mu.Unlock()
		return
	}
	r.browsing = true
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	found, err := a.roomOptions().Discovery.Browse(ctx)
	cancel()
	if err != nil {
		log.Printf("mixmusic: looking for rooms: %v", err)
	}
	now := time.Now()
	r.mu.Lock()
	r.browsing, r.browsed = false, now
	if r.found == nil {
		r.found = map[string]roomFound{}
	}
	for _, f := range found {
		r.found[f.ID] = roomFound{found: f, seen: now}
	}
	for id, f := range r.found {
		if now.Sub(f.seen) > castForget {
			delete(r.found, id)
		}
	}
	r.mu.Unlock()
	if a.relistRooms() {
		a.notifyRooms("")
	}
}

// relistRooms makes the list the windows show of the rooms found, and
// reports whether it changed.
func (a *App) relistRooms() bool {
	self := ""
	if id := a.remote.id.Load(); id != nil {
		self = id.Fingerprint().ID()
	}
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	shown := make([]FoundRoom, 0, len(r.found))
	for id, f := range r.found {
		if id != self {
			shown = append(shown, FoundRoom{ID: id, Name: f.found.Name})
		}
	}
	slices.SortFunc(shown, func(x, y FoundRoom) int {
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

// foundRoom is where the room of the host id was found, looking for it
// once more when it wasn't.
func (a *App) foundRoom(id string, look bool) (room.Found, bool) {
	r := &a.room
	r.mu.Lock()
	f, ok := r.found[id]
	r.mu.Unlock()
	if !ok && look {
		a.browseRooms()
		r.mu.Lock()
		f, ok = r.found[id]
		r.mu.Unlock()
	}
	if !ok {
		// Where its invite link reaches it, from outside the network.
		return a.linkRoom(id)
	}
	return f.found, ok
}

// OnRooms has fn told, from any goroutine, as what rooms show changes,
// until cancel: note, when not "", is what to tell, as a guest that
// joined.
func (a *App) OnRooms(fn func(note string)) (cancel func()) {
	f := &fn
	r := &a.room
	r.mu.Lock()
	r.watchers = append(slices.Clone(r.watchers), f)
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		r.watchers = slices.DeleteFunc(slices.Clone(r.watchers), func(g *func(string)) bool { return g == f })
		r.mu.Unlock()
	}
}

func (a *App) notifyRooms(note string) {
	r := &a.room
	r.mu.Lock()
	fns := r.watchers
	r.mu.Unlock()
	for _, fn := range fns {
		(*fn)(note)
	}
}
