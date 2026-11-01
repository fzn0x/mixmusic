package app

import (
	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// Preferences reads and changes the user's settings.
type Preferences struct{ a *App }

// Get returns the settings.
func (p *Preferences) Get() settings.Settings { return p.a.settings.Get().Clean() }

// Set replaces the settings, puts their values in range, and tells every
// window. It returns the settings as kept.
func (p *Preferences) Set(s settings.Settings) (settings.Settings, error) {
	s = s.Clean()
	prev := p.a.settings.Get()
	if err := p.a.settings.Set(s); err != nil {
		return prev, err
	}
	if prev.SpotifyClientID != s.SpotifyClientID && p.a.spotify.SignedIn() {
		// The token belongs to the former app.
		p.a.signOutSpotify()
	}
	p.a.apply(s)
	notify()
	return s, nil
}

// Reset puts back the settings of a new install, keeping the accounts, the
// tools set up, and the folder the downloads are in.
func (p *Preferences) Reset() (settings.Settings, error) {
	cur := p.a.settings.Get()
	d := settings.Default()
	d.SpotifyClientID = cur.SpotifyClientID
	d.YouTubeChannel = cur.YouTubeChannel
	d.YTDLPPath, d.JSRuntime = cur.YTDLPPath, cur.JSRuntime
	d.DownloadsFolder = cur.DownloadsFolder
	return p.Set(d)
}

// OpenAtLogin reports whether MixMusic starts when the user logs in.
func (p *Preferences) OpenAtLogin() bool { return mygo.App.OpenAtLogin() }

// SetOpenAtLogin makes MixMusic start when the user logs in, or not.
func (p *Preferences) SetOpenAtLogin(open bool) error { return mygo.App.SetOpenAtLogin(open) }
