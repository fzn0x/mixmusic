package app

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/cast/casttest"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// newRemoteAppOf is newRemoteApp of an app whose options set changes.
func newRemoteAppOf(t *testing.T, n *remotetest.Network, name string, set func(o *Options)) *App {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	a := newQuietApp(t, func(o *Options) {
		o.Remote = &RemoteOptions{Discovery: n, Listen: remotetest.Listen, Identity: id, Name: name, Timing: fastRemote}
		set(o)
	})
	t.Cleanup(a.closeRemote)
	return a
}

// TestRemoteBringBetweenApps brings what plays on another computer to this
// one, which plays it from where it was, with its tracks of the services,
// its own file of the song of a file of the other, and without the file
// it doesn't have; the other pauses, and says so. With only files this
// one doesn't have, nothing moves; with nothing playing there, it says so.
func TestRemoteBringBetweenApps(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	deskNotes := notesOf(t, desk)
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	mine := musicOf(t, lap, "01 Slow Tide.flac")
	if len(mine) != 1 {
		t.Fatalf("the laptop's music %+v", mine)
	}
	elsewhere := t.TempDir()
	same := mine[0]
	same.ID = localfiles.TrackID(filepath.Join(elsewhere, "Slow Tide.flac"))
	other := media.Track{ID: localfiles.TrackID(filepath.Join(elsewhere, "Kelp.flac")), Source: media.SourceFile, Title: "Kelp",
		Artists: []media.Ref{{Name: "Harbour Lights"}}, DurationMs: 180000}
	yt := media.Track{ID: "yt:aaaaaaaaaaa", Source: media.SourceYouTube, Title: "Paper Boats", DurationMs: 240000,
		Artists: []media.Ref{{ID: "yt:UCaaaa", Name: "Harbour Lights"}}}

	pairApps(t, lap, desk)
	deskID := idOf(desk)
	if _, err := lap.Remote().Bring(deskID); err == nil || ErrorWords(err) != "Nothing plays on Desktop." {
		t.Errorf("nothing playing there: %v", err)
	}

	lp, dp := lap.GoPlayer(), desk.GoPlayer()
	dp.PlayTracks([]media.Track{yt, same, other}, 0)
	waitPlayer(t, dp, "the desktop playing", func(s player.State) bool { return s.Playing && s.Index == 0 })
	dp.Seek(30 * time.Second)
	waitPlayer(t, dp, "the seek", func(s player.State) bool { return s.Position >= 30*time.Second })
	dp.SetShuffle(true)
	dp.SetRepeat(player.RepeatOne)
	waitPlayer(t, dp, "repeating", func(s player.State) bool { return s.Repeat == player.RepeatOne && s.Shuffle })
	v := untilView(t, lap, deskID, "what plays there", func(v RemoteView) bool { return v.State.Repeat == "one" })
	if !v.CanBring || !v.CanModes || !v.CanOutputs {
		t.Errorf("the desktop can't hand over, nor take the modes: %+v", v)
	}

	got, err := lap.Remote().Bring(deskID)
	if err != nil {
		t.Fatal(err)
	}
	if got != (HandedOver{Kept: 2, Left: 1}) {
		t.Errorf("brought %+v", got)
	}
	waitPlayer(t, lp, "the laptop playing it", func(s player.State) bool { return s.Playing && len(s.Queue) == 2 })
	st := lp.State()
	if cur, _ := st.Current(); cur.Track.ID != yt.ID || st.Position < 30*time.Second || st.Repeat != player.RepeatOne || !st.Shuffle {
		t.Errorf("the laptop plays %s at %v, repeat %q, shuffle %v", cur.Track.ID, st.Position, st.Repeat, st.Shuffle)
	}
	if ids := queueIDs(st); !slices.Contains(ids, mine[0].ID) || slices.Contains(ids, same.ID) {
		t.Errorf("the laptop's queue %v", ids)
	}
	waitPlayer(t, dp, "the desktop paused", func(s player.State) bool { return !s.Playing })
	until(t, "the desktop's note", func() bool { return deskNotes.has("Moved to Laptop, which asked for it, and paused here") })

	// Only a file the laptop hasn't: nothing moves, and the desktop plays on.
	dp.PlayTracks([]media.Track{other}, 0)
	waitPlayer(t, dp, "the desktop playing the file", func(s player.State) bool { return s.Playing && len(s.Queue) == 1 })
	if got, err = lap.Remote().Bring(deskID); err != nil || got != (HandedOver{Left: 1, CurrentLeft: true}) {
		t.Errorf("brought %+v, %v", got, err)
	}
	if !dp.State().Playing {
		t.Error("the desktop paused, though the laptop took nothing")
	}
	if cur, _ := lp.State().Current(); cur.Track.ID != yt.ID {
		t.Errorf("the laptop plays %s", cur.Track.ID)
	}
}

