package app

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
)

// fastRemote is the protocol's timing in these tests. Silence stays the
// app's 6 s: the computer paired with keeps the other in remote.json,
// put on the disk, before it tells the pairing went, which a busy runner
// can make take seconds; no test waits for a silence to run out.
var fastRemote = remote.Timing{Ping: 100 * time.Millisecond, Silence: 6 * time.Second, Code: 10 * time.Second,
	StateEvery: 10 * time.Millisecond, Rest: 100 * time.Millisecond}

// newRemoteApp is a quiet app named name on the network n, which listens on
// 127.0.0.1 alone, with a key of its own.
func newRemoteApp(t *testing.T, n *remotetest.Network, name string) *App {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	a := newQuietApp(t, func(o *Options) {
		o.Remote = &RemoteOptions{Discovery: n, Listen: remotetest.Listen, Identity: id, Name: name, Timing: fastRemote}
	})
	t.Cleanup(a.closeRemote)
	return a
}

// remoteNotes keeps what remote control tells the windows of an app.
type remoteNotes struct {
	mu   sync.Mutex
	told []string
}

func notesOf(t *testing.T, a *App) *remoteNotes {
	n := &remoteNotes{}
	t.Cleanup(a.OnRemote(func(note string) {
		if note != "" {
			n.mu.Lock()
			n.told = append(n.told, note)
			n.mu.Unlock()
		}
	}))
	return n
}

func (n *remoteNotes) has(sub string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.ContainsFunc(n.told, func(s string) bool { return strings.Contains(s, sub) })
}

// untilView waits for the page of the computer id to show what ok wants,
// failing with what it showed.
func untilView(t *testing.T, a *App, id, what string, ok func(RemoteView) bool) RemoteView {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		v := a.Remote().View(id)
		if ok(v) {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s; the page shows %+v", what, v)
		}
	}
}

func idOf(a *App) string { return a.opts.Remote.Identity.Fingerprint().ID() }

// pairApps pairs lap with desk, which has remote control on, by the code
// desk shows, and waits for lap's page of desk to show what plays there.
func pairApps(t *testing.T, lap, desk *App) {
	t.Helper()
	deskID := idOf(desk)
	lap.browseRemote()
	lap.Remote().Pair(deskID)
	untilView(t, lap, deskID, "the code asked", func(v RemoteView) bool { return v.Step == RemoteCodeAsked })
	code, ok := desk.Remote().Code()
	if !ok || code.Name != "Laptop" || len(code.Code) != 6 {
		t.Fatalf("the code shown: %+v, %v", code, ok)
	}
	lap.Remote().EnterCode(deskID, code.Code[:3]+" "+code.Code[3:])
	untilView(t, lap, deskID, "the session", func(v RemoteView) bool { return v.Step == RemoteConnected && v.HasState })
}

