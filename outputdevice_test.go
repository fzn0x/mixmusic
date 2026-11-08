package main

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestOutputChoices lists the system's default, with the device it is now,
// then each device by name, two of one name told apart, and the device
// chosen, while it isn't connected, which says so once the devices are
// listed.
func TestOutputChoices(t *testing.T) {
	devices := []app.OutputDevice{
		{ID: "a", Name: "24G2W1G4 (NVIDIA High Definition Audio)"},
		{ID: "b", Name: "24G2W1G4 (NVIDIA High Definition Audio)"},
		{ID: "hp", Name: "Headphones (Arctis Nova 7)", Default: true},
	}
	titles := func(cs []outputChoice) (out []string) {
		for _, c := range cs {
			title := c.title()
			if c.Chosen {
				title = "* " + title
			}
			out = append(out, title)
		}
		return out
	}
	for _, c := range []struct {
		name    string
		devices []app.OutputDevice
		listed  bool
		chosen  settings.OutputDevice
		want    []string
	}{
		{"the default", devices, true, settings.OutputDevice{}, []string{
			"* System default · Headphones (Arctis Nova 7)",
			"24G2W1G4 (NVIDIA High Definition Audio) · 1",
			"24G2W1G4 (NVIDIA High Definition Audio) · 2",
			"Headphones (Arctis Nova 7)",
		}},
		{"a monitor", devices, true, settings.OutputDevice{ID: "b", Name: "24G2W1G4 (NVIDIA High Definition Audio)"}, []string{
			"System default · Headphones (Arctis Nova 7)",
			"24G2W1G4 (NVIDIA High Definition Audio) · 1",
			"* 24G2W1G4 (NVIDIA High Definition Audio) · 2",
			"Headphones (Arctis Nova 7)",
		}},
		{"a DAC unplugged", devices[2:], true, settings.OutputDevice{ID: "dac", Name: "USB DAC"}, []string{
			"System default · Headphones (Arctis Nova 7)",
			"Headphones (Arctis Nova 7)",
			"* USB DAC · not connected",
		}},
		{"before the devices are listed", nil, false, settings.OutputDevice{ID: "dac", Name: "USB DAC"}, []string{
			"System default",
			"* USB DAC",
		}},
		{"a device of no name", nil, true, settings.OutputDevice{ID: "dac"}, []string{
			"System default",
			"* The device chosen · not connected",
		}},
	} {
		if got := titles(outputChoices(c.devices, c.listed, c.chosen)); !slices.Equal(got, c.want) {
			t.Errorf("%s:\n%q\nwant\n%q", c.name, got, c.want)
		}
	}
}

// outputShell is a test shell of an app whose computer has the output
// devices that devices returns. Spotify tracks play from YouTube Music, on
// every system: on Linux, a new install's Spotify Connect shows its
// devices button in the player bar too, which takes the room of the
// device button at 1280.
func outputShell(t *testing.T, devices func() []app.OutputDevice) *shell {
	t.Helper()
	s := newTestShellOf(t, func(o *app.Options) {
		o.OutputDevices = func() ([]app.OutputDevice, error) { return devices(), nil }
	})
	st := s.svc.Preferences.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackMatch
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	return s
}

// TestOutputButtonBesideSpotifyConnect gives the device button its room
// beside Spotify Connect's devices button in a wider window: at 1280, the
// two don't fit with the others, and the device button waits.
func TestOutputButtonBesideSpotifyConnect(t *testing.T) {
	s := outputShell(t, func() []app.OutputDevice {
		return []app.OutputDevice{{ID: "hp", Name: "Headphones"}, {ID: "sp", Name: "Speakers", Default: true}}
	})
	st := s.svc.Preferences.Get()
	st.SpotifyPlayback = settings.SpotifyPlaybackConnect
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt := tester(t, s)
	outputFrames(s, tt)
	if _, ok := tt.Find("Output device: System default"); ok {
		t.Error("a device button beside Spotify Connect's at 1280")
	}
	for _, label := range []string{"Spotify Connect devices", "Volume", "Open the mini player"} {
		if r, ok := tt.Find(label); !ok || r.X+r.W > 1280 {
			t.Errorf("%q is at %v, out of a window 1280 wide", label, r)
		}
	}
	tt.SetSize(1600, 900)
	settleOutput(t, s, tt)
	devices, _ := tt.Find("Spotify Connect devices")
	output, ok := tt.Find("Output device: System default")
	if !ok || output.X <= devices.X+devices.W {
		t.Errorf("the device button is at %v, Spotify Connect's at %v", output, devices)
	}
}

