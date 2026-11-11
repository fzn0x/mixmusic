package app

// Remote control (#353): Settings › Remote control lets the other
// MixMusic apps on the network find this computer, pair with it by a code
// it shows, and control what it plays (remotehost.go); and this computer
// finds the others, pairs with them and controls them, from the page of
// each (remotecontrol.go). internal/remote is the protocol;
// docs/architecture/remote.md tells why it is so.
//
// remote.json in the data folder keeps whether it is on and the computers
// paired; the credential store keeps the computer's key.

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/store"
)

// RemoteOptions configure remote control in place of the network and the
// credential store: tests set them, so that nothing listens but on
// 127.0.0.1.
type RemoteOptions struct {
	// Discovery finds and announces the computers in place of mDNS.
	Discovery remote.Discovery
	// Listen listens for the others' connections in place of every
	// network.
	Listen func() (net.Listener, error)
	// Identity is the computer's key in place of the one the credential
	// store keeps, and Name its name in place of its host name.
	Identity *remote.Identity
	Name     string
	// Timing is the protocol's.
	Timing remote.Timing
	// Player is the player the others control, in place of the app's:
	// the window's, in tests of the windows.
	Player func() *player.Player
}

// remoteSecret is the key of the computer's key in the credential store.
const remoteSecret = "remote control key"

// remoteFile is remote.json: whether remote control is on, and the
// computers paired with this one.
type remoteFile struct {
	// On lets the other computers find this one, pair with it, and control
	// it. It is off at first.
	On bool `json:"on"`
	// Computers are those paired, in the order they paired.
	Computers []PairedComputer `json:"computers"`
}

// PairedComputer is a computer paired with this one: its ID, its key's
// fingerprint, its name as it told it, and what each does to the other.
type PairedComputer struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	// ControlsThis is set when it may control this computer, and Controlled
	// when this one controls it.
	ControlsThis bool      `json:"controlsThis,omitempty"`
	Controlled   bool      `json:"controlled,omitempty"`
	Paired       time.Time `json:"paired"`
}

// remoteKey is the computer's key, as the credential store keeps it.
type remoteKey struct {
	Seed []byte `json:"seed"`
}

// remoteState is remote control's, on both sides.
type remoteState struct {
	// once opens remote.json, at the first use of remote control.
	once     sync.Once
	file     *store.File[remoteFile]
	fileErr  error
	flushing sync.Once

	// id is the computer's key, once read or made. idMu is held while it
	// is read or made, apart from mu: see identity.
	id   atomic.Pointer[remote.Identity]
	idMu sync.Mutex
	// keys keeps the key: the credential store when nil; tests give
	// theirs. idMu is held.
	keys *secretStore

	mu sync.Mutex
	// server runs while remote control is on, announced until unannounce.
	server     *remote.Server
	unannounce func()
	// code is the code this computer shows while another pairs with it.
	code *RemoteCode
	// The computers found, and when a look last ran.
	found    map[string]remoteFound
	browsing bool
	browsed  time.Time
	shown    atomic.Pointer[[]RemoteComputer]
	// control is the computer whose page shows, and pairing a pairing
	// this computer runs with another.
	control  *remoteControl
	pairing  *remotePairing
	watchers []*func(string)
}

// Remote is remote control, for the windows.
type Remote struct{ a *App }

// Remote returns remote control.
func (a *App) Remote() *Remote { return &Remote{a} }

// remoteOptions are the options of remote control, the network's when the
// app's have none.
func (a *App) remoteOptions() RemoteOptions {
	var o RemoteOptions
	if a.opts.Remote != nil {
		o = *a.opts.Remote
	}
	if o.Discovery == nil {
		o.Discovery = remote.MDNS{}
	}
	if o.Listen == nil {
		// Every network of the computer, at a port the system picks: the
		// others learn it from the announcement.
		o.Listen = func() (net.Listener, error) { return net.Listen("tcp", ":0") }
	}
	if o.Name == "" {
		o.Name = computerName()
	}
	return o
}

// computerName is the computer's name, which the others show: its host
// name, without a domain.
func computerName() string {
	name, err := os.Hostname()
	if err != nil {
		return "Another computer"
	}
	name, _, _ = strings.Cut(name, ".")
	if name == "" {
		return "Another computer"
	}
	return name
}

// remoteFile opens remote.json at its first use.
func (a *App) remoteFile() (*store.File[remoteFile], error) {
	r := &a.remote
	r.once.Do(func() {
		r.file, r.fileErr = store.Open(filepath.Join(a.opts.DataDir, "remote.json"), remoteFile{})
		if r.fileErr != nil {
			r.fileErr = fmt.Errorf("opening remote.json: %w", r.fileErr)
		}
	})
	return r.file, r.fileErr
}

