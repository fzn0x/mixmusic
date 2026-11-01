package app

import (
	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
)

// Preferences reads and changes the user's settings.
type Preferences struct{ a *App }

// Get returns the settings.
func (p *Preferences) Get() settings.Settings { return p.a.settings.Get().Clean() }

// Set replaces the settings, puts their values in range, and tells every
// window. It returns the settings as kept.
func (p *Preferences) Set(s settings.Settings) (settings.Settings, error) {
	return p.Update(func(cur *settings.Settings) { *cur = s })
}

// Update changes the settings with fn, puts their values in range, and
// tells every window. fn changes the settings as they are now, which an
// edit of settings.json may have changed since a window read them: unlike
// Set, Update keeps what fn doesn't change. It returns the settings as
// kept.
func (p *Preferences) Update(fn func(s *settings.Settings)) (settings.Settings, error) {
	a := p.a
	prev := a.settings.Get().Clean()
	if err := a.settings.Update(func(cur *settings.Settings) error {
		fn(cur)
		*cur = cur.Clean()
		return nil
	}); err != nil {
		return prev, err
	}
	// The file has a mistake: the change applies, and waits to be saved.
	if held := a.settings.Held(); held != nil {
		a.settingsBroken(held)
	}
	s := a.settings.Get().Clean()
	a.settingsChanged(prev, s, false)
	a.rewatch()
	return s, nil
}

// Change keeps what a window changed of the settings it read, base, which
// it holds as next: only what differs, over the settings as they are now.
// A window whose settings are older than an edit of settings.json that
// applied meanwhile then never saves the edit away.
func (p *Preferences) Change(base, next settings.Settings) (settings.Settings, error) {
	var err error
	s, uerr := p.Update(func(cur *settings.Settings) {
		var merged settings.Settings
		if merged, err = store.Merge(base, *cur, next); err == nil {
			*cur = merged
		}
	})
	if err != nil {
		return s, err
	}
	return s, uerr
}

// Reset puts back the settings of a new install, keeping the accounts and
// the tools set up. A settings.json with a mistake is kept as
// settings.json.broken.
func (p *Preferences) Reset() (settings.Settings, error) {
	a := p.a
	cur := a.settings.Get()
	prev := cur.Clean()
	d := settings.Default()
	d.SpotifyClientID = cur.SpotifyClientID
	d.YouTubeChannel = cur.YouTubeChannel
	d.YTDLPPath, d.JSRuntime = cur.YTDLPPath, cur.JSRuntime
	d = d.Clean()
	if err := a.settings.Replace(d); err != nil {
		return prev, err
	}
	a.settingsRead()
	a.settingsChanged(prev, d, false)
	a.rewatch()
	return d, nil
}

// OpenAtLogin reports whether MixMusic starts when the user logs in.
func (p *Preferences) OpenAtLogin() bool { return mygo.App.OpenAtLogin() }

// SetOpenAtLogin makes MixMusic start when the user logs in, or not.
func (p *Preferences) SetOpenAtLogin(open bool) error { return mygo.App.SetOpenAtLogin(open) }
