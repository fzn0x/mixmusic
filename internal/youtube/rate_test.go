package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

// rating is a request to one of the endpoints that rate.
type rating struct {
	path, auth string
	body       map[string]any
}

// ratingServer answers what the client rates with answer, and keeps the
// requests.
func ratingServer(t *testing.T, status int, answer []byte) (*Client, func() []rating) {
	t.Helper()
	var mu sync.Mutex
	var got []rating
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(data, &body)
		mu.Lock()
		got = append(got, rating{r.URL.Path, r.Header.Get("Authorization"), body})
		mu.Unlock()
		w.WriteHeader(status)
		w.Write(answer)
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	return c, func() []rating {
		mu.Lock()
		defer mu.Unlock()
		return append([]rating(nil), got...)
	}
}

// TestDislike dislikes a video, signed in, as YouTube Music's website
// does: like/dislike rates it, and like/removelike takes the rating back,
// both with the video as the target and the request signed. Signed out,
// nothing is asked.
func TestDislike(t *testing.T) {
	c, requests := ratingServer(t, http.StatusOK, []byte(`{"responseContext":{}}`))
	ctx := context.Background()
	if err := c.SetDisliked(ctx, "vid", true); err != ErrSignedOut {
		t.Errorf("signed out, disliking gave %v", err)
	}
	if n := len(requests()); n != 0 {
		t.Fatalf("signed out, disliking asked YouTube %d times", n)
	}
	c.SetCookies(session)
	if err := c.SetDisliked(ctx, "vid", true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDisliked(ctx, "vid", false); err != nil {
		t.Fatal(err)
	}
	got := requests()
	want := []string{"/youtubei/v1/like/dislike", "/youtubei/v1/like/removelike"}
	if len(got) != len(want) {
		t.Fatalf("requests = %+v", got)
	}
	for i, w := range want {
		r := got[i]
		if r.path != w {
			t.Errorf("request %d went to %s, want %s", i, r.path, w)
		}
		if target, _ := r.body["target"].(map[string]any); target["videoId"] != "vid" || len(target) != 1 {
			t.Errorf("request %d rated %v", i, r.body["target"])
		}
		// The website's dislikeParams are left out, as likeParams are.
		if _, ok := r.body["params"]; ok {
			t.Errorf("request %d sent params: %v", i, r.body["params"])
		}
		if !strings.HasPrefix(r.auth, "SAPISIDHASH ") {
			t.Errorf("request %d wasn't signed: %q", i, r.auth)
		}
	}
	if c.Rejected() {
		t.Error("a dislike YouTube took rejected the session")
	}
}

// TestDislikeRefused takes YouTube's answer to a dislike of a session it
// doesn't take, as recorded signed out: the error tells YouTube's words,
// and the client asks nothing more of the account, as for a like.
func TestDislikeRefused(t *testing.T) {
	answer, err := os.ReadFile("testdata/like-signed-out.json")
	if err != nil {
		t.Fatal(err)
	}
	c, requests := ratingServer(t, http.StatusUnauthorized, answer)
	c.SetCookies(session)
	err = c.SetDisliked(context.Background(), "vid", true)
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusUnauthorized || api.Message != "You must be signed in to perform this operation." {
		t.Fatalf("SetDisliked() = %v", err)
	}
	if !c.Rejected() {
		t.Error("a 401 to a signed dislike didn't reject the session")
	}
	if err := c.SetDisliked(context.Background(), "vid", false); !errors.Is(err, ErrSessionRejected) {
		t.Errorf("after the refusal, SetDisliked() = %v, want ErrSessionRejected", err)
	}
	if n := len(requests()); n != 1 {
		t.Errorf("YouTube was asked %d times, want once", n)
	}
}
