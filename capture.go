package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// captureFromEnv saves screenshots of the window, for development.
// MIXMUSIC_CAPTURE names the folder; MIXMUSIC_CAPTURE_ROUTES the steps,
// separated by |: a path shows its page and saves it, act:shot:<name> saves
// the window as it is, act:mute mutes the player, act:play:<search> plays
// the songs a search finds, act:resume plays the queue, act:wait:<duration>
// waits, act:palette opens the command palette, act:panel:<queue|lyrics|none>
// shows a side panel, act:scroll:<y|end> scrolls the page, act:theme:<id>
// wears a theme, act:colors:<off|pywal|matugen|path> takes the colours
// from a file, act:mini opens the mini player, which saves itself,
// act:spotify logs whether MixMusic's own Spotify device could play,
// act:title logs the window's title, which screenshots leave out,
// act:memory logs the Go heap, and act:heap saves a profile of it. Nothing
// plays unless the player is muted, or plays into no device
// (MIXMUSIC_SILENT).
// MIXMUSIC_CAPTURE_WAIT is how long each page gets, MIXMUSIC_CAPTURE_SIZE
// the window's size as 1280x820, MIXMUSIC_CAPTURE_COMMUNITY a copy of the
// repository's community folder for the marketplace to read, and
// MIXMUSIC_CAPTURE_QUIT quits after.
func (s *shell) captureFromEnv() {
	dir := os.Getenv("MIXMUSIC_CAPTURE")
	if dir == "" {
		return
	}
	steps := []string{"/"}
	if r := os.Getenv("MIXMUSIC_CAPTURE_ROUTES"); r != "" {
		steps = strings.Split(r, "|")
	}
	wait := captureWait()
	if w, h, ok := strings.Cut(os.Getenv("MIXMUSIC_CAPTURE_SIZE"), "x"); ok {
		wi, _ := strconv.Atoi(w)
		hi, _ := strconv.Atoi(h)
		if wi > 0 && hi > 0 {
			s.win.SetSize(wi, hi)
		}
	}
	if community := os.Getenv("MIXMUSIC_CAPTURE_COMMUNITY"); community != "" {
		s.a.ReadCommunityFrom(community)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	s.win.Update(func() { s.capScroll = &ui.ScrollState{} })
	// mayPlay tells whether playing can't be heard.
	mayPlay := func(what string) bool {
		if s.p.State().Muted || app.Silent() {
			return true
		}
		log.Printf("mixmusic: capture: not playing %s, as the player isn't muted", what)
		return false
	}
	go func() {
		time.Sleep(wait) // the first frames, and the home feed
		n := 0
		save := func(name string) {
			png, err := s.win.CapturePage()
			if err != nil {
				log.Printf("mixmusic: capture of %s: %v", name, err)
				return
			}
			n++
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d-%s.png", n, name)), png, 0o644); err != nil {
				log.Printf("mixmusic: capture: %v", err)
			}
		}
		for _, step := range steps {
			act, arg, _ := strings.Cut(strings.TrimPrefix(step, "act:"), ":")
			if !strings.HasPrefix(step, "act:") {
				act = ""
			}
			switch act {
			case "mute":
				if !s.p.State().Muted {
					s.p.ToggleMute()
				}
				// The player mutes on its own goroutine.
				for deadline := time.Now().Add(5 * time.Second); !s.p.State().Muted && time.Now().Before(deadline); {
					time.Sleep(20 * time.Millisecond)
				}
			case "mini":
				s.showMini()
			case "spotify":
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				result, err := s.a.SpotifyHereCheck(ctx)
				cancel()
				log.Printf("mixmusic: capture: MixMusic's own Spotify device: %s %v", result, err)
			case "title":
				log.Printf("mixmusic: capture: the window's title is %q", s.win.Title())
			case "memory":
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				log.Printf("mixmusic: capture: Go heap %d MB in use, %d MB from the system, %d GCs",
					m.HeapInuse>>20, m.Sys>>20, m.NumGC)
			case "heap":
				runtime.GC()
				if f, err := os.Create(filepath.Join(dir, "heap.pprof")); err == nil {
					_ = pprof.WriteHeapProfile(f)
					f.Close()
				}
			case "play":
				if !mayPlay(strconv.Quote(arg)) {
					continue
				}
				res, err := s.svc.Catalog.Search(context.Background(), arg)
				if err != nil || len(res.YouTube.Tracks) == 0 {
					log.Printf("mixmusic: capture: searching %q: %v", arg, err)
					continue
				}
				s.p.PlayTracks(res.YouTube.Tracks, 0)
			case "resume":
				if mayPlay("the queue") {
					s.p.Play()
				}
			case "wait":
				if d, err := time.ParseDuration(arg); err == nil {
					time.Sleep(d)
				}
			case "palette":
				s.win.Update(func() { s.paletteOpen = true })
			case "panel":
				s.win.Update(func() { s.panel = strings.TrimSuffix(arg, "none") })
			case "scroll":
				y, err := strconv.ParseFloat(arg, 32)
				if arg == "end" {
					y, err = math.MaxFloat32, nil
				}
				if err == nil {
					s.win.Update(func() { s.capScroll.Y = float32(y) })
				}
			case "theme":
				s.win.Update(func() {
					s.set.Theme, s.set.FollowSystem = arg, false
					s.save()
				})
			case "colors":
				s.win.Update(func() {
					colorsStep(&s.set, arg)
					s.save()
				})
			case "shot":
				time.Sleep(wait)
				save(arg)
			default:
				s.win.Update(func() {
					s.capScroll.Y, s.paletteOpen = 0, false
					s.open(step)
				})
				time.Sleep(wait)
				name := strings.Trim(strings.NewReplacer("/", "-", "?", "-", "=", "-", ":", "-").Replace(step), "-")
				if name == "" {
					name = "home"
				}
				save(name)
			}
		}
		if os.Getenv("MIXMUSIC_CAPTURE_QUIT") != "" {
			mygo.App.Quit()
		}
	}()
}

// colorsStep takes the colours from where act:colors:<arg> says: off,
// pywal, matugen, or the path of a file.
func colorsStep(set *settings.Settings, arg string) {
	switch arg {
	case "off", "":
		set.ColorSource = settings.ColorsTheme
	case "pywal":
		set.ColorSource = settings.ColorsPywal
	case "matugen":
		set.ColorSource = settings.ColorsMatugen
	default:
		set.ColorSource, set.ColorFile = settings.ColorsFile, arg
	}
}

// captureWait is how long a page gets before its screenshot.
func captureWait() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("MIXMUSIC_CAPTURE_WAIT")); err == nil {
		return d
	}
	return 5 * time.Second
}

// captureMiniFromEnv saves a screenshot of the mini player to
// MIXMUSIC_CAPTURE's folder, a page's wait after it opens.
func (s *shell) captureMiniFromEnv() {
	dir := os.Getenv("MIXMUSIC_CAPTURE")
	if dir == "" {
		return
	}
	go func() {
		time.Sleep(captureWait())
		png, err := s.win.CapturePage()
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, "mini.png"), png, 0o644)
		}
		if err != nil {
			log.Printf("mixmusic: capture of the mini player: %v", err)
		}
	}()
}
