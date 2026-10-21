package app

// The audio engine: MixMusic's own player, which plays the stream of each
// track, and the player's view of it.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
)

// errNoNative is returned where the system has no audio engine.
var errNoNative = errors.New("MixMusic's audio engine isn't available on this system")

// engineEvents passes the engine's events to the player, once there is one.
type engineEvents struct{ a *App }

// to returns the events' destination: the player, or nowhere.
func (e engineEvents) to() audio.Events {
	if p := e.a.goPlayer.Load(); p != nil {
		return p
	}
	return noEvents{}
}

func (e engineEvents) Time(position, duration time.Duration) { e.to().Time(position, duration) }
func (e engineEvents) Ended()                                { e.to().Ended() }
func (e engineEvents) Error(message string)                  { e.to().Error(message) }
func (e engineEvents) Playing(playing bool)                  { e.to().Playing(playing) }
func (e engineEvents) Buffering(buffering bool)              { e.to().Buffering(buffering) }

// noEvents are the events of the engine before the player is.
type noEvents struct{}

func (noEvents) Time(time.Duration, time.Duration) {}
func (noEvents) Ended()                            {}
func (noEvents) Error(string)                      {}
func (noEvents) Playing(bool)                      {}
func (noEvents) Buffering(bool)                    {}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func fromMs(v float64) time.Duration {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	return time.Duration(v * float64(time.Millisecond))
}

// nativeEngine returns the native engine, made at its first use, or nil
// where the system has none.
func (a *App) nativeEngine() *audio.Engine {
	if !settings.NativeAudio {
		return nil
	}
	a.nativeOnce.Do(func() {
		start := startNative
		if Silent() {
			start = startSilent
		}
		a.native = audio.NewEngine(a.openNative, start, engineEvents{a})
	})
	return a.native
}

// Silent reports whether the engine plays into no device, as
// MIXMUSIC_SILENT asks: for screenshots of what plays, its visualizer and
// lyrics moving, without a sound.
func Silent() bool { return os.Getenv("MIXMUSIC_SILENT") != "" }

// startSilent starts an output that plays into no device: it renders at a
// device's pace, and drops what it renders.
func startSilent(render func([]float32), _ func(error)) (audio.Output, error) {
	log.Print("mixmusic: the audio engine plays into no device (MIXMUSIC_SILENT)")
	o := &silentOutput{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(o.done)
		period := make([]float32, 2*output.Rate/100) // 10 ms, in stereo
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-o.stop:
				return
			case <-tick.C:
				render(period)
			}
		}
	}()
	return o, nil
}

// silentOutput is the output of startSilent.
type silentOutput struct {
	once       sync.Once
	stop, done chan struct{}
}

func (o *silentOutput) Stop() {
	o.once.Do(func() { close(o.stop) })
	<-o.done
}

func (o *silentOutput) Delay() time.Duration { return 0 }

// openNative opens, for the native engine, the audio at a URL PrepareStream
// gave.
func (a *App) openNative(ctx context.Context, rawURL string, interrupted func() bool) (audio.Source, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	videoID, ok := strings.CutPrefix(u.Path, "/yt/")
	if !ok || !stream.ValidVideoID(videoID) {
		return nil, fmt.Errorf("not the audio of a video: %s", rawURL)
	}
	src, err := a.proxy.Open(ctx, videoID, interrupted)
	if err != nil {
		return nil, err
	}
	return src, nil
}

// startNative starts the output of the native engine.
func startNative(render func([]float32), failed func(error)) (audio.Output, error) {
	s, err := output.Start(render, failed)
	switch {
	case errors.Is(err, output.ErrNoDevice):
		return nil, errors.New("no audio output device")
	case err != nil:
		return nil, fmt.Errorf("the audio device couldn't start (%v)", err)
	}
	return s, nil
}

// NativeAnalysis is what the native engine's analyser gives, as an
// AnalyserNode of fftSize 256: Bytes as getByteFrequencyData; Decibels, when
// asked, as getFloatFrequencyData, with -1000 for -Infinity; Waveform, when
// asked, as getFloatTimeDomainData.
type NativeAnalysis struct {
	Bytes    []int     `json:"bytes"`
	Decibels []float64 `json:"decibels"`
	Waveform []float64 `json:"waveform"`
}

// NativeAnalyser returns what the analyser gives now: the native engine's,
// or, while the track plays outside the engine, the listener's, which hears
// Spotify where MixMusic can.
func (p *Player) NativeAnalyser(decibels, waveform bool) NativeAnalysis {
	var a audio.Analysis
	if l := p.a.listen.Load(); l != nil && l.outside.Load() {
		l.tap.Analyse(&a)
	} else {
		e := p.a.nativeEngine()
		if e == nil {
			return NativeAnalysis{}
		}
		e.Analyse(&a)
	}
	out := NativeAnalysis{Bytes: make([]int, len(a.Bytes))}
	for i, b := range a.Bytes {
		out.Bytes[i] = int(b)
	}
	if decibels {
		out.Decibels = make([]float64, len(a.Decibels))
		for i, db := range a.Decibels {
			out.Decibels[i] = max(float64(db), -1000)
		}
	}
	if waveform {
		out.Waveform = make([]float64, len(a.Waveform))
		for i, v := range a.Waveform {
			out.Waveform[i] = float64(v)
		}
	}
	return out
}
