package plugins

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/grafana/sobek"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/volume"
)

// nameRe matches the names of pages, which are parts of their paths, and
// the IDs of what plugins add.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// api makes the API the plugin's activate function gets: api.js over the
// natives of the Go side.
func (pl *plugin) api() (sobek.Value, error) {
	build, err := pl.vm.RunScript("api.js", apiJS)
	if err != nil {
		return nil, err
	}
	fn, ok := sobek.AssertFunction(build)
	if !ok {
		return nil, fmt.Errorf("api.js is not a function")
	}
	return fn(sobek.Undefined(), pl.natives())
}

// natives are the functions of the Go side the API is built over. They
// run on the loop.
func (pl *plugin) natives() *sobek.Object {
	vm, h := pl.vm, pl.h
	svc := h.a.Services()
	n := vm.NewObject()
	set := func(name string, fn any) {
		if err := n.Set(name, fn); err != nil {
			panic(err)
		}
	}
	tracks := func(v sobek.Value) []media.Track {
		var out []media.Track
		pl.toGo(v, &out)
		return out
	}
	track := func(v sobek.Value) media.Track {
		var out media.Track
		pl.toGo(v, &out)
		if out.ID == "" {
			panic(vm.NewTypeError("not a track"))
		}
		return out
	}
	function := func(v sobek.Value, what string) jsFunc {
		fn, ok := sobek.AssertFunction(v)
		if !ok {
			panic(vm.NewTypeError(what + " is not a function"))
		}
		return fn
	}
	name := func(v, what string) {
		if !nameRe.MatchString(v) {
			panic(vm.NewTypeError(fmt.Sprintf("%s %q is not lowercase letters, digits and dashes", what, v)))
		}
	}
	remover := func(remove func()) sobek.Value {
		return vm.ToValue(func(sobek.FunctionCall) sobek.Value {
			remove()
			return sobek.Undefined()
		})
	}

	set("version", svc.System.Info().Version)
	set("plugin", pl.id)

	// The player.
	set("playerState", func() sobek.Value { return pl.fromGo(pl.playerState()) })
	set("play", func() { h.p.Play() })
	set("pause", func() { h.p.Pause() })
	set("toggle", func() { h.p.Toggle() })
	set("next", func() { h.p.Next() })
	set("previous", func() { h.p.Previous() })
	set("seek", func(ms float64) { h.p.Seek(time.Duration(max(ms, 0) * float64(time.Millisecond))) })
	set("setVolume", func(v float64) { h.p.SetVolume(min(max(v, 0), 1)) })
	set("playTracks", func(call sobek.FunctionCall) sobek.Value {
		h.p.PlayTracks(tracks(call.Argument(0)), int(call.Argument(1).ToInteger()))
		return sobek.Undefined()
	})
	set("playNext", func(v sobek.Value) { h.p.PlayNext(tracks(v)) })
	set("enqueue", func(v sobek.Value) { h.p.Enqueue(tracks(v)) })
	set("on", func(call sobek.FunctionCall) sobek.Value {
		return remover(pl.on(call.Argument(0).String(), function(call.Argument(1), "the listener")))
	})
	set("analyser", func() sobek.Value { return pl.analyserObject() })

	// The catalog.
	set("search", func(q string) sobek.Value {
		return pl.async(func(ctx context.Context) (any, error) { return svc.Catalog.Search(ctx, q) })
	})
	set("album", func(id string) sobek.Value {
		return pl.async(func(ctx context.Context) (any, error) { return svc.Catalog.Album(ctx, id) })
	})
	set("artist", func(id string) sobek.Value {
		return pl.async(func(ctx context.Context) (any, error) { return svc.Catalog.Artist(ctx, id) })
	})
	set("playlist", func(id string) sobek.Value {
		return pl.async(func(ctx context.Context) (any, error) { return svc.Catalog.Playlist(ctx, id) })
	})
	set("radio", func(v sobek.Value) sobek.Value {
		t := track(v)
		return pl.async(func(ctx context.Context) (any, error) { return svc.Catalog.Radio(ctx, t) })
	})
	set("lyrics", func(v sobek.Value) sobek.Value {
		t := track(v)
		return pl.async(func(ctx context.Context) (any, error) { return svc.Catalog.Lyrics(ctx, t) })
	})

	// The library. The player hears of a dislike at once, as the
	// interface's own buttons tell it: liking a disliked track takes the
	// dislike back.
	set("isLiked", func(id string) bool { return h.a.IsLiked(id) })
	set("setLiked", func(v sobek.Value, liked bool) sobek.Value {
		t := track(v)
		if liked && h.a.IsDisliked(t.ID) {
			h.p.Disliked(t.ID, false)
		}
		return pl.async(func(ctx context.Context) (any, error) { return none{}, svc.Collection.SetLiked(ctx, t, liked) })
	})
	set("isDisliked", func(id string) bool { return h.a.IsDisliked(id) })
	set("setDisliked", func(v sobek.Value, disliked bool) sobek.Value {
		t := track(v)
		h.p.Disliked(t.ID, disliked)
		return pl.async(func(ctx context.Context) (any, error) {
			err := svc.Collection.SetDisliked(ctx, t, disliked)
			if err != nil {
				// The library keeps what it had: the player too.
				h.p.Disliked(t.ID, !disliked)
			}
			return none{}, err
		})
	})
	set("mixes", func() sobek.Value { return pl.fromGo(svc.Collection.Playlists()) })
	set("createMix", func(title string, v sobek.Value) sobek.Value {
		ts := tracks(v)
		return pl.async(func(context.Context) (any, error) { return svc.Collection.CreatePlaylist(title, "", ts) })
	})

	// The interface.
	set("toast", func(message, tone string) { h.toast(message, tone) })
	set("navigate", func(path string) { h.navigate(path) })
	set("theme", func() sobek.Value { return pl.fromGo(themeJSON(h.currentTheme())) })
	set("addSidebarItem", func(id, label, icon, page string) sobek.Value {
		name(id, "the item's id")
		name(page, "the page")
		return remover(add(h, &h.items, &SidebarItem{Plugin: pl.id, ID: pl.id + "." + id, Label: label, Icon: icon,
			Path: "/plugin/" + pl.id + "/" + page}))
	})
	set("addPage", func(call sobek.FunctionCall) sobek.Value {
		page := call.Argument(0).String()
		name(page, "the page")
		v := pl.newView(function(call.Argument(2), "the page's render"))
		return remover(add(h, &h.pages, &Page{Plugin: pl.id, Name: page, Title: call.Argument(1).String(),
			Path: "/plugin/" + pl.id + "/" + page, View: v}))
	})
	set("addWidget", func(call sobek.FunctionCall) sobek.Value {
		id := call.Argument(0).String()
		name(id, "the widget's id")
		v := pl.newView(function(call.Argument(2), "the widget's render"))
		return remover(add(h, &h.widgets, &Widget{Plugin: pl.id, ID: pl.id + "." + id, Title: call.Argument(1).String(), View: v}))
	})
	set("addTrackAction", func(call sobek.FunctionCall) sobek.Value {
		id := call.Argument(0).String()
		name(id, "the action's id")
		return remover(add(h, &h.actions, &TrackAction{Plugin: pl.id, ID: pl.id + "." + id, Label: call.Argument(1).String(),
			pl: pl, run: function(call.Argument(2), "the action's run")}))
	})
	set("registerCommand", func(call sobek.FunctionCall) sobek.Value {
		id := call.Argument(0).String()
		name(id, "the command's id")
		return remover(add(h, &h.commands, &Command{Plugin: pl.id, ID: pl.id + "." + id, Title: call.Argument(1).String(),
			Shortcut: call.Argument(2).String(), pl: pl, run: function(call.Argument(3), "the command's run")}))
	})

	// Settings, storage and the desktop.
	set("settings", func() sobek.Value {
		// The volume the player's state gives, not the number
		// settings.json keeps.
		st := svc.Preferences.Get()
		st.Volume = volume.Of(st.Volume)
		v, err := pl.freeze(sobek.Undefined(), pl.fromGo(st))
		if err != nil {
			panic(err)
		}
		return v
	})
	set("storageGet", func(key string) sobek.Value {
		return pl.async(func(context.Context) (any, error) {
			values, err := svc.Addons.Storage(pl.id)
			if v, ok := values[key]; ok && err == nil {
				return v, nil
			}
			return none{}, err
		})
	})
	set("storageSet", func(key, value string) sobek.Value {
		return pl.async(func(context.Context) (any, error) { return none{}, svc.Addons.StorageSet(pl.id, key, value) })
	})
	set("storageDelete", func(key string) sobek.Value {
		return pl.async(func(context.Context) (any, error) { return none{}, svc.Addons.StorageDelete(pl.id, key) })
	})
	set("notify", func(title, body string) sobek.Value {
		return pl.async(func(context.Context) (any, error) { return none{}, svc.System.Notify(title, body) })
	})
	set("openExternal", func(url string) sobek.Value {
		return pl.async(func(context.Context) (any, error) { return none{}, svc.System.OpenExternal(url) })
	})
	return n
}

