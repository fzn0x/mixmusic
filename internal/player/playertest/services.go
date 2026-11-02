package playertest

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// Services is the player's services doing nothing: its methods return
// zero values, and Settings settings.Default(). Embed it in a test's fake
// of the app and override only what the test needs.
type Services struct{}

var _ player.Services = Services{}

func (Services) Prepare(context.Context, media.Track) (player.Source, error) {
	return player.Source{}, nil
}

func (Services) PrepareStream(context.Context, media.Track) (player.Source, error) {
	return player.Source{}, nil
}

func (Services) StreamFailure(media.Track) string { return "" }

func (Services) RecordPlay(media.Track) {}

func (Services) Radio(context.Context, media.Track) ([]media.Track, error) { return nil, nil }

func (Services) Skip(media.Track) string { return "" }

func (Services) Settings() settings.Settings { return settings.Default() }

func (Services) SaveVolume(float64) {}

func (Services) Report(player.Now) {}

func (Services) SaveQueue(player.Saved) {}
