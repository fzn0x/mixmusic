package app

// The output device: which of this computer's audio devices MixMusic's own
// player plays on, as Settings › Playback, the player bar and the mini
// player choose it, and what the windows hear when the device chosen goes
// away and comes back.

import (
	"context"
	"errors"
	"sync"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// OutputDevice is an audio output device of this computer.
type OutputDevice struct {
	// ID is the system's ID of the device, which the settings keep.
	ID string
	// Name is what the system calls it.
	Name string
	// Default is set on the device the system plays on by default, where
	// the system tells which it is.
	Default bool
}

// outputState is the output device the engine plays on.
type outputState struct {
	mu sync.Mutex
	// chosen is the device the settings chose, as the engine opens it next;
	// fallback is set while the engine plays on the system's default device
	// as chosen isn't there, or can't play.
	chosen   settings.OutputDevice
	fallback bool
	watchers []*func(note string)
}

// errNoOutputs is why no output device can be listed: the system has no
// audio output MixMusic can play on.
var errNoOutputs = errors.New("MixMusic can't play sound on this system")

// OutputDevices lists this computer's audio output devices, sorted by
// name, the default one marked.
func (p *Player) OutputDevices(ctx context.Context) ([]OutputDevice, error) {
	if list := p.a.opts.OutputDevices; list != nil {
		return list()
	}
	return systemOutputs()
}

// systemOutputs lists the system's audio output devices.
func systemOutputs() ([]OutputDevice, error) {
	found, err := output.Devices()
	switch {
	case errors.Is(err, output.ErrNoDevice):
		return nil, nil
	case errors.Is(err, output.ErrUnsupported):
		return nil, errNoOutputs
	case err != nil:
		return nil, err
	}
	devices := make([]OutputDevice, len(found))
	for i, d := range found {
		devices[i] = OutputDevice{ID: d.ID, Name: d.Name, Default: d.Default}
	}
	return devices, nil
}

// OutputFallback reports whether MixMusic plays on the system's default
// device in place of the one chosen, which isn't there or can't play, as
// the engine found when it last played.
func (p *Player) OutputFallback() bool {
	o := &p.a.output
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.fallback
}

// OnOutputDevice has fn told, from any goroutine, a note for the user when
// the engine stops playing on the device chosen, as it went away, and when
// it plays there again. It returns how to stop.
func (a *App) OnOutputDevice(fn func(note string)) (cancel func()) {
	o := &a.output
	o.mu.Lock()
	defer o.mu.Unlock()
	p := &fn
	o.watchers = append(o.watchers, p)
	return func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		for i, w := range o.watchers {
			if w == p {
				o.watchers = append(o.watchers[:i:i], o.watchers[i+1:]...)
				return
			}
		}
	}
}

// applyOutputDevice has the engine play on the device d of the settings:
// an output that plays opens again on it, from where the track was.
func (a *App) applyOutputDevice(e *audio.Engine, d settings.OutputDevice) {
	o := &a.output
	o.mu.Lock()
	changed := o.chosen.ID != d.ID
	o.chosen = d
	if changed {
		o.fallback = false
	}
	o.mu.Unlock()
	if changed {
		e.ReopenOutput()
	}
}

// chosenOutput returns the device the engine opens.
func (a *App) chosenOutput() settings.OutputDevice {
	o := &a.output
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.chosen
}

// outputMoved hears from an output that asked for the device chosen
// whether it plays on the default device in its place, and tells the
// windows when that changes, unless the user chose another device since.
func (a *App) outputMoved(chosen settings.OutputDevice, fallback bool) {
	o := &a.output
	o.mu.Lock()
	if o.chosen.ID != chosen.ID || o.fallback == fallback {
		o.mu.Unlock()
		return
	}
	o.fallback = fallback
	fns := o.watchers
	o.mu.Unlock()
	note := outputNote(chosen.Name, fallback)
	for _, fn := range fns {
		(*fn)(note)
	}
}

// outputNote tells the user that the device of the name went away, and the
// system's default device plays, or that it plays again.
func outputNote(name string, fallback bool) string {
	if name == "" {
		name = "The output device chosen"
	}
	if fallback {
		return name + " isn't available: playing on the system's default device until it's back"
	}
	return "Playing on " + name + " again"
}
