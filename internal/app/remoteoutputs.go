package app

// Where this computer plays, as the page of a computer that controls it
// lists it (remote control, FeatureOutputs): the system's default sound
// output and the others, as the settings choose them, and the cast
// devices found; and playing there, as that page asks.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// remoteSoundsEvery is how long the sound outputs listed for the
// computers that control this one hold, before they're listed again.
var remoteSoundsEvery = 10 * time.Second

// remoteSounds are this computer's sound outputs, as listed last for the
// computers that control it.
type remoteSounds struct {
	mu      sync.Mutex
	devices []OutputDevice
	at      time.Time
	listing bool
}

// remoteSounds returns this computer's sound outputs, as listed last, and
// lists them again beside the caller once that was remoteSoundsEvery ago:
// a state, which goes every 2 s, tells the new list.
func (a *App) remoteSounds() []OutputDevice {
	r := &a.remote.sounds
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.listing && time.Since(r.at) >= remoteSoundsEvery {
		r.listing = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			devices, err := (&Player{a}).OutputDevices(ctx)
			if err != nil {
				log.Printf("mixmusic: remote control: listing the sound outputs: %v", err)
			}
			r.mu.Lock()
			r.listing, r.at = false, time.Now()
			if err == nil {
				r.devices = devices
			}
			r.mu.Unlock()
		}()
	}
	return r.devices
}

// remotePlace is a place this computer plays, as a computer that controls
// it knows it, and what it is here: a sound output, the system's default
// of none, or the cast device of the ID cast.
type remotePlace struct {
	remote.Output
	sound settings.OutputDevice
	cast  string
}

// outputID is the ID a computer that controls this one knows a place by,
// of its kind and its own ID: it tells nothing of the device.
func outputID(kind, id string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + id))
	return kind + "-" + hex.EncodeToString(sum[:8])
}

// remotePlaces are where this computer can play, and the ID of where it
// plays: the system's default sound output, the others, the one the
// settings chose among them, and the cast devices found.
func (a *App) remotePlaces() ([]remotePlace, string) {
	places := []remotePlace{{Output: remote.Output{ID: remote.OutputDefault, Kind: remote.OutputDefault}}}
	at := remote.OutputDefault
	if settings.NativeAudio {
		chosen := a.settings.Get().OutputDevice
		listed := false
		for _, d := range a.remoteSounds() {
			id := outputID(remote.OutputSound, d.ID)
			places = append(places, remotePlace{Output: remote.Output{ID: id, Kind: remote.OutputSound, Name: d.Name},
				sound: settings.OutputDevice{ID: d.ID, Name: d.Name}})
			if chosen.ID != "" && d.ID == chosen.ID {
				at, listed = id, true
			}
		}
		if chosen.ID != "" && !listed {
			// Chosen, but not listed: not connected, or not listed yet.
			id := outputID(remote.OutputSound, chosen.ID)
			places = append(places, remotePlace{Output: remote.Output{ID: id, Kind: remote.OutputSound, Name: chosen.Name}, sound: chosen})
			at = id
		}
	}
	svc := &Player{a}
	for _, d := range svc.CastDevices() {
		id := outputID(remote.OutputCast, d.ID)
		places = append(places, remotePlace{Output: remote.Output{ID: id, Kind: castKindOf(d), Name: d.Name, Model: d.Model}, cast: d.ID})
	}
	if d, on := svc.CastingTo(); on {
		at = outputID(remote.OutputCast, d.ID)
	}
	return places, at
}

// castKindOf is the kind of place a cast device is, as a computer that
// controls this one names it.
func castKindOf(d CastDevice) string {
	switch {
	case d.Group:
		return remote.OutputGroup
	case d.Kind == CastRenderer:
		return remote.OutputRenderer
	case d.Kind == CastFCast:
		return remote.OutputFCast
	}
	return remote.OutputCast
}

// remoteOutputs are where this computer can play, as its state tells a
// computer that controls it, and the ID of where it plays.
func (a *App) remoteOutputs() ([]remote.Output, string) {
	places, at := a.remotePlaces()
	out := make([]remote.Output, len(places))
	for i, p := range places {
		out[i] = p.Output
	}
	return out, at
}

// playOn has this computer play on the place of the ID id, as a computer
// that controls it asks: a cast device found, or a sound output, as the
// settings choose it, which brings back what plays on a cast device.
func (a *App) playOn(id string) {
	places, _ := a.remotePlaces()
	for _, p := range places {
		if p.ID != id {
			continue
		}
		if p.cast != "" {
			go func() {
				if err := (&Player{a}).Cast(context.Background(), p.cast); err != nil {
					log.Printf("mixmusic: remote control: casting as asked: %v", err)
					a.notifyRemote(err.Error())
				}
			}()
			return
		}
		a.stopCasting()
		if settings.NativeAudio && a.settings.Get().OutputDevice.ID != p.sound.ID {
			if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.OutputDevice = p.sound }); err != nil {
				log.Printf("mixmusic: remote control: choosing the output device: %v", err)
			}
		}
		return
	}
}

// PlaceName is how the windows name a place another computer can play,
// as its state tells it: the system's default sound output, a sound output
// by its name, or a cast device by its name, with its model or its kind as
// the detail.
func PlaceName(o remote.Output) (name, detail string) {
	switch o.Kind {
	case remote.OutputDefault:
		return locales.OutputChoiceSystemDefault(), ""
	case remote.OutputSound:
		if o.Name == "" {
			return locales.OutputChoiceChosen(), ""
		}
		return o.Name, ""
	}
	d := CastDevice{Name: o.Name, Model: o.Model}
	switch o.Kind {
	case remote.OutputGroup:
		d.Group = true
	case remote.OutputRenderer:
		d.Kind = CastRenderer
	case remote.OutputFCast:
		d.Kind = CastFCast
	}
	return o.Name, ModelOf(d)
}