// remoteKept returns what remote.json keeps, or nothing when it doesn't
// open.
func (a *App) remoteKept() remoteFile {
	f, err := a.remoteFile()
	if err != nil {
		return remoteFile{}
	}
	return f.Get()
}

// errKeyBusy is what identity returns while another call reads or makes
// the key.
var errKeyBusy = errors.New("this computer's key is being read: try again in a moment")

// identity returns the computer's key, made and kept at its first use.
//
// r.mu must not be held, nor any lock a frame takes: on the main thread,
// mygo reads and keeps a secret on another goroutine and runs the window's
// events while it waits, and a frame then takes r.mu (Code, View) (#464).
// For the same reason, a call while another reads or makes the key returns
// errKeyBusy, unless wait: on the main thread, the other may be this very
// thread's, under a click it handles meanwhile. Only callers on another
// goroutine wait.
func (a *App) identity(wait bool) (*remote.Identity, error) {
	r := &a.remote
	if id := r.id.Load(); id != nil {
		return id, nil
	}
	// What runs of remote control, on either side, ends as MixMusic
	// quits.
	r.flushing.Do(func() { a.OnFlush(a.closeRemote) })
	if o := a.opts.Remote; o != nil && o.Identity != nil {
		r.id.CompareAndSwap(nil, o.Identity)
		return r.id.Load(), nil
	}
	if wait {
		r.idMu.Lock()
	} else if !r.idMu.TryLock() {
		return nil, errKeyBusy
	}
	defer r.idMu.Unlock()
	if id := r.id.Load(); id != nil {
		return id, nil
	}
	if r.keys == nil {
		r.keys = newSecretStore(a.opts.DataDir, remoteSecret, "remote-key.json", "remote-key.dpapi")
	}
	keep := jsonSecret[remoteKey]{r.keys}
	k, err := keep.Load()
	if err == nil {
		var id *remote.Identity
		if id, err = remote.IdentityOf(k.Seed); err == nil {
			r.id.Store(id)
			return id, nil
		}
		log.Printf("mixmusic: the remote control key doesn't read (%v): making another, which the computers paired don't know", err)
	} else if !errors.Is(err, mygo.ErrSecretNotFound) {
		return nil, fmt.Errorf("reading this computer's key: %w", err)
	}
	id, err := remote.NewIdentity()
	if err != nil {
		return nil, err
	}
	if err := keep.Save(remoteKey{Seed: id.Seed()}); err != nil {
		return nil, fmt.Errorf("keeping this computer's key: %w", err)
	}
	r.id.Store(id)
	return id, nil
}

// On reports whether remote control is on: whether the others find this
// computer, and may pair with it.
func (rc *Remote) On() bool { return rc.a.remoteKept().On }

// SetOn turns remote control on or off, now and at the next start.
func (rc *Remote) SetOn(on bool) error {
	a := rc.a
	f, err := a.remoteFile()
	if err != nil {
		return err
	}
	if on {
		if err := a.startRemote(); err != nil {
			return err
		}
	} else {
		a.stopRemote()
	}
	if err := f.Update(func(v *remoteFile) error { v.On = on; return nil }); err != nil {
		return err
	}
	a.notifyRemote("")
	return nil
}

// Name returns the name the others show this computer by.
func (rc *Remote) Name() string { return rc.a.remoteOptions().Name }

// Paired returns the computers paired with this one, in the order they
// paired.
func (rc *Remote) Paired() []PairedComputer { return slices.Clone(rc.a.remoteKept().Computers) }

// Forget forgets the computer id: it may no longer control this one, and
// this one no longer controls it. A session of it ends at once.
func (rc *Remote) Forget(id string) error {
	a := rc.a
	f, err := a.remoteFile()
	if err != nil {
		return err
	}
	var gone PairedComputer
	err = f.Update(func(v *remoteFile) error {
		i := slices.IndexFunc(v.Computers, func(c PairedComputer) bool { return c.ID == id })
		if i < 0 {
			return nil
		}
		gone = v.Computers[i]
		v.Computers = slices.Delete(slices.Clone(v.Computers), i, i+1)
		return nil
	})
	if err != nil {
		return err
	}
	if gone.ID == "" {
		return nil
	}
	log.Printf("mixmusic: remote control: forgot %s", gone.Name)
	r := &a.remote
	r.mu.Lock()
	server := r.server
	r.mu.Unlock()
	if key, err := remote.ParseFingerprint(gone.Key); err == nil && server != nil {
		server.Drop(key)
	}
	a.endControl(id)
	a.relistRemote()
	a.notifyRemote("")
	return nil
}

