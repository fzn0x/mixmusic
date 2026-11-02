package playertest

import (
	"context"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// Device is the player's Spotify device doing nothing: its methods return
// zero values, and State that it plays nothing. Embed it in a test's fake
// device and override only what the test needs.
type Device struct{}

var _ player.Device = Device{}

func (Device) Play(context.Context, media.Track, time.Duration) error { return nil }

func (Device) Pause(context.Context) error { return nil }

func (Device) Resume(context.Context) error { return nil }

func (Device) Seek(context.Context, time.Duration) error { return nil }

func (Device) SetVolume(context.Context, float64) error { return nil }

func (Device) State(context.Context) (*player.DeviceState, error) { return nil, nil }
