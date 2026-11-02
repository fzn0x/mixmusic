package stream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// surroundInfo is what yt-dlp prints of a video of stereo and 5.1 audio,
// with the stereo Opus selected, its formats those of formats, a JSON
// list's items.
func surroundInfo(formats ...string) []byte {
	return fmt.Appendf(nil, `{"format_id": "251", "url": "https://example.com/opus?expire=%d", "ext": "webm", "acodec": "opus",
		"abr": 138, "audio_channels": 2, "filesize": 1000, "duration": 207.5, "formats": [%s]}`,
		time.Now().Add(6*time.Hour).Unix(), strings.Join(formats, ", "))
}

// ytSurround is a 5.1 format, as yt-dlp lists it, of id and codec, with
// fields that replace its own.
func ytSurround(id, codec string, abr float64, fields string) string {
	f := fmt.Sprintf(`"format_id": "%s", "url": "https://example.com/%s", "ext": "m4a", "protocol": "https", "acodec": "%s",
		"vcodec": "none", "abr": %v, "audio_channels": 6, "filesize": 9000, "has_drm": false, "http_headers": {"User-Agent": "test"}`,
		id, id, codec, abr)
	if fields != "" {
		f += ", " + fields
	}
	return "{" + f + "}"
}

func TestYTDLPKeepsSurroundBesideStereo(t *testing.T) {
	stereo := `{"format_id": "251", "url": "https://example.com/251", "ext": "webm", "acodec": "opus", "abr": 138, "audio_channels": 2}`
	for _, c := range []struct {
		name    string
		formats []string
		want    string // the surround format's ID, "" for none
	}{
		{"stereo only", []string{stereo}, ""},
		{"AC-3 and E-AC-3", []string{stereo, ytSurround("328", "ec-3", 384, ""), ytSurround("380", "ac-3", 384, "")}, "380"},
		{"E-AC-3 alone", []string{stereo, ytSurround("328", "ec-3", 384, "")}, "328"},
		{"of the most bits", []string{ytSurround("380", "ac-3", 256, ""), ytSurround("380b", "ac-3", 384, "")}, "380b"},
		{"stable volume", []string{ytSurround("380-drc", "ac-3", 384, "")}, ""},
		{"encrypted", []string{ytSurround("380", "ac-3", 384, `"has_drm": true`), ytSurround("381", "ac-3", 384, `"has_drm": "maybe"`)}, ""},
		{"of four channels", []string{ytSurround("380", "ac-3", 384, `"audio_channels": 4`)}, ""},
		{"of video", []string{ytSurround("380", "ac-3", 384, `"vcodec": "avc1"`)}, ""},
		{"in fragments", []string{ytSurround("380", "ac-3", 384, `"protocol": "m3u8_native"`)}, ""},
		{"surround Opus", []string{ytSurround("338", "opus", 480, `"ext": "webm"`)}, ""},
		{"without a URL", []string{ytSurround("380", "ac-3", 384, `"url": ""`)}, ""},
	} {
		s, err := parseInfo(surroundInfo(c.formats...))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if s.URL == "" || s.Codec != "opus" {
			t.Errorf("%s: the stereo stream is %+v", c.name, s)
		}
		switch {
		case c.want == "" && s.Surround != nil:
			t.Errorf("%s: surround %+v", c.name, s.Surround)
		case c.want == "":
		case s.Surround == nil:
			t.Errorf("%s: no surround", c.name)
		case s.Surround.URL != "https://example.com/"+c.want || s.Surround.MimeType != "audio/mp4" ||
			s.Surround.Size != 9000 || s.Surround.Duration != 207500*time.Millisecond ||
			s.Surround.Header.Get("User-Agent") != "test" || s.Surround.Surround != nil:
			t.Errorf("%s: surround %+v", c.name, s.Surround)
		}
	}
}

