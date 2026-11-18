package stream

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ytAAC is an AAC format, as yt-dlp lists it, with fields that replace its
// own.
func ytAAC(id string, abr float64, fields string) string {
	f := fmt.Sprintf(`"format_id": "%s", "url": "https://example.com/%s", "ext": "m4a", "protocol": "https", "acodec": "mp4a.40.2",
		"vcodec": "none", "abr": %v, "audio_channels": 2, "filesize": 4000, "has_drm": false`, id, id, abr)
	if fields != "" {
		f += ", " + fields
	}
	return "{" + f + "}"
}

// TestYTDLPKeepsAACBesideOpus keeps the AAC-LC of a video beside its Opus,
// for a cast device that plays no WebM: in stereo, of the original
// dynamics, unencrypted, whole.
func TestYTDLPKeepsAACBesideOpus(t *testing.T) {
	for _, c := range []struct {
		name    string
		formats []string
		want    string
	}{
		{"none", nil, ""},
		{"itag 140", []string{ytAAC("140", 129, "")}, "140"},
		{"the original dynamics", []string{ytAAC("140-drc", 140, ""), ytAAC("140", 129, "")}, "140"},
		{"of the most bits", []string{ytAAC("140a", 96, ""), ytAAC("140b", 129, "")}, "140b"},
		{"HE-AAC", []string{ytAAC("139", 48, `"acodec": "mp4a.40.5"`)}, ""},
		{"encrypted", []string{ytAAC("140", 129, `"has_drm": true`)}, ""},
		{"of six channels", []string{ytAAC("141", 384, `"audio_channels": 6`)}, ""},
		{"in fragments", []string{ytAAC("140", 129, `"protocol": "m3u8_native"`)}, ""},
	} {
		s, err := parseInfo(surroundInfo(c.formats...))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		switch {
		case c.want == "" && s.AAC != nil:
			t.Errorf("%s: AAC %+v", c.name, s.AAC)
		case c.want == "":
		case s.AAC == nil:
			t.Errorf("%s: no AAC", c.name)
		case s.AAC.URL != "https://example.com/"+c.want || s.AAC.MimeType != "audio/mp4" || s.AAC.Size != 4000:
			t.Errorf("%s: AAC %+v", c.name, s.AAC)
		}
	}
	// A stream of AAC itself has none beside.
	s, err := parseInfo([]byte(`{"format_id": "140", "url": "https://example.com/140", "ext": "m4a", "acodec": "mp4a.40.2",
		"formats": [` + ytAAC("140", 129, "") + `]}`))
	if err != nil || s.AAC != nil {
		t.Errorf("AAC beside AAC: %+v %v", s.AAC, err)
	}
}

// TestDirectFindsAACBesideOpus keeps the AAC-LC of itag 140 beside the
// Opus the direct route finds.
func TestDirectFindsAACBesideOpus(t *testing.T) {
	f := newFakeTube(t)
	f.answers(func(playerRequest) []byte { return f.recorded("") })
	s, err := f.direct().Stream(context.Background(), songID, QualityBest)
	if err != nil {
		t.Fatal(err)
	}
	if s.Codec != "opus" || s.AAC == nil || s.AAC.Codec != "mp4a.40.2" || !strings.Contains(s.AAC.URL, "itag=140") ||
		!strings.HasPrefix(s.AAC.MimeType, "audio/mp4") {
		t.Errorf("stream %+v, AAC %+v", s, s.AAC)
	}
}

// TestProxyServesAAC serves a video's AAC at its own path, cached by its
// own name, never what is kept, which holds the stream's own format.
func TestProxyServesAAC(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 3000, &minGen)
	srvAAC, dataAAC := upstream(t, 4000, &minGen)
	for i := range dataAAC {
		dataAAC[i] ^= 0x33
	}
	r := NewResolver(nil, "")
	r.Run = func(context.Context, ...string) ([]byte, error) {
		expire := time.Now().Add(6 * time.Hour).Unix()
		return fmt.Appendf(nil, `{"url": "%s/a?expire=%d", "ext": "webm", "acodec": "opus", "filesize": %d, "formats": [
			{"format_id": "140", "url": "%s/a?expire=%d", "ext": "m4a", "protocol": "https", "acodec": "mp4a.40.2", "vcodec": "none",
			"audio_channels": 2, "filesize": %d}]}`, srv.URL, expire, len(data), srvAAC.URL, expire, len(dataAAC)), nil
	}
	p := NewProxy(r)
	p.Cache = NewCache(t.TempDir(), 1<<20)
	p.Kept = keptStore{testID: "the Opus downloaded"}
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}
	if rec := get(AACPath(testID)); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), dataAAC) || rec.Header().Get("Content-Type") != "audio/mp4" {
		t.Fatalf("AAC: %d, %d bytes of %q", rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
	if !p.Cache.Has(AACKey(testID)) || p.Cache.Has(testID) {
		t.Error("the AAC isn't cached by its own name")
	}
	if rec := get(Path(testID)); rec.Body.String() != "the Opus downloaded" {
		t.Errorf("the stream: %q", rec.Body.String())
	}
	if rec := get(Path(testID) + AACSuffix + SurroundSuffix); rec.Code != 404 {
		t.Errorf("AAC and 5.1 at once: %d", rec.Code)
	}
}
