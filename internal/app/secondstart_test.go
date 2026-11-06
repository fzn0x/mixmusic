package app

import (
	"testing"

	"github.com/egoist/mygo"
)

// TestStartedAgainWhileStarting starts MixMusic again, and activates it,
// before MyGo is ready, as a second start can while the first still starts
// (#305): no window opens before then, which would panic in MyGo.
func TestStartedAgainWhileStarting(t *testing.T) {
	if mygo.App.IsReady() {
		t.Skip("MyGo is ready in this test binary")
	}
	a := newQuietApp(t, func(o *Options) {
		o.Interface = func(*App, bool) *mygo.Window {
			t.Fatal("the main window opened before MyGo was ready")
			return nil
		}
	})
	a.SecondInstance(nil, "")
	a.Activate(false)
}
