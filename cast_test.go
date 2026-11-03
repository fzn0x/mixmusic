package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/cast"
)

// newCastShell is a test shell of an app that finds devices on the
// network.
func newCastShell(t *testing.T, devices ...cast.Device) *shell {
	t.Helper()
	return newTestShellOf(t, func(o *app.Options) {
		o.CastBrowse = func(context.Context) ([]cast.Device, error) { return devices, nil }
	})
}

// TestCastButton shows the cast button once a cast device is on the
// network, whose menu lists the devices by name and model beside this
// computer, which plays.
func TestCastButton(t *testing.T) {
	s := newCastShell(t,
		cast.Device{ID: "tv", Name: "Living Room TV", Model: "Chromecast", Host: "192.0.2.1", Port: 8009},
		cast.Device{ID: "mini", Name: "Kitchen", Model: "Google Nest Mini", Host: "192.0.2.2", Port: 8009},
	)
	tt := tester(t, s)
	tt.Frame()
	if _, ok := tt.Find("Cast to a device"); ok {
		t.Fatal("a cast button with no device on the network")
	}
	room := s.room(1920, 1080)

	s.svc.Player.FindCastDevices()
	for deadline := time.Now().Add(5 * time.Second); len(s.svc.Player.CastDevices()) == 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the devices weren't found")
		}
	}
	tt.Frame()
	if _, ok := tt.Find("Cast to a device"); !ok {
		t.Fatal("no cast button with devices on the network")
	}
	if s.room(1920, 1080) >= room {
		t.Error("the cast button takes no room")
	}
	if err := tt.Click("Cast to a device"); err != nil {
		t.Fatal(err)
	}
	want := []string{"This computer", "Kitchen · Google Nest Mini", "Living Room TV · Chromecast"}
	if got := tt.Menu(); !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
}
