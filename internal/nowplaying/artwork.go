//go:build darwin && (amd64 || arm64)

package nowplaying

// macOS shows the artwork MixMusic hands it, which it downloads: MPRIS
// passes its address on.

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// artworkMax bounds the artwork downloaded, in bytes.
const artworkMax = 8 << 20

// artworkClient downloads artwork.
var artworkClient = &http.Client{Timeout: 15 * time.Second}

func download(rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" {
		return nil, errors.New("nowplaying: only https artwork is read")
	}
	res, err := artworkClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nowplaying: artwork: HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, artworkMax))
}
