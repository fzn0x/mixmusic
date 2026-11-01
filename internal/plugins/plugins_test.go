package plugins

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// testServices are the app as the player's services, playing nothing.
type testServices struct{}

func (testServices) Prepare(_ context.Context, t media.Track) (player.Source, error) {
	return player.Source{Mode: player.ModeStream, URL: "url:" + t.ID, Track: t}, nil
}
func (s testServices) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return s.Prepare(ctx, t)
}
func (testServices) StreamFailure(media.Track) string                          { return "" }
func (testServices) RecordPlay(media.Track)                                    {}
func (testServices) Radio(context.Context, media.Track) ([]media.Track, error) { return nil, nil }
func (testServices) Skip(media.Track) string                                   { return "" }
func (testServices) Settings() settings.Settings                               { return settings.Default() }
func (testServices) SaveVolume(float64)                                        {}
func (testServices) Report(player.Now)                                         {}
func (testServices) SaveQueue(player.Saved)                                    {}

// testStream plays at once, silently.
type testStream struct {
	mu sync.Mutex
	p  *player.Player
}

func (s *testStream) get() *player.Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.p
}
func (s *testStream) Load(_ string, _ time.Duration, autoplay bool) error {
	if autoplay {
		s.get().Playing(true)
	}
	return nil
}
func (s *testStream) Preload(string)                        {}
func (s *testStream) Crossfade(string, time.Duration) error { return nil }
func (s *testStream) Play() error                           { s.get().Playing(true); return nil }
func (s *testStream) Pause()                                { s.get().Playing(false) }
func (s *testStream) Stop()                                 {}
func (s *testStream) Unload()                               {}
func (s *testStream) Seek(time.Duration)                    {}
func (s *testStream) SetVolume(float64)                     {}

type testDevice struct{}

func (testDevice) Play(context.Context, media.Track, time.Duration) error { return nil }
func (testDevice) Pause(context.Context) error                            { return nil }
func (testDevice) Resume(context.Context) error                           { return nil }
func (testDevice) Seek(context.Context, time.Duration) error              { return nil }
func (testDevice) SetVolume(context.Context, float64) error               { return nil }
func (testDevice) State(context.Context) (*player.DeviceState, error)     { return nil, nil }

// testUI keeps what the plugins showed.
type testUI struct {
	mu      sync.Mutex
	toasts  []string
	paths   []string
	changes int
}

func (u *testUI) Toast(message, tone string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.toasts = append(u.toasts, tone+": "+message)
}
func (u *testUI) Navigate(path string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.paths = append(u.paths, path)
}
func (u *testUI) Changed() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.changes++
}

func (u *testUI) said(text string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return slices.ContainsFunc(u.toasts, func(t string) bool { return strings.Contains(t, text) })
}

// testHost is a host of an app in a folder of its own.
type testHost struct {
	*Host
	t   *testing.T
	a   *app.App
	p   *player.Player
	ui  *testUI
	dir string
}

func newTestHost(t *testing.T) *testHost {
	t.Helper()
	dir := t.TempDir()
	a, err := app.New(app.Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"),
		LogsDir: filepath.Join(dir, "logs"), Builtin: builtin.FS, Version: "0.5.0"})
	if err != nil {
		t.Fatal(err)
	}
	stream := &testStream{}
	p := player.New(testServices{}, stream, testDevice{}, player.Saved{})
	stream.mu.Lock()
	stream.p = p
	stream.mu.Unlock()
	t.Cleanup(p.Close)
	h := &testHost{Host: New(a, p), t: t, a: a, p: p, ui: &testUI{}, dir: filepath.Join(dir, "data")}
	h.SetUI(h.ui)
	t.Cleanup(h.Close)
	return h
}

// install writes a plugin of files, its plugin.json made of manifest.
func (h *testHost) install(id, manifest string, files map[string]string) {
	h.t.Helper()
	dir := filepath.Join(h.dir, "addons", "plugins", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		h.t.Fatal(err)
	}
	files["plugin.json"] = manifest
	for name, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			h.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			h.t.Fatal(err)
		}
	}
	h.a.Services().Addons.Reload()
}

// enable runs the plugins of ids.
func (h *testHost) enable(ids ...string) { h.Sync(ids, h.a.Services().Addons.List().Plugins) }

