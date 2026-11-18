package player

import (
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// albumTrack is the track of id, number n of disc d of the album of
// albumID: 0 for a number not told.
func albumTrack(id, albumID string, n, d int) media.Track {
	t := track(id, "Ada Field")
	t.Album = &media.Ref{ID: albumID, Name: "Harbour Lights"}
	t.TrackNumber, t.DiscNumber = n, d
	return t
}

// gaplessHarness plays queue with a crossfade of 5 s, from its first
// track, which has played past where the next one is found.
func gaplessHarness(t *testing.T, queue []media.Track, setup ...func(*fakeServices)) *harness {
	t.Helper()
	return gaplessHarnessFrom(t, queue, 0, setup...)
}

// gaplessHarnessFrom is gaplessHarness, from the track at start.
func gaplessHarnessFrom(t *testing.T, queue []media.Track, start int, setup ...func(*fakeServices)) *harness {
	t.Helper()
	h := newHarness(t, Saved{}, append([]func(*fakeServices){func(f *fakeServices) { f.settings.Crossfade = 5 }}, setup...)...)
	h.p.PlayTracks(queue, start)
	id := queue[start].ID
	h.wait("the track playing", func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == id && p.s.Playing && !p.s.Buffering
	})
	h.p.Time(11*time.Second, 200*time.Second)
	return h
}

// fades tells the time of the last seconds of the track playing, and
// reports whether the player crossfades into the next one.
func fades(h *harness) bool {
	h.p.Time(196*time.Second, 200*time.Second)
	var fading bool
	h.on(func(p *Player) { fading = p.crossfading })
	return fading
}

// TestGaplessAlbum plays the songs of one album straight on, with
// Crossfade on: the next one starts as the one before ends, as with
// Crossfade off. With Keep albums gapless off, they crossfade.
func TestGaplessAlbum(t *testing.T) {
	album := []media.Track{albumTrack("a", "yt:MPREb_1", 1, 0), albumTrack("b", "yt:MPREb_1", 2, 0)}
	h := gaplessHarness(t, album)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
	if fades(h) {
		t.Fatal("the album's songs crossfade")
	}
	h.p.Time(199*time.Second, 200*time.Second)
	h.p.Ended()
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b 0s true" {
		t.Errorf("b loaded as %q", got)
	}
	if n := h.stream.count("crossfade"); n != 0 {
		t.Errorf("crossfaded: %q", h.stream.copy())
	}

	h = gaplessHarness(t, album, func(f *fakeServices) { f.settings.GaplessAlbums = false })
	if !fades(h) {
		t.Fatal("the album's songs don't crossfade with Keep albums gapless off")
	}
	h.wait("b crossfading in", playing("b"))
	if !h.stream.has("crossfade url:yt:b 5s") {
		t.Errorf("crossfaded %q", h.stream.copy())
	}
}

