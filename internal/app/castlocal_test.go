package app

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/cast/casttest"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// flacOf writes a FLAC file of the test's, at rate Hz, in dir: the
// package's stereo file, whose STREAMINFO and first frame tell rate, one of
// those a frame codes.
func flacOf(t *testing.T, dir, name string, rate int) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "flac", "testdata", "stereo-16.flac"))
	if err != nil {
		t.Fatal(err)
	}
	codes := map[int]byte{44100: 9, 48000: 10, 96000: 11, 192000: 3}
	code, ok := codes[rate]
	if !ok {
		t.Fatalf("no code of %d Hz", rate)
	}
	// The sample rate is the 20 bits after the STREAMINFO's sizes of
	// blocks and frames, 10 bytes past its block's header.
	data[18], data[19] = byte(rate>>12), byte(rate>>4)
	data[20] = data[20]&0x0f | byte(rate&0xf)<<4
	// The first frame comes after the last block of metadata.
	at := 4
	for {
		last := data[at]&0x80 != 0
		at += 4 + (int(data[at+1])<<16 | int(data[at+2])<<8 | int(data[at+3]))
		if last {
			break
		}
	}
	if data[at] != 0xff || data[at+1]&0xfe != 0xf8 {
		t.Fatalf("no frame at %d", at)
	}
	block := data[at+2] >> 4
	data[at+2] = data[at+2]&0xf0 | code
	// The header goes on with the frame's number, in UTF-8's way, and the
	// size of a block coded at its end; then its CRC-8.
	end := at + 4 + 1
	for b := data[at+4]; b&0x80 != 0 && b&0x40 != 0; b <<= 1 {
		end++
	}
	switch block {
	case 6:
		end++
	case 7:
		end += 2
	}
	crc := byte(0)
	for _, b := range data[at:end] {
		crc ^= b
		for range 8 {
			if crc&0x80 != 0 {
				crc = crc<<1 ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	data[end] = crc
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// wavOf writes a WAV file of a moment of silence, in dir, of the format
// format, or of an extensible format of it when extensible.
func wavOf(t *testing.T, dir, name string, format uint16, extensible bool) string {
	t.Helper()
	fmtChunk := binary.LittleEndian.AppendUint16(nil, format)
	if extensible {
		fmtChunk = binary.LittleEndian.AppendUint16(nil, 0xFFFE)
	}
	fmtChunk = binary.LittleEndian.AppendUint16(fmtChunk, 2)
	fmtChunk = binary.LittleEndian.AppendUint32(fmtChunk, 48000)
	fmtChunk = binary.LittleEndian.AppendUint32(fmtChunk, 48000*8)
	fmtChunk = binary.LittleEndian.AppendUint16(fmtChunk, 8)
	fmtChunk = binary.LittleEndian.AppendUint16(fmtChunk, 32)
	if extensible {
		fmtChunk = binary.LittleEndian.AppendUint16(fmtChunk, 22)
		fmtChunk = binary.LittleEndian.AppendUint16(fmtChunk, 32)
		fmtChunk = binary.LittleEndian.AppendUint32(fmtChunk, 3)
		fmtChunk = binary.LittleEndian.AppendUint16(fmtChunk, format)
		fmtChunk = append(fmtChunk, "\x00\x00\x00\x00\x10\x00\x80\x00\x00\xaa\x00\x38\x9b\x71"...)
	}
	var b []byte
	chunk := func(id string, body []byte) {
		b = append(b, id...)
		b = binary.LittleEndian.AppendUint32(b, uint32(len(body)))
		b = append(b, body...)
		if len(body)%2 == 1 {
			b = append(b, 0)
		}
	}
	chunk("LIST", []byte("INFOx")) // a chunk of an odd size before the format
	chunk("fmt ", fmtChunk)
	chunk("data", make([]byte, 8*48))
	data := append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(4+len(b)))...)
	data = append(append(data, "WAVE"...), b...)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestGoogleBeyond turns down the songs a Google Cast device's media
// player doesn't play, of formats it plays: FLAC beyond 96 kHz, WAV of
// floating-point samples, and nothing else.
func TestGoogleBeyond(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.flac")
	if err := os.WriteFile(broken, []byte("fLaC and more"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		file string
		want string
	}{
		{flacOf(t, dir, "cd.flac", 44100), ""},
		{flacOf(t, dir, "hires.flac", 96000), ""},
		{flacOf(t, dir, "beyond.flac", 192000), "Kitchen plays FLAC only up to 96 kHz and 24 bits, which this file goes beyond"},
		{filepath.Join("..", "..", "flac", "testdata", "mono-24.flac"), ""},
		{broken, ""},
		{wavOf(t, dir, "pcm.wav", 1, false), ""},
		{wavOf(t, dir, "pcmx.wav", 1, true), ""},
		{wavOf(t, dir, "float.wav", 3, false), "Kitchen doesn't play WAV files of floating-point samples"},
		{wavOf(t, dir, "floatx.wav", 3, true), "Kitchen doesn't play WAV files of floating-point samples"},
		{filepath.Join(dir, "gone.wav"), ""},
	} {
		f := fileFormats[filepath.Ext(c.file)]
		err := googleBeyond(c.file, f, "Kitchen")
		if got := ErrorWords(err); got != c.want {
			t.Errorf("%s: %q, want %q", filepath.Base(c.file), got, c.want)
		}
	}
}

// TestCastServerKeeps serves the files handed to the device only while
// their streams are cast or queued, and a song's cover as an image.
func TestCastServerKeeps(t *testing.T) {
	loop := net.IPv4(127, 0, 0, 1)
	s, err := serveToCast(loop, loop, http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	dir := t.TempDir()
	song, cover := filepath.Join(dir, "a.mp3"), filepath.Join(dir, "c.png")
	os.WriteFile(song, []byte("the song's audio"), 0o644)
	os.WriteFile(cover, []byte("\x89PNG\r\n\x1a\nthe cover"), 0o644)
	s.serve("/file/a.mp3", castServed{file: song, contentType: "audio/mpeg", of: "http://stream/file/a?retry=1"})
	s.serve("/art/c.png", castServed{file: cover, cover: true, of: "http://stream/file/a"})
	s.serve("/file/b.mp3", castServed{file: song, contentType: "audio/mpeg", of: "http://stream/file/b"})
	get := func(path string, ranged bool) (int, string, string) {
		req, _ := http.NewRequest(http.MethodGet, s.url(path), nil)
		if ranged {
			req.Header.Set("Range", "bytes=4-7")
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Content-Type"), string(body)
	}
	if code, typ, body := get("/file/a.mp3", true); code != http.StatusPartialContent || typ != "audio/mpeg" || body != "song" {
		t.Errorf("the song's range: %d %q %q", code, typ, body)
	}
	if code, typ, _ := get("/art/c.png", false); code != 200 || typ != "image/png" {
		t.Errorf("the cover: %d %q", code, typ)
	}
	fetched := s.fetched.Load()
	s.keep("http://stream/file/a?retry=2", "")
	if code, _, _ := get("/file/b.mp3", false); code != http.StatusNotFound {
		t.Errorf("a song neither cast nor queued: %d", code)
	}
	if code, _, _ := get("/file/a.mp3", false); code != 200 {
		t.Errorf("the song cast: %d", code)
	}
	s.keep()
	for _, path := range []string{"/file/a.mp3", "/art/c.png"} {
		if code, _, _ := get(path, false); code != http.StatusNotFound {
			t.Errorf("%s after casting it: %d", path, code)
		}
	}
	if n := s.fetched.Load() - fetched; n != 1 {
		t.Errorf("counted %d fetches of audio, want 1", n)
	}
}

// TestCastingSongsToGoogleCast casts songs on this computer to a Google
// Cast device: in their own format, with their cover, served by ranges;
// it says why the device can't play one beyond its media player's bounds.
func TestCastingSongsToGoogleCast(t *testing.T) {
	dev := casttest.New(t)
	r := casttest.NewResponder(t, dev.Entry())
	a, p := newFarApp(t, func(o *Options) {
		o.CastBrowse = func(ctx context.Context) ([]cast.Device, error) {
			return cast.Browse(ctx, cast.BrowseOptions{To: r.Addr(), Wait: 200 * time.Millisecond})
		}
	})
	dir := t.TempDir()
	art, ok := a.localFiles.ArtFile("localart:0123456789abcdef.jpg")
	if !ok {
		t.Fatal("no folder of covers")
	}
	os.MkdirAll(filepath.Dir(art), 0o755)
	if err := os.WriteFile(art, []byte("\xff\xd8\xff\xe0 a cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	song := func(file string) media.Track {
		return media.Track{ID: localfiles.TrackID(file), Source: media.SourceFile, Title: filepath.Base(file),
			DurationMs: 100000, Artists: []media.Ref{{Name: "Artist"}}, Artwork: []media.Image{{URL: "localart:0123456789abcdef.jpg"}}}
	}
	songs := []media.Track{song(flacOf(t, dir, "Song.flac", 44100)), song(flacOf(t, dir, "Studio.flac", 192000)),
		song(wavOf(t, dir, "Float.wav", 3, false))}
	a.browseCast()
	p.PlayTracks(songs, 0)
	waitPlayer(t, p, "the song", func(s player.State) bool { return s.Playing && !s.Loading })
	if err := (&Player{a}).Cast(context.Background(), dev.ID); err != nil {
		t.Fatal(err)
	}
	load := dev.Wait(t, "LOAD", 1)
	m := load.Media()
	contentID, _ := m["contentId"].(string)
	meta, _ := m["metadata"].(map[string]any)
	images, _ := meta["images"].([]any)
	if !strings.HasSuffix(contentID, ".flac") || m["contentType"] != "audio/flac" || len(images) != 1 {
		t.Fatalf("loaded %v", load.Payload)
	}
	image, _ := images[0].(map[string]any)["url"].(string)
	server := contentID[:strings.Index(contentID, "/file/")]
	if image != server+"/art/0123456789abcdef.jpg" {
		t.Errorf("the cover at %q, the audio at %q", image, contentID)
	}
	req, _ := http.NewRequest(http.MethodGet, contentID, nil)
	req.Header.Set("Range", "bytes=0-3")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusPartialContent || string(body) != "fLaC" || res.Header.Get("Content-Type") != "audio/flac" {
		t.Errorf("fetched %d %q %q", res.StatusCode, res.Header.Get("Content-Type"), body)
	}
	res, err = http.Get(image)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("the cover: %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	waitPlayer(t, p, "the song on the device", func(s player.State) bool { return s.Remote && s.Playing && !s.Loading })

	p.Next()
	waitPlayer(t, p, "told the bounds", func(s player.State) bool {
		return s.Error == "Living Room plays FLAC only up to 96 kHz and 24 bits, which this file goes beyond"
	})
	p.Next()
	waitPlayer(t, p, "told the samples", func(s player.State) bool {
		return s.Error == "Living Room doesn't play WAV files of floating-point samples"
	})
	if n := dev.Count("LOAD"); n != 1 {
		t.Errorf("%d loads, want the first song's alone", n)
	}
}

// TestCastingToGroup lists a group of speakers as one, a speaker group,
// and plays on it as on a device.
func TestCastingToGroup(t *testing.T) {
	dev := casttest.New(t)
	dev.Name, dev.Model = "Everywhere", "Google Cast Group"
	r := casttest.NewResponder(t, dev.Entry())
	a, p, _ := newCastApp(t, r)
	svc := &Player{a}
	a.browseCast()
	devices := svc.CastDevices()
	if len(devices) != 1 || !devices[0].Group || ModelOf(devices[0]) != "Speaker group" || englishModel(devices[0]) != "speaker group" {
		t.Fatalf("found %+v", devices)
	}
	p.PlayTracks([]media.Track{castTrack("aaaaaaaaaaa")}, 0)
	waitPlayer(t, p, "playing", func(s player.State) bool { return s.Playing && !s.Loading })
	if err := svc.Cast(context.Background(), dev.ID); err != nil {
		t.Fatal(err)
	}
	if load := dev.Wait(t, "LOAD", 1); !strings.HasSuffix(load.Media()["contentId"].(string), "/yt/aaaaaaaaaaa") {
		t.Errorf("loaded %v", load.Payload)
	}
	waitPlayer(t, p, "on the group", func(s player.State) bool { return s.Remote && s.Playing && !s.Loading })
}

// queueSession is a fakeSession that takes the next track ahead, or turns
// it down with nextErr.
type queueSession struct {
	*fakeSession
	nextErr error
}

func (q *queueSession) Next(_ context.Context, m cast.Media) error {
	q.record("next %s", m.URL)
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.nextErr
}

// TestCastOutServesCastAndQueued has the server serve the stream cast and
// the one the device took ahead alone: not one it turned down, nor one
// after the next load.
func TestCastOutServesCastAndQueued(t *testing.T) {
	ev := &heardEvents{}
	q := &queueSession{fakeSession: &fakeSession{volume: cast.Volume{Level: 0.5}}}
	o := newCastOut(ev)
	o.served = func(path string) string { return "http://lan" + path }
	o.fetched = func() int64 { return 0 }
	o.contentType = func(string, bool) string { return "audio/webm" }
	var mu sync.Mutex
	var kept [][]string
	o.keep = func(of ...string) {
		mu.Lock()
		kept = append(kept, slices.Clone(of))
		mu.Unlock()
	}
	last := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return kept[len(kept)-1]
	}
	o.start(q)
	t.Cleanup(o.close)
	a, b, c := streamURL("aaaaaaaaaaa"), streamURL("bbbbbbbbbbb"), streamURL("ccccccccccc")
	o.Load(a, audio.Cue{Autoplay: true})
	q.wait(t, "load http://lan/yt/aaaaaaaaaaa 0s true")
	ev.wait(t, "playing true")
	o.Preload(b)
	q.wait(t, "next http://lan/yt/bbbbbbbbbbb")
	if got := last(); !slices.Equal(got, []string{a, b}) {
		t.Errorf("served %q, want the one cast and the one next", got)
	}
	q.mu.Lock()
	q.nextErr = errors.New("no next")
	q.mu.Unlock()
	o.Preload(c)
	q.wait(t, "next http://lan/yt/ccccccccccc")
	for deadline := time.Now().Add(5 * time.Second); !slices.Equal(last(), []string{a, ""}); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("served %q, with a next the device turned down", last())
		}
	}
	o.Load(b, audio.Cue{Autoplay: true})
	if got := last(); !slices.Equal(got, []string{b, ""}) {
		t.Errorf("served %q after the next load", got)
	}
	o.Unload()
	if got := last(); !slices.Equal(got, []string{"", ""}) {
		t.Errorf("served %q after the unload", got)
	}
}
