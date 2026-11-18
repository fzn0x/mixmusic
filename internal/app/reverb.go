package app

import (
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The reverb and the echo, as the settings have them (#546).

// rooms are the engine's rooms of the settings' IDs.
var rooms = map[string]audio.Room{
	settings.RoomSmall: audio.RoomSmall,
	settings.RoomHall:  audio.RoomHall,
	settings.RoomPlate: audio.RoomPlate,
	settings.RoomLarge: audio.RoomLarge,
}

// reverbOf is what the reverb plays of the settings q: off while it or
// Sound effects is, else its room at the wet and dry levels of its mix. A
// sound mood (#547) will give the engine the hall at its own levels in
// place of it.
func reverbOf(q settings.Equalizer) audio.Reverb {
	r := q.Reverb
	room, ok := rooms[r.Room]
	if !q.Effects || !r.On || !ok {
		return audio.Reverb{}
	}
	wet, dry := audio.MixLevels(float64(r.Mix) / settings.MaxMix)
	return audio.Reverb{On: true, Room: room, Wet: wet, Dry: dry}
}

// echoOf is what the echo plays of the settings q: off while it or Sound
// effects is, else its time and feedback, at the wet and dry levels of its
// mix.
func echoOf(q settings.Equalizer) audio.Echo {
	x := q.Echo
	if !q.Effects || !x.On {
		return audio.Echo{}
	}
	wet, dry := audio.MixLevels(float64(x.Mix) / settings.MaxMix)
	return audio.Echo{
		On: true, Delay: time.Duration(x.Time) * time.Millisecond,
		Feedback: float64(x.Feedback) / 100, Wet: wet, Dry: dry,
	}
}
