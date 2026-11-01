package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
)

// keptTracks are tracks of real video IDs, which downloads keep files of.
var keptTracks = []media.Track{
	testTrack("yt:aaaaaaaaaaa", "Teardrop", "Massive Attack", "Mezzanine", media.SourceYouTube),
	testTrack("yt:bbbbbbbbbbb", "Angel", "Massive Attack", "Mezzanine", media.SourceYouTube),
}

// seedDownloads makes a folder of downloads holding tracks, the album
// Mezzanine downloaded whole, as an earlier run kept them.
func seedDownloads(t *testing.T, tracks ...media.Track) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "downloads")
	audio, err := os.ReadFile(filepath.Join("webm", "testdata", "stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		Track   media.Track `json:"track"`
		Quality string      `json:"quality"`
		Pins    []string    `json:"pins"`
		Seq     int         `json:"seq"`
	}
	type file struct {
		Path   string `json:"path"`
		Bytes  int    `json:"bytes"`
		Format string `json:"format"`
		Codec  string `json:"codec"`
	}
	idx := map[string]any{"version": 1, "seq": len(tracks)}
	entries, files := map[string]entry{}, map[string]file{}
	var ids []string
	for i, tr := range tracks {
		_, video, _ := media.ParseID(tr.ID)
		name := "audio/" + video + ".webm"
		if err := os.MkdirAll(filepath.Join(dir, "audio"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), audio, 0o644); err != nil {
			t.Fatal(err)
		}
		entries[tr.ID] = entry{Track: tr, Quality: "best", Pins: []string{"album:al-mezz"}, Seq: i + 1}
		files[video] = file{Path: name, Bytes: len(audio), Format: "webm", Codec: "opus"}
		ids = append(ids, tr.ID)
	}
	idx["tracks"], idx["files"] = entries, files
	idx["collections"] = map[string]downloads.Collection{"album:al-mezz": {ID: "album:al-mezz", Kind: "album", Title: "Mezzanine",
		Source: media.SourceYouTube, Tracks: ids, Added: time.Now()}}
	data, _ := json.Marshal(idx)
	if err := os.WriteFile(filepath.Join(dir, "index.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// newKeptShell is a test shell whose downloads hold tracks.
func newKeptShell(t *testing.T, tracks ...media.Track) (*shell, string) {
	dir := seedDownloads(t, tracks...)
	return newTestShellOf(t, func(o *app.Options) { o.DownloadsDir = dir }), dir
}

// TestDownloadsPage lists the tracks downloaded, where they are and the
// album downloaded, and removes a download from its track's menu.
func TestDownloadsPage(t *testing.T) {
	s, dir := newKeptShell(t, keptTracks...)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/downloads")
	for _, want := range []string{"2 tracks · ", dir, "Teardrop", "Angel", "Mezzanine", "Downloaded", "remove all downloads"} {
		if !tt.HasText(want) {
			t.Errorf("the page doesn't show %q: %q", want, tt.Texts())
		}
	}
	if err := tt.RightClick("Angel"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Remove download"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if _, err := os.Stat(filepath.Join(dir, "audio", "bbbbbbbbbbb.webm")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the file stays: %v", err)
	}
	tt.Frame()
	if _, ok := tt.Find("Angel"); ok || !tt.HasText("1 track · ") {
		t.Errorf("after the removal, the page shows %q", tt.Texts())
	}
	// The album shows its tracks alone.
	if err := tt.Click("Mezzanine"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.downloadsOf != "album:al-mezz" || !tt.HasText("show all") {
		t.Errorf("chose %q; the page shows %q", s.downloadsOf, tt.Texts())
	}
}

func TestDownloadsEmpty(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/library/downloads")
	if !tt.HasText("nothing downloaded yet") {
		t.Errorf("the page shows %q", tt.Texts())
	}
	// Nothing downloaded, the rows keep no room for a mark, nor the sidebar
	// a link.
	if s.kept.Any() || tt.HasText("Waiting to download") {
		t.Error("the downloads hold something")
	}
}

// TestDownloadMenu downloads a track from its menu, which then offers to
// stop the download, as its row's mark shows it waiting.
func TestDownloadMenu(t *testing.T) {
	s := newTestShell(t)
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Source: media.SourceYouTube, Tracks: keptTracks}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	choose := func(item string) {
		t.Helper()
		if err := tt.RightClick("Teardrop"); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(item); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		s.drain()
		tt.Frame()
		tt.Frame()
	}
	choose("Download")
	if k := s.kept.State("yt:aaaaaaaaaaa").Kind; k != downloads.Waiting {
		t.Fatalf("the track is %v", k)
	}
	if !tt.HasText("Downloading Teardrop") || !tt.HasText("Waiting to download") {
		t.Errorf("the window shows %q", tt.Texts())
	}
	choose("Cancel download")
	if k := s.kept.State("yt:aaaaaaaaaaa").Kind; k != downloads.None {
		t.Errorf("canceled, the track is %v", k)
	}
	// A Spotify track downloads as its recording on YouTube Music, as the
	// toast says.
	s.download([]media.Track{testTracks[1]})
	tt.Frame()
	if !tt.HasText("Downloading Teardrop: its recording on YouTube Music") {
		t.Errorf("the window shows %q", tt.Texts())
	}
}

// TestCollectionDownloadButton downloads an album from its head, asking
// first when it's big, and tells a downloaded one.
func TestCollectionDownloadButton(t *testing.T) {
	s, _ := newKeptShell(t, keptTracks...)
	s.loads["album:al-mezz"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-mezz", Title: "Mezzanine", Source: media.SourceYouTube, Tracks: keptTracks}}
	var many []media.Track
	for i := range 150 {
		tr := testTrack("yt:"+strings.Repeat(string(rune('c'+i%20)), 10)+count(i%10), "Song "+count(i), "Someone", "Big", media.SourceYouTube)
		many = append(many, tr)
	}
	s.loads["album:al-big"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-big", Title: "Big", Source: media.SourceYouTube, Tracks: many}}
	s.loads["album:al-new"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-new", Title: "New", Source: media.SourceYouTube,
		Tracks: testTracks[:1]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-mezz")
	if !tt.HasText("downloaded") {
		t.Errorf("a downloaded album shows %q", tt.Texts())
	}
	// It grew since.
	s.loads["album:al-mezz"].(*loaded[media.Album]).value.Tracks = append(slices.Clone(keptTracks), testTracks[0])
	tt.Frame()
	if !tt.HasText("download 1 more") {
		t.Errorf("an album that grew shows %q", tt.Texts())
	}

	show(s, tt, "/album/al-new")
	click(t, s, tt, "download")
	tt.Frame()
	if !tt.HasText("downloading 0 of 1") {
		t.Errorf("an album downloading shows %q", tt.Texts())
	}

	show(s, tt, "/album/al-big")
	click(t, s, tt, "download")
	if s.asking == nil || !strings.Contains(s.asking.message, "That's about") || s.asking.title != "download 150 tracks" {
		t.Fatalf("a big album asks %+v", s.asking)
	}
}

// TestDownloadSettings shows the downloads in Settings › Playback, their
// folder, and keeps whether they go on over a metered connection.
func TestDownloadSettings(t *testing.T) {
	s, dir := newKeptShell(t, keptTracks...)
	// The downloads are far down the page.
	tt := testerOf(t, s, 1280, 3000)
	show(s, tt, "/settings/playback")
	for _, want := range []string{"downloads", "Folder", dir, "Download on metered connections", "2 tracks in "} {
		if !tt.HasText(want) {
			t.Errorf("the settings don't show %q", want)
		}
	}
	if tt.HasText("use the default") {
		t.Error("the default folder offers to go back to itself")
	}
	// The row's title carries the switch's words too: the keyboard reaches
	// the switch.
	const label = "Download on metered connections"
	focused := func() bool {
		return tt.Focused(label) && !tt.Focused("Off: downloads wait on a metered connection, such as a phone's hotspot.")
	}
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatal("Tab doesn't reach the switch")
	}
	tt.Key(0, ui.KeySpace)
	s.drain()
	tt.Frame()
	if !s.svc.Preferences.Get().DownloadOnMetered {
		t.Error("the switch wasn't kept")
	}
}

// TestDownloadMarks marks the rows of tracks downloaded, in the lists of any
// page, and the sidebar links to the downloads.
func TestDownloadMarks(t *testing.T) {
	s, _ := newKeptShell(t, keptTracks[:1]...)
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Source: media.SourceYouTube, Tracks: keptTracks}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	marks := 0
	for _, text := range tt.Texts() {
		if text == "Downloaded" {
			marks++
		}
	}
	if marks != 1 {
		t.Errorf("%d marks of 1 track downloaded", marks)
	}
	if _, ok := tt.Find("downloads"); !ok {
		t.Error("the sidebar doesn't link to the downloads")
	}
	if err := tt.RightClick("Teardrop"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); !slices.Contains(got, "Remove download") {
		t.Errorf("a downloaded track's menu holds %q", got)
	}
	tt.CloseMenu()
}

// TestOfflineToast tells once the network went that the downloads still
// play, with a way to them.
func TestOfflineToast(t *testing.T) {
	s, _ := newKeptShell(t, keptTracks...)
	tt := tester(t, s)
	s.followNetwork(true) // back: nothing to tell
	if len(s.actionToasts) != 0 {
		t.Errorf("toasts %+v", s.actionToasts)
	}
	s.followNetwork(false)
	if len(s.actionToasts) != 1 || s.actionToasts[0].action != "downloads" {
		t.Fatalf("toasts %+v", s.actionToasts)
	}
	s.actionToasts[0].run()
	if got := s.router.Path(); got != "/library/downloads" {
		t.Errorf("the toast goes to %q", got)
	}
	tt.Frame()
	if !tt.HasText("You're offline: your downloads still play") {
		t.Errorf("the window shows %q", tt.Texts())
	}
	// Without downloads, it tells nothing.
	empty := newTestShell(t)
	empty.followNetwork(false)
	if len(empty.actionToasts) != 0 {
		t.Errorf("toasts without downloads %+v", empty.actionToasts)
	}
}

// TestRenderDownloads draws the downloads page and a list with marks to
// MIXMUSIC_RENDER, for a look.
func TestRenderDownloads(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s, _ := newKeptShell(t, keptTracks...)
	s.svc.Downloads.Add(testTracks[:3])
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Source: media.SourceYouTube,
		Tracks: append(slices.Clone(keptTracks), testTracks...)}}
	tt := ui.NewTester(s.view, 1280, 820)
	s.navigate("/library/downloads")
	renderTo(t, s, tt, dir, "library-downloads")
	s.navigate("/album/al-x")
	renderTo(t, s, tt, dir, "album-downloads")
	// The downloads are far down the settings.
	tall := ui.NewTester(s.view, 1280, 2600)
	s.navigate("/settings/playback")
	renderTo(t, s, tall, dir, "settings-downloads")
}

// TestDownloadsUnreachable tells that the folder of the downloads can't
// be reached, rather than that nothing is downloaded.
func TestDownloadsUnreachable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newTestShellOf(t, func(o *app.Options) { o.DownloadsDir = filepath.Join(file, "downloads") })
	tt := tester(t, s)
	show(s, tt, "/library/downloads")
	if !tt.HasText("Your downloads can't be reached") || tt.HasText("nothing downloaded yet") {
		t.Errorf("the page shows %q", tt.Texts())
	}
}
