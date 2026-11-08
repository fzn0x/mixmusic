package main

import (
	"slices"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
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
// devices that devices returns.
func outputShell(t *testing.T, devices func() []app.OutputDevice) *shell {
	t.Helper()
	return newTestShellOf(t, func(o *app.Options) {
		o.OutputDevices = func() ([]app.OutputDevice, error) { return devices(), nil }
	})
}

// settleOutput lets the window list the devices, and the player bar find
// its room.
func settleOutput(s *shell, tt *ui.Tester) {
	for range 3 {
		s.drain()
		tt.Frame()
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
	settleOutput(s, tt)
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
	settleOutput(s, tt)
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
	settleOutput(s, tt)
	if err := tt.Click("Output device: Headphones (USB DAC)"); err != nil {
		t.Fatal(err)
	}
	settleOutput(s, tt)
	if got, want := tt.Menu(), []string{"System default", "Speakers", "Headphones (USB DAC)"}; !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if err := tt.ChooseMenuItem("System default"); err != nil {
		t.Fatal(err)
	}
	settleOutput(s, tt)
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
	settleOutput(one, tt)
	if _, ok := tt.Find("Output device: System default"); ok {
		t.Error("a device button with one device")
	}

	two := outputShell(t, func() []app.OutputDevice {
		return []app.OutputDevice{{ID: "hp", Name: "Headphones"}, {ID: "sp", Name: "Speakers", Default: true}}
	})
	tt = testerOf(t, two, 940, 600)
	settleOutput(two, tt)
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
	settleOutput(two, tt)
	if _, ok := tt.Find("Output device: System default"); !ok {
		t.Errorf("no device button at 1280: %q", tt.Texts())
	}
}

// TestOutputDeviceSetting chooses the output device in Settings ›
// Playback › Sound, from the system's default and the devices.
func TestOutputDeviceSetting(t *testing.T) {
	s := outputShell(t, func() []app.OutputDevice {
		return []app.OutputDevice{{ID: "hp", Name: "Headphones"}, {ID: "sp", Name: "Speakers", Default: true}}
	})
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	settleOutput(s, tt)
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
