package playertest

import (
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
)

// Stream is the player's stream doing nothing: its methods return zero
// values, and it tells the player nothing, not even that it plays. Embed
// it in a test's fake engine and override only what the test needs.
type Stream struct{}

var _ player.Stream = Stream{}

func (Stream) Load(string, audio.Cue) error { return nil }

func (Stream) Preload(string) {}

func (Stream) Crossfade(string, time.Duration, audio.Cue) error { return nil }

func (Stream) Play() error { return nil }

func (Stream) Pause() {}

func (Stream) Stop() {}

func (Stream) Unload() {}

func (Stream) Seek(time.Duration) {}

func (Stream) SetVolume(float64) {}

func (Stream) SetSpeed(float64) {}
