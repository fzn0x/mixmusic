package stream

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// TestLiveAccountStream has the yt-dlp MixMusic installed find a song's
// stream as a signed-in account, and logs the audio YouTube listed and
// the one it picked: of a YouTube Music Premium account, Opus of format
// 774, else AAC-LC of 141, near 256 kbit/s. It runs only with a session
// given to it, which it keeps nowhere, as the cookie header of a request
// of a music.youtube.com tab signed in to the account:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_COOKIES='LOGIN_INFO=…; SAPISID=…; …' go test ./internal/stream -run LiveAccountStream -v
//
// MIXMUSIC_YT_RECORD names a folder to keep yt-dlp's answer in, as
// ytdlp-premium.json, its URLs taken out, for the tests here.
func TestLiveAccountStream(t *testing.T) {
	header := os.Getenv("MIXMUSIC_YT_COOKIES")
	if os.Getenv("MIXMUSIC_LIVE") == "" || header == "" {
		t.Skip("set MIXMUSIC_LIVE=1 and MIXMUSIC_YT_COOKIES to find a stream as a signed-in account")
	}
	cookies, err := http.ParseCookie(header)
	if err != nil {
		t.Fatal(err)
	}
	config, _ := os.UserConfigDir()
	tm := tools.New(filepath.Join(config, "MixMusic", "bin"))
	if _, err := tm.YTDLP(); err != nil {
		t.Skip("no yt-dlp")
	}
	if _, _, ok := tm.JSRuntime(); !ok {
		t.Skip("no JavaScript runtime, which yt-dlp needs to ask as an account")
	}
	r := NewResolver(tm, t.TempDir())
	var answer []byte
	r.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		out, err := r.runYTDLP(ctx, args...)
		if slices.Contains(args, "--cookies") {
			answer = out
		}
		return out, err
	}
	r.Session = func() []*http.Cookie { return cookies }
	r.SetAsAccount(true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	start := time.Now()
	s, err := r.Resolve(ctx, songID)
	if err != nil {
		t.Fatal(err)
	}
	// Never the URL: it carries this machine's IP address.
	t.Logf("found in %s, as the account %v: format %d, %s at %.0f kbit/s", time.Since(start).Round(time.Millisecond),
		s.AsAccount, s.Itag(), s.Codec, s.Bitrate)
	var in info
	if err := json.Unmarshal(answer, &in); err == nil {
		for _, f := range in.Formats {
			if f.ACodec != "" && f.ACodec != "none" && (f.VCodec == "" || f.VCodec == "none") {
				t.Logf("listed: format %s, %s at %.0f kbit/s", f.FormatID, f.ACodec, f.ABR)
			}
		}
	}
	if !s.AsAccount {
		t.Fatal("the stream wasn't found as the account; the log above tells why")
	}
	if s.Itag() != 774 && s.Itag() != 141 {
		t.Log("no audio of Premium's: the account may not have Premium, or yt-dlp's clients didn't get it")
	}
	if dir := os.Getenv("MIXMUSIC_YT_RECORD"); dir != "" {
		in.URL = recordedURL(in.FormatID)
		for i := range in.Formats {
			in.Formats[i].URL = recordedURL(in.Formats[i].FormatID)
		}
		data, err := json.MarshalIndent(in, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ytdlp-premium.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// recordedURL stands in for the URL of a format in a recorded answer, which
// carries the user's IP address: tests give it a later expiry.
func recordedURL(formatID string) string {
	return "https://rr1---sn-test.googlevideo.com/videoplayback?expire=1793591621&itag=" + formatID
}
