package app

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

// issuesURL is where problems are reported.
const issuesURL = "https://github.com/fzn0x/mixmusic/issues/new"

// LogsFolder returns the folder of MixMusic's log and of its crash
// reports.
func (s *System) LogsFolder() string { return s.a.opts.LogsDir }

// OpenLogsFolder opens the folder of the log in the file manager.
func (s *System) OpenLogsFolder() error {
	if s.a.opts.LogsDir == "" {
		return fmt.Errorf("MixMusic keeps no log here")
	}
	return mygo.Shell.OpenPath(s.a.opts.LogsDir)
}

// LastCrash returns the name of the crash report of the run before, when
// MixMusic crashed then and the user has not answered yet; "" otherwise.
func (s *System) LastCrash() string {
	s.a.mu.Lock()
	defer s.a.mu.Unlock()
	if s.a.lastCrash == "" {
		return ""
	}
	return filepath.Base(s.a.lastCrash)
}

// DismissCrash forgets the crash of the run before; its report stays in
// the logs folder.
func (s *System) DismissCrash() {
	s.a.mu.Lock()
	defer s.a.mu.Unlock()
	s.a.lastCrash = ""
}

// ReportURL returns the address of a new problem report on GitHub, with
// the version and the setup filled in: the system, how MixMusic was
// installed, the tools, the theme and the plugins. Nothing of the user's
// accounts or library goes in it.
func (s *System) ReportURL(ctx context.Context) string {
	info := s.Info()
	set := s.a.settings.Get()
	tools := s.a.tools.Status(ctx)
	var b strings.Builder
	fmt.Fprintf(&b, "MixMusic %s on %s %s", info.Version, runtime.GOOS, runtime.GOARCH)
	if kind := installKind(); kind != "" {
		fmt.Fprintf(&b, ", installed as %s", kind)
	}
	b.WriteString("\n")
	outdated := ""
	if tools.YTDLP.Latest != "" {
		outdated = ", older than the latest, " + tools.YTDLP.Latest
	}
	switch {
	case !tools.YTDLP.Found:
		b.WriteString("yt-dlp: not installed\n")
	case tools.YTDLP.Managed:
		fmt.Fprintf(&b, "yt-dlp %s, installed by MixMusic%s\n", tools.YTDLP.Version, outdated)
	default:
		fmt.Fprintf(&b, "yt-dlp %s, the system's%s\n", tools.YTDLP.Version, outdated)
	}
	if tools.JSRuntime.Found {
		fmt.Fprintf(&b, "JavaScript runtime: %s %s\n", tools.JSRuntime.Name, tools.JSRuntime.Version)
	} else {
		b.WriteString("JavaScript runtime: none\n")
	}
	fmt.Fprintf(&b, "Theme: %s\n", set.Theme)
	if len(set.EnabledPlugins) > 0 {
		fmt.Fprintf(&b, "Plugins: %s\n", strings.Join(set.EnabledPlugins, ", "))
	}
	b.WriteString("\nThe log, mixmusic.log, is in the folder Settings › About opens: read it before you attach it.\n")
	q := url.Values{"template": {"bug_report.yml"}, "version": {info.Version}, "more": {b.String()}}
	return issuesURL + "?" + q.Encode()
}