func TestPickSurround(t *testing.T) {
	ac3 := playerFormat{Itag: 380, MimeType: `audio/mp4; codecs="ac-3"`, URL: "https://media/a?itag=380", AudioChannels: 6, Bitrate: 384000}
	eac3 := playerFormat{Itag: 328, MimeType: `audio/mp4; codecs="ec-3"`, URL: "https://media/a?itag=328", AudioChannels: 6, Bitrate: 384000}
	opus := playerFormat{Itag: 251, MimeType: `audio/webm; codecs="opus"`, URL: "https://media/a?itag=251", AudioChannels: 2, Bitrate: 150000}
	opus51 := playerFormat{Itag: 338, MimeType: `audio/webm; codecs="opus"`, URL: "https://media/a?itag=338", AudioChannels: 6, Bitrate: 480000}
	ciphered := ac3
	ciphered.URL, ciphered.SignatureCipher = "", "s=AAAA&sp=sig&url=x"
	drc := ac3
	drc.IsDrc = true
	dub, original := ac3, eac3
	dub.AudioTrack = &struct {
		DisplayName    string `json:"displayName"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	}{"French", false}
	original.AudioTrack = &struct {
		DisplayName    string `json:"displayName"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	}{"English original", true}
	short := ac3
	short.ApproxDurationMs = 30000
	for _, c := range []struct {
		name    string
		formats []playerFormat
		want    int // the itag, 0 for none
	}{
		{"stereo only", []playerFormat{opus}, 0},
		{"AC-3 before E-AC-3", []playerFormat{opus, eac3, ac3}, 380},
		{"E-AC-3", []playerFormat{opus, eac3}, 328},
		{"surround Opus", []playerFormat{opus51}, 0},
		{"needs yt-dlp", []playerFormat{ciphered}, 0},
		{"stable volume", []playerFormat{drc}, 0},
		{"the original track before a dub", []playerFormat{dub, original}, 328},
		{"cut short", []playerFormat{short}, 0},
	} {
		got, ok := pickSurround(c.formats, 200*time.Second)
		if ok != (c.want != 0) || ok && got.Itag != c.want {
			t.Errorf("%s: %d, %v", c.name, got.Itag, ok)
		}
	}
	// Stereo stays what pickAudio picks.
	if got, err := pickAudio([]playerFormat{ac3, eac3, opus}, QualityBest, 0); err != nil || got.Itag != 251 {
		t.Errorf("pickAudio picked %d, %v", got.Itag, err)
	}
}

func TestDirectFindsSurroundBesideStereo(t *testing.T) {
	f := newFakeTube(t)
	// The recorded answer, with a 5.1 AC-3 stream of the media server's.
	f.answers(func(playerRequest) []byte {
		b := f.recorded("")
		ac3 := fmt.Sprintf(`{"itag": 380, "url": "%s/media/videoplayback?itag=380&clen=9000", "mimeType": "audio/mp4; codecs=\"ac-3\"", "bitrate": 384000,
			"audioChannels": 6, "contentLength": "9000", "approxDurationMs": "0"}, `, f.srv.URL)
		return bytes.Replace(b, []byte(`"adaptiveFormats": [`), []byte(`"adaptiveFormats": [`+ac3), 1)
	})
	s, err := f.direct().Stream(context.Background(), songID, QualityBest)
	if err != nil {
		t.Fatal(err)
	}
	if s.Codec != "opus" || s.Surround == nil || s.Surround.Codec != "ac-3" || s.Surround.Size != 9000 ||
		!strings.Contains(s.Surround.URL, "itag=380") || s.Surround.Header.Get("User-Agent") == "" {
		t.Errorf("stream %+v, surround %+v", s, s.Surround)
	}
	// Without, none.
	f.answers(func(playerRequest) []byte { return f.recorded("") })
	if s, err := f.direct().Stream(context.Background(), songID, QualityBest); err != nil || s.Surround != nil {
		t.Errorf("without 5.1: %+v, %v", s.Surround, err)
	}
}