// wait waits for ok, failing the test after a few seconds.
func (h *testHost) wait(what string, ok func() bool) {
	h.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			h.ui.mu.Lock()
			toasts := h.ui.toasts
			h.ui.mu.Unlock()
			h.t.Fatalf("waiting for %s; the plugins said %q", what, toasts)
		}
	}
}

func manifest(id string) string {
	return fmt.Sprintf(`{"id": %q, "name": %q, "version": "1.0.0", "author": "A", "main": "index.js", "api": 2}`, id, strings.ToUpper(id))
}

func TestActivateAndStop(t *testing.T) {
	h := newTestHost(t)
	h.install("hello", manifest("hello"), map[string]string{"index.js": `
export default function activate(api) {
  api.commands.register({ id: "shout", title: "Shout", shortcut: "Mod+Shift+S", run: () => api.ui.toast("shouted " + api.plugin) });
  api.ui.addSidebarItem({ id: "item", label: "hello", page: "home" });
  api.ui.addPage({ name: "home", title: "hello", render(view) { view.set(api.ui.text("hi")) } });
  api.ui.addWidget({ id: "w", title: "widget", render(view) {} });
  api.ui.addTrackAction({ id: "copy", label: "Copy", run: (t) => api.ui.toast("action on " + t.title) });
  return () => api.ui.toast("stopped");
}`})
	h.enable("hello")
	h.wait("the command", func() bool { return len(h.Commands()) == 1 })
	cmd := h.Commands()[0]
	if cmd.ID != "hello.shout" || cmd.Title != "Shout" || cmd.Shortcut != "Mod+Shift+S" {
		t.Errorf("the command is %+v", cmd)
	}
	cmd.Run()
	h.wait("the command to run", func() bool { return h.ui.said("shouted hello") })
	if items := h.SidebarItems(); len(items) != 1 || items[0].Path != "/plugin/hello/home" {
		t.Errorf("the sidebar items are %+v", items)
	}
	if h.Page("/plugin/hello/home") == nil || len(h.Widgets()) != 1 || len(h.TrackActions()) != 1 {
		t.Fatal("the page, the widget or the track action is missing")
	}
	h.TrackActions()[0].Run(media.Track{ID: "yt:1", Title: "Strobe"})
	h.wait("the track action", func() bool { return h.ui.said("action on Strobe") })

	h.enable()
	h.wait("the plugin to stop", func() bool { return h.ui.said("stopped") })
	if len(h.Commands())+len(h.SidebarItems())+len(h.Widgets())+len(h.TrackActions()) != 0 || h.Page("/plugin/hello/home") != nil {
		t.Error("what the plugin added stayed")
	}
}

func TestWebPlugins(t *testing.T) {
	h := newTestHost(t)
	h.install("old", `{"id": "old", "name": "Old", "version": "1.0.0", "author": "A", "main": "index.js"}`,
		map[string]string{"index.js": `export default function activate(api) {}`})
	h.enable("old")
	// The host keeps the failure, then tells it.
	h.wait("the failure", func() bool { return h.Failure("old") != "" && h.ui.said("failed to start") })
	if !strings.Contains(h.Failure("old"), "web interface") {
		t.Errorf("the failure says %q", h.Failure("old"))
	}
	// It isn't tried again until it changes.
	h.enable("old")
	if h.Running("old") {
		t.Error("the plugin that failed started again")
	}
}

func TestModules(t *testing.T) {
	h := newTestHost(t)
	h.install("parts", manifest("parts"), map[string]string{
		"index.js":     `import { greet } from "./lib/greet.js"; export default (api) => api.ui.toast(greet("modules"));`,
		"lib/greet.js": `import { exclaim } from "../exclaim.js"; export const greet = (who) => exclaim("hello " + who);`,
		"exclaim.js":   `export function exclaim(s) { return s + "!" }`,
	})
	h.install("bare", manifest("bare"), map[string]string{"index.js": `import x from "lodash"; export default () => {}`})
	h.install("thrower", manifest("thrower"), map[string]string{"index.js": `export default () => { throw new Error("no luck") }`})
	h.install("async", manifest("async"), map[string]string{"index.js": `
export default async function activate(api) {
  await null;
  return () => api.ui.toast("async stopped");
}`})
	h.enable("parts", "bare", "thrower", "async")
	h.wait("the imports", func() bool { return h.ui.said("hello modules!") })
	h.wait("the failures", func() bool { return h.Failure("bare") != "" && h.Failure("thrower") != "" })
	if !strings.Contains(h.Failure("bare"), "its own files") {
		t.Errorf("a bare import failed with %q", h.Failure("bare"))
	}
	if !strings.Contains(h.Failure("thrower"), "no luck") {
		t.Errorf("a throwing plugin failed with %q", h.Failure("thrower"))
	}
	h.wait("the async plugin", func() bool { return h.Running("async") })
	h.enable("parts")
	h.wait("the async plugin's cleanup", func() bool { return h.ui.said("async stopped") })
}

