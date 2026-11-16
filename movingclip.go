package main

// The clips of albums' moving covers on the now playing page (#569). With
// Show moving covers on and a source of clips set, which waits for its
// choice (internal/app/movingcovers.go), the sleeve loops the clip of the
// album that plays, muted and cut to fill its square, wherever the track
// plays: a loop follows no clock. It plays in the loop mode of the page
// of the picture of music videos (video.go, public/video.js), whose web
// view, link and fade it shares, and pauses with the music.
//
// The picture of a music video wins, and nothing shows while the mini
// player is open. The cover shows until the clip's first frame, and on any
// failure: a clip that didn't play is tried once more, of another
// rendition, then the track keeps its cover until MixMusic starts again.
// While the desktop asks for less motion, a button plays it.

import (
	"context"
	"log"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// movingClips is what the window keeps of the clips of the tracks played,
// by their IDs: the clip that didn't play, which the next ask leaves out,
// those whose clips failed twice, which keep their covers, and those the
// user asked to play while the desktop asks for less motion.
type movingClips struct {
	failed         map[string]string
	broken, played map[string]bool
}

// clipKey is the key of what fetch got of the clip of track id.
func clipKey(id string) string { return "clip:" + id }

// clipSleeve builds the sleeve of t, size wide, as the clip of its album
// that loops, and reports whether it did: not while the clip is looked
// for, nor for an album of none.
func (s *shell) clipSleeve(c *ui.Context, t media.Track, size float32) bool {
	m := &s.moving.clips
	if !s.set.MovingCovers || !s.svc.Player.HasClips() || m.broken[t.ID] || s.videoProblem() != "" || s.svc.System.MiniPlayerOpen() {
		return false
	}
	res := s.clipSource(t)
	if !res.done || res.err != nil {
		return false
	}
	src := &res.value
	k := &s.lk
	v := &s.video
	asked := !c.Preferences().ReduceMotion || m.played[t.ID]
	playing := s.st.Playing && !s.st.Buffering
	shown := v.status.Src == src.Src && v.status.Shown
	ui.Box(c.Key("sleeve-clip")).Size(size, size).Shrink(0).Radius(k.radiusSm).Clip().Children(func() {
		// A GIF cover holds still under the clip that shows.
		s.movingCover(c, "sleeve", t.Artwork, size, playing && !shown, locales.CollectionMovingCover(t.Title))
		switch {
		case !asked:
			b := s.buttonBase(c.Key("sleeve-clip-play")).Size(42, 42).Radius(21).Background(k.accent).Absolute().
				Right(8).Bottom(8).Label(locales.MovingClipPlay()).Tooltip(locales.MovingClipPlay()).Cursor(ui.CursorPointer)
			if b.Clicked() {
				if m.played == nil {
					m.played = map[string]bool{}
				}
				m.played[t.ID] = true
			}
			b.Children(func() { ui.Icon(c, icon("play")).FontSize(18).TextColor(k.accentText) })
			return
		case v.view == nil:
			s.makeVideoView()
			return
		}
		target := float32(0)
		if shown {
			target = 1
		}
		web := ui.WebView(c.Key("clip "+src.Src), v.view).Absolute().Left(0).Top(0).Size(size, size).Label(locales.MovingClipAlbum(t.Title))
		// Until its first frame, the page shows nothing over the cover.
		web.Opacity(web.Animate("fade", target, videoFade))
		ui.Box(c.Key("sleeve-clip-over")).Absolute().Left(0).Top(0).Size(size, size).ContextMenu(func(m *ui.Menu) { s.trackMenu(m, t) })
		filter := ""
		if k.fx.phosphor {
			filter = videoFilter
		}
		v.want = videoWant{src: src, track: t.ID, active: true, playing: playing, filter: filter}
		v.built = true
	})
	return true
}

// clipSource returns the clip of the album of t, looked for once, for the
// largest sleeve, so that a window resized keeps it; one that didn't play
// is left out. It follows the player, as the picture does: a refresh of
// the page doesn't ask for it again.
func (s *shell) clipSource(t media.Track) *loaded[app.VideoSource] {
	key := clipKey(t.ID)
	o := app.CoverClipOptions{Px: s.artPixels(videoFrame), Failed: s.moving.clips.failed[t.ID]}
	l := fetch(s, key, func(ctx context.Context) (app.VideoSource, error) { return s.svc.Player.MovingCover(ctx, t, o) })
	delete(s.showing, key)
	return l
}

// clipTold takes in that the page couldn't play the clip of what plays,
// and reports whether it was that clip: it is looked for again, leaving
// it out, once; then the track keeps its cover, and the clip goes.
func (s *shell) clipTold(st videoStatus) bool {
	cur, ok := s.st.Current()
	if !ok {
		return false
	}
	t := cur.Track
	l, ok := s.loads[clipKey(t.ID)].(*loaded[app.VideoSource])
	if !ok || !l.done || l.err != nil || l.value.Src != st.Src {
		return false
	}
	m := &s.moving.clips
	if _, again := m.failed[t.ID]; again {
		if m.broken == nil {
			m.broken = map[string]bool{}
		}
		if !m.broken[t.ID] {
			m.broken[t.ID] = true
			log.Printf("mixmusic: the moving cover of %s doesn't play: %s", t.ID, st.Failed)
			s.svc.Player.ForgetMovingCover(t)
		}
		return true
	}
	if m.failed == nil {
		m.failed = map[string]string{}
	}
	m.failed[t.ID] = st.Src
	s.forget(clipKey(t.ID))
	return true
}

// movingCoverItem is Find the moving cover again, in the menu of a track,
// while albums' clips are looked for: what was found of its album's, or
// that it has none, is forgotten, and looked for again.
func (s *shell) movingCoverItem(m *ui.Menu, t media.Track) {
	if !s.set.MovingCovers || !s.svc.Player.HasClips() || t.IsEpisode() {
		return
	}
	if m.Item(locales.MovingClipFindAgain()).Chosen() {
		s.svc.Player.ForgetMovingCover(t)
		c := &s.moving.clips
		delete(c.failed, t.ID)
		delete(c.broken, t.ID)
		s.forget(clipKey(t.ID))
		s.notes = append(s.notes, notice(locales.MovingClipLooking(t.Title)))
	}
}
