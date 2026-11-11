package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tags"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// The songs here are made up.

// exportTracks are a song of YouTube Music, on an album, with a cover, and
// one of Spotify, which downloads as its recording there, with an ISRC.
var exportTracks = []media.Track{
	{ID: "yt:aaaaaaaaaaa", Source: media.SourceYouTube, Title: "Slow Ferry", Artists: []media.Ref{{Name: "Ola Brine"}},
		Album: &media.Ref{ID: "yt:al-harbour", Name: "Harbour Lights"}, Artwork: []media.Image{{URL: "https://example.com/ferry.png", Width: 800}}},
	{ID: "sp:tin", Source: media.SourceSpotify, Title: "Tin Roof Rain", Artists: []media.Ref{{Name: "Tam Reed"}, {Name: "Ola Brine"}},
		Album: &media.Ref{ID: "sp:al-tin", Name: "Rooftops"}, ISRC: "QZES81900007"},
}

// readTagsOf reads the tags of the file at path.
func readTagsOf(t *testing.T, path string) tags.Tags {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tg, err := tags.Read(f)
	if err != nil {
		t.Fatalf("reading %s: %v", filepath.Base(path), err)
	}
	return tg
}

// TestExport exports two downloads, with their tags, the cover the
// downloads keep and what their albums' pages tell, and a song with
// copies on this computer as its best copy, as it is; a track neither
// downloaded nor on this computer fails, and the downloads stay as they
// were.
func TestExport(t *testing.T) {
	a, music := localApp(t)
	for _, name := range []string{"Shore/01 Slow Tide.flac", "Shore/02 Low Water.flac"} {
		os.Remove(filepath.Join(music, filepath.FromSlash(name)))
	}
	mp3Copy(t, music, "Singles/Paper Kites.mp3")
	tagged(t, music, map[string]tagwrite.Tags{"Ada Field/Kites/01.flac": paperKites("Kites")})
	flac := filepath.Join(music, "Ada Field", "Kites", "01.flac")
	scanned(t, a)
	l := &LocalFiles{a}
	song := trackNamed(t, l, "Paper Kites")
	if !localfiles.IsSong(song.ID) {
		t.Fatalf("the copies aren't one song: %q", song.ID)
	}

	cover := pngOf(t)
	audio, err := os.ReadFile(filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	a.downloads = downloads.Open(filepath.Join(t.TempDir(), "downloads"), downloads.Options{Fetcher: webmFetcher{audio},
		Matcher: matches{"sp:tin": {ID: "yt:ccccccccccc", Source: media.SourceYouTube, Title: "Tin Roof Rain"}},
		Art:     func(context.Context, string) ([]byte, error) { return cover, nil }})
	t.Cleanup(a.downloads.Close)
	a.downloads.Add(exportTracks, "best")
	a.downloads.Start()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(2 * time.Millisecond) {
		_, art := a.downloads.ArtworkFile(exportTracks[0].Artwork[0].URL)
		if a.downloads.Status().Tracks == 2 && art {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the downloads didn't finish: %+v", a.downloads.Status())
		}
	}

	d := &Downloads{a}
	gone := media.Track{ID: "yt:ddddddddddd", Source: media.SourceYouTube, Title: "Gulls at Dawn"}
	for _, tr := range append(slices.Clone(exportTracks), song) {
		if !d.Exportable(tr) {
			t.Errorf("%s doesn't export", tr.Title)
		}
	}
	if d.Exportable(gone) {
		t.Error("a track not downloaded exports")
	}

	dir := t.TempDir()
	var asked []string
	var progress [][2]int
	res := d.Export(context.Background(), append(slices.Clone(exportTracks), song, gone), dir, ExportOptions{
		Album: func(_ context.Context, id string) (media.Album, error) {
			asked = append(asked, id)
			if id != "yt:al-harbour" {
				return media.Album{}, errors.New("offline")
			}
			return media.Album{ID: id, Year: "2019-04-02", Artists: []media.Ref{{Name: "Ola Brine"}},
				Tracks: []media.Track{{ID: "yt:zzzzzzzzzzz", Title: "Paper Lanterns"}, exportTracks[0]}}, nil
		},
		Progress: func(done, total int) { progress = append(progress, [2]int{done, total}) },
	})
	var names []string
	for _, p := range res.Files {
		names = append(names, filepath.Base(p))
	}
	if want := []string{"Ola Brine - Slow Ferry.opus", "Tam Reed, Ola Brine - Tin Roof Rain.opus", "Ada Field - Paper Kites.flac"}; !slices.Equal(names, want) {
		t.Fatalf("exported %q, want %q; failed %+v", names, want, res.Failed)
	}
	if len(res.Failed) != 1 || res.Failed[0].Track.ID != gone.ID {
		t.Errorf("failed %+v", res.Failed)
	}
	if len(progress) != 4 || progress[3] != [2]int{4, 4} {
		t.Errorf("progress %v", progress)
	}
	if want := []string{"yt:al-harbour", "sp:al-tin"}; !slices.Equal(asked, want) {
		t.Errorf("asked for the albums %q, want %q", asked, want)
	}

	ferry := readTagsOf(t, res.Files[0])
	if ferry.Title != "Slow Ferry" || !slices.Equal(ferry.Artists, []string{"Ola Brine"}) || ferry.Album != "Harbour Lights" ||
		ferry.Year != 2019 || ferry.Track != 2 || !slices.Equal(ferry.AlbumArtists, []string{"Ola Brine"}) {
		t.Errorf("the first reads back as %+v", ferry)
	}
	if ferry.Picture == nil || !bytes.Equal(ferry.Picture.Data, cover) {
		t.Error("the first has no cover, or another")
	}
	tin := readTagsOf(t, res.Files[1])
	if tin.Title != "Tin Roof Rain" || tin.ISRC != "QZES81900007" || tin.Year != 0 || tin.Album != "Rooftops" ||
		!slices.Equal(tin.Artists, []string{"Tam Reed", "Ola Brine"}) {
		t.Errorf("the second reads back as %+v", tin)
	}
	want, _ := os.ReadFile(flac)
	if got, _ := os.ReadFile(res.Files[2]); !bytes.Equal(got, want) {
		t.Error("the song on this computer isn't its best copy as it is")
	}
	kept, _ := a.downloads.Lookup(exportTracks[0].ID)
	if got, _ := os.ReadFile(kept.Path); !bytes.Equal(got, audio) {
		t.Error("the download changed")
	}
}

// TestExportAlbumsOffline stops asking for the albums' pages once two in a
// row failed, as they do without the network.
func TestExportAlbumsOffline(t *testing.T) {
	asked := 0
	p := albumPages{get: func(context.Context, string) (media.Album, error) {
		asked++
		return media.Album{}, context.DeadlineExceeded
	}}
	for _, id := range []string{"yt:a", "yt:b", "yt:c", "yt:a"} {
		tg := tagwrite.Tags{}
		p.fill(context.Background(), media.Track{Title: "Slow Ferry", Album: &media.Ref{ID: id}}, &tg)
		if tg.Year != 0 || tg.Track != 0 {
			t.Errorf("an album that failed gave %+v", tg)
		}
	}
	if asked != 2 {
		t.Errorf("asked %d times", asked)
	}
}
