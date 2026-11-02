package portal

import (
	"runtime"
	"testing"
)

func TestRequestPath(t *testing.T) {
	if got, want := requestPath(":1.42", "mixmusic3"), "/org/freedesktop/portal/desktop/request/1_42/mixmusic3"; got != want {
		t.Errorf("requestPath = %q, want %q", got, want)
	}
}

// A desktop without the Background portal answers with one of D-Bus's
// errors, which GIO's message names.
func TestMissing(t *testing.T) {
	for _, c := range []struct {
		message string
		missing bool
	}{
		{"GDBus.Error:org.freedesktop.DBus.Error.UnknownMethod: No such interface “org.freedesktop.portal.Background” on object at path /org/freedesktop/portal/desktop", true},
		{"GDBus.Error:org.freedesktop.DBus.Error.ServiceUnknown: The name org.freedesktop.portal.Desktop was not provided by any .service files", true},
		{"GDBus.Error:org.freedesktop.DBus.Error.UnknownInterface: Unknown interface", true},
		{"GDBus.Error:org.freedesktop.portal.Error.InvalidArgument: Commandline can't be empty", false},
		{"GDBus.Error:org.freedesktop.DBus.Error.NoReply: Message recipient disconnected from message bus without replying", false},
		{"Timeout was reached", false},
	} {
		if got := missing(remoteError(c.message)); got != c.missing {
			t.Errorf("%q: missing = %v, want %v", c.message, got, c.missing)
		}
	}
	if got := remoteError("GDBus.Error:org.freedesktop.DBus.Error.UnknownMethod: No such method"); got != "org.freedesktop.DBus.Error.UnknownMethod" {
		t.Errorf("remoteError = %q", got)
	}
}

// Where there are no portals to ask, the request says so.
func TestUnsupported(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux has the portals")
	}
	if _, err := RequestBackground(t.Context(), Background{Autostart: true}); err != ErrUnsupported {
		t.Errorf("RequestBackground = %v, want ErrUnsupported", err)
	}
}
