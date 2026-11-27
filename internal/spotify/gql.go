package spotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

const (
	gqlURL          = "https://api-partner.spotify.com/pathfinder/v2/query"
	remoteHashesURL = "https://francescograzioso.github.io/Meld/spotify-gql-hashes.json"

	hashSyncCooldown = 5 * time.Minute
)

type hashEntry struct {
	Hash         string
	PreviousHash string
}

type gqlHashStore struct {
	mu       sync.Mutex
	entries  map[string]hashEntry
	syncedAt time.Time
}

func newGQLHashStore() *gqlHashStore {
	return &gqlHashStore{
		entries: map[string]hashEntry{
			"profileAttributes": {
				Hash:         "08ffb4730af3746e04a8301396f20875dbbce10c75243803091a9274eacc8ac0",
				PreviousHash: "53bcb064f6cd18c23f752bc324a791194d20df612d8e1239c735144ab0399ced",
			},
			"libraryV3": {
				Hash:         "390c78e5b951029bad359785e69b07b536a509c581cbcd0aded5e5067f187455",
				PreviousHash: "973e511ca44261fda7eebac8b653155e7caee3675abb4fb110cc1b8c78b091c3",
			},
			"fetchLibraryTracks": {
				Hash: "087278b20b743578a6262c2b0b4bcd20d879c503cc359a2285baf083ef944240",
			},
			"fetchPlaylist": {
				Hash:         "21ea11a6ac75a5f3e01fd365d29c18e0a4a8b7dc09f89bbb5d73910190c6acaf",
				PreviousHash: "346811f856fb0b7e4f6c59f8ebea78dd081c6e2fb01b77c954b26259d5fc6763",
			},
			"getAlbum": {
				Hash:         "0b6ce3511e1b02e4a40fa95440f992acbe85080eb308af9194322d708fc1914f",
				PreviousHash: "b9bfabef66ed756e5e13f68a942deb60bd4125ec1f1be8cc42769dc0259b4b10",
			},
			"queryAlbumTracks": {
				Hash: "0b6ce3511e1b02e4a40fa95440f992acbe85080eb308af9194322d708fc1914f",
			},
			"queryArtistOverview": {
				Hash:         "eafb1620d783ece118e72fbfede2b0064483e029d3c1aafafa1c2de0e25a89e7",
				PreviousHash: "5b9e64f43843fa3a9b6a98543600299b0a2cbbbccfdcdcef2402eb9c1017ca4c",
			},
			"queryArtistDiscographyAll": {
				Hash: "5e07d323febb57b4a56a42abbf781490e58764aa45feb6e3dc0591564fc56599",
			},
			"searchDesktop": {
				Hash:         "4801118d4a100f756e833d33984436a3899cff359c532f8fd3aaf174b60b3b49",
				PreviousHash: "db61238974d27839a136c9dc02bfdbe3fab7635f21cf85976ebff9a1ee281345",
			},
			"home": {
				Hash:         "5fb7da7a03ee2776c0670856609cb85a189a3cecc36cc160b91cde79063b9cf1",
				PreviousHash: "3357ffed7961629ba92b4e0a41514e4d5004a14355c964c23ce442205c9e44a1",
			},
			"getTrack": {
				Hash:         "e7b1bf4e95cb3d733d48b442f93006e9712fd504e8b88d7a82e6e3bac9306a6b",
				PreviousHash: "612585ae06ba435ad26369870deaae23b5c8800a256cd8a57e08eddc25a37294",
			},
			"addToLibrary": {
				Hash:         "896ebcb47815681340860d121cb5d494e157e2a78d3950385cd54e0393c67148",
				PreviousHash: "7c5a69420e2bfae3da5cc4e14cbc8bb3f6090f80afc00ffc179177f19be3f33d",
			},
			"removeFromLibrary": {
				Hash:         "896ebcb47815681340860d121cb5d494e157e2a78d3950385cd54e0393c67148",
				PreviousHash: "7c5a69420e2bfae3da5cc4e14cbc8bb3f6090f80afc00ffc179177f19be3f33d",
			},
			"areEntitiesInLibrary": {
				Hash: "134337999233cc6fdd6b1e6dbf94841409f04a946c5c7b744b09ba0dfe5a85ed",
			},
			"addToPlaylist": {
				Hash:         "b907ad6b088d5bf7680fa81198996d58398c2d55dfcdf59fcc959e0be98a418a",
				PreviousHash: "47b2a1234b17748d332dd0431534f22450e9ecbb3d5ddcdacbd83368636a0990",
			},
			"removeFromPlaylist": {
				Hash:         "b907ad6b088d5bf7680fa81198996d58398c2d55dfcdf59fcc959e0be98a418a",
				PreviousHash: "47b2a1234b17748d332dd0431534f22450e9ecbb3d5ddcdacbd83368636a0990",
			},
			"moveItemsInPlaylist": {
				Hash:         "b907ad6b088d5bf7680fa81198996d58398c2d55dfcdf59fcc959e0be98a418a",
				PreviousHash: "47b2a1234b17748d332dd0431534f22450e9ecbb3d5ddcdacbd83368636a0990",
			},
			"addItemsToRootlist": {
				Hash: "bd9c5cae1ee80ebca05d7ed12fd394216f49a20ce72d9dc762868df0f14522ea",
			},
			"removeItemsFromRootlist": {
				Hash: "3422f1866532820a1d8d0560f88542dbdbd35d0377ee5ecd49593590f5f1e86b",
			},
			"queryShowMetadataV2": {
				Hash: "b475447846f37cc426add800b995a7859c169c53c72b62de6338c0c40a5dacea",
			},
			"queryPodcastEpisodes": {
				Hash: "3539d746cf882f3909660de40b4ef472b3f5893a0761d617c282541de5d412c5",
			},
			"getEpisodeOrChapter": {
				Hash: "5f77db47e5a2ce6680330bb61317aa09e17ddd1d189513cb218e7cd3e43288be",
			},
		},
	}
}

