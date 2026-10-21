package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// releasesAPI answers with the newest release of MixMusic.
const releasesAPI = "https://api.github.com/repos/fzn0x/mixmusic/releases/latest"

const (
	// firstUpdateCheck is how long after start MixMusic looks for a new
	// version.
	firstUpdateCheck = 10 * time.Second
	// updateEvery is how often it looks again while it runs.
	updateEvery = 6 * time.Hour
	// updateRetry is how soon it looks again after a check failed.
	updateRetry = time.Hour
	// putOff is how long "later" puts off the version offered while
	// MixMusic runs; the next start offers it again.
	putOff = 24 * time.Hour
)

// updatePrefs are the user's choices of updates. They are kept in
// updater.json, where MyGo's update window kept them until MixMusic 0.4.0,
// so that the choices made there carry over.
type updatePrefs struct {
	// AutomaticChecks is nil until the user chose, which checks.
	AutomaticChecks    *bool `json:"automaticChecks,omitempty"`
	AutomaticDownloads bool  `json:"automaticDownloads,omitempty"`
	// SkippedVersion is the version the user chose not to be offered.
	SkippedVersion string    `json:"skippedVersion,omitempty"`
	LastCheck      time.Time `json:"lastCheck,omitzero"`
}

// checks reports whether MixMusic looks for new versions.
func (p updatePrefs) checks() bool { return p.AutomaticChecks == nil || *p.AutomaticChecks }

// UpdateStatus is whether MixMusic updates itself, and how.
type UpdateStatus struct {
	// Enabled is false for builds that cannot replace themselves: those
	// of development, and installs of a package manager.
	Enabled bool `json:"enabled"`
	// Kind is how this copy was installed, which decides the download of
	// a release that replaces it: "installer", "portable", "app",
	// "archive", "deb", "rpm", "appimage" or "development".
	Kind string `json:"kind"`
	// AutomaticChecks looks for a new version at start and every few
	// hours.
	AutomaticChecks bool `json:"automaticChecks"`
	// AutomaticDownloads installs new versions without asking; they run
	// at the next launch.
	AutomaticDownloads bool `json:"automaticDownloads"`
	// LastCheck is when MixMusic last checked, as RFC 3339, or "".
	LastCheck string `json:"lastCheck"`
	// Installed is the version InstallUpdate put in place of this one,
	// which runs once MixMusic restarts, or "".
	Installed string `json:"installed"`
}

func (a *App) updateStatus() UpdateStatus {
	p := a.updates.Get()
	s := UpdateStatus{
		Enabled:            mygo.Updater.Enabled(),
		Kind:               installKind(),
		AutomaticChecks:    p.checks(),
		AutomaticDownloads: p.AutomaticDownloads,
	}
	if !p.LastCheck.IsZero() {
		s.LastCheck = p.LastCheck.Format(time.RFC3339)
	}
	a.mu.Lock()
	s.Installed = a.installed
	a.mu.Unlock()
	return s
}

// watchUpdates looks for a new version a few seconds after start, then
// every few hours, when the computer wakes up after a check is due, and
// when the user turns checks on. Development builds never look.
func (a *App) watchUpdates() {
	if installKind() == "development" {
		return
	}
	mygo.Power.OnResume(func() {
		if time.Since(a.updates.Get().LastCheck) >= updateEvery {
			a.checkUpdatesNow()
		}
	})
	go func() {
		timer := time.NewTimer(firstUpdateCheck)
		for {
			select {
			case <-timer.C:
			case <-a.checkUpdates:
				timer.Stop()
			}
			timer.Reset(a.lookForUpdate())
		}
	}()
}

// checkUpdatesNow has MixMusic look for a new version at once.
func (a *App) checkUpdatesNow() {
	select {
	case a.checkUpdates <- struct{}{}:
	default: // a check is coming already
	}
}

// updateAction is what MixMusic does of the newest release.
type updateAction int

const (
	updateNothing updateAction = iota
	updateInstall
	updateOffer
)

// decideUpdate tells what to do of release r, under the user's choices p,
// when installed is the version installed to run at the next start and
// canInstall tells whether this copy can replace itself: install it when
// the user chose so, else offer it, unless they skipped it.
func decideUpdate(r Release, p updatePrefs, installed string, canInstall bool) updateAction {
	switch {
	case !r.Newer || r.Version == installed:
		return updateNothing
	case p.AutomaticDownloads && canInstall:
		return updateInstall
	case r.Version == p.SkippedVersion:
		return updateNothing
	}
	return updateOffer
}

