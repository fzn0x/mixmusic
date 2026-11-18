package settings

import "slices"

// The reverb and the echo of Settings › Playback (#546), which Sound
// effects turns off with the equalizer.

// The reverb's rooms, by ID.
const (
	RoomSmall = "small-room"
	RoomHall  = "hall"
	RoomPlate = "plate"
	RoomLarge = "large-space"
)

// Rooms are the IDs of the reverb's rooms, in the order Settings lists
// them.
var Rooms = []string{RoomSmall, RoomHall, RoomPlate, RoomLarge}

// The ranges of the reverb's and the echo's settings.
const (
	// MaxMix is a mix's, in percent: from 0, the music's own sound alone,
	// to MaxMix, the effect's alone.
	MaxMix = 100
	// MinEchoTime and MaxEchoTime are the echo's time's, in milliseconds,
	// by EchoTimeStep.
	MinEchoTime  = 20
	MaxEchoTime  = 2000
	EchoTimeStep = 10
	// MaxEchoFeedback is the echo's feedback's, in percent: its repeats
	// always fade.
	MaxEchoFeedback = 90
)

// Reverb plays the music in a room.
type Reverb struct {
	// On plays the room. It is off at first.
	On bool `json:"on"`
	// Room is the ID of one of Rooms: the hall at first.
	Room string `json:"room"`
	// Mix is how much of the room plays against the music's own sound, in
	// percent: from 0, none, to MaxMix, the room alone. 25 at first.
	Mix int `json:"mix"`
}

// Echo repeats the music after a moment, each repeat softer.
type Echo struct {
	// On plays the repeats. It is off at first.
	On bool `json:"on"`
	// Preset is the ID of the one of EchoPresets that Time, Feedback and
	// Mix are, or EchoCustom for those of the user's own.
	Preset string `json:"preset"`
	// Time is how long after the music its first repeat plays, and each
	// repeat after the one before, in milliseconds: from MinEchoTime to
	// MaxEchoTime, by EchoTimeStep.
	Time int `json:"time"`
	// Feedback is how loud each repeat plays against the one before, in
	// percent: from 0, a single repeat, to MaxEchoFeedback.
	Feedback int `json:"feedback"`
	// Mix is how much of the repeats plays against the music's own sound,
	// in percent: from 0, none, to MaxMix, the repeats alone.
	Mix int `json:"mix"`
}

// EchoCustom is the Preset of an echo of the user's own time, feedback
// and mix.
const EchoCustom = "custom"

// EchoPreset is a preset of the echo.
type EchoPreset struct {
	ID                  string
	Time, Feedback, Mix int
}

// EchoPresets are the echo's presets: a short slapback, a repeat about a
// quarter note later, at 120 beats a minute, and a long tail that feeds
// back.
var EchoPresets = []EchoPreset{
	{ID: "slapback", Time: 120, Feedback: 0, Mix: 30},
	{ID: "quarter", Time: 500, Feedback: 30, Mix: 25},
	{ID: "tail", Time: 750, Feedback: 60, Mix: 30},
}

// defaultReverb is the reverb of a new install: off, the hall at 25%.
func defaultReverb() Reverb { return Reverb{Room: RoomHall, Mix: 25} }

// defaultEcho is the echo of a new install: off, the slapback.
func defaultEcho() Echo {
	var e Echo
	e.Apply(EchoPresets[0])
	return e
}

// Apply has the echo play preset p's time, feedback and mix.
func (e *Echo) Apply(p EchoPreset) {
	e.Preset, e.Time, e.Feedback, e.Mix = p.ID, p.Time, p.Feedback, p.Mix
}

// clean is r with a room of Rooms, the hall in place of another, and its
// mix in its range.
func (r Reverb) clean() Reverb {
	if !slices.Contains(Rooms, r.Room) {
		r.Room = RoomHall
	}
	r.Mix = min(max(r.Mix, 0), MaxMix)
	return r
}

// clean is e with each value in its range, its time a whole number of
// steps, and a preset whose values it has, else EchoCustom.
func (e Echo) clean() Echo {
	t := min(max(e.Time, MinEchoTime), MaxEchoTime)
	e.Time = (t + EchoTimeStep/2) / EchoTimeStep * EchoTimeStep
	e.Feedback = min(max(e.Feedback, 0), MaxEchoFeedback)
	e.Mix = min(max(e.Mix, 0), MaxMix)
	i := slices.IndexFunc(EchoPresets, func(p EchoPreset) bool { return p.ID == e.Preset })
	if i < 0 || EchoPresets[i] != (EchoPreset{e.Preset, e.Time, e.Feedback, e.Mix}) {
		e.Preset = EchoCustom
	}
	return e
}