// surroundProxy returns a proxy whose yt-dlp resolves to a video of stereo
// audio of srv and, when surround, 5.1 audio of srv5.
func surroundProxy(srv, srv5 *httptest.Server, size, size5 int, surround bool) (*Proxy, *atomic.Int32) {
	var runs atomic.Int32
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		runs.Add(1)
		expire := time.Now().Add(6 * time.Hour).Unix()
		formats := ""
		if surround {
			formats = fmt.Sprintf(`{"format_id": "380", "url": "%s/a?expire=%d", "ext": "m4a", "protocol": "https", "acodec": "ac-3",
				"vcodec": "none", "abr": 384, "audio_channels": 6, "filesize": %d}`, srv5.URL, expire, size5)
		}
		return fmt.Appendf(nil, `{"url": "%s/a?expire=%d", "ext": "webm", "acodec": "opus", "filesize": %d, "formats": [%s]}`,
			srv.URL, expire, size, formats), nil
	}
	return NewProxy(r), &runs
}

func TestProxyServesSurround(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 3000, &minGen)
	srv5, data5 := upstream(t, 5000, &minGen)
	for i := range data5 {
		data5[i] ^= 0x55 // not the stereo audio
	}
	p, runs := surroundProxy(srv, srv5, len(data), len(data5), true)
	dir := t.TempDir()
	p.Cache = NewCache(dir, 1<<20)
	get := func(path, header string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if header != "" {
			req.Header.Set("Range", header)
		}
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		return rec
	}
	if rec := get(SurroundPath(testID), ""); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data5) || rec.Header().Get("Content-Type") != "audio/mp4" {
		t.Fatalf("5.1: %d, %d bytes of %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
	if rec := get(Path(testID), "bytes=0-"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("stereo: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if runs.Load() != 1 {
		t.Errorf("yt-dlp ran %d times for one video", runs.Load())
	}
	// Both are cached, each by its name.
	if _, err := os.Stat(filepath.Join(dir, SurroundKey(testID)+".m4a")); err != nil || !p.Cache.Has(SurroundKey(testID)) || !p.Cache.Has(testID) {
		t.Fatalf("5.1 not cached beside stereo: %v", err)
	}
	srv.Close()
	srv5.Close() // from now on only the cache can answer
	if rec := get(SurroundPath(testID), "bytes=10-19"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data5[10:20]) {
		t.Errorf("5.1 from the cache: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	if rec := get(Path(testID), "bytes=10-19"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data[10:20]) {
		t.Errorf("stereo from the cache: %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

func TestProxySaysThereIsNoSurround(t *testing.T) {
	var minGen atomic.Int32
	srv, _ := upstream(t, 3000, &minGen)
	p, _ := surroundProxy(srv, nil, 3000, 0, false)
	s, err := p.OpenSurround(context.Background(), testID, nil)
	if err == nil {
		s.Close()
		t.Fatal("5.1 of a video without any")
	}
	if err.Error() != errNoSurround.Error() || p.Failure(testID) != errNoSurround.Error() {
		t.Errorf("failed with %q, the proxy tells %q", err, p.Failure(testID))
	}
	// The stereo audio plays.
	st, err := p.Open(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if b, err := io.ReadAll(st); err != nil || len(b) != 3000 {
		t.Errorf("stereo: %d bytes, %v", len(b), err)
	}
}

func TestSourceOpensSurround(t *testing.T) {
	var minGen atomic.Int32
	srv, _ := upstream(t, 3000, &minGen)
	srv5, data5 := upstream(t, chunkSize+777, &minGen)
	p, _ := surroundProxy(srv, srv5, 3000, len(data5), true)
	s, err := p.OpenSurround(context.Background(), testID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Seek(int64(chunkSize), io.SeekStart); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(s)
	if err != nil || !bytes.Equal(b, data5[chunkSize:]) {
		t.Errorf("%d bytes from a seek, %v", len(b), err)
	}
}