// TestRemoteControlBetweenApps turns remote control on on one app, which
// another finds, pairs with and controls; then the first removes it.
func TestRemoteControlBetweenApps(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	deskNotes, lapNotes := notesOf(t, desk), notesOf(t, lap)
	if desk.Remote().On() {
		t.Fatal("remote control is on at first")
	}
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	// The laptop, on too, doesn't find itself.
	if err := lap.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(desk.opts.DataDir, "remote.json"))
	if err != nil {
		t.Fatal(err)
	}
	var kept remoteFile
	if err := json.Unmarshal(data, &kept); err != nil || !kept.On {
		t.Errorf("remote.json holds %s", data)
	}
	lap.browseRemote()
	deskID, lapID := idOf(desk), idOf(lap)
	if got, want := lap.Remote().Computers(), []RemoteComputer{{ID: deskID, Name: "Desktop"}}; !slices.Equal(got, want) {
		t.Fatalf("found %+v, want %+v", got, want)
	}
	if v := lap.Remote().View(deskID); v.Step != RemoteUnpaired {
		t.Errorf("the page of a computer not paired shows %+v", v)
	}

	p := desk.GoPlayer()
	p.PlayTracks([]media.Track{
		{ID: "yt:a", Source: media.SourceYouTube, Title: "Teardrop", Artists: []media.Ref{{Name: "Massive Attack"}}, DurationMs: 330000,
			Artwork: []media.Image{{URL: "https://example.com/a.jpg", Width: 544}}},
		{ID: "yt:b", Source: media.SourceYouTube, Title: "Angel", DurationMs: 380000},
	}, 0)
	waitPlayer(t, p, "a playing", func(s player.State) bool { return s.Playing })

	pairApps(t, lap, desk)
	if !lapNotes.has("Paired with Desktop") || !deskNotes.has("Paired with Laptop") {
		t.Errorf("told %q and %q", lapNotes.told, deskNotes.told)
	}
	if got := desk.Remote().Paired(); len(got) != 1 || got[0].ID != lapID || got[0].Name != "Laptop" || !got[0].ControlsThis || got[0].Controlled {
		t.Errorf("the desktop keeps %+v", got)
	}
	if got := lap.Remote().Paired(); len(got) != 1 || got[0].ID != deskID || !got[0].Controlled || got[0].ControlsThis {
		t.Errorf("the laptop keeps %+v", got)
	}
	if got := lap.Remote().Computers(); len(got) != 1 || !got[0].Paired {
		t.Errorf("found %+v", got)
	}
	v := untilView(t, lap, deskID, "what plays there", func(v RemoteView) bool {
		return v.State.Track != nil && v.State.Track.Title == "Teardrop" && v.State.Playing
	})
	if tr := v.State.Track; !slices.Equal(tr.Artists, []string{"Massive Attack"}) || tr.Artwork != "https://example.com/a.jpg" ||
		v.State.DurationMs != 330000 || len(v.State.Next) != 1 || v.State.Next[0].Title != "Angel" {
		t.Errorf("the state %+v", v.State)
	}

	do := func(c remote.Command) {
		t.Helper()
		if err := lap.Remote().Do(deskID, c); err != nil {
			t.Fatal(err)
		}
	}
	do(remote.Command{Op: remote.OpPause})
	waitPlayer(t, p, "paused", func(s player.State) bool { return !s.Playing })
	untilView(t, lap, deskID, "paused there", func(v RemoteView) bool { return !v.State.Playing })
	do(remote.Command{Op: remote.OpVolume, Volume: 0.3})
	waitPlayer(t, p, "the volume", func(s player.State) bool { return math.Abs(s.Volume-0.3) < 1e-9 })
	do(remote.Command{Op: remote.OpSeek, PositionMs: 61000})
	waitPlayer(t, p, "the seek", func(s player.State) bool { return s.Position >= 61*time.Second })
	do(remote.Command{Op: remote.OpPlay})
	waitPlayer(t, p, "playing", func(s player.State) bool { return s.Playing })
	do(remote.Command{Op: remote.OpNext})
	waitPlayer(t, p, "the next track", func(s player.State) bool { return s.Index == 1 })
	untilView(t, lap, deskID, "the next track there", func(v RemoteView) bool {
		return v.State.Track != nil && v.State.Track.Title == "Angel"
	})
	do(remote.Command{Op: remote.OpPrevious})
	waitPlayer(t, p, "the track before", func(s player.State) bool { return s.Index == 0 })

	// The desktop removes the laptop: its session ends at once, and its page
	// says so, and offers to pair again.
	if err := desk.Remote().Forget(lapID); err != nil {
		t.Fatal(err)
	}
	v = untilView(t, lap, deskID, "removed", func(v RemoteView) bool { return v.Step == RemoteUnpaired })
	if !strings.Contains(v.Problem, "Desktop removed this computer") {
		t.Errorf("the page says %q", v.Problem)
	}
	if got := lap.Remote().Paired(); len(got) != 0 {
		t.Errorf("the laptop still keeps %+v", got)
	}
	if err := lap.Remote().Do(deskID, remote.Command{Op: remote.OpPause}); err == nil {
		t.Error("a command went to a computer that removed this one")
	}

	// Remote control off: the desktop is no longer found.
	if err := desk.Remote().SetOn(false); err != nil {
		t.Fatal(err)
	}
	if n.Announced() != 1 {
		t.Errorf("%d computers announced", n.Announced())
	}
}

// TestRemoteWrongCode fails a pairing with a code other than the one shown,
// on both computers, which say so.
func TestRemoteWrongCode(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	deskNotes := notesOf(t, desk)
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	deskID := idOf(desk)
	lap.browseRemote()
	lap.Remote().Pair(deskID)
	untilView(t, lap, deskID, "the code asked", func(v RemoteView) bool { return v.Step == RemoteCodeAsked })
	code, _ := desk.Remote().Code()
	wrong := "000000"
	if code.Code == wrong {
		wrong = "000001"
	}
	lap.Remote().EnterCode(deskID, wrong)
	v := untilView(t, lap, deskID, "the pairing failed", func(v RemoteView) bool { return v.Step == RemoteUnpaired && v.Problem != "" })
	if !strings.Contains(v.Problem, "That isn't the code Desktop showed") {
		t.Errorf("the page says %q", v.Problem)
	}
	until(t, "the desktop's note", func() bool { return deskNotes.has("the code typed there wasn't the one shown") })
	if _, ok := desk.Remote().Code(); ok {
		t.Error("the code still shows")
	}
	if got := desk.Remote().Paired(); len(got) != 0 {
		t.Errorf("kept %+v", got)
	}
	// Pair again pairs, with a new code.
	time.Sleep(fastRemote.Rest)
	pairApps(t, lap, desk)
}

