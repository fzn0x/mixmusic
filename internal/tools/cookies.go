package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fzn0x/mixmusic/internal/proc"
)

// NetscapeHeader begins a cookies file of the Netscape format, which yt-dlp
// reads and writes.
const NetscapeHeader = "# Netscape HTTP Cookie File\n"

// The reasons yt-dlp gives for not reading a browser's cookies.
var (
	// ErrBrowserOpen is returned when the browser holds its cookies while
	// it runs, as the Chromium browsers do on Windows.
	ErrBrowserOpen = errors.New("the browser is open and holds its cookies: close it, then try again")
	// ErrCookiesSealed is returned when the browser encrypts its cookies
	// so that other programs can't decrypt them, as recent Chrome does on
	// Windows, or keeps their key where yt-dlp can't reach it.
	ErrCookiesSealed = errors.New("the browser seals its cookies so that other programs can't read them")
	// ErrNoBrowserCookies is returned when there are no cookies of the
	// browser where yt-dlp looked: it isn't installed, or its profile is
	// elsewhere.
	ErrNoBrowserCookies = errors.New("no cookies of the browser were found")
	// ErrCookiesDenied is returned when the system keeps the browser's
	// cookies from MixMusic, as macOS keeps Safari's without Full Disk
	// Access.
	ErrCookiesDenied = errors.New("the system keeps the browser's cookies from MixMusic")
)

// cookieErrors tell the reason of an error line of yt-dlp, by a part of it,
// as yt-dlp words them: the first that matches applies.
var cookieErrors = []struct {
	match string
	err   error
}{
	{"could not copy chrome cookie database", ErrBrowserOpen},
	{"database is locked", ErrBrowserOpen},
	{"failed to decrypt with dpapi", ErrCookiesSealed},
	{"keyring", ErrCookiesSealed},
	{"keychain", ErrCookiesSealed},
	{"kwallet", ErrCookiesSealed},
	{"cookies database", ErrNoBrowserCookies},
	{"local state", ErrNoBrowserCookies},
	{"operation not permitted", ErrCookiesDenied},
}

// CookiesFromBrowser has yt-dlp copy the cookies of a browser into file, in
// the Netscape format. browser is the browser as yt-dlp's
// --cookies-from-browser names it, with its profile and container, such as
// "firefox:PROFILE::none". file must exist and begin with NetscapeHeader:
// yt-dlp reads it before it writes it, and an empty file is not one it
// reads.
//
// Given no video, yt-dlp says it needs one and exits with an error, once it
// wrote the cookies: what file holds tells whether it worked, not how
// yt-dlp exited. The error is why yt-dlp could not read the browser, when
// it said; undecrypted reports that it could not decrypt some of the
// cookies. What yt-dlp prints is not logged: it may name the user's
// folders.
func (m *Manager) CookiesFromBrowser(ctx context.Context, browser, file string) (undecrypted bool, err error) {
	path, err := m.YTDLP()
	if err != nil {
		return false, err
	}
	var stdout, stderr bytes.Buffer
	cmd := proc.Command(ctx, path, "--ignore-config", "--no-progress", "--cookies-from-browser", browser, "--cookies", file)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if line := ErrorLine(stderr.String()); line != "" {
		lower := strings.ToLower(line)
		for _, e := range cookieErrors {
			if strings.Contains(lower, e.match) {
				return false, e.err
			}
		}
		return false, errors.New("yt-dlp: " + line)
	}
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return false, fmt.Errorf("yt-dlp: %w", runErr)
	}
	said := strings.ToLower(stdout.String() + stderr.String())
	return strings.Contains(said, "could not be decrypted") || strings.Contains(said, "cannot decrypt"), nil
}

// ErrorLine returns the last error line yt-dlp printed, without its
// prefixes, or "" when it printed none.
func ErrorLine(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); strings.HasPrefix(line, "ERROR:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
			if _, rest, ok := strings.Cut(line, "] "); ok && strings.HasPrefix(line, "[") {
				line = rest
			}
			return line
		}
	}
	return ""
}
