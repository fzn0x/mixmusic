// Command mixmusic is a desktop music player that brings Spotify and
// YouTube Music together in one customizable interface.
package main

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/diag"
)

//go:embed resources/tray.png
var trayIcon []byte

// allowAudio lets pages play sound no click started: the hidden page of
// MixMusic's own Spotify device gets none. WebView2 takes the arguments of
// its browser from this variable, which must be set before its first page.
func allowAudio() {
	const name, arg = "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--autoplay-policy=no-user-gesture-required"
	if args := os.Getenv(name); !strings.Contains(args, "--autoplay-policy") {
		os.Setenv(name, strings.TrimSpace(args+" "+arg))
	}
}

// version is the version of builds made without the mygo CLI, which links
// the version of mygo.config.ts into the app.
const version = "0.5.6"

func main() {
	// MIXMUSIC_DATA_DIR keeps everything in one folder: a portable copy,
	// or a test run apart from the user's own data.
	portable := os.Getenv("MIXMUSIC_DATA_DIR")
	if dir := portable; dir != "" {
		mygo.App.SetPath(mygo.PathUserData, filepath.Join(dir, "data"))
		mygo.App.SetPath(mygo.PathCache, filepath.Join(dir, "cache"))
		mygo.App.SetPath(mygo.PathLogs, filepath.Join(dir, "logs"))
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return // the running instance takes over
	}
	allowAudio()
	// The log and the crash reports, once this is the instance that runs.
	logsDir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil {
		log.Fatal(err)
	}
	running := mygo.App.Version()
	if running == "" {
		running = version
	}
	logs, lastCrash, err := diag.Start(logsDir, running)
	if err != nil {
		log.Printf("mixmusic: writing the log: %v", err)
		logsDir = ""
	} else {
		defer logs.Close()
	}
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	cacheDir, err := mygo.App.Path(mygo.PathCache)
	if err != nil {
		log.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	a, err := app.New(app.Options{
		DataDir:      dataDir,
		CacheDir:     cacheDir,
		DownloadsDir: defaultDownloadsDir(runtime.GOOS, portable, dataDir, cacheDir, home, os.Getenv("XDG_DATA_HOME")),
		Builtin:      builtin.FS,
		TrayIcon:     trayIcon,
		Version:      version,
		LogsDir:      logsDir,
		LastCrash:    lastCrash,
		Interface:    Open,
		Mini:         OpenMini,
	})
	if err != nil {
		log.Fatal(err)
	}
	a.Bind()
	mygo.App.OnSecondInstance(a.SecondInstance)
	mygo.App.OnActivate(a.Activate)
	mygo.App.OnOpenURL(a.HandleURL)
	mygo.App.WhenReady(a.Start)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// defaultDownloadsDir is the folder the downloads are in on goos, unless
// the user chose another: in the folder of the app's data that the system
// keeps, never its cache, whose files may go, nor the Windows folder that
// roams. A portable copy keeps them in its own folder.
//
//   - Windows: %LocalAppData%\MixMusic\downloads, beside the cache
//   - macOS: ~/Library/Application Support/MixMusic/downloads
//   - Linux and others: $XDG_DATA_HOME/MixMusic/downloads, of
//     ~/.local/share without it; a relative XDG_DATA_HOME doesn't count,
//     as the XDG specification says.
func defaultDownloadsDir(goos, portable, dataDir, cacheDir, home, xdgDataHome string) string {
	switch {
	case portable != "":
		return filepath.Join(portable, "downloads")
	case goos == "windows":
		return filepath.Join(cacheDir, "downloads")
	case goos == "darwin":
		return filepath.Join(dataDir, "downloads")
	}
	base := xdgDataHome
	if !filepath.IsAbs(base) {
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, filepath.Base(dataDir), "downloads")
}