// TestRemoteCodeCanceled ends a pairing whose code the computer
// controlled's user refused.
func TestRemoteCodeCanceled(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	deskID := idOf(desk)
	lap.browseRemote()
	lap.Remote().Pair(deskID)
	untilView(t, lap, deskID, "the code asked", func(v RemoteView) bool { return v.Step == RemoteCodeAsked })
	desk.Remote().CancelCode()
	v := untilView(t, lap, deskID, "the pairing canceled", func(v RemoteView) bool { return v.Step == RemoteUnpaired })
	if !strings.Contains(v.Problem, "canceled on Desktop") {
		t.Errorf("the page says %q", v.Problem)
	}
	if _, ok := desk.Remote().Code(); ok {
		t.Error("the code still shows")
	}
	lap.Remote().CancelPairing(deskID)
	if v := lap.Remote().View(deskID); v.Problem != "" {
		t.Errorf("the page still says %q", v.Problem)
	}
}

// TestRemoteSessionGoesIdle ends the session once its page has been gone a
// while, and the server's with it; and connects again once the page shows.
func TestRemoteSessionGoesIdle(t *testing.T) {
	idle := remoteIdle
	remoteIdle = 300 * time.Millisecond
	t.Cleanup(func() { remoteIdle = idle })
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	pairApps(t, lap, desk)
	server := func() *remote.Server {
		desk.remote.mu.Lock()
		defer desk.remote.mu.Unlock()
		return desk.remote.server
	}()
	until(t, "the session's end", func() bool { return server.Sessions() == 0 })
	lap.remote.mu.Lock()
	ctl := lap.remote.control
	lap.remote.mu.Unlock()
	if ctl != nil {
		t.Error("the laptop keeps the session of a page gone")
	}
	untilView(t, lap, idOf(desk), "the session again", func(v RemoteView) bool { return v.Step == RemoteConnected })
}

// TestRemoteHostStartsAgain tells that remote control turned off on the
// computer controlled, and connects again once it is back on, at another
// port, which a new look finds.
func TestRemoteHostStartsAgain(t *testing.T) {
	retry, most, again := remoteRetry, remoteRetryMost, castBrowseAgain
	remoteRetry, remoteRetryMost, castBrowseAgain = 20*time.Millisecond, 100*time.Millisecond, 0
	t.Cleanup(func() { remoteRetry, remoteRetryMost, castBrowseAgain = retry, most, again })
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	pairApps(t, lap, desk)
	deskID := idOf(desk)
	if err := desk.Remote().SetOn(false); err != nil {
		t.Fatal(err)
	}
	v := untilView(t, lap, deskID, "remote control off there", func(v RemoteView) bool { return v.Step != RemoteConnected })
	if v.Problem != "" && !strings.Contains(v.Problem, "Remote control is off on Desktop") && !strings.Contains(v.Problem, "Couldn't reach Desktop") {
		t.Errorf("the page says %q", v.Problem)
	}
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	untilView(t, lap, deskID, "the session again", func(v RemoteView) bool { return v.Step == RemoteConnected && v.HasState })
}

// TestRemoteKeyKept makes the computer's key once, and reads it back at
// the next start.
func TestRemoteKeyKept(t *testing.T) {
	keep := newKeeper()
	key := func() remote.Fingerprint {
		a := newQuietApp(t, nil)
		a.remote.keys = &secretStore{key: remoteSecret, keep: keep}
		id, err := a.identity(false)
		if err != nil {
			t.Fatal(err)
		}
		return id.Fingerprint()
	}
	first := key()
	if again := key(); again != first {
		t.Error("the next start made another key")
	}
	if _, ok := keep.kept[remoteSecret]; !ok {
		t.Errorf("the credential store keeps %v", keep.kept)
	}
}

// frameKeeper is a credential store that runs a frame as it reads or
// keeps a secret, as mygo runs the window's events on the main thread
// while the store answers.
type frameKeeper struct {
	*fakeKeeper
	frame func()
}

func (k frameKeeper) Get(key string) ([]byte, error) {
	k.frame()
	return k.fakeKeeper.Get(key)
}

func (k frameKeeper) Set(key string, v []byte) error {
	k.frame()
	return k.fakeKeeper.Set(key, v)
}

