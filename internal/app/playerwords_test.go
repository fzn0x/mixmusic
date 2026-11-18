package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPlayerErrorWords says, in en-XA, why a track couldn't play as the
// windows show it: the cast device's errors, those of MixMusic's own
// Spotify player, the engine's, the stream proxy's and those of the files
// of this computer, while the logs keep their English (#800).
func TestPlayerErrorWords(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	bracketed := func(s string) bool {
		return strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") && !strings.Contains(s, "] [")
	}
	o := &castOut{}
	errs := []error{
		castUnreachable(),
		o.loadFailure(&cast.RequestError{Type: "LOAD_FAILED"}, false),
		o.loadFailure(&cast.RequestError{Type: "LOAD_FAILED"}, true),
		o.loadFailure(fmt.Errorf("loading: %w", context.DeadlineExceeded), false),
		o.loadFailure(&cast.RequestError{Type: "LOAD_CANCELLED"}, false),
		noNative(), fileMissing(), notIndexed("L3NvbmcubXAz"),
		engineWords(stream.ErrLoad), engineWords(&stream.ServeError{Status: http.StatusServiceUnavailable}),
		engineWords(&stream.ServeError{Status: http.StatusTooManyRequests}), engineWords(&stream.ServeError{Unreachable: true}),
	}
	for _, rawURL := range []string{"http://127.0.0.1" + stream.LivePath("dQw4w9WgXcQ"), "http://127.0.0.1/file/L3NvbmcubXAz",
		"http://127.0.0.1/yt/x"} {
		_, _, err := videoOf(rawURL)
		errs = append(errs, err)
	}
	for name := range hereProblems {
		errs = append(errs, pageError("["+name+"] what the page says"))
	}
	for _, err := range errs {
		if got := ErrorWords(err); !bracketed(got) {
			t.Errorf("%v says %q, which no catalog translates", err, got)
		}
		if strings.Contains(err.Error(), "[") {
			t.Errorf("the log of %q isn't English", err.Error())
		}
	}
	for _, reason := range []string{audio.ReasonNetwork, audio.ReasonDecode, audio.ReasonUnsupported, audio.ReasonFile} {
		if got, ok := deckText(reason); !ok || !bracketed(got) {
			t.Errorf("the engine's failure %q says %q", reason, got)
		}
	}
	// The engine's errors, the proxy's and the cast device's still tell
	// what they are, and the page's its own words.
	if !errors.Is(fileMissing(), errFileMissing) || !errors.Is(noNative(), errNoNative) || !errors.Is(engineWords(stream.ErrLoad), stream.ErrLoad) {
		t.Error("an error worded is no longer what it wraps")
	}
	if _, _, err := videoOf("http://127.0.0.1/file/L3NvbmcubXAz"); !errors.Is(err, errLocalCast) {
		t.Errorf("a file to cast: %v", err)
	}
	if err := pageError("[premium] Spotify plays in MixMusic with Spotify Premium only"); err.Error() != "Spotify plays in MixMusic with Spotify Premium only" ||
		ErrorWords(err) != locales.SpotifyHerePremium() {
		t.Errorf("the page's problem logs %q, and says %q", err, ErrorWords(err))
	}
	for _, message := range []string{"[new] a problem of no words yet", "Spotify's player failed"} {
		if err := pageError(message); ErrorWords(err) != strings.TrimPrefix(message, "[new] ") {
			t.Errorf("%q says %q", message, ErrorWords(err))
		}
	}
	// Errors MixMusic has no words for keep theirs.
	other := errors.New("decoder: out of memory")
	if engineWords(other) != other || engineWords(nil) != nil {
		t.Error("an error of no words got some")
	}
}

// TestHereProblemsWorded has words for each problem the page of
// MixMusic's own Spotify device names, public/spotify.html's.
func TestHereProblemsWorded(t *testing.T) {
	page, err := os.ReadFile("../../public/spotify.html")
	if err != nil {
		t.Fatal(err)
	}
	problems := regexp.MustCompile(`(?s)const problems = \{(.*?)\};`).FindSubmatch(page)
	if problems == nil {
		t.Fatal("the page names no problems")
	}
	var names []string
	for _, m := range regexp.MustCompile(`(?m)^\s*(\w+):`).FindAllSubmatch(problems[1], -1) {
		names = append(names, string(m[1]))
	}
	slices.Sort(names)
	if want := slices.Sorted(maps.Keys(hereProblems)); !slices.Equal(names, want) {
		t.Errorf("the page names the problems %q, which have words: %q", names, want)
	}
}
