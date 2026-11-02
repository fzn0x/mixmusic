package plugins

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/grafana/sobek"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

//go:embed api.js
var apiJS string

// jsFunc is a function of a plugin's, which only its loop calls.
type jsFunc = sobek.Callable

// asyncTimeout bounds the work of a call of the API: a search, a request.
const asyncTimeout = time.Minute

// none is a result of the API's work that JavaScript sees as undefined.
type none struct{}

// plugin is a plugin that runs. Its fields belong to its loop, but where
// they say otherwise.
type plugin struct {
	h                 *Host
	id, name, version string
	loop              *loop
	// runtime is the plugin's runtime, for stopping it from another
	// goroutine; ctx ends the work of its calls when it stops.
	runtime atomic.Pointer[sobek.Runtime]
	ctx     context.Context
	cancel  context.CancelFunc

	vm               *sobek.Runtime
	parse, stringify jsFunc
	freeze           jsFunc
	cleanups         []func()
	timers           map[int]*time.Timer
	nextID           int
	start            time.Time
	listeners        map[player.Event][]*listener
	unsubscribe      func()
	analyser         sobek.Value
	views            []*View
	stopping         bool
	// wants tells, beside the loop, which events of the player the
	// plugin listens to; progressing is set while a progress event waits
	// on the loop.
	wants       [4]atomic.Bool
	progressing atomic.Bool
}

type listener struct{ fn jsFunc }

func newPlugin(h *Host, p addons.Plugin) *plugin {
	ctx, cancel := context.WithCancel(context.Background())
	return &plugin{h: h, id: p.ID, name: p.Name, version: p.Version, loop: newLoop(), ctx: ctx, cancel: cancel,
		timers: map[int]*time.Timer{}, listeners: map[player.Event][]*listener{}, start: time.Now()}
}

// activate loads the plugin's module and calls its default export with
// the API.
func (pl *plugin) activate(p addons.Plugin) error {
	if p.API != addons.PluginAPI {
		return errors.New("it was made for the web interface MixMusic had before: its author can update it for MixMusic's own interface (\"api\": 2)")
	}
	files, err := pl.h.a.AddonFiles(addons.KindPlugin, p.ID)
	if err != nil {
		return err
	}
	vm := sobek.New()
	pl.vm = vm
	pl.runtime.Store(vm)
	pl.globals()
	mod, err := pl.load(files, p.Main)
	if err != nil {
		return err
	}
	ns := vm.NamespaceObjectFor(mod)
	activate, ok := sobek.AssertFunction(ns.Get("default"))
	if !ok {
		if activate, ok = sobek.AssertFunction(ns.Get("activate")); !ok {
			return errors.New("its module exports no activate function")
		}
	}
	api, err := pl.api()
	if err != nil {
		return err
	}
	result, err := activate(sobek.Undefined(), api)
	if err != nil {
		return jsError(err)
	}
	pl.settle(result)
	return nil
}

// settle keeps what activate returned: a function MixMusic calls when it
// stops the plugin, or a promise of one.
func (pl *plugin) settle(result sobek.Value) {
	if fn, ok := sobek.AssertFunction(result); ok {
		pl.cleanups = append(pl.cleanups, func() { pl.call(fn) })
		return
	}
	promise, ok := result.Export().(*sobek.Promise)
	if !ok {
		return
	}
	then, _ := sobek.AssertFunction(result.ToObject(pl.vm).Get("then"))
	fulfilled := pl.vm.ToValue(func(call sobek.FunctionCall) sobek.Value {
		pl.settle(call.Argument(0))
		return sobek.Undefined()
	})
	rejected := pl.vm.ToValue(func(call sobek.FunctionCall) sobek.Value {
		pl.h.fail(pl, jsValueError(call.Argument(0)))
		return sobek.Undefined()
	})
	if then != nil && promise != nil {
		then(result, fulfilled, rejected)
	}
}

