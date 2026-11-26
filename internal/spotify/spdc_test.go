package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGenerateTOTP(t *testing.T) {
	// RFC 6238 SHA1 test vector: ASCII "12345678901234567890" = base32 "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	// At t=59 (counter=1), 8-digit RFC value is 94287082 -> 6-digit is 287082.
	code, err := generateTOTP(rfcSecret, 59)
	if err != nil {
		t.Fatal(err)
	}
	if code != "287082" {
		t.Errorf("generateTOTP(59) = %q, want 287082", code)
	}

	// Verify defaultNuance decodes cleanly and produces 6 digits.
	code2, err := generateTOTP(defaultNuance.S, 1795664820)
	if err != nil {
		t.Fatal(err)
	}
	if len(code2) != 6 {
		t.Errorf("len(code2) = %d, want 6 (%q)", len(code2), code2)
	}
}

func TestExchangeSPDCSuccessAndCacheBust(t *testing.T) {
	var nuanceCalls atomic.Int32
	var tokenCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nuances.json":
			if nuanceCalls.Add(1) == 1 {
				w.Write([]byte(`[{"s":"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ","v":60}]`))
				return
			}
			w.Write([]byte(`[{"s":"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ","v":61}]`))
		case "/server-time":
			w.Write([]byte(`{"serverTime":1795664820}`))
		case "/api/token":
			if !strings.Contains(r.Header.Get("Cookie"), "sp_dc=my-cookie") || !strings.Contains(r.Header.Get("Cookie"), "sp_key=my-key") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			q := r.URL.Query()
			if q.Get("reason") != "transport" || q.Get("productType") != "web-player" || q.Get("totp") == "" || q.Get("totp") != q.Get("totpServer") {
				t.Errorf("unexpected token query: %v", q)
			}
			if tokenCalls.Add(1) == 2 && q.Get("totpVer") == "60" {
				// Simulate rotated secret on the second call when v=60 comes from cache.
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"invalid totp"}`))
				return
			}
			exp := time.Now().Add(time.Hour).UnixMilli()
			w.Write([]byte(`{"accessToken":"WEB-TOKEN","accessTokenExpirationTimestampMs":` + strconvFormatInt(exp) + `,"isAnonymous":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	auth := NewAuth("")
	auth.WebTokenURL = srv.URL + "/api/token"
	auth.ServerTimeURL = srv.URL + "/server-time"
	auth.NuancesURL = srv.URL + "/nuances.json"
	auth.NuancesFallbackURL = ""

	// First call populates the nuance cache with v=60.
	if _, err := auth.ExchangeSPDC(context.Background(), "my-cookie", "my-key", "my-t"); err != nil {
		t.Fatal(err)
	}
	// Second call hits the cached v=60, gets HTTP 400, cache-busts to v=61, and succeeds.
	tok, err := auth.ExchangeSPDC(context.Background(), "my-cookie", "my-key", "my-t")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "WEB-TOKEN" || tok.SPDC != "my-cookie" || tok.SPKey != "my-key" || tok.SPT != "my-t" || !tok.IsWebSession() {
		t.Errorf("unexpected token: %+v", tok)
	}
	if nuanceCalls.Load() != 2 || tokenCalls.Load() != 3 {
		t.Errorf("expected cache-bust retry: nuanceCalls=%d tokenCalls=%d", nuanceCalls.Load(), tokenCalls.Load())
	}
}

func TestExchangeSPDCRejectsAnonymousOrEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nuances.json":
			w.Write([]byte(`[{"s":"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ","v":61}]`))
		case "/server-time":
			w.Write([]byte(`{"serverTime":1795664820}`))
		case "/api/token":
			w.Write([]byte(`{"accessToken":"ANON","accessTokenExpirationTimestampMs":1800000000000,"isAnonymous":true}`))
		}
	}))
	defer srv.Close()

	auth := NewAuth("")
	auth.WebTokenURL = srv.URL + "/api/token"
	auth.ServerTimeURL = srv.URL + "/server-time"
	auth.NuancesURL = srv.URL + "/nuances.json"
	auth.NuancesFallbackURL = ""

	if _, err := auth.ExchangeSPDC(context.Background(), " ", "", ""); !errors.Is(err, ErrInvalidSPDC) {
		t.Errorf("empty sp_dc err = %v, want ErrInvalidSPDC", err)
	}
	_, err := auth.ExchangeSPDC(context.Background(), "bad-cookie", "", "")
	if !errors.Is(err, ErrInvalidSPDC) || !errors.Is(err, ErrSignInExpired) {
		t.Errorf("anonymous token err = %v, want ErrInvalidSPDC and ErrSignInExpired", err)
	}
}

func TestClientAutoRefreshesExpiredSPDC(t *testing.T) {
	var refreshed atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nuances.json":
			w.Write([]byte(`[{"s":"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ","v":61}]`))
		case "/server-time":
			w.Write([]byte(`{"serverTime":1795664820}`))
		case "/api/token":
			refreshed.Add(1)
			exp := time.Now().Add(time.Hour).UnixMilli()
			w.Write([]byte(`{"accessToken":"FRESH-WEB","accessTokenExpirationTimestampMs":` + strconvFormatInt(exp) + `,"isAnonymous":false}`))
		}
	}))
	defer srv.Close()

	auth := NewAuth("")
	auth.WebTokenURL = srv.URL + "/api/token"
	auth.ServerTimeURL = srv.URL + "/server-time"
	auth.NuancesURL = srv.URL + "/nuances.json"
	auth.NuancesFallbackURL = ""

	client := NewClient(auth)
	var saved Token
	var wg sync.WaitGroup
	wg.Add(1)
	client.OnToken = func(tok Token) {
		saved = tok
		wg.Done()
	}
	client.SetToken(Token{
		AccessToken: "OLD-WEB",
		Expiry:      time.Now().Add(-time.Minute),
		SPDC:        "cookie-123",
	})
	if !client.SignedIn() || !client.IsWebSession() {
		t.Fatal("client should report SignedIn and IsWebSession")
	}

	got, err := client.AccessToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "FRESH-WEB" || refreshed.Load() != 1 {
		t.Errorf("AccessToken = %q, refreshed = %d", got, refreshed.Load())
	}
	wg.Wait()
	if saved.AccessToken != "FRESH-WEB" || saved.SPDC != "cookie-123" {
		t.Errorf("saved token = %+v", saved)
	}
}

func strconvFormatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [24]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
