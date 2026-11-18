package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/fcast"
	"github.com/fzn0x/mixmusic/internal/upnp"
	"github.com/fzn0x/mixmusic/locales"
)

// newFarShell is a test shell of an app that finds a Google Cast device,
// two DLNA renderers, one that names its model and one that doesn't, and
// an FCast receiver, without the network.
func newFarShell(t *testing.T) *shell {
	t.Helper()
	return newTestShellOf(t, func(o *app.Options) {
		o.CastBrowse = func(context.Context) ([]cast.Device, error) {
			return []cast.Device{{ID: "mini", Name: "Kitchen", Model: "Google Nest Mini", Host: "192.0.2.2", Port: 8009}}, nil
		}
		o.RendererSearch = func(context.Context) ([]upnp.Renderer, error) {
			return []upnp.Renderer{{ID: "uuid:tv", Name: "Living Room TV", Model: "Bravia", Host: "192.0.2.3"},
				{ID: "uuid:amp", Name: "Study", Host: "192.0.2.4"}}, nil
		}
		o.FCastBrowse = func(context.Context) ([]fcast.Receiver, error) {
			return []fcast.Receiver{{ID: "fcast:key", Name: "Bedroom TV", Host: "192.0.2.5", Port: 46899, Version: 4}}, nil
		}
	})
}

// openCastMenu finds the devices and opens the cast button's menu.
func openCastMenu(t *testing.T, s *shell, tt *ui.Tester) []string {
	t.Helper()
	s.svc.Player.FindCastDevices()
	for deadline := time.Now().Add(5 * time.Second); len(s.svc.Player.CastDevices()) < 4; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("found %+v", s.svc.Player.CastDevices())
		}
	}
	tt.Frame()
	if err := tt.Click(locales.CastToDevice()); err != nil {
		t.Fatal(err)
	}
	return tt.Menu()
}

// TestCastMenuRenderers lists the DLNA renderers and FCast receivers
// beside the Google Cast devices, by name, each with its model, or what it
// is where it names none.
func TestCastMenuRenderers(t *testing.T) {
	s := newFarShell(t)
	want := []string{"This computer", "Bedroom TV · FCast receiver", "Kitchen · Google Nest Mini", "Living Room TV · Bravia",
		"Study · DLNA renderer", "-", "Join a room…"}
	if got := openCastMenu(t, s, tester(t, s)); !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	kinds := map[string]app.CastKind{}
	for _, d := range s.svc.Player.CastDevices() {
		kinds[d.Name] = d.Kind
	}
	if kinds["Kitchen"] != app.CastGoogle || kinds["Study"] != app.CastRenderer || kinds["Bedroom TV"] != app.CastFCast {
		t.Errorf("kinds %v", kinds)
	}
}

// TestPseudoLocaleCastMenu lists the renderers and receivers in en-XA:
// what each is, where it names no model, is the language's.
func TestPseudoLocaleCastMenu(t *testing.T) {
	usePseudoLocale(t)
	s := newFarShell(t)
	tt := tester(t, s)
	tt.SetLocale("en-XA")
	menu := slices.DeleteFunc(openCastMenu(t, s, tt), func(item string) bool { return item == "-" })
	checkPseudo(t, "The cast menu", menu, "Bedroom TV", "Kitchen", "Google Nest Mini", "Living Room TV", "Bravia", "Study")
	xa := locales.In("en-XA")
	if !slices.Contains(menu, "Study · "+xa.CastRendererModel()) || !slices.Contains(menu, "Bedroom TV · "+xa.CastReceiverModel()) {
		t.Errorf("menu %q", menu)
	}
	// The words casting tells as it plays on these, in the language.
	for _, said := range []string{xa.CastRendererFormat("Study", "FLAC"), xa.CastReceiverKey(), xa.CastReceiverNoKey(),
		xa.CastReceiverOlder(), xa.SpeedMenuLockedCast("Study")} {
		checkPseudo(t, "Casting's words", []string{said}, "Study", "FLAC")
	}
}
