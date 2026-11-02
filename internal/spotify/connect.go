package spotify

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Device is a Spotify Connect device: a Spotify app, a speaker, a TV.
type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Active     bool   `json:"active"`
	Restricted bool   `json:"restricted"`
	Volume     int    `json:"volume"`
}

// PlaybackState is what a Spotify Connect device is playing.
type PlaybackState struct {
	Device     Device       `json:"device"`
	Playing    bool         `json:"playing"`
	ProgressMs int          `json:"progressMs"`
	Track      *media.Track `json:"track"`
}

type device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	IsActive      bool   `json:"is_active"`
	IsRestricted  bool   `json:"is_restricted"`
	VolumePercent int    `json:"volume_percent"`
}

func (d device) convert() Device {
	return Device{ID: d.ID, Name: d.Name, Type: d.Type, Active: d.IsActive, Restricted: d.IsRestricted, Volume: d.VolumePercent}
}

// Devices returns the user's Spotify Connect devices.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	var res struct {
		Devices []device `json:"devices"`
	}
	if err := c.get(ctx, "/me/player/devices", nil, &res); err != nil {
		return nil, err
	}
	out := []Device{}
	for _, d := range res.Devices {
		out = append(out, d.convert())
	}
	return out, nil
}

// deviceQuery targets a device, or the active one when id is empty.
func deviceQuery(id string) url.Values {
	if id == "" {
		return nil
	}
	return url.Values{"device_id": {id}}
}

// Play plays tracks, by their Spotify URIs, on a device, from positionMs
// into the first. Playback control needs Spotify Premium.
func (c *Client) Play(ctx context.Context, deviceID string, uris []string, positionMs int) error {
	body := map[string]any{"uris": uris, "position_ms": positionMs}
	return c.do(ctx, http.MethodPut, "/me/player/play", deviceQuery(deviceID), body, nil)
}

// Resume resumes playback on a device.
func (c *Client) Resume(ctx context.Context, deviceID string) error {
	return c.do(ctx, http.MethodPut, "/me/player/play", deviceQuery(deviceID), nil, nil)
}

// Pause pauses playback on a device.
func (c *Client) Pause(ctx context.Context, deviceID string) error {
	return c.do(ctx, http.MethodPut, "/me/player/pause", deviceQuery(deviceID), nil, nil)
}

// Seek moves playback on a device to positionMs.
func (c *Client) Seek(ctx context.Context, deviceID string, positionMs int) error {
	q := url.Values{"position_ms": {strconv.Itoa(positionMs)}}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	return c.do(ctx, http.MethodPut, "/me/player/seek", q, nil, nil)
}

// SetVolume sets the volume of a device, from 0 to 100.
func (c *Client) SetVolume(ctx context.Context, deviceID string, percent int) error {
	q := url.Values{"volume_percent": {strconv.Itoa(min(max(percent, 0), 100))}}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	return c.do(ctx, http.MethodPut, "/me/player/volume", q, nil, nil)
}

// Shuffle has a device shuffle what it plays, or play it in its order.
func (c *Client) Shuffle(ctx context.Context, deviceID string, on bool) error {
	q := url.Values{"state": {strconv.FormatBool(on)}}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	return c.do(ctx, http.MethodPut, "/me/player/shuffle", q, nil, nil)
}

// Repeat has a device repeat the track it plays ("track"), all it plays
// ("context"), or nothing ("off").
func (c *Client) Repeat(ctx context.Context, deviceID, state string) error {
	q := url.Values{"state": {state}}
	if deviceID != "" {
		q.Set("device_id", deviceID)
	}
	return c.do(ctx, http.MethodPut, "/me/player/repeat", q, nil, nil)
}

// Transfer moves playback to a device.
func (c *Client) Transfer(ctx context.Context, deviceID string, play bool) error {
	return c.do(ctx, http.MethodPut, "/me/player", nil, map[string]any{"device_ids": []string{deviceID}, "play": play}, nil)
}

// State returns what the user's active device plays, a track or a
// podcast's episode, or nil when no device is active.
func (c *Client) State(ctx context.Context) (*PlaybackState, error) {
	var res struct {
		Device     *device      `json:"device"`
		IsPlaying  bool         `json:"is_playing"`
		ProgressMs int          `json:"progress_ms"`
		Item       *playingItem `json:"item"`
	}
	// Without additional_types, Spotify tells no item while an episode
	// plays.
	if err := c.get(ctx, "/me/player", url.Values{"additional_types": {"track,episode"}}, &res); err != nil {
		return nil, err
	}
	if res.Device == nil {
		return nil, nil // 204: nothing is playing anywhere
	}
	s := &PlaybackState{Device: res.Device.convert(), Playing: res.IsPlaying, ProgressMs: res.ProgressMs}
	if res.Item != nil {
		s.Track = res.Item.t
	}
	return s, nil
}
