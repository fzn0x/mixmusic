package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// chromiumConfig is a chain to render, in the browser and here.
type chromiumConfig struct {
	Name       string      `json:"name"`
	EQ         bool        `json:"eq"`
	Preamp     float64     `json:"preamp"`
	Gains      [10]float64 `json:"gains"`
	Compressor bool        `json:"compressor"`
	// Analyse reads the analyser every this many frames, a multiple of
	// quantum, when not 0.
	Analyse int `json:"analyse"`
}

var chromiumConfigs = []chromiumConfig{
	{Name: "bass", EQ: true, Preamp: -3, Gains: [10]float64{6, 5, 4, 2, 0, 0, 0, 0, 0, 0}},
	{Name: "lofi", EQ: true, Preamp: 2, Gains: [10]float64{-3, 0, 3, 4, 2, 0, -2, -4, -6, -8}},
	{Name: "level", Compressor: true},
	{Name: "eq-level", EQ: true, Gains: [10]float64{5, 4, 1, 0, -1, 1, 0, 1, 4, 5}, Compressor: true},
	{Name: "analyser", Analyse: 1024},
	{Name: "lowshelf", EQ: true, Gains: [10]float64{6, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	{Name: "peak", EQ: true, Gains: [10]float64{0, 0, 0, 0, 6, 0, 0, 0, 0, 0}},
	{Name: "highshelf", EQ: true, Gains: [10]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 6}},
	{Name: "flat", EQ: true},
}

// testSignal is two seconds of stereo, loud and soft in turns, to move the
// compressor both ways, with tones and noise for the filters.
func testSignal() []float32 {
	const frames = 2 * Rate
	out := make([]float32, 2*frames)
	seed := uint32(1)
	noise := func() float64 {
		seed = seed*1664525 + 1013904223
		return float64(int32(seed)) / (1 << 31)
	}
	for i := range frames {
		t := float64(i) / Rate
		env1, env2 := 0.08, 0.05
		if math.Mod(t, 0.5) < 0.25 {
			env1 = 1
		}
		if math.Mod(t+0.1, 0.7) < 0.3 {
			env2 = 0.9
		}
		out[2*i] = float32(env1*(0.55*math.Sin(2*math.Pi*110*t)+0.25*math.Sin(2*math.Pi*1234*t)) + 0.05*noise())
		out[2*i+1] = float32(env2*(0.5*math.Sin(2*math.Pi*220.5*t+0.3)+0.2*math.Sin(2*math.Pi*5000*t)) + 0.05*noise())
	}
	return out
}

// renderHere runs the chain of cfg on input, by quanta, and returns its
// output and the analyser's readings.
func renderHere(input []float32, cfg chromiumConfig) ([]float32, [][analyserBins]byte) {
	out := append([]float32(nil), input...)
	eq := newEqualizer()
	eq.set(cfg.EQ, cfg.Gains, cfg.Preamp)
	// Constant parameters, as the browser's: set, not glided to.
	eq.preamp.value = eq.preamp.target
	for i := range eq.gains {
		eq.gains[i].value = eq.gains[i].target
	}
	comp := newCompressor()
	an := newAnalyser()
	var spectra [][analyserBins]byte
	for start := 0; start < len(out); start += 2 * quantum {
		block := out[start:min(start+2*quantum, len(out))]
		if cfg.EQ {
			eq.process(block)
		}
		if cfg.Compressor {
			comp.process(block, levelThreshold, levelKnee, levelRatio, levelAttack, levelRelease)
		}
		if cfg.Analyse > 0 {
			an.writeInput(block)
			if end := (start + len(block)) / 2; end%cfg.Analyse == 0 {
				var d [analyserBins]byte
				an.byteFrequencyData(&d)
				spectra = append(spectra, d)
			}
		}
	}
	return out, spectra
}

const chromiumHarness = `<!doctype html><meta charset="utf-8"><script>
const BANDS = [32, 64, 125, 250, 500, 1000, 2000, 4000, 8000, 16000];
async function render(input, cfg) {
  const frames = input.length / 2;
  const ctx = new OfflineAudioContext(2, frames, 48000);
  const buf = ctx.createBuffer(2, frames, 48000);
  const l = buf.getChannelData(0), r = buf.getChannelData(1);
  for (let i = 0; i < frames; i++) { l[i] = input[2 * i]; r[i] = input[2 * i + 1]; }
  const src = ctx.createBufferSource();
  src.buffer = buf;
  let node = src;
  if (cfg.eq) {
    const pre = ctx.createGain();
    pre.gain.value = Math.pow(10, cfg.preamp / 20);
    node.connect(pre); node = pre;
    BANDS.forEach((hz, i) => {
      const f = ctx.createBiquadFilter();
      f.type = i === 0 ? "lowshelf" : i === BANDS.length - 1 ? "highshelf" : "peaking";
      f.frequency.value = hz; f.Q.value = 1.1; f.gain.value = cfg.gains[i];
      node.connect(f); node = f;
    });
  }
  if (cfg.compressor) {
    const c = ctx.createDynamicsCompressor();
    c.threshold.value = -24; c.knee.value = 24; c.ratio.value = 4; c.attack.value = 0.01; c.release.value = 0.25;
    node.connect(c); node = c;
  }
  const spectra = [];
  if (cfg.analyse) {
    const a = ctx.createAnalyser();
    a.fftSize = 256; a.smoothingTimeConstant = 0.8;
    node.connect(a); node = a;
    for (let f = cfg.analyse; f < frames; f += cfg.analyse) {
      ctx.suspend(f / 48000).then(() => {
        const d = new Uint8Array(a.frequencyBinCount);
        a.getByteFrequencyData(d);
        spectra.push(Array.from(d));
        ctx.resume();
      });
    }
  }
  node.connect(ctx.destination);
  src.start();
  const rendered = await ctx.startRendering();
  const out = new Float32Array(2 * frames);
  const ol = rendered.getChannelData(0), or = rendered.getChannelData(1);
  for (let i = 0; i < frames; i++) { out[2 * i] = ol[i]; out[2 * i + 1] = or[i]; }
  return { out, spectra };
}
(async () => {
  try {
    const input = new Float32Array(await (await fetch("/input")).arrayBuffer());
    const configs = await (await fetch("/configs")).json();
    for (const cfg of configs) {
      const { out, spectra } = await render(input, cfg);
      await fetch("/out?name=" + cfg.name, { method: "POST", body: out.buffer });
      await fetch("/spectra?name=" + cfg.name, { method: "POST", body: JSON.stringify(spectra) });
    }
    await fetch("/done", { method: "POST", body: navigator.userAgent });
  } catch (err) {
    await fetch("/done", { method: "POST", body: "error: " + err });
  }
})();
</script>`

// TestAgainstChromium renders a signal through the equalizer, the
// compressor and the analyser of this package, and through the Web Audio
// nodes of the web player that they reproduce, in the Chromium browser
// MIXMUSIC_BROWSER names (msedge.exe, chrome): they must give the same
// samples, bit for bit, as they did with Edge 155.
func TestAgainstChromium(t *testing.T) {
	browser := os.Getenv("MIXMUSIC_BROWSER")
	if browser == "" {
		t.Skip("MIXMUSIC_BROWSER names a Chromium browser to compare Web Audio with")
	}
	input := testSignal()
	var mu sync.Mutex
	outs := map[string][]float32{}
	spectra := map[string][][analyserBins]byte{}
	done := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/harness.html", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, chromiumHarness)
	})
	mux.HandleFunc("/input", func(w http.ResponseWriter, r *http.Request) {
		binary.Write(w, binary.LittleEndian, input)
	})
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(chromiumConfigs)
	})
	mux.HandleFunc("/out", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f := make([]float32, len(b)/4)
		binary.Read(bytes.NewReader(b), binary.LittleEndian, f)
		mu.Lock()
		outs[r.URL.Query().Get("name")] = f
		mu.Unlock()
	})
	mux.HandleFunc("/spectra", func(w http.ResponseWriter, r *http.Request) {
		var s [][analyserBins]byte
		var raw [][]int
		json.NewDecoder(r.Body).Decode(&raw)
		for _, row := range raw {
			var d [analyserBins]byte
			for i := range min(len(row), analyserBins) {
				d[i] = byte(row[i])
			}
			s = append(s, d)
		}
		mu.Lock()
		spectra[r.URL.Query().Get("name")] = s
		mu.Unlock()
	})
	mux.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		done <- string(b)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// The browser's processes keep files of their profile open a moment
	// after it is killed: removed with retries, not as t.TempDir.
	profile, err := os.MkdirTemp("", "mixmusic-chromium-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for range 50 {
			if os.RemoveAll(profile) == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	cmd := exec.CommandContext(ctx, browser, "--headless=new", "--disable-gpu", "--no-first-run",
		"--no-default-browser-check", "--user-data-dir="+profile, srv.URL+"/harness.html")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()
	var agent string
	select {
	case agent = <-done:
	case <-ctx.Done():
		t.Fatal("the browser did not render in time")
	}
	if len(agent) > 6 && agent[:6] == "error:" {
		t.Fatal(agent)
	}
	t.Logf("browser: %s", agent)

	mu.Lock()
	defer mu.Unlock()
	for _, cfg := range chromiumConfigs {
		got, gotSpectra := renderHere(input, cfg)
		want := outs[cfg.Name]
		if len(want) != len(got) {
			t.Errorf("%s: %d samples from the browser, %d here", cfg.Name, len(want), len(got))
			continue
		}
		var maxDiff, sig, noise float64
		for i := range got {
			d := float64(got[i]) - float64(want[i])
			maxDiff = max(maxDiff, math.Abs(d))
			sig += float64(want[i]) * float64(want[i])
			noise += d * d
		}
		snr := 10 * math.Log10(sig/max(noise, 1e-30))
		t.Logf("%s: largest difference %.3g (%.2f LSB of 16 bits), SNR %.1f dB", cfg.Name, maxDiff, maxDiff*32768, snr)
		if maxDiff != 0 {
			t.Errorf("%s: the output differs from the browser's, which may compute otherwise than the Blink this reproduces", cfg.Name)
		}
		if cfg.Analyse > 0 {
			ws := spectra[cfg.Name]
			if len(ws) != len(gotSpectra) {
				t.Errorf("%s: %d readings from the browser, %d here", cfg.Name, len(ws), len(gotSpectra))
				continue
			}
			worst, differ := 0, 0
			for i := range ws {
				for k := range ws[i] {
					d := int(ws[i][k]) - int(gotSpectra[i][k])
					if d != 0 {
						differ++
						worst = max(worst, d, -d)
					}
				}
			}
			t.Logf("%s: %d readings of %d bins, %d bytes differ, by %d at most", cfg.Name, len(ws), analyserBins, differ, worst)
			if differ > 0 {
				t.Errorf("%s: the analyser differs from the browser's", cfg.Name)
			}
		}
	}
}
