package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
)

// offline refuses every request, as a computer with no network does.
type offline struct{ gets int }

func (o *offline) RoundTrip(*http.Request) (*http.Response, error) {
	o.gets++
	return nil, errors.New("offline")
}

// TestControlsCoverOfADownloadOffline gives the system's controls the cover
// a download keeps, offline, with the artwork store empty: the covers were
// cleared. A download whose files are gone falls back to the store.
func TestControlsCoverOfADownloadOffline(t *testing.T) {
	a, _ := localApp(t)
	cover := pngOf(t)
	audio, err := os.ReadFile(filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	a.downloads = downloads.Open(filepath.Join(t.TempDir(), "downloads"), downloads.Options{Fetcher: webmFetcher{audio},
		Art: func(context.Context, string) ([]byte, error) { return cover, nil }})
	t.Cleanup(a.downloads.Close)
	track := exportTracks[0]
	a.downloads.Add([]media.Track{track}, "best")
	a.downloads.Start()
	url := track.Artwork[0].URL
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(2 * time.Millisecond) {
		if _, ok := a.downloads.ArtworkFile(url); ok && a.downloads.Status().Tracks == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the download didn't finish: %+v", a.downloads.Status())
		}
	}

	net := &offline{}
	a.artStore.HTTP = &http.Client{Transport: net}
	got, err := a.coverBytes(context.Background(), url)
	if err != nil || !bytes.Equal(got, cover) {
		t.Fatalf("the controls got %d bytes, %v; want the %d of the download's cover", len(got), err, len(cover))
	}
	if net.gets != 0 {
		t.Errorf("the cover asked the network %d times", net.gets)
	}

	// The drive of the downloads is gone: the store is asked, and offline
	// it has nothing.
	path, _ := a.downloads.ArtworkFile(url)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := a.coverBytes(context.Background(), url); err == nil || net.gets == 0 {
		t.Errorf("without the download's file: err %v, %d network asks; want the store tried", err, net.gets)
	}
}
