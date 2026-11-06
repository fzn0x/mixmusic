package main

import (
	"os"
	"strings"
	"testing"

	"github.com/grafana/sobek"
)

// spotifyPage runs the script of public/spotify.html, the page of
// MixMusic's own Spotify device, without a document, in a webview whose
// EME grants the key systems of granted, or that has no EME when granted
// is nil, and returns its runtime. asked lists the key systems the page
// asked for.
func spotifyPage(t *testing.T, secure bool, granted []string) *sobek.Runtime {
	t.Helper()
	src, err := os.ReadFile("public/spotify.html")
	if err != nil {
		t.Fatal(err)
	}
	_, script, _ := strings.Cut(string(src), "<script>")
	script, _, ok := strings.Cut(script, "</script>")
	if !ok {
		t.Fatal("public/spotify.html has no script")
	}
	vm := sobek.New()
	if err := vm.Set("isSecureContext", secure); err != nil {
		t.Fatal(err)
	}
	if err := vm.Set("granted", granted); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunString(`
		var window = globalThis;
		var asked = [];
		var navigator = granted === null ? {} : {
			requestMediaKeySystemAccess(system) {
				asked.push(system);
				return granted.includes(system)
					? Promise.resolve({ createMediaKeys: () => Promise.resolve({}) })
					: Promise.reject(new Error("NotSupportedError"));
			},
		};`); err != nil {
		t.Fatal(err)
	}
	if _, err := vm.RunScript("spotify.html", script); err != nil {
		t.Fatal(err)
	}
	return vm
}

// TestSpotifyPageDecryptsWithFairPlay has the page of MixMusic's own
// Spotify device take FairPlay, which macOS's webview has, as it takes
// Widevine and PlayReady in WebView2, and nothing else: WebKitGTK has no
// EME, and ClearKey doesn't play Spotify's audio (#295).
func TestSpotifyPageDecryptsWithFairPlay(t *testing.T) {
	for _, c := range []struct {
		name    string
		secure  bool
		granted []string
		want    bool
	}{
		{"WebView2, with Widevine", true, []string{"com.widevine.alpha", "org.w3.clearkey"}, true},
		{"WebView2, with PlayReady", true, []string{"com.microsoft.playready.recommendation"}, true},
		{"macOS, with FairPlay", true, []string{"com.apple.fps", "com.apple.fps.1_0", "org.w3.clearkey"}, true},
		{"ClearKey alone", true, []string{"org.w3.clearkey"}, false},
		{"WebKitGTK, without EME", true, nil, false},
		{"a page that isn't secure", false, []string{"com.apple.fps"}, false},
	} {
		vm := spotifyPage(t, c.secure, c.granted)
		if _, err := vm.RunString(`var supported = "pending"; mm.supported().then((v) => (supported = v), (e) => (supported = String(e)));`); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := vm.Get("supported").Export(); got != c.want {
			t.Errorf("%s: mm.supported() = %v, want %v (asked %v)", c.name, got, c.want, vm.Get("asked").Export())
		}
	}
	// Widevine first, as before: WebView2 asks for nothing it didn't.
	vm := spotifyPage(t, true, []string{"com.widevine.alpha", "com.apple.fps"})
	if _, err := vm.RunString(`mm.supported()`); err != nil {
		t.Fatal(err)
	}
	if got := vm.Get("asked").Export(); len(got.([]any)) != 1 || got.([]any)[0] != "com.widevine.alpha" {
		t.Errorf("WebView2 asked for %v, want Widevine alone", got)
	}
}