// load parses, links and evaluates the plugin's main module, which may
// import other files of its folder.
func (pl *plugin) load(files fs.FS, main string) (sobek.ModuleRecord, error) {
	records := map[string]sobek.ModuleRecord{}
	paths := map[sobek.ModuleRecord]string{}
	var resolve sobek.HostResolveImportedModuleFunc
	resolve = func(ref interface{}, specifier string) (sobek.ModuleRecord, error) {
		base := "."
		if m, ok := ref.(sobek.ModuleRecord); ok {
			base = path.Dir(paths[m])
			if !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") {
				return nil, fmt.Errorf("it imports %q: a plugin imports its own files only, as \"./file.js\"", specifier)
			}
		}
		name := path.Clean(path.Join(base, specifier))
		if !fs.ValidPath(name) {
			return nil, fmt.Errorf("it imports %q, outside its folder", specifier)
		}
		if m, ok := records[name]; ok {
			return m, nil
		}
		src, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		m, err := sobek.ParseModule(name, string(src), resolve)
		if err != nil {
			return nil, err
		}
		records[name], paths[m] = m, name
		return m, nil
	}
	m, err := resolve(nil, main)
	if err != nil {
		return nil, err
	}
	cyclic, ok := m.(sobek.CyclicModuleRecord)
	if !ok {
		return nil, errors.New("its module can't be evaluated")
	}
	if err := cyclic.Link(); err != nil {
		return nil, err
	}
	promise := pl.vm.CyclicModuleRecordEvaluate(cyclic, resolve)
	switch promise.State() {
	case sobek.PromiseStateRejected:
		return nil, jsValueError(promise.Result())
	case sobek.PromiseStatePending:
		return nil, errors.New("its module awaits at its top level, which plugins can't")
	}
	return m, nil
}

// globals gives the runtime what plugins expect of JavaScript beyond the
// language: console, timers, performance.now, queueMicrotask and fetch.
func (pl *plugin) globals() {
	vm := pl.vm
	json := vm.Get("JSON").ToObject(vm)
	pl.parse, _ = sobek.AssertFunction(json.Get("parse"))
	pl.stringify, _ = sobek.AssertFunction(json.Get("stringify"))
	pl.freeze, _ = sobek.AssertFunction(vm.Get("Object").ToObject(vm).Get("freeze"))

	console := vm.NewObject()
	for _, level := range []string{"log", "info", "warn", "error", "debug"} {
		console.Set(level, func(call sobek.FunctionCall) sobek.Value {
			pl.log(level, call.Arguments)
			return sobek.Undefined()
		})
	}
	vm.Set("console", console)
	vm.Set("setTimeout", func(call sobek.FunctionCall) sobek.Value { return pl.timer(call, false) })
	vm.Set("setInterval", func(call sobek.FunctionCall) sobek.Value { return pl.timer(call, true) })
	vm.Set("clearTimeout", pl.clearTimer)
	vm.Set("clearInterval", pl.clearTimer)
	performance := vm.NewObject()
	performance.Set("now", func() float64 { return float64(time.Since(pl.start).Microseconds()) / 1000 })
	vm.Set("performance", performance)
	vm.Set("fetch", pl.fetch)
	if _, err := vm.RunString(`globalThis.queueMicrotask = (fn) => void Promise.resolve().then(fn);`); err != nil {
		panic(err)
	}
}

// log writes what the plugin logs to MixMusic's log.
func (pl *plugin) log(level string, args []sobek.Value) {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = pl.describe(a)
	}
	log.Printf("mixmusic: plugin %s: %s%s", pl.id, prefix(level), strings.Join(parts, " "))
}

func prefix(level string) string {
	if level == "warn" || level == "error" {
		return level + ": "
	}
	return ""
}

// describe writes a value as console.log does, more or less: strings as
// they are, objects as JSON.
func (pl *plugin) describe(v sobek.Value) string {
	if v == nil || sobek.IsUndefined(v) {
		return "undefined"
	}
	if s, ok := v.Export().(string); ok {
		return s
	}
	if obj, ok := v.(*sobek.Object); ok {
		if _, isFunc := sobek.AssertFunction(obj); isFunc {
			return "[function]"
		}
		if errorLike(obj) {
			return obj.String()
		}
		if out, err := pl.stringify(sobek.Undefined(), v); err == nil && !sobek.IsUndefined(out) {
			return out.String()
		}
	}
	return v.String()
}

