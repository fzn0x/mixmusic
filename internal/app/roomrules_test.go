package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// roomNotes collects what a's rooms tell the windows.
func roomNotes(t *testing.T, a *App) func() []string {
	var mu sync.Mutex
	var got []string
	t.Cleanup(a.OnRooms(func(note string) {
		if note != "" {
			mu.Lock()
			got = append(got, note)
			mu.Unlock()
		}
	}))
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// toldNote waits for notes to have told note.
func toldNote(t *testing.T, notes func() []string, note string) {
	t.Helper()
	until(t, "the note "+note, func() bool { return slices.Contains(notes(), note) })
}

// findSongs has the host ra find the songs its guests add on a service
// of its own, in place of YouTube Music's: the song of the letter x is
// "Song x" by Band, explicit if its ID is among explicit.
func findSongs(ra *roomApp, explicit ...string) {
	ra.opts.Room.Song = func(_ context.Context, t media.Track) (media.Track, error) {
		_, native, err := media.ParseID(t.ID)
		if err != nil || native == "" {
			return media.Track{}, errors.New("not found")
		}
		found := tracksOf(native[:1])[0]
		found.Explicit = slices.Contains(explicit, t.ID)
		return found, nil
	}
}

// setRules sets the room's rules in the settings of a.
func setRules(t *testing.T, a *App, set func(*settings.Settings)) {
	t.Helper()
	st := a.settings.Get()
	set(&st)
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
}

// TestRoomSongsAskFirst has a guest's song wait for the host's user, who
// is told, and accepts it: it joins the end of the host's queue, and the
// guest hears of each step.
func TestRoomSongsAskFirst(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	findSongs(desk)
	lap := newRoomApp(t, n, "Lap")
	deskNotes, lapNotes := roomNotes(t, desk.App), roomNotes(t, lap.App)
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a", "b"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	until(t, "the host's rules", func() bool { r, ok := lap.Rooms().RoomRules(); return ok && r.Songs == room.SongsAsk && !r.Control })
	if err := lap.Rooms().AddSong(tracksOf("s")[0]); err != nil {
		t.Fatal(err)
	}
	toldNote(t, lapNotes, "Song s waits for Desk to accept it")
	toldNote(t, deskNotes, "Lap asks to add Song s")
	reqs := desk.Rooms().Requests()
	if len(reqs) != 1 || reqs[0].From != "Lap" || reqs[0].Title != "Song s" || reqs[0].Artist != "Band" {
		t.Fatalf("requests %+v", reqs)
	}
	if err := desk.Rooms().Accept(reqs[0].ID); err != nil {
		t.Fatal(err)
	}
	toldNote(t, lapNotes, "Desk added Song s to the room")
	waitPlayer(t, dp, "the song at the end of the queue", func(s player.State) bool {
		return len(s.Queue) == 3 && s.Queue[2].Track.ID == vid("s")
	})
	if len(desk.Rooms().Requests()) != 0 {
		t.Fatal("the song accepted still waits")
	}
	// A song declined; then songs off.
	lap.Rooms().AddSong(tracksOf("t")[0])
	until(t, "the second request", func() bool { return len(desk.Rooms().Requests()) == 1 })
	desk.Rooms().Decline(desk.Rooms().Requests()[0].ID)
	toldNote(t, lapNotes, "Desk declined Song t")
	setRules(t, desk.App, func(s *settings.Settings) { s.RoomSongs = settings.RoomSongsOff })
	desk.Rooms().RulesChanged()
	until(t, "songs off", func() bool { r, _ := lap.Rooms().RoomRules(); return r.Songs == room.SongsOff })
	lap.Rooms().AddSong(tracksOf("u")[0])
	toldNote(t, lapNotes, "Desk takes no songs from guests")
	if len(dp.State().Queue) != 3 {
		t.Fatal("a song was added while songs are off")
	}
}

// TestRoomSongsAddedAtOnce adds a guest's song to the host's queue at
// once, as the settings say, unless the host's settings hide its explicit
// content.
func TestRoomSongsAddedAtOnce(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	findSongs(desk, vid("e"))
	lap := newRoomApp(t, n, "Lap")
	deskNotes, lapNotes := roomNotes(t, desk.App), roomNotes(t, lap.App)
	setRules(t, desk.App, func(s *settings.Settings) { s.RoomSongs, s.HideExplicit = settings.RoomSongsAdd, true })
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	lap.Rooms().AddSong(tracksOf("s")[0])
	toldNote(t, lapNotes, "Desk added Song s to the room")
	toldNote(t, deskNotes, "Lap added Song s")
	waitPlayer(t, dp, "the song added", func(s player.State) bool { return len(s.Queue) == 2 && s.Queue[1].Track.ID == vid("s") })
	// The service says it's explicit, whatever the guest says.
	lap.Rooms().AddSong(tracksOf("e")[0])
	toldNote(t, lapNotes, "Desk can't play Song e")
	h, _ := desk.Rooms().Hosted()
	if last := h.Activity[len(h.Activity)-1]; last.What != room.Added || last.Song != "Song s" {
		t.Fatalf("activity %+v", h.Activity)
	}
}

// TestRoomGuestControls has a guest pause the host's player and move it to
// the next track, while the settings let guests control it: refused
// before.
func TestRoomGuestControls(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	lapNotes := roomNotes(t, lap.App)
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a", "b"), 0)
	waitPlayer(t, dp, "a playing", func(s player.State) bool { return s.Playing })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	lap.Rooms().Control(room.Pause)
	toldNote(t, lapNotes, "Desk doesn't let guests control playback")
	if !dp.State().Playing {
		t.Fatal("a guest paused the host's player against its settings")
	}
	setRules(t, desk.App, func(s *settings.Settings) { s.RoomGuestControl = true })
	desk.Rooms().RulesChanged()
	until(t, "control on", func() bool { r, _ := lap.Rooms().RoomRules(); return r.Control })
	lap.Rooms().Control(room.Pause)
	waitPlayer(t, dp, "paused by the guest", func(s player.State) bool { return !s.Playing })
	lap.Rooms().Control(room.Next)
	waitPlayer(t, dp, "b by the guest", func(s player.State) bool { return s.Index == 1 })
	if err := desk.Rooms().Control(room.Play); err == nil {
		t.Fatal("the host controls a room it's in none of")
	}
}

// TestRoomAskBeforeJoin has a computer that joins wait for the host's user,
// who is told, and lets it in.
func TestRoomAskBeforeJoin(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	deskNotes := roomNotes(t, desk.App)
	setRules(t, desk.App, func(s *settings.Settings) { s.RoomAskJoin = true })
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.Rooms().Hosted()
	lap.browseRooms()
	lap.Rooms().Knock(desk.opts.Remote.Identity.Fingerprint().ID(), hosted.Code)
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	lap.Rooms().Join()
	until(t, "asking", func() bool { return lap.Rooms().Asking() && len(desk.Rooms().Knocks()) == 1 })
	toldNote(t, deskNotes, "Lap asks to join your room")
	k := desk.Rooms().Knocks()[0]
	if k.Name != "Lap" {
		t.Fatalf("knock %+v", k)
	}
	desk.Rooms().LetIn(k.ID, true)
	untilGuest(t, lap.App, "in the room", func(v GuestView) bool { return v.Step == RoomIn })
	if lap.Rooms().Asking() {
		t.Fatal("in, and still asking")
	}
}

// TestRoomTurnedAway turns a computer away that asks to join: it's told
// why.
func TestRoomTurnedAway(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	setRules(t, desk.App, func(s *settings.Settings) { s.RoomAskJoin = true })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.Rooms().Hosted()
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	lap.Rooms().Join()
	until(t, "the knock", func() bool { return len(desk.Rooms().Knocks()) == 1 })
	desk.Rooms().LetIn(desk.Rooms().Knocks()[0].ID, false)
	v := untilGuest(t, lap.App, "turned away", func(v GuestView) bool { return v.Step == RoomEnded })
	if v.Problem != "Desk didn't let you in." {
		t.Fatalf("problem %q", v.Problem)
	}
}
