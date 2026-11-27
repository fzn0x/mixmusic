package spotify

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	webTokenURL        = "https://open.spotify.com/api/token"
	serverTimeURL      = "https://open.spotify.com/api/server-time"
	nuancesURL         = "https://gist.githubusercontent.com/sonic-liberation/22ed9c6ba463899e933427f7de1f0eef/raw/nuances.json"
	nuancesFallbackURL = "https://api.github.com/gists/22ed9c6ba463899e933427f7de1f0eef"

	webPlayerUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"

	nuanceCacheTTL = 6 * time.Hour
)

// ErrInvalidSPDC is returned when the provided sp_dc cookie is missing,
// invalid, or rejected by Spotify as an anonymous session.
var ErrInvalidSPDC = errors.New("spotify: invalid or expired sp_dc cookie")

type totpNuance struct {
	S string `json:"s"`
	V int    `json:"v"`
}

var defaultNuance = totpNuance{
	S: "GM3TMMJTGYZTQNZVGM4DINJZHA4TGOBYGMZTCMRTGEYDSMJRHE4TEOBUG4YTCMRUGQ4DQOJUGQYTAMRRGA2TCMJSHE3TCMBY",
	V: 61,
}

// generateTOTP computes a 6-digit RFC 6238 HMAC-SHA1 TOTP code for unixSec.
func generateTOTP(base32Secret string, unixSec int64) (string, error) {
	cleaned := strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(base32Secret, "=", "")))
	cleaned = strings.Join(strings.Fields(cleaned), "")
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("spotify: decoding TOTP secret: %w", err)
	}
	counter := uint64(unixSec / 30)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", code%1_000_000), nil
}