func TestPlayer(t *testing.T) {
	h := newTestHost(t)
	h.install("listen", manifest("listen"), map[string]string{"index.js": `
export default function activate(api) {
  api.player.on("trackchange", () => {
    const s = api.player.state();
    api.ui.toast("now " + s.track?.title + " at " + s.index + " of " + s.queue.length);
  });
  api.commands.register({ id: "play", title: "Play", run: () => api.player.playTracks([
    { id: "yt:1", source: "youtube", title: "Strobe", artists: [{ name: "deadmau5" }], durationMs: 1000, artwork: [] },
    { id: "yt:2", source: "youtube", title: "Teardrop", artists: [{ name: "Massive Attack" }], durationMs: 1000, artwork: [] },
  ], 1) });
  api.commands.register({ id: "bad", title: "Bad", run: () => api.player.on("nothing", () => {}) });
}`})
	h.enable("listen")
	h.wait("the commands", func() bool { return len(h.Commands()) == 2 })
	h.Commands()[0].Run()
	h.wait("the track change", func() bool { return h.ui.said("now Teardrop at 1 of 2") })
	if cur, ok := h.p.State().Current(); !ok || cur.Track.ID != "yt:2" {
		t.Errorf("the player plays %+v", cur)
	}
	// A bad call throws in the plugin, which goes on.
	h.Commands()[1].Run()
	h.Commands()[0].Run()
	h.wait("the plugin to go on", func() bool { return h.Running("listen") })
}

func TestViews(t *testing.T) {
	h := newTestHost(t)
	h.install("views", manifest("views"), map[string]string{"index.js": `
export default function activate(api) {
  const { text, button, tiles, tile, row, canvas, toggle } = api.ui;
  let clicks = 0;
  api.ui.addPage({
    name: "home",
    title: "views",
    render(view) {
      const draw = () => view.set(
        text("clicked " + clicks, { tone: "muted", size: "subtitle" }),
        row([button("more", () => { clicks++; draw() }, { primary: true }), toggle("loud", true, (on) => api.ui.toast("loud " + on))]),
        tiles([1, 2].map((n) => tile(n + " min", () => api.ui.toast("tile " + n), { tone: n }))),
        canvas({ height: 100, animate: true, draw(ctx, w, h) {
          ctx.clearRect(0, 0, w, h);
          ctx.strokeStyle = api.ui.theme().accent;
          ctx.lineWidth = 2;
          ctx.save();
          ctx.translate(w / 2, h / 2);
          ctx.beginPath();
          ctx.moveTo(0, 0);
          ctx.lineTo(10, 0);
          ctx.stroke();
          ctx.restore();
          ctx.fillRect(0, 0, 5, 5);
        } }),
        null, false,
      );
      draw();
      return () => api.ui.toast("closed");
    },
  });
}`})
	h.SetTheme(Theme{Accent: "#00bde3"})
	h.enable("views")
	h.wait("the page", func() bool { return h.Page("/plugin/views/home") != nil })
	v := h.Page("/plugin/views/home").View
	v.Mount()
	h.wait("the page to render", func() bool { return len(v.Nodes()) == 4 })
	nodes := v.Nodes()
	if nodes[0].Text != "clicked 0" || nodes[0].Tone != "muted" || nodes[1].Kind != "row" || nodes[2].Children[1].Tone != "2" || nodes[3].Kind != "canvas" {
		t.Fatalf("the page shows %+v", nodes)
	}
	v.Click(nodes[1].Children[0].Handler)
	h.wait("the click", func() bool { return v.Nodes()[0].Text == "clicked 1" })
	v.Toggle(v.Nodes()[1].Children[1].Handler, false)
	h.wait("the toggle", func() bool { return h.ui.said("loud false") })

	c := v.Canvas(0)
	c.Request(200, 100)
	h.wait("the canvas to draw", func() bool { return len(c.Frame()) == 2 })
	stroke, fill := c.Frame()[0], c.Frame()[1]
	if stroke.Fill || stroke.Color != "#00bde3" || stroke.Width != 2 || !slices.Equal(stroke.Path[0].Points, []float32{100, 50, 110, 50}) {
		t.Errorf("the stroke is %+v", stroke)
	}
	if !fill.Fill || fill.Color != "#000000" || !fill.Path[0].Closed || len(fill.Path[0].Points) != 8 {
		t.Errorf("the fill is %+v", fill)
	}
	v.Unmount()
	h.wait("the page to close", func() bool { return h.ui.said("closed") })
}

