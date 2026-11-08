package settings

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/volume"
)

// TestVolumeCurve plays the volume on the Natural curve in new installs,
// in the settings of earlier versions, which don't have the key, and for
// a curve MixMusic doesn't know, until the user chooses Linear (#383). The
// volume stays the position it was, whatever the curve.
func TestVolumeCurve(t *testing.T) {
	for file, want := range map[string]volume.Curve{
		`{}`:                         volume.Natural,
		`{"volume":0.5}`:             volume.Natural,
		`{"volumeCurve":"natural"}`:  volume.Natural,
		`{"volumeCurve":"linear"}`:   volume.Linear,
		`{"volumeCurve":"LINEAR"}`:   volume.Natural,
		`{"volumeCurve":"decibels"}`: volume.Natural,
		`{"volumeCurve":""}`:         volume.Natural,
	} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		if got := s.Clean().VolumeCurve; got != want {
			t.Errorf("%s plays the volume on %q, not %q", file, got, want)
		}
	}
	if c := Default().VolumeCurve; c != volume.Natural {
		t.Errorf("a new install plays the volume on %q", c)
	}
	s, err := Decode([]byte(`{"volumeCurve":"linear","volume":0.5}`))
	if err != nil {
		t.Fatal(err)
	}
	if v := s.Clean().Volume; v != 0.5 {
		t.Errorf("on Linear, the volume kept is %v, not 0.5", v)
	}
}
