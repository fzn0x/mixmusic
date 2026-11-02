//go:build darwin && (amd64 || arm64)

package nowplaying

import (
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	testOnce sync.Once

	msgUint       func(obj objc.ID, sel objc.SEL) uint
	msgString     func(obj objc.ID, sel objc.SEL) string
	msgImageSized func(obj objc.ID, sel objc.SEL, size cgSize) objc.ID
	// positionEvent is a class whose instances tell a positionTime, as
	// MPChangePlaybackPositionCommandEvent's do.
	positionEvent objc.Class
)

// double is a float64 returned as Objective-C returns a double, which a
// struct of one is on arm64 and amd64.
type double struct{ v float64 }

func setUp(t *testing.T) {
	t.Helper()
	if err := loadMediaPlayer(); err != nil {
		t.Fatal(err)
	}
	testOnce.Do(func() {
		purego.RegisterFunc(&msgUint, msgSend)
		purego.RegisterFunc(&msgString, msgSend)
		purego.RegisterFunc(&msgImageSized, msgSend)
		var err error
		positionEvent, err = objc.RegisterClass("MixMusicTestPositionEvent", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{
			Cmd: sel("positionTime"),
			Fn:  func(objc.ID, objc.SEL) double { return double{42.5} },
		}})
		if err != nil {
			panic(err)
		}
	})
}

// shown is what Now Playing shows.
type shown struct {
	none                    bool
	state                   uint
	title, artist, album    string
	duration, elapsed, rate float64
	// artwork is set when the artwork gives an image.
	artwork bool
}

func nowPlaying() (s shown) {
	withPool(func() {
		center := msgID(class("MPNowPlayingInfoCenter"), sel("defaultCenter"))
		s.state = msgUint(center, sel("playbackState"))
		dict := msgID(center, sel("nowPlayingInfo"))
		if dict == 0 {
			s.none = true
			return
		}
		get := func(key objc.ID) objc.ID { return msgIDID(dict, sel("objectForKey:"), key) }
		str := func(key objc.ID) string {
			if v := get(key); v != 0 {
				return msgString(v, sel("UTF8String"))
			}
			return ""
		}
		num := func(key objc.ID) float64 {
			if v := get(key); v != 0 {
				return msgFloat(v, sel("doubleValue"))
			}
			return -1
		}
		s.title, s.artist, s.album = str(keyTitle), str(keyArtist), str(keyAlbum)
		s.duration, s.elapsed, s.rate = num(keyDuration), num(keyElapsed), num(keyRate)
		if art := get(keyArtwork); art != 0 {
			s.artwork = msgImageSized(art, sel("imageWithSize:"), cgSize{64, 64}) != 0
		}
	})
	return s
}

// invoke calls a block as Objective-C does, through its invoke function.
func invoke(b objc.Block, arg objc.ID) int {
	layout := **(**[4]uintptr)(unsafe.Pointer(&b)) // isa, flags, invoke, descriptor
	r, _, _ := purego.SyscallN(layout[2], uintptr(b), uintptr(arg))
	return int(r)
}

func TestNowPlaying(t *testing.T) {
	setUp(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		png.Encode(w, image.NewRGBA(image.Rect(0, 0, 32, 32)))
	}))
	defer srv.Close()
	artworkClient = srv.Client()

	got := make(chan Request, requests)
	c, err := Open(func(r Request) { got <- r }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	info := Info{
		ID:       "yt:1",
		Title:    "Song",
		Artists:  []string{"Ann", "Bo"},
		Album:    "Album",
		Artwork:  srv.URL + "/art.png",
		Duration: 200 * time.Second,
		Position: 10 * time.Second,
		At:       time.Now(),
		Playing:  true,
	}
	c.Show(&info)
	var s shown
	eventually(t, "Now Playing showing the track and its artwork", func() bool {
		s = nowPlaying()
		return s.artwork
	})
	if s.title != "Song" || s.artist != "Ann, Bo" || s.album != "Album" || s.duration != 200 || s.rate != 1 {
		t.Errorf("Now Playing shows %+v", s)
	}
	if s.elapsed < 10 || s.elapsed > 12 {
		t.Errorf("elapsed %v s, want about 10", s.elapsed)
	}
	faster := info
	faster.Rate, faster.At = 1.5, time.Now()
	c.Show(&faster)
	eventually(t, "Now Playing at 1.5×", func() bool {
		s = nowPlaying()
		return s.rate == 1.5
	})
	if s.state != mpPlaybackStatePlaying {
		t.Errorf("playback state %d", s.state)
	}

	// The commands' handlers, called as MediaPlayer calls them.
	m := c.sys.(*macControls)
	if len(m.blocks) != len(commands)+1 {
		t.Fatalf("%d handlers", len(m.blocks))
	}
	for i, cmd := range commands {
		if status := invoke(m.blocks[i], 0); status != mpCommandSuccess {
			t.Errorf("%s answered %d", cmd.name, status)
		}
		if r := <-got; r.Command != cmd.cmd {
			t.Errorf("%s asked %v", cmd.name, r.Command)
		}
	}
	event := objc.ID(positionEvent).Send(sel("new"))
	defer release(event)
	if status := invoke(m.blocks[len(commands)], event); status != mpCommandSuccess {
		t.Errorf("changePlaybackPositionCommand answered %d", status)
	}
	if r := <-got; r.Command != Seek || r.Position != 42500*time.Millisecond {
		t.Errorf("changePlaybackPositionCommand asked %+v", r)
	}

	paused := info
	paused.Playing, paused.Position, paused.At = false, 30*time.Second, time.Now()
	c.Show(&paused)
	eventually(t, "Now Playing pausing", func() bool {
		s = nowPlaying()
		return s.state == mpPlaybackStatePaused
	})
	if s.rate != 0 || s.elapsed != 30 || !s.artwork {
		t.Errorf("paused, Now Playing shows %+v", s)
	}

	other := paused
	other.ID, other.Title, other.Artwork = "yt:2", "Other", ""
	c.Show(&other)
	eventually(t, "Now Playing showing another track", func() bool {
		s = nowPlaying()
		return s.title == "Other"
	})
	if s.artwork {
		t.Error("another track kept the artwork of the one before")
	}

	c.Show(nil)
	eventually(t, "Now Playing showing nothing", func() bool {
		s = nowPlaying()
		return s.none && s.state == mpPlaybackStateStopped
	})
}
