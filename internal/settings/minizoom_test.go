package settings

import "testing"

// TestMiniZoom keeps the zoom the mini player's window was sized for: none
// in new installs and in the settings of earlier versions, the nearest
// step of the zoom for one between them, and none for one outside them
// (#717).
func TestMiniZoom(t *testing.T) {
	for file, want := range map[string]float64{
		`{}`:                 0,
		`{"miniPanel":""}`:   0,
		`{"miniZoom":1}`:     1,
		`{"miniZoom":1.25}`:  1.25,
		`{"miniZoom":1.2}`:   1.25,
		`{"miniZoom":0}`:     0,
		`{"miniZoom":-1}`:    0,
		`{"miniZoom":3}`:     0,
		`{"miniZoom":0.001}`: 0,
	} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Clean().MiniZoom; got != want {
			t.Errorf("%s keeps the mini player's zoom %v, not %v", file, got, want)
		}
	}
	if Default().MiniZoom != 0 {
		t.Error("a new install says the mini player's window was sized for a zoom")
	}
}