// lookForUpdate looks for a new version once, installs it or offers it,
// and returns when to look again.
func (a *App) lookForUpdate() time.Duration {
	p := a.updates.Get()
	if !p.checks() {
		return updateEvery
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	r, err := latestRelease(ctx, http.DefaultClient, releasesAPI, installKind(), runtime.GOARCH, a.version())
	cancel()
	if err != nil {
		log.Printf("mixmusic: looking for a new version: %v", err)
		return updateRetry
	}
	if err := a.updates.Update(func(p *updatePrefs) error { p.LastCheck = time.Now(); return nil }); err != nil {
		log.Printf("mixmusic: keeping the update preferences: %v", err)
	}
	notify()
	a.mu.Lock()
	installed := a.installed
	a.mu.Unlock()
	switch decideUpdate(r, p, installed, mygo.Updater.Enabled()) {
	case updateInstall:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := a.installUpdate(ctx, nil); err != nil {
			// As when GitHub lists the release before its update files.
			log.Printf("mixmusic: installing MixMusic %s: %v", r.Version, err)
			return updateRetry
		}
	case updateOffer:
		a.offerUpdate(r)
	}
	return updateEvery
}

// offerUpdate has the main window offer release r, unless the user put it
// off.
func (a *App) offerUpdate(r Release) {
	a.mu.Lock()
	if a.putOff == r.Version && time.Now().Before(a.putOffUntil) {
		a.mu.Unlock()
		return
	}
	a.offer = &r
	a.mu.Unlock()
	notify()
}

// Updates returns whether MixMusic updates itself, and how.
func (s *System) Updates() UpdateStatus { return s.a.updateStatus() }

// SetUpdates turns the checks for new versions and the installs without
// asking on or off. Turned on, MixMusic looks for a new version at once.
func (s *System) SetUpdates(automaticChecks, automaticDownloads bool) (UpdateStatus, error) {
	before := s.a.updates.Get()
	err := s.a.updates.Update(func(p *updatePrefs) error {
		p.AutomaticChecks = &automaticChecks
		p.AutomaticDownloads = automaticDownloads && automaticChecks
		return nil
	})
	if err != nil {
		return UpdateStatus{}, err
	}
	if automaticChecks && (!before.checks() || automaticDownloads && !before.AutomaticDownloads) {
		s.a.checkUpdatesNow()
	}
	status := s.a.updateStatus()
	notify()
	return status, nil
}

// UpdateOffer returns the new version the main window offers, until the
// user answers, or nil.
func (s *System) UpdateOffer() *Release {
	s.a.mu.Lock()
	defer s.a.mu.Unlock()
	if s.a.offer == nil {
		return nil
	}
	r := *s.a.offer
	return &r
}

// DismissUpdate answers the offer of a new version with later, which
// offers it again at the next start or in a day, or with skip, which
// never offers that version again.
func (s *System) DismissUpdate(skip bool) error {
	a := s.a
	a.mu.Lock()
	offer := a.offer
	a.offer = nil
	if offer != nil {
		a.putOff, a.putOffUntil = offer.Version, time.Now().Add(putOff)
	}
	a.mu.Unlock()
	if offer == nil || !skip {
		return nil
	}
	return a.updates.Update(func(p *updatePrefs) error { p.SkippedVersion = offer.Version; return nil })
}

// Release is the newest release of MixMusic, as this copy gets it.
type Release struct {
	// Version is the newest version, without its "v".
	Version string `json:"version"`
	// Newer is whether it is newer than this copy.
	Newer bool `json:"newer"`
	// Date is when it was published, as RFC 3339.
	Date string `json:"date"`
	// Notes tells what the release brings: the first paragraph of its
	// notes, in plain text, or "".
	Notes string `json:"notes"`
	// Page is the release's page: its notes and all of its downloads.
	Page string `json:"page"`
	// Download is the address of the release's file made for this copy,
	// such as its .deb, which copies that do not update themselves are
	// replaced with; "" when the release has none.
	Download string `json:"download"`
	// File is the name of that file.
	File string `json:"file"`
}

// LatestRelease asks GitHub for the newest release of MixMusic.
func (s *System) LatestRelease(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return latestRelease(ctx, http.DefaultClient, releasesAPI, installKind(), runtime.GOARCH, s.a.version())
}

// installUpdate installs the newest version, telling progress, when not
// nil, how far its download went.
func (a *App) installUpdate(ctx context.Context, progress func(done, total int64)) (string, error) {
	if !a.updating.TryLock() {
		return "", errors.New("the update is downloading already")
	}
	defer a.updating.Unlock()
	up, err := mygo.Updater.Check(ctx)
	switch {
	case errors.Is(err, mygo.ErrUpdatesDisabled):
		return "", errors.New("this copy of MixMusic cannot replace its own files: download the new version instead")
	case err != nil:
		return "", err
	case up == nil:
		// GitHub lists a release before its update files are uploaded.
		return "", errors.New("no update is ready to install yet; try again in a few minutes")
	}
	if err := up.Install(ctx, progress); err != nil {
		return "", err
	}
	a.mu.Lock()
	a.installed = up.Version
	if a.offer != nil && !newerVersion(a.offer.Version, up.Version) {
		a.offer = nil
	}
	a.mu.Unlock()
	notify()
	return up.Version, nil
}

// Restart quits MixMusic and starts it again, as the update it installed.
func (s *System) Restart() { mygo.App.Relaunch() }

// githubRelease is what GitHub tells of a release.
type githubRelease struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// latestRelease reads the newest release from api, GitHub's, for a copy
// of the version running, installed as kind on arch.
func latestRelease(ctx context.Context, client *http.Client, api, kind, arch, running string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "MixMusic/"+running)
	res, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("asking GitHub for the newest version: %w", err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests:
		return Release{}, errors.New("GitHub limits how often it answers; try again in an hour")
	case res.StatusCode != http.StatusOK:
		return Release{}, fmt.Errorf("asking GitHub for the newest version: it answered %s", res.Status)
	}
	var gh githubRelease
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&gh); err != nil {
		return Release{}, fmt.Errorf("reading the newest version: %w", err)
	}
	version := strings.TrimPrefix(gh.TagName, "v")
	if version == "" {
		return Release{}, errors.New("reading the newest version: the release has no version")
	}
	r := Release{Version: version, Newer: newerVersion(version, running), Date: gh.PublishedAt, Notes: summary(gh.Body), Page: gh.HTMLURL}
	if file := releaseFile(kind, version, arch); file != "" {
		for _, a := range gh.Assets {
			if a.Name == file {
				r.Download, r.File = a.URL, file
			}
		}
	}
	return r, nil
}