// keepPaired keeps the computer of key as paired, as one that controls
// this one or one this one controls, in place of what was kept of it.
func (a *App) keepPaired(key remote.Fingerprint, name string, controlsThis bool) error {
	f, err := a.remoteFile()
	if err != nil {
		return err
	}
	return f.Update(func(v *remoteFile) error {
		c := PairedComputer{ID: key.ID(), Key: key.String(), Name: name, Paired: time.Now().UTC()}
		cs := slices.Clone(v.Computers)
		if i := slices.IndexFunc(cs, func(o PairedComputer) bool { return o.Key == c.Key }); i >= 0 {
			c.ControlsThis, c.Controlled = cs[i].ControlsThis, cs[i].Controlled
			cs = slices.Delete(cs, i, i+1)
		}
		if controlsThis {
			c.ControlsThis = true
		} else {
			c.Controlled = true
		}
		v.Computers = append(cs, c)
		return nil
	})
}

// dropControlled keeps that this computer no longer controls the one of
// key, which removed it: what that one may do to this one stays.
func (a *App) dropControlled(key remote.Fingerprint) {
	f, err := a.remoteFile()
	if err != nil {
		return
	}
	_ = f.Update(func(v *remoteFile) error {
		cs := slices.Clone(v.Computers)
		i := slices.IndexFunc(cs, func(o PairedComputer) bool { return o.Key == key.String() })
		if i < 0 {
			return nil
		}
		if cs[i].ControlsThis {
			cs[i].Controlled = false
		} else {
			cs = slices.Delete(cs, i, i+1)
		}
		v.Computers = cs
		return nil
	})
}

// startRemote starts the server and announces it, unless it runs.
func (a *App) startRemote() error {
	// The key first, with r.mu free: see identity.
	id, err := a.identity(false)
	if err != nil {
		return err
	}
	r := &a.remote
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.server != nil {
		return nil
	}
	o := a.remoteOptions()
	ln, err := o.Listen()
	if err != nil {
		return fmt.Errorf("remote control can't listen on the network: %w", err)
	}
	srv := &remote.Server{Identity: id, Name: o.Name, Host: remoteHost{a}, Timing: o.Timing}
	go func() {
		if err := srv.Serve(ln); err != nil {
			log.Printf("mixmusic: remote control: %v", err)
		}
	}()
	port := 0
	if addr, ok := ln.Addr().(*net.TCPAddr); ok {
		port = addr.Port
	}
	stop, err := o.Discovery.Announce(remote.Announcement{ID: id.Fingerprint().ID(), Name: o.Name, Port: port})
	if err != nil {
		srv.Close()
		return fmt.Errorf("remote control can't announce this computer on the network: %w", err)
	}
	r.server, r.unannounce = srv, stop
	log.Printf("mixmusic: remote control: on, at port %d, as %s", port, o.Name)
	return nil
}

// stopRemote stops the server, its sessions and the announcement, and
// ends a pairing that shows its code.
func (a *App) stopRemote() {
	r := &a.remote
	r.mu.Lock()
	srv, stop := r.server, r.unannounce
	r.server, r.unannounce = nil, nil
	code := r.code
	r.mu.Unlock()
	if code != nil {
		code.cancel()
	}
	if stop != nil {
		stop()
	}
	if srv != nil {
		srv.Close()
		log.Print("mixmusic: remote control: off")
	}
}

// closeRemote stops remote control on both sides, as MixMusic quits.
func (a *App) closeRemote() {
	a.stopRemote()
	r := &a.remote
	r.mu.Lock()
	ctl, p := r.control, r.pairing
	r.control, r.pairing = nil, nil
	r.mu.Unlock()
	if ctl != nil {
		ctl.close()
	}
	if p != nil {
		p.cancel()
	}
}

// watchRemote starts the server at start, where remote control was on,
// and looks for the other computers a moment after, then every
// castBrowseEvery, as for cast devices.
func (a *App) watchRemote() {
	if a.remoteKept().On {
		if err := a.startRemote(); err != nil {
			log.Printf("mixmusic: remote control: %v", err)
		}
	}
	go func() {
		time.Sleep(castFirstBrowse)
		for {
			a.browseRemote()
			time.Sleep(castBrowseEvery)
		}
	}()
}

// OnRemote has fn told, from any goroutine, as what remote control shows
// changes, until cancel: note, when not "", is what to tell, as a pairing
// that ended.
func (a *App) OnRemote(fn func(note string)) (cancel func()) {
	f := &fn
	r := &a.remote
	r.mu.Lock()
	r.watchers = append(slices.Clone(r.watchers), f)
	r.mu.Unlock()
	return func() {
		r.mu.Lock()
		r.watchers = slices.DeleteFunc(slices.Clone(r.watchers), func(g *func(string)) bool { return g == f })
		r.mu.Unlock()
	}
}

func (a *App) notifyRemote(note string) {
	r := &a.remote
	r.mu.Lock()
	fns := r.watchers
	r.mu.Unlock()
	for _, fn := range fns {
		(*fn)(note)
	}
}
