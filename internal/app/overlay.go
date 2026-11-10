package app

// Now playing for overlays (#344): while Settings › On the desktop › Now
// playing for overlays is on, what plays is served on 127.0.0.1 for the
// browser source of a stream (internal/overlay). It hears of what plays
// where the system's media controls and Discord do: the player's report.

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/fzn0x/mixmusic/internal/overlay"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// overlayHost keeps the server of overlays, made at its first use.
type overlayHost struct {
	once   sync.Once
	server atomic.Pointer[overlay.Server]
}

// overlayServer returns the server of overlays, not yet listening.
func (a *App) overlayServer() *overlay.Server {
	a.overlay.once.Do(func() {
		s := overlay.New()
		s.Cover = a.coverBytes
		s.Listen = a.opts.OverlayListen
		a.overlay.server.Store(s)
	})
	return a.overlay.server.Load()
}

// applyOverlay has the server listen as the settings say, or stop. A port
// that another program holds shows in the settings page, from its status.
func (a *App) applyOverlay(s settings.Settings) {
	if !s.OverlayServer && a.overlay.server.Load() == nil {
		return
	}
	srv := a.overlayServer()
	if err := srv.Apply(s.OverlayServer, s.OverlayPort); err == nil && s.OverlayServer {
		// What plays now, for the clients that connect first.
		a.mu.Lock()
		np := a.nowPlaying
		a.mu.Unlock()
		a.reportOverlay(np)
	}
}

// reportOverlay tells the server of overlays what plays.
func (a *App) reportOverlay(np NowPlaying) {
	srv := a.overlay.server.Load()
	if srv == nil {
		return
	}
	info := overlay.Info{PositionMs: np.PositionMs, DurationMs: np.DurationMs, Playing: np.plays(),
		Live: np.Live, Speed: np.Speed}
	if t := np.Track; t != nil {
		info.ID, info.Title, info.Album = t.ID, oneLine(t.Title), ""
		for _, ar := range t.Artists {
			if name := oneLine(ar.Name); name != "" {
				info.Artists = append(info.Artists, name)
			}
		}
		if t.Album != nil {
			info.Album = oneLine(t.Album.Name)
		}
		info.Source = string(discordSource(*t))
		info.CoverURL = controlsArtwork(t.Artwork)
		if info.DurationMs == 0 && !np.Live {
			info.DurationMs = t.DurationMs
		}
	}
	srv.Update(info)
}

// OverlayStatus is what the server of overlays does, for the settings
// page: the address it listens on, or why it can't.
func (s *System) OverlayStatus() overlay.Status {
	if srv := s.a.overlay.server.Load(); srv != nil {
		return srv.Status()
	}
	return overlay.Status{}
}

// stopOverlay stops the server of overlays.
func (a *App) stopOverlay() {
	if srv := a.overlay.server.Load(); srv != nil {
		srv.Close()
	}
}

// OverlayListen is a func that opens the listener of the server of
// overlays, which tests give.
type OverlayListen = func(network, addr string) (net.Listener, error)
