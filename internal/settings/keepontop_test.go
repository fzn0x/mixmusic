package settings

import "testing"

// TestKeepOnTop leaves the main window among the others in new installs
// and in the settings of earlier versions, which don't have the key, until
// the user keeps it on top (#350).
func TestKeepOnTop(t *testing.T) {
	for file, want := range map[string]bool{`{}`: false, `{"showInTitle":false}`: false, `{"keepOnTop":true}`: true, `{"keepOnTop":false}`: false} {
		s, err := Decode([]byte(file))
		if err != nil {
			t.Fatal(err)
		}
		if s.Clean().KeepOnTop != want {
			t.Errorf("%s keeps the window on top %v, not %v", file, s.KeepOnTop, want)
		}
	}
	if Default().KeepOnTop {
		t.Error("a new install keeps the window on top")
	}
}