// settleOutput lets the player bar find its room, and the window list the
// devices, until it took the answer to its last ask, which comes beside the
// main thread.
func settleOutput(t *testing.T, s *shell, tt *ui.Tester) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		s.drain()
		tt.Frame()
		l, _ := s.loads[outputDevicesKey].(*loaded[[]app.OutputDevice])
		if l != nil && l.done && !l.again && l.answers == l.asks {
			s.drain()
			tt.Frame()
			tt.Frame()
			return
		}
		if time.Now().After(deadline) {
			if l == nil {
				t.Fatalf("the window never asked for the output devices; the player bar shows %q", tt.Texts())
			}
			t.Fatalf("the window didn't take the output devices: done %v, again %v, asks %d, answers %d, %d devices, error %v",
				l.done, l.again, l.asks, l.answers, len(l.value), l.err)
		}
	}
}

// outputFrames builds a few frames, as a window shows once its devices would
// have loaded.
func outputFrames(s *shell, tt *ui.Tester) {
	for range 5 {
		s.drain()
		tt.Frame()
		time.Sleep(time.Millisecond)
	}
}

// TestOutputButton shows the device button beside the volume, whose menu
// lists the system's default and each device, and chooses one, which the
// settings keep with its name; once the device isn't there, the menu still
// shows it, chosen.
func TestOutputButton(t *testing.T) {
	var mu sync.Mutex
	devices := []app.OutputDevice{{ID: "hp", Name: "Headphones (USB DAC)"}, {ID: "sp", Name: "Speakers", Default: true}}
	s := outputShell(t, func() []app.OutputDevice {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(devices)
	})
	tt := tester(t, s)
	settleOutput(t, s, tt)
	button, ok := tt.Find("Output device: System default")
	if !ok {
		t.Fatalf("no device button: %q", tt.Texts())
	}
	if volume, _ := tt.Find("Mute"); button.X > volume.X || volume.X-button.X > 40 {
		t.Errorf("the device button is at %v, not beside the volume at %v", button, volume)
	}
	if err := tt.Click("Output device: System default"); err != nil {
		t.Fatal(err)
	}
	if got, want := tt.Menu(), []string{"System default", "Headphones (USB DAC)", "Speakers"}; !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if err := tt.ChooseMenuItem("Headphones (USB DAC)"); err != nil {
		t.Fatal(err)
	}
	settleOutput(t, s, tt)
	if got := s.svc.Preferences.Get().OutputDevice; got != (settings.OutputDevice{ID: "hp", Name: "Headphones (USB DAC)"}) {
		t.Errorf("the settings keep %+v", got)
	}
	if !tt.HasText("Output device: Headphones (USB DAC)") {
		t.Errorf("the button doesn't name the headphones: %q", tt.Texts())
	}

	// Unplugged, the headphones stay chosen.
	mu.Lock()
	devices = devices[1:]
	mu.Unlock()
	s.refetch(outputDevicesKey)
	settleOutput(t, s, tt)
	if err := tt.Click("Output device: Headphones (USB DAC)"); err != nil {
		t.Fatal(err)
	}
	settleOutput(t, s, tt)
	if got, want := tt.Menu(), []string{"System default", "Speakers", "Headphones (USB DAC)"}; !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if err := tt.ChooseMenuItem("System default"); err != nil {
		t.Fatal(err)
	}
	settleOutput(t, s, tt)
	if got := s.svc.Preferences.Get().OutputDevice; got != (settings.OutputDevice{}) {
		t.Errorf("the settings keep %+v", got)
	}
}

// TestOutputButtonHidden shows no device button where there's no choice
// to make, one device and none chosen, nor where the player bar has no
// room for it beside the others, at the window's least width.
func TestOutputButtonHidden(t *testing.T) {
	one := outputShell(t, func() []app.OutputDevice { return []app.OutputDevice{{ID: "sp", Name: "Speakers", Default: true}} })
	tt := tester(t, one)
	settleOutput(t, one, tt)
	if _, ok := tt.Find("Output device: System default"); ok {
		t.Error("a device button with one device")
	}

	two := outputShell(t, func() []app.OutputDevice {
		return []app.OutputDevice{{ID: "hp", Name: "Headphones"}, {ID: "sp", Name: "Speakers", Default: true}}
	})
	tt = testerOf(t, two, 940, 600)
	outputFrames(two, tt)
	if _, ok := tt.Find("Output device: System default"); ok {
		t.Error("a device button at the least width")
	}
	for _, label := range []string{"Volume", "Open the mini player"} {
		if r, ok := tt.Find(label); !ok || r.X+r.W > 940 {
			t.Errorf("%q is at %v, out of a window 940 wide", label, r)
		}
	}
	// Widened, the window has room for it.
	tt.SetSize(1280, 820)
	settleOutput(t, two, tt)
	if _, ok := tt.Find("Output device: System default"); !ok {
		t.Errorf("no device button at 1280: %q", tt.Texts())
	}
}

