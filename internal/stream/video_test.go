package stream

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
)

// The music video of testdata/ytdlp-omv.json and visionos-player-video.json.
const musicVideoID = "Kq-7hp9i4lE"

// itags returns the itags of vs, in order.
func itags(vs []Video) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Itag
	}
	return out
}

// TestYTDLPVideos lists the video streams yt-dlp found beside the audio, in
// the run that found the audio: those served whole over HTTPS, without
// sound. Storyboards, HLS and the audio go.
func TestYTDLPVideos(t *testing.T) {
	raw, err := os.ReadFile("testdata/ytdlp-omv.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := parseInfo(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.Codec != "opus" || s.Size != 7705312 || !strings.HasPrefix(s.MimeType, "audio/webm") {
		t.Errorf("the audio changed: %+v", s)
	}
	want := []string{"160", "278", "394", "133", "242", "395", "134", "243", "396", "135", "244", "397", "298", "302", "398", "299", "303", "399"}
	if got := itags(s.Videos); !slices.Equal(got, want) {
		t.Fatalf("videos %v, want %v", got, want)
	}
	families := map[string]int{}
	for _, v := range s.Videos {
		families[v.Family]++
		if v.URL == "" || v.Header.Get("User-Agent") == "" || v.Size <= 0 || v.Width <= 0 || v.Height <= 0 {
			t.Errorf("%s: %+v", v.Itag, v)
		}
	}
	if families["vp9"] != 6 || families["avc1"] != 6 || families["av01"] != 6 {
		t.Errorf("families %v", families)
	}
	i := slices.IndexFunc(s.Videos, func(v Video) bool { return v.Itag == "243" })
	if v := s.Videos[i]; v.MimeType != `video/webm; codecs="vp9"` || v.Width != 640 || v.Height != 360 || v.FPS != 30 || v.Size != 21115748 {
		t.Errorf("243: %+v", v)
	}
	i = slices.IndexFunc(s.Videos, func(v Video) bool { return v.Itag == "134" })
	if v := s.Videos[i]; v.MimeType != `video/mp4; codecs="avc1.4D401E"` || v.Family != "avc1" {
		t.Errorf("134: %+v", v)
	}
}

// TestDirectVideos lists the video streams YouTube's player API gave the
// direct route, with the audio it plays.
func TestDirectVideos(t *testing.T) {
	f := newFakeTube(t)
	f.answersWith("-video")
	s, err := f.direct().Stream(context.Background(), musicVideoID, QualityBest)
	if err != nil {
		t.Fatal(err)
	}
	if s.Codec != "opus" {
		t.Errorf("the audio is %s", s.Codec)
	}
	if len(s.Videos) != 18 {
		t.Fatalf("%d videos: %v", len(s.Videos), itags(s.Videos))
	}
	for _, v := range s.Videos {
		if v.Header.Get("User-Agent") != visionAgent || v.Size <= 0 || v.Family == "" {
			t.Errorf("%s: %+v", v.Itag, v)
		}
	}
	i := slices.IndexFunc(s.Videos, func(v Video) bool { return v.Itag == "134" })
	if i < 0 {
		t.Fatal("no 134")
	}
	if v := s.Videos[i]; v.MimeType != `video/mp4; codecs="avc1.4D401E"` || v.Width != 640 || v.Height != 360 || v.FPS != 30 || v.Size != 31051398 {
		t.Errorf("134: %+v", v)
	}
}

func TestPickVideo(t *testing.T) {
	raw, err := os.ReadFile("testdata/ytdlp-omv.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := parseInfo(raw)
	if err != nil {
		t.Fatal(err)
	}
	portrait := []Video{
		{Itag: "p360", Family: "vp9", Width: 360, Height: 640, FPS: 30},
		{Itag: "p480", Family: "vp9", Width: 480, Height: 854, FPS: 30},
		{Itag: "p720", Family: "vp9", Width: 720, Height: 1280, FPS: 30},
	}
	smooth := []Video{
		{Itag: "60", Family: "vp9", Width: 640, Height: 360, FPS: 60, Bitrate: 400},
		{Itag: "30", Family: "vp9", Width: 640, Height: 360, FPS: 30, Bitrate: 500},
	}
	for _, c := range []struct {
		name     string
		vs       []Video
		families []string
		w, h     int
		saver    bool
		want     string
	}{
		// The largest frame, 420 × 236, at 1.5 pixels a point.
		{"vp9 first", s.Videos, []string{"vp9", "avc1"}, 630, 354, false, "243"},
		{"avc1 first", s.Videos, []string{"avc1", "vp9"}, 630, 354, false, "134"},
		{"av01 only", s.Videos, []string{"av01"}, 630, 354, false, "396"},
		{"a family it lacks", s.Videos, []string{"hev1", "vp9"}, 630, 354, false, "243"},
		// At twice that, 480p; larger frames take 480p too.
		{"retina", s.Videos, []string{"vp9"}, 840, 472, false, "244"},
		{"no more than 480p", s.Videos, []string{"vp9"}, 1300, 731, false, "244"},
		{"data saver", s.Videos, []string{"vp9", "avc1"}, 630, 354, true, "278"},
		{"a small frame", s.Videos, []string{"avc1"}, 390, 219, false, "133"},
		{"portrait", portrait, []string{"vp9"}, 354, 630, false, "p360"},
		{"portrait, large", portrait, []string{"vp9"}, 700, 1200, false, "p480"},
		{"30 fps first", smooth, []string{"vp9"}, 630, 354, false, "30"},
		{"30 fps first, saver", smooth, []string{"vp9"}, 630, 354, true, "30"},
		{"30 fps first, largest", smooth, []string{"vp9"}, 2000, 2000, false, "30"},
	} {
		v, ok := PickVideo(c.vs, c.families, c.w, c.h, c.saver)
		if !ok || v.Itag != c.want {
			t.Errorf("%s: picked %q, %v; want %s", c.name, v.Itag, ok, c.want)
		}
	}
	if v, ok := PickVideo(s.Videos, nil, 630, 354, false); ok {
		t.Errorf("no codec the page plays: picked %s", v.Itag)
	}
	if v, ok := PickVideo(s.Videos, []string{"hev1"}, 630, 354, false); ok {
		t.Errorf("a codec the video lacks: picked %s", v.Itag)
	}
	if v, ok := PickVideo(nil, []string{"vp9"}, 630, 354, false); ok {
		t.Errorf("no video: picked %s", v.Itag)
	}
}
