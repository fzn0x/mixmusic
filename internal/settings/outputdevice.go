package settings

import "strings"

// OutputDevice is the audio output device of this computer that MixMusic's
// own player plays on, as Settings › Playback chooses it.
type OutputDevice struct {
	// ID is the system's ID of the device: its endpoint ID on Windows, its
	// UID on macOS, and on Linux the name of its PulseAudio sink, or
	// "alsa:" and its ALSA name. Empty, the player plays on the system's
	// default device, and follows it as it changes.
	ID string `json:"id"`
	// Name is the device's name as the system told it when it was chosen,
	// which shows while the device isn't there.
	Name string `json:"name"`
}

// clean is d with its name trimmed, and none for the system's default.
func (d OutputDevice) clean() OutputDevice {
	d.ID = strings.TrimSpace(d.ID)
	if d.ID == "" {
		return OutputDevice{}
	}
	d.Name = strings.TrimSpace(d.Name)
	return d
}
