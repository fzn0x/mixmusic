//go:build windows

package sysproxy

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// TestDefaultTransportPicks checks that http.DefaultTransport, and so every
// client cloning it, picks its proxies here.
func TestDefaultTransportPicks(t *testing.T) {
	got := http.DefaultTransport.(*http.Transport).Proxy
	if reflect.ValueOf(got).Pointer() != reflect.ValueOf(Proxy).Pointer() {
		t.Error("http.DefaultTransport doesn't use Proxy")
	}
}

// TestReadSettings reads this computer's settings, changing nothing.
func TestReadSettings(t *testing.T) {
	s, err := readSettings()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("this computer's proxy settings: %s", s)
}

// TestAskWinHTTPForAScriptThatIsNotThere asks for a setup script no server
// serves: a failure of the whole setup, in words.
func TestAskWinHTTPForAScriptThatIsNotThere(t *testing.T) {
	_, _, global, err := askWinHTTP(Settings{Script: "http://127.0.0.1:9/proxy.pac"}, "https://music.youtube.com/")
	if err == nil || !global {
		t.Fatalf("err = %v, global = %v", err, global)
	}
	if !strings.Contains(err.Error(), "the setup script didn't download") {
		t.Errorf("err = %v", err)
	}
}
