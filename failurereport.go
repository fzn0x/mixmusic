package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// The details of a track that couldn't play (#351), as text: what the
// details show, and what Copy puts on the clipboard, ready for an issue.
// Neither holds the accounts' names, cookies or tokens, nor the computer's
// addresses or the user's home folder.

// privacy is what the details of a failure leave out: the names of the
// user's accounts, and their home folder, which a path names as ~.
type privacy struct {
	names []string
	words []*regexp.Regexp // the names, each where it is a word of its own
	home  string
}

// privacyOf is what the details leave out of the accounts st, and of the
// home folder home.
func privacyOf(st app.AccountsStatus, home string) privacy {
	pr := privacy{home: home}
	if p := st.Spotify.Profile; p != nil {
		pr.names = append(pr.names, p.Name, p.ID)
	}
	if a := st.YouTube.Account; a != nil {
		pr.names = append(pr.names, a.Name, a.Handle, strings.TrimPrefix(a.Handle, "@"))
	}
	pr.names = append(pr.names, st.Lastfm.Name)
	// The longest first, as a handle holds the name without its @.
	pr.names = slices.DeleteFunc(pr.names, func(n string) bool { return len([]rune(strings.TrimSpace(n))) < 3 })
	slices.SortFunc(pr.names, func(a, b string) int { return len(b) - len(a) })
	for _, n := range pr.names {
		q := regexp.QuoteMeta(strings.TrimSpace(n))
		pr.words = append(pr.words, regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}])`+q+`($|[^\p{L}\p{N}])`))
	}
	return pr
}

var (
	// queryRe matches what follows a URL's path: its query, which may
	// hold a token or the computer's address, and its fragment, but for
	// the punctuation after it.
	queryRe = regexp.MustCompile(`(https?://[^\s?#"'<>]*)[?#][^\s"'<>]*?([:;,.)]?(?:\s|$))`)
	// headerRe matches the cookies or the authorization a message gives,
	// to the end of its line; bearerRe a bearer token; and secretRe a
	// token or a key, as "token=…".
	headerRe = regexp.MustCompile(`(?i)\b((?:set-)?cookies?|authorization)\s*[:=][^\n]*`)
	bearerRe = regexp.MustCompile(`(?i)\b(bearer)\s+[^\s,;]+`)
	secretRe = regexp.MustCompile(`(?i)\b(access_token|refresh_token|token|sapisid|apisid|password|secret|api_?key|key)\s*[:=]\s*("[^"]*"|[^\s,;]+)`)
	// addressRe matches an IPv4 address, or an IPv6 one in brackets, with
	// its port.
	addressRe = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}(?::\d+)?\b|\[[0-9A-Fa-f:.%]+\](?::\d+)?`)
)

// clean is text without what pr leaves out, nor URLs' queries, secrets or
// addresses. A name is left out where it is a word of its own, or words.
func (pr privacy) clean(text string) string {
	text = pr.inHome(text)
	for _, word := range pr.words {
		text = word.ReplaceAllString(text, "${1}(account)${2}")
	}
	text = queryRe.ReplaceAllString(text, "$1$2")
	text = headerRe.ReplaceAllString(text, "$1: (hidden)")
	text = bearerRe.ReplaceAllString(text, "$1 (hidden)")
	text = secretRe.ReplaceAllString(text, "$1=(hidden)")
	return addressRe.ReplaceAllString(text, "(address)")
}

// inHome is text with the home folder as ~, with either slash.
func (pr privacy) inHome(text string) string {
	for _, home := range []string{pr.home, strings.ReplaceAll(pr.home, `\`, "/")} {
		if len(home) > 1 {
			text = strings.ReplaceAll(text, home, "~")
		}
	}
	return text
}

// failureTrack names the track of a failure: its title, its artists and
// its album.
func failureTrack(t media.Track) string {
	a := names(t.Artists)
	on := t.Album != nil && t.Album.Name != "" && t.Album.Name != t.Title
	switch {
	case a != "" && on:
		return locales.FailureTrackByOn(t.Title, a, t.Album.Name)
	case a != "":
		return locales.FailureTrackBy(t.Title, a)
	case on:
		return locales.FailureTrackOn(t.Title, t.Album.Name)
	}
	return t.Title
}

// failureFrom tells where a track of a failure comes from: its service,
// and its ID there, or, for a track on this computer, its file's path,
// which its ID would tell too.
func failureFrom(t media.Track, pr privacy) string {
	if t.Source == media.SourceFile {
		if path, ok := localfiles.PathOf(t.ID); ok {
			return locales.FailureFileAt(pr.inHome(path))
		}
		return locales.FailureFile()
	}
	return locales.FailureSourceID(sourceName(t.Source), t.ID)
}

// failureStep tells a step of a failure: what it tried, how many times,
// and why it failed.
func failureStep(st player.Step, pr privacy) string {
	what := sentenceStart(st.What)
	if st.Times > 1 {
		what = locales.FailureTimes(what, st.Times)
	}
	return locales.FailureStep(what, pr.clean(st.Why))
}

// failureStopped tells that the player stopped after a failure.
func failureStopped() string { return locales.FailureStopped() }

// failureWhen is the time of a failure, as the details show it: its
// time of day, and its day too when it isn't today.
func failureWhen(at, now time.Time) string {
	if y, m, d := at.Date(); y == now.Year() && m == now.Month() && d == now.Day() {
		return at.Format("15:04:05")
	}
	return at.Format("2 Jan 2006, 15:04:05")
}

// failureReport is the details of a failure as text for an issue: the
// track, where it comes from, when it failed, what was tried, and the
// version of MixMusic and the system it ran on.
func failureReport(f player.Failure, info app.Info, pr privacy) string {
	var b strings.Builder
	b.WriteString(locales.FailureReportHead())
	fmt.Fprintf(&b, "- %s\n", locales.FailureReportTrack(pr.clean(failureTrack(f.Track))))
	fmt.Fprintf(&b, "- %s\n", locales.FailureReportFrom(failureFrom(f.Track, pr)))
	if r := f.Recording; r.ID != "" {
		fmt.Fprintf(&b, "- %s\n", locales.FailureReportPlayedAs(pr.clean(failureTrack(r)), failureFrom(r, pr)))
	}
	fmt.Fprintf(&b, "- %s\n", locales.FailureReportWhen(f.At.Format("2006-01-02 15:04:05 -07:00")))
	fmt.Fprintf(&b, "- %s\n", locales.FailureReportTried())
	for i, st := range f.Steps {
		fmt.Fprintf(&b, "  %s\n", locales.FailureStepNumbered(i+1, failureStep(st, pr)))
	}
	if f.Stopped {
		fmt.Fprintf(&b, "- %s\n", failureStopped())
	}
	fmt.Fprintf(&b, "- %s\n", locales.FailureReportSystem(info.Version, info.Platform, info.Arch))
	return b.String()
}

// sentenceStart is text with its first letter in capitals, but for
// yt-dlp's name, which has none.
func sentenceStart(text string) string {
	if strings.HasPrefix(text, "yt-dlp") {
		return text
	}
	r, n := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(r)) + text[n:]
}