// TestOutputButtonNarrowed shows no device button that comes with the
// devices after the window narrowed below its room, and shows it once the
// window is wide again.
func TestOutputButtonNarrowed(t *testing.T) {
	var mu sync.Mutex
	devices := []app.OutputDevice{{ID: "sp", Name: "Speakers", Default: true}}
	s := outputShell(t, func() []app.OutputDevice {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(devices)
	})
	tt := tester(t, s)
	settleOutput(t, s, tt)
	tt.SetSize(1150, 820)
	outputFrames(s, tt)
	mu.Lock()
	devices = append(devices, app.OutputDevice{ID: "hp", Name: "Headphones"})
	mu.Unlock()
	s.refetch(outputDevicesKey)
	tt.SetSize(1280, 820)
	tt.Frame()
	tt.SetSize(1150, 820)
	settleOutput(t, s, tt)
	if _, ok := tt.Find("Output device: System default"); ok {
		t.Error("a device button where the player bar has no room for it")
	}
	tt.SetSize(1280, 820)
	outputFrames(s, tt)
	if _, ok := tt.Find("Output device: System default"); !ok {
		t.Errorf("no device button in a window wide again: %q", tt.Texts())
	}
}

// TestOutputButtonKeepsTheVolume turns the volume with a touchpad across
// the device button's coming: the scroll it had before counts after.
func TestOutputButtonKeepsTheVolume(t *testing.T) {
	var mu sync.Mutex
	devices := []app.OutputDevice{{ID: "sp", Name: "Speakers", Default: true}}
	s := outputShell(t, func() []app.OutputDevice {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(devices)
	})
	s.p.PlayTracks(testTracks, 0)
	s.p.SetVolume(0.5)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Volume == 0.5 })
	tt := tester(t, s)
	settleOutput(t, s, tt)
	r, ok := tt.Find("Volume")
	if !ok {
		t.Fatalf("no volume slider: %q", tt.Texts())
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Scroll(x, y, 0, volumeScroll*0.6)
	mu.Lock()
	devices = append(devices, app.OutputDevice{ID: "hp", Name: "Headphones"})
	mu.Unlock()
	s.refetch(outputDevicesKey)
	settleOutput(t, s, tt)
	if _, ok := tt.Find("Output device: System default"); !ok {
		t.Fatalf("no device button: %q", tt.Texts())
	}
	if r2, _ := tt.Find("Volume"); r2 != r {
		t.Errorf("the volume moved from %v to %v as the device button came", r, r2)
	}
	tt.Scroll(x, y, 0, volumeScroll*0.6)
	s.settle(t, volumeNear(0.45))
}

// TestOutputDeviceSetting chooses the output device in Settings ›
// Playback › Sound, from the system's default and the devices.
func TestOutputDeviceSetting(t *testing.T) {
	s := outputShell(t, func() []app.OutputDevice {
		return []app.OutputDevice{{ID: "hp", Name: "Headphones"}, {ID: "sp", Name: "Speakers", Default: true}}
	})
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	settleOutput(t, s, tt)
	if !tt.HasText("Output device") || !tt.HasText(outputDescription) {
		t.Fatalf("no output device row: %q", tt.Texts())
	}
	click(t, s, tt, "System default · Speakers")
	click(t, s, tt, "Headphones")
	if got := s.svc.Preferences.Get().OutputDevice; got != (settings.OutputDevice{ID: "hp", Name: "Headphones"}) {
		t.Errorf("the settings keep %+v", got)
	}
}

// TestOutputDeviceFound finds the output device's row by typing in the box
// of the settings, by its words.
func TestOutputDeviceFound(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 6000)
	for _, q := range []string{"output device", "headphones", "speakers"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText("Output device") {
			t.Errorf("searching %q finds no output device: %q", q, tt.Texts())
		}
		show(s, tt, "/")
	}
}