// TestGaplessAlbumSteps crossfades the step out of an album, and each
// step within one that isn't the next track of it.
func TestGaplessAlbumSteps(t *testing.T) {
	for _, c := range []struct {
		what  string
		a, b  media.Track
		joins bool
	}{
		{"the next track", albumTrack("a", "yt:MPREb_1", 3, 0), albumTrack("b", "yt:MPREb_1", 4, 0), true},
		{"tracks without numbers", albumTrack("a", "yt:MPREb_1", 0, 0), albumTrack("b", "yt:MPREb_1", 0, 0), true},
		{"a track without its number", albumTrack("a", "yt:MPREb_1", 3, 0), albumTrack("b", "yt:MPREb_1", 0, 0), true},
		{"the first track of the next disc", albumTrack("a", "file:album-1", 12, 1), albumTrack("b", "file:album-1", 1, 2), true},
		{"the next track of the same disc", albumTrack("a", "sp:al1", 1, 2), albumTrack("b", "sp:al1", 2, 2), true},
		{"a track the album skips", albumTrack("a", "yt:MPREb_1", 3, 0), albumTrack("b", "yt:MPREb_1", 5, 0), false},
		{"the track before", albumTrack("a", "yt:MPREb_1", 4, 0), albumTrack("b", "yt:MPREb_1", 3, 0), false},
		{"the second track of the next disc", albumTrack("a", "file:album-1", 12, 1), albumTrack("b", "file:album-1", 2, 2), false},
		{"a track of another disc", albumTrack("a", "file:album-1", 3, 1), albumTrack("b", "file:album-1", 4, 2), false},
		{"another album", albumTrack("a", "yt:MPREb_1", 3, 0), albumTrack("b", "yt:MPREb_2", 4, 0), false},
		{"an album of the same title on another service", albumTrack("a", "yt:MPREb_1", 3, 0), albumTrack("b", "sp:MPREb_1", 4, 0), false},
		{"a track of no album", albumTrack("a", "yt:MPREb_1", 3, 0), track("b", "Ada Field"), false},
		{"albums without IDs", albumTrack("a", "", 3, 0), albumTrack("b", "", 4, 0), false},
	} {
		t.Run(c.what, func(t *testing.T) {
			if got := follows(c.a, c.b) && sameAlbum(c.a, c.b); got != c.joins {
				t.Errorf("joins %v", got)
			}
		})
	}
	h := gaplessHarness(t, []media.Track{albumTrack("a", "yt:MPREb_1", 1, 0), albumTrack("b", "yt:MPREb_2", 2, 0)})
	if !fades(h) {
		t.Error("the step out of an album doesn't crossfade")
	}
}

// TestGaplessAlbumOrder joins only the tracks next to each other in the
// queue's own order: a track between them, Repeat all's step back to the
// first, and the radio's tracks crossfade.
func TestGaplessAlbumOrder(t *testing.T) {
	// The album's tracks tell no numbers: the queue's order alone tells.
	a, b := albumTrack("a", "yt:MPREb_1", 0, 0), albumTrack("b", "yt:MPREb_1", 0, 0)
	label := func(i int, label func(*Item)) func(*Player) {
		return func(p *Player) {
			q := slices.Clone(p.s.Queue)
			label(&q[i])
			p.s.Queue = q
		}
	}
	h := gaplessHarness(t, []media.Track{a, track("x"), b})
	h.on(label(1, func(it *Item) { it.Skip = SkipDisliked }))
	if !fades(h) {
		t.Error("a track the queue passes over between two of an album doesn't crossfade")
	}

	h = gaplessHarnessFrom(t, []media.Track{a, b}, 1)
	h.p.CycleRepeat()
	h.on(func(p *Player) {
		if next, _ := p.s.UpNext(); p.s.Repeat != RepeatAll || next.Track.ID != "yt:a" {
			t.Fatalf("repeat %s, next %s", p.s.Repeat, next.Track.ID)
		}
	})
	if !fades(h) {
		t.Error("Repeat all's step back to the first track doesn't crossfade")
	}

	h = gaplessHarness(t, []media.Track{a, b})
	h.on(label(1, func(it *Item) { it.Radio = true }))
	if !fades(h) {
		t.Error("the step into the radio doesn't crossfade")
	}
	h = gaplessHarness(t, []media.Track{a, b})
	h.on(label(0, func(it *Item) { it.Radio = true }))
	if !fades(h) {
		t.Error("the step out of the radio doesn't crossfade")
	}
}