// TestRemoteKeyWhileFramesRun turns remote control on, as at a start with
// it on, and pairs, each the first use of the key, while frames run as
// the key is read and kept: they show the code and the page, and a click
// asks for the key again, which is told to try again (#464).
func TestRemoteKeyWhileFramesRun(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	deskID := idOf(desk)
	// frozen is set once a call froze: it holds r.mu for good, which
	// closing would wait for.
	frozen := false
	// app is an app whose key the credential store keeps, which runs
	// frames as it answers, each telling what the click got.
	app := func(name string) (*App, *[]error) {
		a := newQuietApp(t, func(o *Options) {
			o.Remote = &RemoteOptions{Discovery: n, Listen: remotetest.Listen, Name: name, Timing: fastRemote}
		})
		t.Cleanup(func() {
			if !frozen {
				a.closeRemote()
			}
		})
		var clicks []error
		frame := func() {
			a.Remote().Code()
			a.Remote().View(deskID)
			_, err := a.identity(false)
			clicks = append(clicks, err)
		}
		a.remote.keys = &secretStore{key: remoteSecret, keep: frameKeeper{newKeeper(), frame}}
		return a, &clicks
	}
	// unfrozen runs fn, failing if it doesn't return.
	unfrozen := func(what string, fn func()) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			defer close(done)
			fn()
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			frozen = true
			t.Fatalf("%s froze", what)
		}
	}
	busy := func(clicks []error) {
		t.Helper()
		if len(clicks) == 0 {
			t.Fatal("no frame ran as the key was read")
		}
		for _, err := range clicks {
			if !errors.Is(err, errKeyBusy) {
				t.Errorf("a click as the key was read got %v", err)
			}
		}
	}

	tab, clicks := app("Tablet")
	var err error
	unfrozen("turning remote control on", func() { err = tab.Remote().SetOn(true) })
	if err != nil {
		t.Fatal(err)
	}
	busy(*clicks)
	tab.remote.mu.Lock()
	on := tab.remote.server != nil
	tab.remote.mu.Unlock()
	if !on {
		t.Error("remote control isn't on")
	}

	lap, clicks := app("Laptop")
	lap.browseRemote()
	unfrozen("pairing", func() { lap.Remote().Pair(deskID) })
	busy(*clicks)
	untilView(t, lap, deskID, "the code asked", func(v RemoteView) bool { return v.Step == RemoteCodeAsked })
	code, ok := desk.Remote().Code()
	if !ok || code.Name != "Laptop" {
		t.Fatalf("the code shown: %+v, %v", code, ok)
	}
	lap.Remote().EnterCode(deskID, code.Code)
	untilView(t, lap, deskID, "the session", func(v RemoteView) bool { return v.Step == RemoteConnected })
}

// TestRemoteStateOf tells the track that plays, with its cover only from
// https, its length when the engine doesn't know it yet, and the tracks it
// moves on to, but those the queue passes over.
func TestRemoteStateOf(t *testing.T) {
	tr := func(id, title string) media.Track { return media.Track{ID: id, Title: title, DurationMs: 1000} }
	cur := tr("a", "A")
	cur.Artwork = []media.Image{{URL: "local:cover"}}
	cur.Album = &media.Ref{Name: "Album"}
	st := player.State{Queue: []player.Item{{Track: tr("z", "Z")}, {Track: cur}, {Track: tr("b", "B"), Skip: player.SkipDisliked}, {Track: tr("c", "C")}},
		Index: 1, Playing: true, Position: 500 * time.Millisecond, Volume: 0.4, Muted: true}
	got := remoteStateOf(st)
	if got.Track == nil || got.Track.Title != "A" || got.Track.Album != "Album" || got.Track.Artwork != "" {
		t.Errorf("track %+v", got.Track)
	}
	if !got.Playing || got.PositionMs != 500 || got.DurationMs != 1000 || got.Volume != 0.4 || !got.Muted {
		t.Errorf("state %+v", got)
	}
	if len(got.Next) != 1 || got.Next[0].Title != "C" {
		t.Errorf("next %+v", got.Next)
	}
	if empty := remoteStateOf(player.State{Index: -1, Volume: 1}); empty.Track != nil || empty.Volume != 1 {
		t.Errorf("nothing playing: %+v", empty)
	}
}

// TestRemoteProblems tells each failure in the interface's words.
func TestRemoteProblems(t *testing.T) {
	for err, want := range map[error]string{
		remote.ErrWrongCode:    "That isn't the code Desk showed",
		remote.ErrOff:          "Remote control is off on Desk",
		remote.ErrBusy:         "Desk is pairing with another computer",
		remote.ErrNotSame:      "Another computer answered in place of Desk",
		remote.ErrLost:         "Desk stopped answering",
		os.ErrDeadlineExceeded: "Couldn't reach Desk",
	} {
		if got := remoteProblem("Desk", err); !strings.Contains(got, want) {
			t.Errorf("%v: %q", err, got)
		}
	}
}
