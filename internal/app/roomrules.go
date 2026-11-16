package app

// The host's rules for the guests of the room this computer hosts (#734),
// as Settings › Desktop › Listen with friends chooses them: what becomes of
// the songs they add, whether they control the player, and whether the
// user lets in each computer that joins. internal/room checks and bounds
// each ask; this adds the songs, has the player do the controls, and tells
// the user what waits for them, with a notification that opens the room's
// page while MixMusic is in the background.

import (
	"context"
	"errors"
	"log"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// roomGuests is what the room this computer hosts does as its guests ask.
func (a *App) roomGuests() *room.Guests {
	return &room.Guests{Rules: a.roomRules, Look: a.lookGuestSong, Add: a.addGuestSong, Control: a.guestControl, Asks: a.roomAsks}
}

// roomRules are the host's rules, as the settings choose them now.
func (a *App) roomRules() room.Rules {
	d := a.settings.Get().Clean()
	r := room.Rules{Songs: room.SongsAsk, Control: d.RoomGuestControl, Ask: d.RoomAskJoin}
	switch d.RoomSongs {
	case settings.RoomSongsAdd:
		r.Songs = room.SongsAdd
	case settings.RoomSongsOff:
		r.Songs = room.SongsOff
	}
	return r
}

// errCantPlaySong: a song a guest added that this computer can't play, or
// whose explicit content the settings hide.
var errCantPlaySong = errors.New("this computer can't play that song")

// guestSong is the song t a guest added, as this computer plays it.
func (a *App) guestSong(t remote.QueueTrack) (media.Track, error) {
	mt, ok := a.roomTrack(t)
	if !ok || a.settings.Get().HideExplicit && mt.ExplicitSong() {
		return media.Track{}, errCantPlaySong
	}
	return mt, nil
}

// lookGuestSong finds the song t a guest adds by its ID: on its service,
// or, of a file of the guest's, among this computer's own files, as a
// hand-over finds one. What the guest says of it, its title, artists,
// covers and whether it's explicit, is taken for nothing, as the host
// shows it, plays it and tells Discord and the scrobblers of it.
func (a *App) lookGuestSong(ctx context.Context, t remote.QueueTrack) (remote.QueueTrack, error) {
	mt, ok := a.roomTrack(t)
	if !ok {
		return remote.QueueTrack{}, errCantPlaySong
	}
	if mt.Source != media.SourceFile {
		var err error
		if mt, err = a.findSong(ctx, mt); err != nil {
			log.Printf("mixmusic: room: finding a guest's song: %v", err)
			return remote.QueueTrack{}, errCantPlaySong
		}
	}
	return queueTrackOf(mt), nil
}

// findSong finds t, a track of a service, by its ID, as its service tells
// it: none of Spotify's while the user isn't signed in to it, as nothing
// else tells what it is.
func (a *App) findSong(ctx context.Context, t media.Track) (media.Track, error) {
	if o := a.opts.Room; o != nil && o.Song != nil {
		return o.Song(ctx, t)
	}
	_, native, err := media.ParseID(t.ID)
	if err != nil {
		return media.Track{}, err
	}
	switch {
	case t.Source == media.SourceYouTube:
		return a.youtube.Song(ctx, native)
	case t.Source == media.SourceSpotify && a.spotify.SignedIn():
		return a.spotify.Track(ctx, native)
	}
	return media.Track{}, errCantPlaySong
}

// addGuestSong adds the song t, which the guest from added, as
// lookGuestSong found it, to the end of the queue, unless the settings
// hide it.
func (a *App) addGuestSong(from room.Member, t remote.QueueTrack) error {
	mt, err := a.guestSong(t)
	if err != nil {
		return err
	}
	p := a.goPlayer.Load()
	if p == nil {
		return errCantPlaySong
	}
	p.Enqueue([]media.Track{mt})
	log.Printf("mixmusic: room: %s added %s", from.Name, mt.Title)
	return nil
}

// guestControl has the player do what the guest from asks.
func (a *App) guestControl(from room.Member, do string) error {
	p := a.goPlayer.Load()
	if p == nil {
		return errCantPlaySong
	}
	switch do {
	case room.Play:
		p.Play()
	case room.Pause:
		p.Pause()
	case room.Next:
		p.Next()
	case room.Previous:
		p.Previous()
	}
	log.Printf("mixmusic: room: %s: %s", from.Name, do)
	return nil
}

// roomAsks tells the user of what waits for their answer in their room:
// in the windows, and in a notification, which opens the room's page, while
// MixMusic is in the background. The room waits on neither.
func (a *App) roomAsks(note string) {
	a.notifyRooms(note)
	if !a.settings.Get().Notifications || a.inFront() {
		return
	}
	n := mygo.NewNotification(mygo.NotificationOptions{ID: "room", Title: "Listen with friends", Body: note})
	n.OnClick(func() {
		a.showMain()
		a.navigateTo(RoomPage)
	})
	go func() {
		if err := showNote(n); err != nil && !errors.Is(err, mygo.ErrNotificationsDenied) {
			log.Printf("mixmusic: notification: %v", err)
		}
	}()
}

// RoomRequest is a song a guest added, which waits for the user to accept
// it: its ID, as Accept and Decline name it, who added it, and the song.
type RoomRequest struct {
	ID            int64
	From          string
	Title, Artist string
}

// Requests returns the songs that wait for the user to accept them, in the
// room this computer hosts, the oldest first.
func (rs *Rooms) Requests() []RoomRequest {
	h := rs.a.hostedRoom()
	if h == nil {
		return nil
	}
	var out []RoomRequest
	for _, r := range h.server.Requests() {
		out = append(out, RoomRequest{ID: r.ID, From: r.From.Name, Title: r.Track.Title, Artist: roomArtist(r.Track)})
	}
	return out
}

// roomArtist names the first artist of t.
func roomArtist(t remote.QueueTrack) string {
	if len(t.Artists) == 0 {
		return ""
	}
	return t.Artists[0].Name
}

// Accept adds the song of the request id to the queue.
func (rs *Rooms) Accept(id int64) error {
	if h := rs.a.hostedRoom(); h != nil {
		return h.server.Accept(id)
	}
	return nil
}

// Decline declines the song of the request id.
func (rs *Rooms) Decline(id int64) {
	if h := rs.a.hostedRoom(); h != nil {
		h.server.Decline(id)
	}
}

// RoomKnock is a computer that asks to join the room this computer hosts:
// its ID, as LetIn names it, its name in rooms, the start of its key's ID,
// which tells two of one name apart, and whether it knocks from this
// computer's own networks.
type RoomKnock struct {
	ID    int64
	Name  string
	Key   string
	Local bool
}

// Knocks returns the computers that wait for the user to let them in, the
// oldest first.
func (rs *Rooms) Knocks() []RoomKnock {
	h := rs.a.hostedRoom()
	if h == nil {
		return nil
	}
	var out []RoomKnock
	for _, k := range h.server.Knocks() {
		out = append(out, RoomKnock{ID: k.ID, Name: k.Name, Key: k.Key, Local: k.Local})
	}
	return out
}

// LetIn lets the computer of the knock id in, or turns it away.
func (rs *Rooms) LetIn(id int64, yes bool) {
	if h := rs.a.hostedRoom(); h != nil {
		h.server.LetIn(id, yes)
	}
}

// RulesChanged tells the guests of the room this computer hosts the rules,
// as the settings changed them.
func (rs *Rooms) RulesChanged() {
	if h := rs.a.hostedRoom(); h != nil {
		h.server.RulesChanged()
	}
}
