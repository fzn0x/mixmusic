package youtube

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"
)

// TestLiveSessionAccounts tells, of a session signed in to several Google
// accounts, which of them are still signed in, and which the accounts
// list names, asked as the first: if it names that one alone, the list of
// the Account and channel row needs another source (#582). It logs indexes
// alone, nothing of the accounts, and runs only with a session given to
// it, which it keeps nowhere:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_COOKIES='SAPISID=…; …' go test ./internal/youtube -run LiveSessionAccounts -v
func TestLiveSessionAccounts(t *testing.T) {
	header := os.Getenv("MIXMUSIC_YT_COOKIES")
	if os.Getenv("MIXMUSIC_LIVE") == "" || header == "" {
		t.Skip("set MIXMUSIC_LIVE=1 and MIXMUSIC_YT_COOKIES to read the accounts of a session")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := signedInClient(t, header)
	var live []int
	for i := range maxAccounts {
		_, err := c.probeAccount(ctx, &i)
		switch {
		case err == nil:
			live = append(live, i)
		case !errors.Is(err, errAccountOut):
			t.Logf("index %d: %v", i, err)
		}
	}
	t.Logf("signed in: the accounts of indexes %v", live)
	if len(live) == 0 {
		t.Fatal("no account of the session is signed in")
	}
	if _, err := c.UseFirstAccount(ctx); err != nil {
		t.Fatal(err)
	}
	list, err := c.Channels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var listed []int
	for _, ch := range list {
		if !slices.Contains(listed, ch.AuthUser) {
			listed = append(listed, ch.AuthUser)
		}
	}
	t.Logf("the accounts list names the accounts of indexes %v, of %d channels", listed, len(list))
	if len(live) > 1 && len(listed) < 2 {
		t.Error("the accounts list names the account asked as alone")
	}
}