func errorLike(obj *sobek.Object) bool {
	name := obj.Get("name")
	return name != nil && strings.HasSuffix(name.String(), "Error") && obj.Get("message") != nil
}

// timer starts a timeout or an interval, which runs its callback on the
// loop.
func (pl *plugin) timer(call sobek.FunctionCall, repeat bool) sobek.Value {
	fn, ok := sobek.AssertFunction(call.Argument(0))
	if !ok {
		panic(pl.vm.NewTypeError("the timer's callback is not a function"))
	}
	ms := call.Argument(1).ToFloat()
	if math.IsNaN(ms) || ms < 0 {
		ms = 0
	}
	d := time.Duration(ms * float64(time.Millisecond))
	if repeat {
		d = max(d, 10*time.Millisecond)
	}
	var args []sobek.Value
	if len(call.Arguments) > 2 {
		args = append(args, call.Arguments[2:]...)
	}
	pl.nextID++
	id := pl.nextID
	var fire func()
	fire = func() {
		pl.loop.post(func() {
			if _, ok := pl.timers[id]; !ok || pl.stopping {
				return
			}
			if repeat {
				pl.timers[id] = time.AfterFunc(d, fire)
			} else {
				delete(pl.timers, id)
			}
			pl.call(fn, args...)
		})
	}
	pl.timers[id] = time.AfterFunc(d, fire)
	return pl.vm.ToValue(id)
}

func (pl *plugin) clearTimer(call sobek.FunctionCall) sobek.Value {
	id := int(call.Argument(0).ToInteger())
	if t, ok := pl.timers[id]; ok {
		t.Stop()
		delete(pl.timers, id)
	}
	return sobek.Undefined()
}

// call calls a function of the plugin's and returns what it gives, but
// for what it throws, which it logs.
func (pl *plugin) call(fn jsFunc, args ...sobek.Value) sobek.Value {
	if fn == nil || pl.stopping {
		return sobek.Undefined()
	}
	v, err := fn(sobek.Undefined(), args...)
	if err != nil {
		log.Printf("mixmusic: plugin %s: %v", pl.id, err)
		return sobek.Undefined()
	}
	return v
}

// shutdown runs on the loop as the plugin stops: its views and listeners
// go, then its cleanups run, the last first.
func (pl *plugin) shutdown() {
	for _, v := range pl.views {
		v.unmount()
	}
	pl.views = nil
	if pl.unsubscribe != nil {
		pl.unsubscribe()
		pl.unsubscribe = nil
	}
	for i := len(pl.cleanups) - 1; i >= 0; i-- {
		pl.cleanups[i]()
	}
	pl.cleanups = nil
	for id, t := range pl.timers {
		t.Stop()
		delete(pl.timers, id)
	}
	pl.stopping = true
}

// fromGo turns a value of Go into JavaScript's, through JSON: tracks,
// albums and the like, as the plugin API names their fields.
func (pl *plugin) fromGo(v any) sobek.Value {
	if _, ok := v.(none); ok {
		return sobek.Undefined()
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic(pl.vm.NewGoError(err))
	}
	out, err := pl.parse(sobek.Undefined(), pl.vm.ToValue(string(data)))
	if err != nil {
		panic(err)
	}
	return out
}

// toGo turns a value of JavaScript into Go's, through JSON.
func (pl *plugin) toGo(v sobek.Value, out any) {
	s, err := pl.stringify(sobek.Undefined(), v)
	if err != nil {
		panic(err)
	}
	if s == nil || sobek.IsUndefined(s) {
		return
	}
	if err := json.Unmarshal([]byte(s.String()), out); err != nil {
		panic(pl.vm.NewTypeError(fmt.Sprintf("not what MixMusic takes: %v", err)))
	}
}

