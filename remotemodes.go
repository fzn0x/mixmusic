package main

// More of a computer controlled, on its page (remote control, #556): its
// shuffle, repeat, mute and like, which show as they are there, the place
// it plays, one of its sound outputs or a cast device it found, and Move
// here, which brings what plays there to this computer
// (internal/app's remotehandover.go). A computer whose MixMusic is older
// shows none of them.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/locales"
)

// remoteShuffle shuffles the queue of the computer id, or no longer.
func (s *shell) remoteShuffle(c *ui.Context, id string, v app.RemoteView) {
	if !v.CanModes {
		return
	}
	st := v.State
	if s.iconButton(c, "remote-shuffle", "shuffle", locales.PlayerShuffle(), st.Shuffle, 18).Clicked() {
		s.remoteDo(id, remote.Command{Op: remote.OpShuffle, Shuffle: !st.Shuffle})
	}
}

// remoteRepeat repeats the queue of the computer id, then its track, then
// nothing, as the player bar's repeat does.
func (s *shell) remoteRepeat(c *ui.Context, id string, v app.RemoteView) {
	if !v.CanModes {
		return
	}
	name, label, next := "repeat", locales.PlayerRepeatOff(), "all"
	switch v.State.Repeat {
	case "all":
		label, next = locales.PlayerRepeatAll(), "one"
	case "one":
		name, label, next = "repeatOne", locales.PlayerRepeatOne(), ""
	}
	if s.iconButton(c, "remote-repeat", name, label, v.State.Repeat != "", 18).Clicked() {
		s.remoteDo(id, remote.Command{Op: remote.OpRepeat, Repeat: next})
	}
}

// remoteMute mutes the computer id, or unmutes it, where it takes it; the
// volume's icon tells it, as it did, where not.
func (s *shell) remoteMute(c *ui.Context, id string, v app.RemoteView) {
	st := v.State
	name, label := "volume", locales.VolumeMute()
	if st.Muted || st.Volume == 0 {
		name = "volumeMute"
	}
	if st.Muted {
		label = locales.VolumeUnmute()
	}
	if !v.CanModes {
		ui.Icon(c, icon(name)).FontSize(18).TextColor(s.lk.muted)
		return
	}
	if s.iconButton(c, "remote-mute", name, label, false, 18).Clicked() {
		s.remoteDo(id, remote.Command{Op: remote.OpMute, Muted: !st.Muted})
	}
}

// remoteLike likes the track that plays on the computer id, there, or no
// longer.
func (s *shell) remoteLike(c *ui.Context, id string, v app.RemoteView) {
	st := v.State
	if !v.CanModes || st.Track == nil {
		return
	}
	label := locales.PlayerLike()
	if st.Liked {
		label = locales.PlayerUnlike()
	}
	if s.ratingButton(c, "remote-like", "heart", label, st.Liked, 18) {
		s.remoteDo(id, remote.Command{Op: remote.OpLike, Liked: !st.Liked, Entry: st.Track.Entry})
	}
}

// remoteOutput is the place the computer id plays, as its state tells
// it: its menu lists its sound outputs, then the cast devices it found,
// and plays on the one chosen there.
func (s *shell) remoteOutput(c *ui.Context, id string, v app.RemoteView) {
	st := v.State
	if !v.CanOutputs || len(st.Outputs) == 0 {
		return
	}
	at := locales.OutputChoiceSystemDefault()
	for _, o := range st.Outputs {
		if o.ID == st.Output {
			at, _ = app.PlaceName(o)
		}
	}
	b := s.iconButton(c, "remote-output", "speaker", locales.OutputChoiceButton(at), st.Output != remote.OutputDefault, 18)
	b.Menu(func(m *ui.Menu) {
		cast := false
		for _, o := range st.Outputs {
			isCast := o.Kind != remote.OutputDefault && o.Kind != remote.OutputSound
			if isCast && !cast {
				m.Separator()
			}
			cast = cast || isCast
			name, detail := app.PlaceName(o)
			item := m.Item(name).Radio(o.ID == st.Output)
			if detail != "" {
				item.Subtitle(detail)
			}
			if item.Chosen() && o.ID != st.Output {
				s.remoteDo(id, remote.Command{Op: remote.OpOutput, Output: o.ID})
			}
		}
	})
}

// remoteMoveHere is the button that brings what plays on the computer id
// to this one, which plays it in place of its own: the other pauses.
func (s *shell) remoteMoveHere(c *ui.Context, id string, v app.RemoteView) {
	if !v.CanBring || v.State.Track == nil {
		return
	}
	label := locales.RemoteMoveHere()
	if s.remote.bringing {
		label = locales.RemoteMovingHere()
	}
	if s.button(c, "remote-move-here", label, false).Disabled(s.remote.bringing).Clicked() && !s.remote.bringing {
		s.bring(id, v.Computer.Name)
	}
}

// bring brings what plays on the computer id, named name, here, beside the
// main thread, and tells how it went.
func (s *shell) bring(id, name string) {
	s.remote.bringing = true
	go func() {
		got, err := s.a.Remote().Bring(id)
		s.update(func() {
			s.remote.bringing = false
			if err != nil {
				s.notes = append(s.notes, remoteNotice(err))
				return
			}
			s.notes = append(s.notes, notice(broughtNote(name, got)))
		})
	}()
}

// broughtNote tells how bringing what plays on the computer named name
// here went.
func broughtNote(name string, got app.HandedOver) string {
	switch {
	case got.Kept == 0:
		return locales.RemoteBroughtNone(name)
	case got.Left == 0:
		return locales.RemoteBroughtPaused(name)
	}
	return locales.RemoteBroughtLeft(name, got.Left, yesNo(got.CurrentLeft))
}
