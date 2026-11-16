package app

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
	"github.com/fzn0x/mixmusic/ogg"
)

// TestLeftOut leaves out of a copy the parts the player passes over: of
// the kinds the settings skip, which fit the audio, joined, one that ends
// the track to its end, and none that would leave less than a second.
func TestLeftOut(t *testing.T) {
	set := settings.Default().SkipParts
	length := 200 * time.Second
	parts := []sponsorblock.Segment{
		{Kind: sponsorblock.NonMusic, From: 0, To: 12 * time.Second, Of: 201 * time.Second},
		{Kind: sponsorblock.Sponsor, From: 10 * time.Second, To: 20 * time.Second},
		{Kind: sponsorblock.Intro, From: 30 * time.Second, To: 40 * time.Second},     // not ticked
		{Kind: sponsorblock.Highlight, From: 50 * time.Second, To: 50 * time.Second}, // a point
		{Kind: sponsorblock.SelfPromo, From: 60 * time.Second, To: 70 * time.Second, Of: 150 * time.Second},
		{Kind: sponsorblock.Sponsor, From: 80 * time.Second, To: 80*time.Second + 500*time.Millisecond},
		{Kind: sponsorblock.SelfPromo, From: 190 * time.Second, To: 199500 * time.Millisecond},
	}
	got := leftOut(parts, set, length)
	want := []tagwrite.Cut{{From: 0, To: 20 * time.Second}, {From: 190 * time.Second, To: length}}
	if !slices.Equal(got, want) {
		t.Errorf("left out %v, want %v", got, want)
	}
	all := []sponsorblock.Segment{{Kind: sponsorblock.Sponsor, From: 0, To: 199500 * time.Millisecond}}
	if got := leftOut(all, set, length); got != nil {
		t.Errorf("leaving less than a second: %v", got)
	}
}

// copyLength is how many samples an Ogg Opus copy plays, as its last
// page's granule position and its pre-skip tell.
func copyLength(t *testing.T, path string) int64 {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := ogg.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.ReadPacket()
	if err != nil {
		t.Fatal(err)
	}
	end := int64(-1)
	for {
		p, err := r.ReadPacket()
		if errors.Is(err, io.EOF) {
			return end - int64(binary.LittleEndian.Uint16(head.Data[10:]))
		}
		if err != nil {
			t.Fatal(err)
		}
		if p.Last {
			end = p.Granule
		}
	}
}

// TestExportLeavesParts leaves the parts SponsorBlock marks out of the
// copy of a download of Opus, while the settings skip them and leave them
// out of exports: nothing is asked of SponsorBlock otherwise. A copy whose
// parts can't be found goes whole.
func TestExportLeavesParts(t *testing.T) {
	f := newFakeSponsorBlock()
	// The download lasts 3.008 s: an intro to 0.8 s and a sponsor read
	// from 1.2 s to 2.2 s, which fit it, and filler, which isn't skipped.
	f.parts["aaaaaaaaaaa"] = []sponsorblock.Segment{{Kind: sponsorblock.Intro, From: 0, To: 800 * time.Millisecond, Of: 3 * time.Second},
		{Kind: sponsorblock.Sponsor, From: 1200 * time.Millisecond, To: 2200 * time.Millisecond},
		{Kind: sponsorblock.Filler, From: 2300 * time.Millisecond, To: 2800 * time.Millisecond}}
	a := &App{settings: testSettings(t)}
	a.parts = openParts(t, filepath.Join(t.TempDir(), "sponsorblock.json"), f, &clock{t: time.Unix(1_800_000_000, 0)})
	tracks := []media.Track{ytTrack("aaaaaaaaaaa"), ytTrack("bbbbbbbbbbb")}
	withDownloads(t, a, nil, tracks...)
	d := &Downloads{a}
	export := func(tracks ...media.Track) []int64 {
		t.Helper()
		res := d.Export(context.Background(), tracks, t.TempDir(), ExportOptions{})
		if len(res.Failed) > 0 {
			t.Fatalf("failed %+v", res.Failed)
		}
		var out []int64
		for _, p := range res.Files {
			out = append(out, copyLength(t, p))
		}
		return out
	}

	whole := export(tracks[0])[0]
	if f.count("aaaaaaaaaaa") != 0 {
		t.Error("SponsorBlock was asked while the settings don't skip parts")
	}
	a.settings.Update(func(st *settings.Settings) error {
		st.SkipParts.On = true
		st.SkipParts.Kinds = append(st.SkipParts.Kinds, sponsorblock.Intro)
		return nil
	})
	if got := export(tracks[0])[0]; got != whole || f.count("aaaaaaaaaaa") != 0 {
		t.Errorf("a copy left parts out, or asked, while the exports keep them: %d samples of %d", got, whole)
	}
	a.settings.Update(func(st *settings.Settings) error {
		st.SkipParts.Exports = true
		return nil
	})
	// 0.8 s from the start, to the sample, and 1 s between, by whole
	// packets of 20 ms.
	if got := export(tracks[0])[0]; whole-got < 86400-960 || whole-got > 86400+960 {
		t.Errorf("the copy plays %d samples, of %d whole: want 1.8 s less", got, whole)
	}

	f.mu.Lock()
	f.errs["bbbbbbbbbbb"] = errors.New("503")
	f.mu.Unlock()
	if got := export(tracks[1])[0]; got != whole || f.count("bbbbbbbbbbb") != 1 {
		t.Errorf("a copy whose lookup failed plays %d samples, of %d whole", got, whole)
	}
}

// TestPartLookupsGiveUp asks no more once two lookups in a row failed, as
// they do without the network: the parts kept count then, however old.
func TestPartLookupsGiveUp(t *testing.T) {
	f := newFakeSponsorBlock()
	f.errs["a"], f.errs["b"] = errors.New("503"), errors.New("503")
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	a := &App{parts: openParts(t, filepath.Join(t.TempDir(), "sponsorblock.json"), f, clk)}
	a.parts.mu.Lock()
	a.parts.found["withAnIntro"] = keptOf(intro, clk.now().Add(-30*24*time.Hour))
	a.parts.mu.Unlock()
	var l partLookups
	ctx := context.Background()
	if _, ok := l.get(ctx, a, "", "c"); !ok {
		t.Error("a video without parts can't be found")
	}
	for _, v := range []string{"a", "b", "d"} {
		if _, ok := l.get(ctx, a, "", v); ok {
			t.Errorf("%s is found", v)
		}
	}
	if got, ok := l.get(ctx, a, "", "withAnIntro"); !ok || !slices.Equal(got, intro) {
		t.Errorf("the parts kept: %v, %v", got, ok)
	}
	if f.count("a") != 1 || f.count("b") != 1 || f.count("c") != 1 || f.count("d") != 0 || f.count("withAnIntro") != 0 {
		t.Errorf("asked %v", f.asked)
	}
}
