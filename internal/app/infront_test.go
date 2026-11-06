package app

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
)

// notes is an app with notifications on, the main window, the mini player
// and the player around the notch made up, front the one with the
// keyboard, and the IDs of the notifications it showed.
type notes struct {
	a     *App
	front *mygo.Window
	shown chan string
	n     int
}

func newNotes(t *testing.T) *notes {
	t.Helper()
	a := newQuietApp(t, nil)
	st := a.settings.Get()
	// The title would wait for MyGo's loop to change.
	st.Notifications, st.ShowInTitle = true, false
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	n := &notes{a: a, shown: make(chan string, 16)}
	a.mu.Lock()
	a.main, a.mini = new(mygo.Window), new(mygo.Window)
	a.mu.Unlock()
	a.notch.mu.Lock()
	a.notch.win = new(mygo.Window)
	a.notch.mu.Unlock()
	wasFocused, wasShow := focused, showNote
	focused = func(w *mygo.Window) bool { return w == n.front }
	showNote = func(note *mygo.Notification) error {
		n.shown <- note.ID()
		return nil
	}
	t.Cleanup(func() { focused, showNote = wasFocused, wasShow })
	return n
}

// start has a track of its own start playing.
func (n *notes) start() {
	n.n++
	t := &media.Track{ID: fmt.Sprintf("yt:%d", n.n), Title: "Made-up song", Artists: []media.Ref{{Name: "Nobody"}}}
	(&Player{n.a}).Report(NowPlaying{Track: t, Playing: true})
}

// rate likes the track, then dislikes it, as a global shortcut does.
func (n *notes) rate() {
	t := media.Track{ID: "yt:rated", Title: "Made-up song"}
	n.a.notifyLiked(t, true)
	n.a.notifyDisliked(t, true)
}

// want waits for the notifications ids, in order, then for any more until
// none comes for a moment.
func (n *notes) want(t *testing.T, what string, ids ...string) {
	t.Helper()
	var got []string
	for range ids {
		select {
		case id := <-n.shown:
			got = append(got, id)
		case <-time.After(5 * time.Second):
		}
	}
	for more := true; more; {
		select {
		case id := <-n.shown:
			got = append(got, id)
		case <-time.After(100 * time.Millisecond):
			more = false
		}
	}
	if !slices.Equal(got, ids) {
		t.Errorf("%s: notifications %q, want %q", what, got, ids)
	}
}

// TestNoNotificationInFront tells nothing while the mini player has the
// keyboard, as while the main window has it: neither the track that
// starts nor a like or a dislike, which the window shows (#289).
func TestNoNotificationInFront(t *testing.T) {
	n := newNotes(t)
	for _, c := range []struct {
		name string
		win  *mygo.Window
	}{
		{"the main window", n.a.main},
		{"the mini player", n.a.mini},
	} {
		n.front = c.win
		n.start()
		n.want(t, c.name+" in front, a track starts")
		n.rate()
		n.want(t, c.name+" in front, liked and disliked")
	}
}

// TestNotificationBehind tells of the track that starts, and of a like or
// a dislike, while no window of MixMusic's has the keyboard, open or not.
func TestNotificationBehind(t *testing.T) {
	n := newNotes(t)
	n.start()
	n.want(t, "a track starts", "now-playing")
	n.rate()
	n.want(t, "liked and disliked", "liked", "disliked")

	n.a.mu.Lock()
	n.a.main, n.a.mini = nil, nil
	n.a.mu.Unlock()
	n.a.notch.mu.Lock()
	n.a.notch.win = nil
	n.a.notch.mu.Unlock()
	n.start()
	n.want(t, "no window, a track starts", "now-playing")
	n.rate()
	n.want(t, "no window, liked and disliked", "liked", "disliked")
}

// TestNotchInFront tells nothing of the track that starts while the
// player around the notch has the keyboard, its panel open, which shows
// what plays; but still of a like or a dislike, which it doesn't show.
func TestNotchInFront(t *testing.T) {
	n := newNotes(t)
	n.front = n.a.notch.win
	n.start()
	n.want(t, "a track starts")
	n.rate()
	n.want(t, "liked and disliked", "liked", "disliked")
}