func (s *gqlHashStore) hash(op string) (primary, prev string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[op]
	return e.Hash, e.PreviousHash
}

// syncRemote refreshes persisted-query hashes from the remote registry, ignoring
// entries whose status is not "verified" (e.g. "not_in_bundle").
func (s *gqlHashStore) syncRemote(ctx context.Context, httpClient *http.Client, hashesURL string) bool {
	if hashesURL == "" {
		return false
	}
	s.mu.Lock()
	if !s.syncedAt.IsZero() && time.Since(s.syncedAt) < hashSyncCooldown {
		s.mu.Unlock()
		return false
	}
	s.syncedAt = time.Now()
	s.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hashesURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", webPlayerUserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false
	}
	var payload struct {
		Operations map[string]struct {
			Hash         string `json:"hash"`
			PreviousHash string `json:"previous_hash"`
			Status       string `json:"status"`
		} `json:"operations"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&payload); err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	updated := false
	for op, remote := range payload.Operations {
		if remote.Hash == "" || (remote.Status != "" && remote.Status != "verified") {
			continue
		}
		cur := s.entries[op]
		if cur.Hash != remote.Hash {
			updated = true
		}
		prev := remote.PreviousHash
		if prev == "" && cur.Hash != "" && cur.Hash != remote.Hash {
			prev = cur.Hash
		}
		s.entries[op] = hashEntry{Hash: remote.Hash, PreviousHash: prev}
	}
	return updated
}

func (c *Client) ensureHashes() *gqlHashStore {
	if c.hashes == nil {
		c.hashes = newGQLHashStore()
	}
	return c.hashes
}

func (c *Client) gqlEndpoint() string {
	if c.GQLURL != "" {
		return c.GQLURL
	}
	return gqlURL
}

func (c *Client) gqlHashesURL() string {
	if c.GQLURL != "" && c.GQLURL != gqlURL && c.HashesURL == remoteHashesURL {
		return ""
	}
	return c.HashesURL
}

// gql invokes a persisted Pathfinder GraphQL operation and decodes its `data`
// object into out.
func (c *Client) gql(ctx context.Context, op string, vars map[string]any, out any) error {
	if vars == nil {
		vars = map[string]any{}
	}
	store := c.ensureHashes()
	primary, prev := store.hash(op)
	if primary == "" {
		return fmt.Errorf("spotify: unknown GraphQL operation %q", op)
	}

	hashes := []string{primary}
	if prev != "" && prev != primary {
		hashes = append(hashes, prev)
	}

	refreshed, remoteSynced := false, false
	for i := 0; i < len(hashes); i++ {
		hash := hashes[i]
		data, retryHash, retryAuth, err := c.gqlAttempt(ctx, op, hash, vars, &refreshed)
		if retryAuth {
			i--
			continue
		}
		if retryHash {
			if i+1 < len(hashes) {
				continue
			}
			if !remoteSynced {
				remoteSynced = true
				if store.syncRemote(ctx, c.HTTP, c.gqlHashesURL()) {
					newPrimary, newPrev := store.hash(op)
					for _, candidate := range []string{newPrimary, newPrev} {
						if candidate != "" && candidate != hash {
							hashes = append(hashes, candidate)
						}
					}
					if i+1 < len(hashes) {
						continue
					}
				}
			}
			return fmt.Errorf("spotify: GraphQL query %s rejected (PersistedQueryNotFound)", op)
		}
		if err != nil {
			return err
		}
		if out == nil || len(bytes.TrimSpace(data)) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("spotify: decoding %s: %w", op, err)
		}
		return nil
	}
	return fmt.Errorf("spotify: GraphQL query %s failed", op)
}

func (c *Client) gqlAttempt(ctx context.Context, op, hash string, vars map[string]any, refreshed *bool) (data json.RawMessage, retryHash, retryAuth bool, err error) {
	reqBody := struct {
		Variables     map[string]any `json:"variables"`
		OperationName string         `json:"operationName"`
		Extensions    struct {
			PersistedQuery struct {
				Version    int    `json:"version"`
				SHA256Hash string `json:"sha256Hash"`
			} `json:"persistedQuery"`
		} `json:"extensions"`
	}{
		Variables:     vars,
		OperationName: op,
	}
	reqBody.Extensions.PersistedQuery.Version = 1
	reqBody.Extensions.PersistedQuery.SHA256Hash = hash

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, false, false, err
	}

	waited := false
	var retryAfter time.Duration
	endpoint := c.gqlEndpoint()

	for {
		token, err := c.accessToken(ctx, false)
		if err != nil {
			return nil, false, false, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, false, false, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", webPlayerUserAgent)
		req.Header.Set("app-platform", "WebPlayer")
		req.Header.Set("Origin", "https://open.spotify.com")
		req.Header.Set("Referer", "https://open.spotify.com/")

		res, err := c.HTTP.Do(req)
		if err != nil {
			return nil, false, false, fmt.Errorf("spotify: %w", err)
		}
		raw, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		res.Body.Close()
		if err != nil {
			return nil, false, false, fmt.Errorf("spotify: %w", err)
		}

		switch {
		case res.StatusCode == http.StatusUnauthorized && !*refreshed:
			*refreshed = true
			if _, err := c.accessToken(ctx, true); err != nil {
				return nil, false, false, err
			}
			return nil, false, true, nil
		case res.StatusCode == http.StatusPreconditionFailed:
			return nil, true, false, nil
		case res.StatusCode == http.StatusTooManyRequests && !waited:
			waited = true
			wait, _ := strconv.Atoi(res.Header.Get("Retry-After"))
			retryAfter = time.Duration(wait) * time.Second
			if retryAfter <= maxRetryAfter {
				select {
				case <-time.After(max(retryAfter/retryAfterShorter, 500*time.Millisecond/retryAfterShorter)):
					continue
				case <-ctx.Done():
					return nil, false, false, ctx.Err()
				}
			}
			if ok, err := media.PageWait(ctx, retryAfter, retryAfter/retryAfterShorter); err != nil {
				return nil, false, false, err
			} else if ok {
				continue
			}
		case res.StatusCode == http.StatusTooManyRequests:
			wait, _ := strconv.Atoi(res.Header.Get("Retry-After"))
			retryAfter = time.Duration(wait) * time.Second
		}

		if res.StatusCode >= 300 {
			apiErr := apiError(res.StatusCode, raw)
			if e, ok := apiErr.(*APIError); ok {
				e.RetryAfter = retryAfter
			}
			return nil, false, false, apiErr
		}

		var env struct {
			Data   json.RawMessage `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, false, false, fmt.Errorf("spotify: decoding %s: %w", op, err)
		}
		if len(env.Errors) > 0 {
			msg := env.Errors[0].Message
			lower := strings.ToLower(msg)
			if strings.Contains(lower, "persistedquerynotfound") {
				return nil, true, false, nil
			}
			if strings.Contains(lower, "not authorized") {
				if !*refreshed {
					*refreshed = true
					if _, err := c.accessToken(ctx, true); err != nil {
						return nil, false, false, err
					}
					return nil, false, true, nil
				}
				return nil, false, false, &APIError{Status: http.StatusUnauthorized, Message: msg}
			}
			if emptyGQLData(env.Data) {
				return nil, false, false, &APIError{Status: http.StatusBadRequest, Message: msg}
			}
		}
		return env.Data, false, false, nil
	}
}

func emptyGQLData(data json.RawMessage) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}"))
}
