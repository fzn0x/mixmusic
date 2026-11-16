package app

import (
	"context"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestRoomSongsAsTheServiceTells adds a guest's song as the host's service
// tells it, by its ID: not under the title the guest gave it, nor clean
// when the guest says so of an explicit song the host's settings hide.
func TestRoomSongsAsTheServiceTells(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	findSongs(desk, vid("e"))
	lap := newRoomApp(t, n, "Lap")
	lapNotes := roomNotes(t, lap.App)
	setRules(t, desk.App, func(s *settings.Settings) { s.RoomSongs, s.HideExplicit = settings.RoomSongsAdd, true })
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	fake := tracksOf("s")[0]
	fake.Title, fake.Artwork = "Buy my album", []media.Image{{URL: "https://example.com/ad.png"}}
	lap.Rooms().AddSong(fake)
	// The guest's own note names it as the guest does.
	toldNote(t, lapNotes, "Desk added Buy my album to the room")
	waitPlayer(t, dp, "the song as the service tells it", func(s player.State) bool {
		return len(s.Queue) == 2 && s.Queue[1].Track.Title == "Song s" && len(s.Queue[1].Track.Artwork) == 0
	})
	clean := tracksOf("e")[0]
	clean.Explicit = false
	lap.Rooms().AddSong(clean)
	toldNote(t, lapNotes, "Desk can't play Song e")
}

// TestRoomSpotifySongUnfound refuses a guest's Spotify song while the host
// isn't signed in to Spotify, as nothing else tells what it is: the
// guest's title would play, matched on YouTube Music, and reach Discord
// and the scrobblers.
func TestRoomSpotifySongUnfound(t *testing.T) {
	a := newRoomApp(t, roomtest.New(), "Desk")
	q := remote.QueueTrack{Source: "spotify", ID: "sp:4uLU6hMCjMI75M1A2tKUQC", Title: "A song", Artists: []remote.Ref{{Name: "Band"}},
		Artwork: []remote.Image{{URL: "https://i.scdn.co/image/ab67616d"}}}
	if _, err := a.lookGuestSong(context.Background(), q); err == nil {
		t.Fatal("took a Spotify song nothing found")
	}
}