func (a *Auth) httpClient() *http.Client {
	if a != nil && a.HTTP != nil {
		return a.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (a *Auth) latestNuance(ctx context.Context, bypassCache bool) (totpNuance, bool) {
	a.nuanceMu.Lock()
	cached := a.nuance
	fresh := cached.S != "" && time.Since(a.nuanceAt) < nuanceCacheTTL
	a.nuanceMu.Unlock()
	if fresh && !bypassCache {
		return cached, true
	}

	rawURL := a.NuancesURL
	if rawURL == "" {
		rawURL = nuancesURL
	}
	n, err := a.fetchNuancesList(ctx, rawURL)
	if err != nil && a.NuancesFallbackURL != "" {
		n, err = a.fetchNuancesGist(ctx, a.NuancesFallbackURL)
	}
	if err == nil && n.S != "" {
		a.nuanceMu.Lock()
		a.nuance = n
		a.nuanceAt = time.Now()
		a.nuanceMu.Unlock()
		return n, false
	}
	if cached.S != "" {
		return cached, false
	}
	return defaultNuance, false
}

func pickHighestNuance(list []totpNuance) (totpNuance, error) {
	var best totpNuance
	for _, n := range list {
		if n.S != "" && (best.S == "" || n.V > best.V) {
			best = n
		}
	}
	if best.S == "" {
		return totpNuance{}, errors.New("spotify: no TOTP secret in nuances response")
	}
	return best, nil
}

func (a *Auth) fetchNuancesList(ctx context.Context, rawURL string) (totpNuance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return totpNuance{}, err
	}
	req.Header.Set("User-Agent", webPlayerUserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := a.httpClient().Do(req)
	if err != nil {
		return totpNuance{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return totpNuance{}, fmt.Errorf("spotify: nuances HTTP %d", res.StatusCode)
	}
	var list []totpNuance
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&list); err != nil {
		return totpNuance{}, err
	}
	return pickHighestNuance(list)
}

func (a *Auth) fetchNuancesGist(ctx context.Context, rawURL string) (totpNuance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return totpNuance{}, err
	}
	req.Header.Set("User-Agent", webPlayerUserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := a.httpClient().Do(req)
	if err != nil {
		return totpNuance{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return totpNuance{}, fmt.Errorf("spotify: gist HTTP %d", res.StatusCode)
	}
	var gist struct {
		Files map[string]struct {
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&gist); err != nil {
		return totpNuance{}, err
	}
	for _, f := range gist.Files {
		var list []totpNuance
		if json.Unmarshal([]byte(f.Content), &list) == nil {
			if n, err := pickHighestNuance(list); err == nil {
				return n, nil
			}
		}
	}
	return totpNuance{}, errors.New("spotify: gist has no nuances")
}

func (a *Auth) serverTime(ctx context.Context) int64 {
	rawURL := a.ServerTimeURL
	if rawURL == "" {
		rawURL = serverTimeURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return time.Now().Unix()
	}
	req.Header.Set("User-Agent", webPlayerUserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := a.httpClient().Do(req)
	if err != nil {
		return time.Now().Unix()
	}
	defer res.Body.Close()
	var body struct {
		ServerTime int64 `json:"serverTime"`
	}
	if res.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&body) == nil && body.ServerTime > 0 {
		return body.ServerTime
	}
	return time.Now().Unix()
}

// ExchangeSPDC trades a Spotify web session's sp_dc cookie (with optional
// sp_key and sp_t cookies) for a web-player access token.
func (a *Auth) ExchangeSPDC(ctx context.Context, spDC, spKey, spT string) (Token, error) {
	spDC = strings.TrimSpace(spDC)
	spKey = strings.TrimSpace(spKey)
	spT = strings.TrimSpace(spT)
	if spDC == "" {
		return Token{}, ErrInvalidSPDC
	}

	nuance, fromCache := a.latestNuance(ctx, false)
	tok, status, err := a.requestWebToken(ctx, spDC, spKey, spT, nuance)
	if err != nil && status != 0 && status != http.StatusOK && fromCache {
		nuance, _ = a.latestNuance(ctx, true)
		tok, _, err = a.requestWebToken(ctx, spDC, spKey, spT, nuance)
	}
	return tok, err
}

func (a *Auth) requestWebToken(ctx context.Context, spDC, spKey, spT string, nuance totpNuance) (Token, int, error) {
	unixSec := a.serverTime(ctx)
	code, err := generateTOTP(nuance.S, unixSec)
	if err != nil {
		return Token{}, 0, err
	}
	rawURL := a.WebTokenURL
	if rawURL == "" {
		rawURL = webTokenURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return Token{}, 0, err
	}
	q := u.Query()
	q.Set("reason", "transport")
	q.Set("productType", "web-player")
	q.Set("totp", code)
	q.Set("totpServer", code)
	q.Set("totpVer", strconv.Itoa(nuance.V))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Token{}, 0, err
	}
	cookie := "sp_dc=" + spDC
	if spKey != "" {
		cookie += "; sp_key=" + spKey
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", webPlayerUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://open.spotify.com")
	req.Header.Set("Referer", "https://open.spotify.com/")

	res, err := a.httpClient().Do(req)
	if err != nil {
		return Token{}, 0, fmt.Errorf("spotify: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Token{}, res.StatusCode, fmt.Errorf("spotify: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		msg := plainText(data)
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			return Token{}, res.StatusCode, fmt.Errorf("%w: %w", ErrInvalidSPDC, ErrSignInExpired)
		}
		if msg != "" {
			return Token{}, res.StatusCode, fmt.Errorf("spotify: web token request failed: %s (HTTP %d)", msg, res.StatusCode)
		}
		return Token{}, res.StatusCode, fmt.Errorf("spotify: web token request failed (HTTP %d)", res.StatusCode)
	}

	var body struct {
		AccessToken string `json:"accessToken"`
		Expiration  int64  `json:"accessTokenExpirationTimestampMs"`
		IsAnonymous bool   `json:"isAnonymous"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return Token{}, res.StatusCode, fmt.Errorf("spotify: web token response: %w", err)
	}
	if body.IsAnonymous || body.AccessToken == "" {
		return Token{}, res.StatusCode, fmt.Errorf("%w: %w", ErrInvalidSPDC, ErrSignInExpired)
	}
	expiry := time.Now().Add(30 * time.Minute)
	if body.Expiration > 0 {
		expiry = time.UnixMilli(body.Expiration)
	}
	return Token{
		AccessToken: body.AccessToken,
		Expiry:      expiry,
		SPDC:        spDC,
		SPKey:       spKey,
		SPT:         spT,
	}, res.StatusCode, nil
}