// themeJSON is the look plugins draw in, as api.ui.theme gives it.
func themeJSON(t Theme) map[string]any {
	return map[string]any{
		"accent": t.Accent, "accentText": t.AccentText, "text": t.Text, "muted": t.Muted,
		"background": t.Background, "surface": t.Surface, "tiles": t.Tiles, "tileText": t.TileText, "dark": t.Dark,
	}
}

// analyserObject is the analyser of MixMusic's own player, for
// visualizers: an object with the reading methods and properties of Web
// Audio's AnalyserNode, of fftSize 256.
func (pl *plugin) analyserObject() sobek.Value {
	if pl.analyser != nil {
		return pl.analyser
	}
	vm := pl.vm
	svc := pl.h.a.Services()
	o := vm.NewObject()
	o.Set("fftSize", 256)
	o.Set("frequencyBinCount", 128)
	o.Set("minDecibels", -100)
	o.Set("maxDecibels", -30)
	o.Set("smoothingTimeConstant", 0.8)
	o.Set("getByteFrequencyData", func(v sobek.Value) {
		if b, ok := v.Export().([]byte); ok {
			for i, x := range svc.Player.NativeAnalyser(false, false).Bytes {
				if i >= len(b) {
					break
				}
				b[i] = byte(x)
			}
		}
	})
	o.Set("getFloatFrequencyData", func(v sobek.Value) {
		if f, ok := v.Export().([]float32); ok {
			for i, x := range svc.Player.NativeAnalyser(true, false).Decibels {
				if i >= len(f) {
					break
				}
				f[i] = float32(x)
			}
		}
	})
	o.Set("getByteTimeDomainData", func(v sobek.Value) {
		if b, ok := v.Export().([]byte); ok {
			for i, x := range svc.Player.NativeAnalyser(false, true).Waveform {
				if i >= len(b) {
					break
				}
				b[i] = byte(min(max(128*(1+x), 0), 255))
			}
		}
	})
	o.Set("getFloatTimeDomainData", func(v sobek.Value) {
		if f, ok := v.Export().([]float32); ok {
			for i, x := range svc.Player.NativeAnalyser(false, true).Waveform {
				if i >= len(f) {
					break
				}
				f[i] = float32(x)
			}
		}
	})
	pl.analyser = o
	return o
}
