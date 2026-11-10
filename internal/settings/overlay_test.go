package settings

import "testing"

// TestOverlay serves nothing in new installs and in the settings of
// earlier versions, and keeps a port that can be listened on, and the
// default for another (#344).
func TestOverlay(t *testing.T) {
	if d := Default(); d.OverlayServer || d.OverlayPort != DefaultOverlayPort {
		t.Errorf("a new install has %v on port %d", d.OverlayServer, d.OverlayPort)
	}
	for file, want := range map[string]int{
		`{}`:                    DefaultOverlayPort,
		`{"overlayPort":0}`:     DefaultOverlayPort,
		`{"overlayPort":80}`:    DefaultOverlayPort,
		`{"overlayPort":1024}`:  1024,
		`{"overlayPort":9000}`:  9000,
		`{"overlayPort":65535}`: 65535,
		`{"overlayPort":65536}`: DefaultOverlayPort,
		`{"overlayPort":-5}`:    DefaultOverlayPort,
	} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Clean().OverlayPort; got != want {
			t.Errorf("%s has port %d, not %d", file, got, want)
		}
	}
}