// TestRemoteModesBetweenApps shuffles, repeats, mutes and likes on another
// computer from its page, whose state tells each.
func TestRemoteModesBetweenApps(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteApp(t, n, "Desktop")
	lap := newRemoteApp(t, n, "Laptop")
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	dp := desk.GoPlayer()
	var tracks []media.Track
	for i := range 45 {
		tracks = append(tracks, media.Track{ID: "yt:" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Source: media.SourceYouTube,
			Title: "Song", DurationMs: 200000})
	}
	dp.PlayTracks(tracks, 0)
	waitPlayer(t, dp, "playing", func(s player.State) bool { return s.Playing })
	pairApps(t, lap, desk)
	deskID := idOf(desk)
	v := untilView(t, lap, deskID, "the next tracks", func(v RemoteView) bool { return len(v.State.Next) == remote.MaxNext })
	if v.State.Shuffle || v.State.Repeat != "" || v.State.Liked || v.State.Muted {
		t.Errorf("the modes at first %+v", v.State)
	}
	do := func(c remote.Command) {
		t.Helper()
		if err := lap.Remote().Do(deskID, c); err != nil {
			t.Fatal(err)
		}
	}
	do(remote.Command{Op: remote.OpShuffle, Shuffle: true})
	do(remote.Command{Op: remote.OpRepeat, Repeat: "all"})
	do(remote.Command{Op: remote.OpMute, Muted: true})
	do(remote.Command{Op: remote.OpLike, Liked: true, Entry: v.State.Track.Entry})
	untilView(t, lap, deskID, "the modes told", func(v RemoteView) bool {
		return v.State.Shuffle && v.State.Repeat == "all" && v.State.Muted && v.State.Liked
	})
	st := dp.State()
	if !st.Shuffle || st.Repeat != player.RepeatAll || !st.Muted || !desk.IsLiked(tracks[0].ID) {
		t.Errorf("the desktop: shuffle %v, repeat %q, muted %v, liked %v", st.Shuffle, st.Repeat, st.Muted, desk.IsLiked(tracks[0].ID))
	}
	do(remote.Command{Op: remote.OpLike})
	do(remote.Command{Op: remote.OpMute})
	untilView(t, lap, deskID, "unliked, unmuted", func(v RemoteView) bool { return !v.State.Muted && !v.State.Liked })
}

// TestRemoteOutputsBetweenApps lists where another computer can play, its
// sound outputs and the cast devices it found, and plays there as asked:
// on a cast device, then on a sound output, as its settings choose it.
func TestRemoteOutputsBetweenApps(t *testing.T) {
	dev := casttest.New(t)
	r := casttest.NewResponder(t, dev.Entry())
	n := remotetest.New()
	desk := newRemoteAppOf(t, n, "Desktop", func(o *Options) {
		o.OutputDevices = func() ([]OutputDevice, error) {
			return []OutputDevice{{ID: "{0.0.0}.{speakers}", Name: "Speakers", Default: true}, {ID: "{0.0.0}.{usb}", Name: "USB DAC"}}, nil
		}
		o.CastBrowse = func(ctx context.Context) ([]cast.Device, error) {
			return cast.Browse(ctx, cast.BrowseOptions{To: r.Addr(), Wait: 200 * time.Millisecond})
		}
	})
	lap := newRemoteApp(t, n, "Laptop")
	t.Cleanup(desk.stopCasting)
	if err := desk.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	desk.browseCast()
	pairApps(t, lap, desk)
	deskID := idOf(desk)
	v := untilView(t, lap, deskID, "the places", func(v RemoteView) bool { return len(v.State.Outputs) == 4 })
	var names []string
	for _, o := range v.State.Outputs {
		name, detail := PlaceName(o)
		names = append(names, name+"/"+detail)
		if o.ID == "" || o.ID == "{0.0.0}.{usb}" {
			t.Errorf("the place's ID %q", o.ID)
		}
	}
	if want := []string{"System default/", "Speakers/", "USB DAC/", "Living Room/Chromecast"}; !slices.Equal(names, want) ||
		v.State.Output != remote.OutputDefault {
		t.Errorf("places %q, playing on %q", names, v.State.Output)
	}
	do := func(o remote.Output) {
		t.Helper()
		if err := lap.Remote().Do(deskID, remote.Command{Op: remote.OpOutput, Output: o.ID}); err != nil {
			t.Fatal(err)
		}
	}

	do(v.State.Outputs[3])
	until(t, "casting there", func() bool { _, on := (&Player{desk}).CastingTo(); return on })
	untilView(t, lap, deskID, "the cast device told", func(w RemoteView) bool { return w.State.Output == v.State.Outputs[3].ID })

	do(v.State.Outputs[2])
	until(t, "the USB DAC chosen", func() bool {
		_, on := (&Player{desk}).CastingTo()
		return !on && desk.settings.Get().OutputDevice == settings.OutputDevice{ID: "{0.0.0}.{usb}", Name: "USB DAC"}
	})
	untilView(t, lap, deskID, "the USB DAC told", func(w RemoteView) bool { return w.State.Output == v.State.Outputs[2].ID })

	do(v.State.Outputs[0])
	until(t, "the system's default", func() bool { return desk.settings.Get().OutputDevice.ID == "" })
}