func TestTimersAndStorage(t *testing.T) {
	h := newTestHost(t)
	h.install("timers", manifest("timers"), map[string]string{"index.js": `
export default async function activate(api) {
  let n = 0;
  const id = setInterval(() => { if (++n === 3) { clearInterval(id); api.ui.toast("ticked " + n) } }, 1);
  setTimeout((what) => api.ui.toast("timed " + what), 5, "out");
  const never = setTimeout(() => api.ui.toast("never"), 10);
  clearTimeout(never);
  await api.storage.setJSON("last", { minutes: 30 });
  const last = await api.storage.getJSON("last", null);
  const missing = await api.storage.getJSON("missing", "fallback");
  api.ui.toast("stored " + last.minutes + " " + missing);
  setInterval(() => {}, 10);
}`})
	h.enable("timers")
	h.wait("the timers", func() bool { return h.ui.said("ticked 3") && h.ui.said("timed out") })
	h.wait("the storage", func() bool { return h.ui.said("stored 30 fallback") })
	time.Sleep(30 * time.Millisecond)
	if h.ui.said("never") {
		t.Error("a cleared timeout ran")
	}
	values, err := h.a.Services().Addons.Storage("timers")
	if err != nil || values["last"] != `{"minutes":30}` {
		t.Errorf("the plugin stored %q, %v", values, err)
	}
}

func TestStuckPlugin(t *testing.T) {
	stopWait = 50 * time.Millisecond
	t.Cleanup(func() { stopWait = 3 * time.Second })
	h := newTestHost(t)
	h.install("stuck", manifest("stuck"), map[string]string{"index.js": `
export default (api) => api.commands.register({ id: "spin", title: "Spin", run: () => { for (;;) {} } });`})
	h.enable("stuck")
	h.wait("the command", func() bool { return len(h.Commands()) == 1 })
	h.Commands()[0].Run()
	time.Sleep(20 * time.Millisecond)
	h.mu.Lock()
	pl := h.running["stuck"]
	h.mu.Unlock()
	h.enable()
	select {
	case <-pl.loop.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stuck plugin wasn't interrupted")
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen", r.Method+" "+r.Header.Get("X-Asked"))
		fmt.Fprint(w, `{"answer": 42}`)
	}))
	defer srv.Close()
	h := newTestHost(t)
	h.install("net", manifest("net"), map[string]string{"index.js": fmt.Sprintf(`
export default async function activate(api) {
  const res = await fetch(%q, { method: "POST", headers: { "X-Asked": "yes" }, body: "hi" });
  const body = await res.json();
  api.ui.toast(res.status + " " + res.ok + " " + res.headers.get("x-seen") + " " + body.answer);
  try { await fetch("file:///etc/passwd") } catch (e) { api.ui.toast("refused: " + e.message) }
}`, srv.URL)})
	h.enable("net")
	h.wait("the fetch", func() bool { return h.ui.said("200 true POST yes 42") })
	h.wait("the refusal", func() bool { return h.ui.said("refused: fetch reads http and https addresses only") })
}

func TestSyncUpdates(t *testing.T) {
	h := newTestHost(t)
	h.install("v", manifest("v"), map[string]string{"index.js": `export default (api) => api.ui.toast("started " + api.version)`})
	h.enable("v")
	h.wait("the first start", func() bool { return h.ui.said("started 0.5.0") })
	h.ui.mu.Lock()
	h.ui.toasts = nil
	h.ui.mu.Unlock()
	// Syncing again with the same version starts nothing.
	h.enable("v")
	time.Sleep(20 * time.Millisecond)
	if h.ui.said("started") {
		t.Fatal("the plugin started again")
	}
	h.install("v", strings.Replace(manifest("v"), "1.0.0", "1.1.0", 1), map[string]string{"index.js": `export default (api) => api.ui.toast("updated")`})
	h.enable("v")
	h.wait("the update", func() bool { return h.ui.said("updated") })
}
