package stream

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// quickArgs are the extractor arguments of yt-dlp's quick run. It asks
// YouTube's player API as yt-dlp's visionos client alone, the client the
// direct route asks as: of the clients of yt-dlp 2026.10.26, the only one
// whose audio streams play with no JavaScript challenge solved and no PO
// token (android's and ios's need one, android_vr's are refused since
// August 2026), and with no pre-roll ad, so YouTube serves them at once;
// yt-dlp picks it alone too when it has no JavaScript runtime. It skips
// the HLS and DASH manifests, which take a request each and hold no
// stream the engine plays but a live one's.
const quickArgs = "youtube:player_client=visionos;skip=hls,dash"

// quickTimeout bounds yt-dlp's quick run, which leaves the rest of
// resolveTimeout to its full run.
const quickTimeout = 30 * time.Second

// ytdlpRun is which run of yt-dlp found a stream, "quick" or "full", and
// how long it took.
type ytdlpRun struct {
	name string
	took time.Duration
}

// findWithYTDLP has yt-dlp find the stream of a video: quickly first, as
// the visionOS client alone, without the JavaScript runtime and without
// the manifests, then, when that finds no audio, the full way, with
// yt-dlp's default clients and the runtime, which solves their challenge,
// several seconds of the computer's time. direct is why the direct route
// found no stream, or nil: when YouTube's player API told that client the
// video has no stream for it, the quick run, which asks as that client
// too, is skipped. Without yt-dlp, the full run isn't tried either.
func (r *Resolver) findWithYTDLP(ctx context.Context, videoID string, quality Quality, direct error) (Stream, ytdlpRun, error) {
	if quickHelps(direct) {
		start := time.Now()
		qctx, cancel := context.WithTimeout(ctx, quickTimeout)
		s, err := r.quickYTDLP(qctx, videoID, quality)
		cancel()
		took := time.Since(start).Round(10 * time.Millisecond)
		switch {
		case err == nil:
			return s, ytdlpRun{"quick", took}, nil
		case errors.Is(err, tools.ErrNoYTDLP), ctx.Err() != nil:
			return Stream{}, ytdlpRun{}, err
		}
		log.Printf("mixmusic: yt-dlp's quick run found no stream of %s in %s (%s); the full run looks for it", videoID, took, err)
	}
	start := time.Now()
	s, err := r.ytdlp(ctx, videoID, quality)
	return s, ytdlpRun{"full", time.Since(start).Round(10 * time.Millisecond)}, err
}

// quickHelps reports whether yt-dlp's quick run may find a stream the
// direct route didn't, for why it didn't: not when YouTube's player API
// told the visionOS client that the video has no stream for it, as of a
// video made for children, or none that plays as given, which the quick
// run, asking as that client, can't make play.
func quickHelps(direct error) bool {
	var refused playabilityError
	switch {
	case errors.As(direct, &refused) && !refused.botCheck():
		return false
	case errors.Is(direct, errNoStream), errors.Is(direct, errNoDirectURL), errors.Is(direct, errNoLiveStream):
		return false
	}
	return true
}

// quickYTDLP asks yt-dlp for the stream of a video quickly: as the
// visionOS client alone, without the JavaScript runtime, and without the
// manifests. It asks signed out, as ytdlp does.
func (r *Resolver) quickYTDLP(ctx context.Context, videoID string, quality Quality) (Stream, error) {
	args := []string{"-J", "--no-playlist", "-f", formats[quality], "--extractor-args", quickArgs}
	if r.Tools != nil {
		args = append(args, withoutJSRuntime(r.Tools.YTDLPArgs(r.CacheDir))...)
	}
	args = append(args, "--", "https://music.youtube.com/watch?v="+videoID)
	out, err := r.Run(ctx, args...)
	if err != nil {
		return Stream{}, err
	}
	s, err := parseInfo(out)
	if err != nil {
		return Stream{}, err
	}
	return s.found(RouteQuick, args), nil
}

// withoutJSRuntime returns args, the arguments of every run of yt-dlp,
// without the JavaScript runtime they name.
func withoutJSRuntime(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--js-runtimes" {
			i++
			continue
		}
		if !strings.HasPrefix(args[i], "--js-runtimes=") {
			out = append(out, args[i])
		}
	}
	return out
}