// markdownLink matches a link of Markdown, its text first.
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// summary returns what release notes tell before their first heading, the
// first paragraph, in plain text: the summary of the changelog's section.
func summary(notes string) string {
	notes = strings.TrimSpace(strings.ReplaceAll(notes, "\r\n", "\n"))
	first, _, _ := strings.Cut(notes, "\n\n")
	if strings.HasPrefix(first, "#") {
		return ""
	}
	first = markdownLink.ReplaceAllString(first, "$1")
	first = strings.NewReplacer("**", "", "`", "").Replace(first)
	return strings.Join(strings.Fields(first), " ")
}

// releaseFile names the file of a release that replaces a copy installed
// as kind on arch, as the release workflow names it; "" when none does.
func releaseFile(kind, version, arch string) string {
	switch kind {
	case "installer":
		return fmt.Sprintf("mixmusic-%s-windows-%s-setup.exe", version, arch)
	case "portable":
		return fmt.Sprintf("mixmusic-%s-windows-%s-portable.zip", version, arch)
	case "app":
		return fmt.Sprintf("mixmusic-%s-macos-universal.dmg", version)
	case "archive":
		return fmt.Sprintf("mixmusic-%s-linux-%s.tar.gz", version, arch)
	case "deb":
		return fmt.Sprintf("mixmusic-%s-linux-%s.deb", version, arch)
	case "rpm":
		return fmt.Sprintf("mixmusic-%s-linux-%s.rpm", version, arch)
	case "appimage":
		return fmt.Sprintf("mixmusic-%s-linux-%s.AppImage", version, arch)
	}
	return ""
}

// installKind tells how this copy of MixMusic was installed.
func installKind() string {
	if mygo.IsDev() {
		return "development"
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return kindOf(runtime.GOOS, filepath.Dir(exe), os.Getenv("APPIMAGE") != "", func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
}

// kindOf tells how a copy whose executable is in dir was installed on
// goos: the Windows installer leaves its uninstaller beside it, and the
// .deb and the .rpm put it in /opt/mixmusic, which dpkg keeps a list of
// for the .deb.
func kindOf(goos, dir string, appImage bool, exists func(string) bool) string {
	switch goos {
	case "windows":
		if exists(filepath.Join(dir, "Uninstall.exe")) {
			return "installer"
		}
		return "portable"
	case "darwin":
		return "app"
	}
	switch {
	case appImage:
		return "appimage"
	case dir == "/opt/mixmusic" && exists("/var/lib/dpkg/info/mixmusic.list"):
		return "deb"
	case dir == "/opt/mixmusic":
		return "rpm"
	}
	return "archive"
}

// newerVersion reports whether version a comes after b: their numbers one
// by one, then a release after its pre-releases.
func newerVersion(a, b string) bool {
	an, apre, _ := strings.Cut(strings.TrimPrefix(a, "v"), "-")
	bn, bpre, _ := strings.Cut(strings.TrimPrefix(b, "v"), "-")
	as, bs := strings.Split(an, "."), strings.Split(bn, ".")
	for i := range max(len(as), len(bs)) {
		if x, y := versionPart(as, i), versionPart(bs, i); x != y {
			return x > y
		}
	}
	if apre == "" || bpre == "" {
		return apre == "" && bpre != ""
	}
	return apre > bpre
}

// versionPart returns the number at i of a version's parts, 0 past its
// end.
func versionPart(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, _ := strconv.Atoi(parts[i])
	return n
}
