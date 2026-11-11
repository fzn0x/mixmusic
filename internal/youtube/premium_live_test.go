package youtube

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLivePremium reads the account's menu as a signed-in account, and logs
// what its answer tells of Premium: the GFEEDBACK parameters MixMusic
// reads it from. It runs only with a session given to it, which it keeps
// nowhere:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_COOKIES='SAPISID=…; …' go test ./internal/youtube -run LivePremium -v
//
// MIXMUSIC_YT_RECORD names a folder to keep those parameters in, without
// the account's name or anything else of the answer, under a made-up
// header, as account-menu-premium.json or account-menu-free.json, for the
// tests here.
func TestLivePremium(t *testing.T) {
	header := os.Getenv("MIXMUSIC_YT_COOKIES")
	if os.Getenv("MIXMUSIC_LIVE") == "" || header == "" {
		t.Skip("set MIXMUSIC_LIVE=1 and MIXMUSIC_YT_COOKIES to read a signed-in account's Premium")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := signedInClient(t, header)
	n, err := c.call(ctx, "account/account_menu", map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	premium, told := readPremium(n)
	var params []any
	for _, s := range n.get("responseContext", "serviceTrackingParams").list() {
		for _, p := range s.get("params").list() {
			switch k := p.get("key").str(); k {
			case "logged_in", "premium_membership", "has_unlimited_entitlement":
				params = append(params, map[string]any{"key": k, "value": p.get("value").str()})
				t.Logf("%s: %s", k, p.get("value").str())
			}
		}
	}
	t.Logf("Premium %v, told %v", premium, told)
	if dir := os.Getenv("MIXMUSIC_YT_RECORD"); dir != "" {
		name := "account-menu-free.json"
		if premium {
			name = "account-menu-premium.json"
		}
		// The menu's header, which the tests read the account from, is
		// made up.
		kept := map[string]any{"responseContext": map[string]any{"serviceTrackingParams": []any{
			map[string]any{"service": "GFEEDBACK", "params": params},
		}}, "actions": []any{map[string]any{"openPopupAction": map[string]any{"popup": map[string]any{"multiPageMenuRenderer": map[string]any{
			"header": map[string]any{"activeAccountHeaderRenderer": map[string]any{"accountName": map[string]any{"runs": []any{map[string]any{"text": "Ada Example"}}}}},
		}}}}}}
		data, _ := json.MarshalIndent(kept, "", " ")
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