// async does work beside the loop, and returns a promise that settles on
// the loop with what it gives, turned into JavaScript by fromGo, or by
// convert when given.
func (pl *plugin) async(work func(ctx context.Context) (any, error), convert ...func(any) sobek.Value) sobek.Value {
	promise, resolve, reject := pl.vm.NewPromise()
	go func() {
		ctx, cancel := context.WithTimeout(pl.ctx, asyncTimeout)
		v, err := work(ctx)
		cancel()
		pl.loop.post(func() {
			if pl.stopping {
				return
			}
			if err != nil {
				reject(pl.vm.NewGoError(err))
				return
			}
			if len(convert) > 0 {
				resolve(convert[0](v))
				return
			}
			resolve(pl.fromGo(v))
		})
	}()
	return pl.vm.ToValue(promise)
}

// events are the events of the player plugins listen to, by name.
var events = map[string]player.Event{
	"trackchange": player.TrackChange,
	"statechange": player.StateChange,
	"progress":    player.Progress,
	"queuechange": player.QueueChange,
}

// on has fn called on an event of the player, until the returned function
// is called.
func (pl *plugin) on(event string, fn jsFunc) func() {
	e, ok := events[event]
	if !ok {
		panic(pl.vm.NewTypeError(fmt.Sprintf("the player has no event %q: it has trackchange, statechange, progress and queuechange", event)))
	}
	l := &listener{fn}
	pl.listeners[e] = append(pl.listeners[e], l)
	pl.wants[e].Store(true)
	if pl.unsubscribe == nil && pl.h.p != nil {
		pl.unsubscribe = pl.h.p.Subscribe(pl.heard)
	}
	return func() {
		ls := pl.listeners[e]
		for i, x := range ls {
			if x == l {
				pl.listeners[e] = append(ls[:i:i], ls[i+1:]...)
				break
			}
		}
		pl.wants[e].Store(len(pl.listeners[e]) > 0)
	}
}

// heard passes an event of the player to the plugin's loop, on the
// player's goroutine. Progress, which comes often, waits on the loop once
// at most.
func (pl *plugin) heard(e player.Event) {
	if int(e) >= len(pl.wants) || !pl.wants[e].Load() {
		return
	}
	if e == player.Progress && !pl.progressing.CompareAndSwap(false, true) {
		return
	}
	pl.loop.post(func() {
		if e == player.Progress {
			pl.progressing.Store(false)
		}
		for _, l := range append([]*listener(nil), pl.listeners[e]...) {
			pl.call(l.fn)
		}
	})
}

// snapshot is what plugins see of the player.
type snapshot struct {
	Track      *media.Track  `json:"track"`
	Playing    bool          `json:"playing"`
	PositionMs int64         `json:"positionMs"`
	DurationMs int64         `json:"durationMs"`
	Volume     float64       `json:"volume"`
	Shuffle    bool          `json:"shuffle"`
	Repeat     player.Repeat `json:"repeat"`
	Queue      []media.Track `json:"queue"`
	Index      int           `json:"index"`
	// Speed is the speed the track plays at, 1 its own.
	Speed float64 `json:"speed"`
}

func (pl *plugin) playerState() snapshot {
	st := pl.h.p.State()
	s := snapshot{Playing: st.Playing, PositionMs: st.Position.Milliseconds(), DurationMs: st.Duration.Milliseconds(),
		Volume: st.Volume, Shuffle: st.Shuffle, Repeat: st.Repeat, Queue: make([]media.Track, len(st.Queue)), Index: st.Index,
		Speed: st.Speed}
	if s.Speed <= 0 {
		s.Speed = 1
	}
	for i, it := range st.Queue {
		s.Queue[i] = it.Track
	}
	if cur, ok := st.Current(); ok {
		t := cur.Track
		s.Track = &t
		if s.DurationMs <= 0 {
			s.DurationMs = int64(t.DurationMs)
		}
	} else {
		s.Index = -1
	}
	return s
}

// jsError is what a call into JavaScript threw, as an error worth telling:
// "Error: message", without the stack.
func jsError(err error) error {
	var exc *sobek.Exception
	if errors.As(err, &exc) {
		log.Printf("mixmusic: plugin: %v", err)
		return jsValueError(exc.Value())
	}
	return err
}

func jsValueError(v sobek.Value) error {
	if v == nil || sobek.IsUndefined(v) {
		return errors.New("it failed")
	}
	if err, ok := v.Export().(error); ok {
		return err
	}
	return errors.New(v.String())
}
