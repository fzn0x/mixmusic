package app

// DLNA and UPnP renderers (internal/upnp) and FCast receivers
// (internal/fcast): the cast menu lists them beside Google Cast devices,
// and the player plays on them as it plays on those, through the same
// cast output (castOut), whose session theirs is, and the same server
// (castServer).

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"

	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/fcast"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/upnp"
	"github.com/fzn0x/mixmusic/locales"
)

// CastKind is the kind of a cast device: Google Cast, of "", a DLNA or
// UPnP renderer, or an FCast receiver.
type CastKind string

const (
	CastGoogle   CastKind = ""
	CastRenderer CastKind = "dlna"
	CastFCast    CastKind = "fcast"
)

// ModelOf is what the windows show a device as beside its name: its model,
// or, for a device that names none, what kind it is.
func ModelOf(d CastDevice) string {
	switch {
	case d.Model != "":
		return d.Model
	case d.Kind == CastRenderer:
		return locales.CastRendererModel()
	case d.Kind == CastFCast:
		return locales.CastReceiverModel()
	}
	return ""
}

// englishModel is ModelOf for the log, which stays English.
func englishModel(d CastDevice) string {
	switch {
	case d.Model != "":
		return d.Model
	case d.Kind == CastRenderer:
		return "DLNA renderer"
	case d.Kind == CastFCast:
		return "FCast receiver"
	}
	return "cast device"
}

// castLink is a session on a cast device of any kind, as castTo starts it.
type castLink interface {
	castSession
	Close(stop bool)
	LocalAddr() net.Addr
	RemoteIP() net.IP
}

// castHooks are what a session tells the output of what its device does.
type castHooks struct {
	media  func(cast.MediaStatus)
	volume func(cast.Volume)
	end    func(error)
}

// farDevice is a cast device found otherwise than as Google Cast's.
type farDevice interface {
	castDevice() CastDevice
	open(ctx context.Context, a *App, h castHooks) (castLink, error)
	// dlna tells whether its server serves as to a DLNA renderer.
	dlna() bool
	// attach has out play on link as the device plays: in the formats it
	// plays, served by server, of addresses it tells MixMusic's by.
	attach(a *App, link castLink, server *castServer, out *castOut, name string)
}

// openCast starts a session on the device f.
func (a *App) openCast(ctx context.Context, f castFound, h castHooks) (castLink, error) {
	if f.far != nil {
		return f.far.open(ctx, a, h)
	}
	s, err := cast.Open(ctx, f.device, cast.SessionOptions{OnMedia: h.media, OnVolume: h.volume, OnEnd: h.end})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// castWhy is why starting to cast failed, as the windows tell it: the
// words of an FCast receiver's key that isn't the one it announced, and
// of others, as they are.
func castWhy(err error) string {
	switch {
	case errors.Is(err, fcast.ErrKey):
		return locales.CastReceiverKey()
	case errors.Is(err, fcast.ErrNoKey):
		return locales.CastReceiverNoKey()
	case errors.Is(err, fcast.ErrDowngrade):
		return locales.CastReceiverOlder()
	}
	return err.Error()
}

// rendererDevice is a DLNA or UPnP media renderer.
type rendererDevice struct{ r upnp.Renderer }

func (d rendererDevice) castDevice() CastDevice {
	return CastDevice{ID: "upnp:" + d.r.ID, Name: d.r.Name, Model: d.r.Model, Kind: CastRenderer}
}

func (d rendererDevice) open(ctx context.Context, _ *App, h castHooks) (castLink, error) {
	s, err := upnp.Open(ctx, d.r, upnp.SessionOptions{OnMedia: h.media, OnVolume: h.volume, OnEnd: h.end})
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (rendererDevice) dlna() bool { return true }

func (rendererDevice) attach(a *App, link castLink, server *castServer, out *castOut, name string) {
	s := link.(*upnp.Session)
	s.SetOurs(server.ours)
	out.source = a.rendererSource(s.Sink(), server, name)
}

// receiverDevice is an FCast receiver.
type receiverDevice struct{ r fcast.Receiver }

func (d receiverDevice) castDevice() CastDevice {
	return CastDevice{ID: d.r.ID, Name: d.r.Name, Kind: CastFCast}
}

func (d receiverDevice) open(ctx context.Context, a *App, h castHooks) (castLink, error) {
	s, err := fcast.Open(ctx, d.r, fcast.SessionOptions{OnMedia: h.media, OnVolume: h.volume, OnEnd: h.end,
		App: "MixMusic", Version: a.opts.Version})
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (receiverDevice) dlna() bool { return false }

func (receiverDevice) attach(a *App, link castLink, server *castServer, out *castOut, name string) {
	s := link.(*fcast.Session)
	s.SetOurs(server.ours)
	out.source = a.receiverSource(s, server)
}

// browseFar finds the renderers and the FCast receivers on the network, at
// once, as MixMusic looks for Google Cast devices: those tests give, when
// they give any finder.
func (a *App) browseFar(ctx context.Context) []farDevice {
	search, browse := a.opts.RendererSearch, a.opts.FCastBrowse
	if a.opts.CastBrowse == nil && search == nil && browse == nil {
		search = func(ctx context.Context) ([]upnp.Renderer, error) { return upnp.Search(ctx, upnp.SearchOptions{}) }
		browse = func(ctx context.Context) ([]fcast.Receiver, error) { return fcast.Browse(ctx, fcast.BrowseOptions{}) }
	}
	var mu sync.Mutex
	var out []farDevice
	var wg sync.WaitGroup
	if search != nil {
		wg.Go(func() {
			rs, err := search(ctx)
			if err != nil {
				log.Printf("mixmusic: looking for DLNA renderers: %v", err)
			}
			mu.Lock()
			for _, r := range rs {
				out = append(out, rendererDevice{r})
			}
			mu.Unlock()
		})
	}
	if browse != nil {
		wg.Go(func() {
			rs, err := browse(ctx)
			if err != nil {
				log.Printf("mixmusic: looking for FCast receivers: %v", err)
			}
			mu.Lock()
			for _, r := range rs {
				out = append(out, receiverDevice{r})
			}
			mu.Unlock()
		})
	}
	wg.Wait()
	return out
}

// CastOwnSpeed returns the device the player plays on, when it plays only
// at its own speed: a renderer that turned another down.
func (p *Player) CastOwnSpeed() (CastDevice, bool) {
	c := &p.a.cast
	c.mu.Lock()
	cur := c.active
	c.mu.Unlock()
	if cur == nil {
		return CastDevice{}, false
	}
	if s, ok := cur.session.(interface{ SpeedRefused() bool }); ok && s.SpeedRefused() {
		return cur.device, true
	}
	return CastDevice{}, false
}

// upNextOf returns what the player plays next, for the device to show as
// it's handed the next track ahead.
func upNextOf(a *App) func() *media.Track {
	return func() *media.Track {
		gp := a.goPlayer.Load()
		if gp == nil {
			return nil
		}
		if next, ok := gp.State().UpNext(); ok {
			t := next.Track
			return &t
		}
		return nil
	}
}