// TestGaplessAlbumShuffled joins the tracks of a shuffled album that land
// in their own order, the order from before Shuffle, and crossfades the
// others.
func TestGaplessAlbumShuffled(t *testing.T) {
	// The album's tracks tell no numbers: the queue's own order alone
	// tells.
	var album []media.Track
	for _, id := range []string{"a", "b", "c", "d"} {
		album = append(album, albumTrack(id, "yt:MPREb_1", 0, 0))
	}
	for _, c := range []struct {
		order []string
		joins bool
	}{{[]string{"a", "b", "d", "c"}, true}, {[]string{"a", "c", "b", "d"}, false}, {[]string{"a", "d", "c", "b"}, false}} {
		h := gaplessHarness(t, album)
		h.on(func(p *Player) {
			p.setShuffle(true)
			byID := map[string]Item{}
			for _, it := range p.s.Queue {
				byID[it.Track.ID] = it
			}
			q := make([]Item, len(c.order))
			for i, id := range c.order {
				q[i] = byID["yt:"+id]
			}
			p.s.Queue, p.s.Index = q, 0
		})
		if got := !fades(h); got != c.joins {
			t.Errorf("shuffled as %v, a joins %s: %v, not %v", c.order, c.order[1], got, c.joins)
		}
	}
	// The tracks between, once gone from the queue, leave it to their
	// neighbours.
	h := gaplessHarness(t, album)
	h.on(func(p *Player) {
		p.setShuffle(true)
		var q []Item
		for _, it := range p.s.Queue {
			if it.Track.ID == "yt:a" || it.Track.ID == "yt:c" {
				q = append(q, it)
			}
		}
		if q[0].Track.ID != "yt:a" {
			q[0], q[1] = q[1], q[0]
		}
		p.s.Queue, p.s.Index = q, 0
	})
	if fades(h) {
		t.Error("a then c, b gone from the queue, crossfade")
	}
}

// TestGaplessAlbumLateEdit asks again at each time the engine tells: the
// queue changed in the last seconds of a track crossfades into the next
// one, now of another album.
func TestGaplessAlbumLateEdit(t *testing.T) {
	h := gaplessHarness(t, []media.Track{albumTrack("a", "yt:MPREb_1", 1, 0), albumTrack("b", "yt:MPREb_1", 2, 0), track("x")})
	if fades(h) {
		t.Fatal("the album's songs crossfade")
	}
	h.p.Move(2, 1)
	h.on(func(*Player) {})
	h.p.Time(197*time.Second, 200*time.Second)
	h.wait("x crossfading in", playing("x"))
	if !h.stream.has("crossfade url:yt:x 5s") {
		t.Errorf("crossfaded %q", h.stream.copy())
	}
}

// TestGaplessSpotifyAlbum joins the tracks of a Spotify album that play
// from their recordings on YouTube Music only when both are songs: a music
// video often starts or ends otherwise.
func TestGaplessSpotifyAlbum(t *testing.T) {
	spotify := func(id string, n int) media.Track {
		t := albumTrack(id, "sp:al1", n, 1)
		t.ID, t.Source = "sp:"+id, media.SourceSpotify
		return t
	}
	for _, c := range []struct {
		what       string
		videoA     bool
		videoB     bool
		joins      bool
		recordings bool
	}{
		{"songs", false, false, true, true},
		{"a music video, then a song", true, false, false, true},
		{"a song, then a music video", false, true, false, true},
		{"recordings not known", false, false, false, false},
	} {
		t.Run(c.what, func(t *testing.T) {
			h := gaplessHarness(t, []media.Track{spotify("a", 1), spotify("b", 2)}, func(f *fakeServices) {
				f.prepare = func(tr media.Track) (Source, error) {
					rec := track("rec-"+tr.ID[len("sp:"):], "Ada Field")
					rec.Video = tr.ID == "sp:a" && c.videoA || tr.ID == "sp:b" && c.videoB
					if !c.recordings {
						rec = tr // as a download kept before MixMusic kept recordings
					}
					return Source{Mode: ModeStream, URL: "url:" + tr.ID, Track: rec}, nil
				}
			})
			h.wait("b prepared", func(p *Player) bool { return p.albums.uid != "" })
			if got := !fades(h); got != c.joins {
				t.Errorf("joins %v", got)
			}
		})
	}
}
