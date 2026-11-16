package lastfm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUserAgent introduces MixMusic as the app tells it, or as itself.
func TestUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	c := NewClient("KEY", "SECRET")
	c.BaseURL = srv.URL
	for _, agent := range []string{"MixMusic/1.2.3 (https://github.com/fzn0x/mixmusic)", ""} {
		c.UserAgent = agent
		if err := c.NowPlaying(context.Background(), "SK", Track{Artist: "A", Title: "T"}); err != nil {
			t.Fatal(err)
		}
		want := agent
		if want == "" {
			want = "MixMusic"
		}
		if got != want {
			t.Errorf("User-Agent = %q, want %q", got, want)
		}
	}
}
