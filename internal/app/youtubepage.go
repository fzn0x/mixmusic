package app

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// YouTubePageSource names lyrics extracted from YouTube video descriptions
// and creator/pinned comments.
const YouTubePageSource = "YouTube video & comments"

const (
	youtubePageWait  = 5 * time.Second
	maxYouTubeVideos = 3
	maxVideoDuration = 15 * time.Minute
)

// youtubePageLyrics returns the highest-scoring plain lyric extract from a
// track's YouTube video or matching YouTube uploads.
func (c *Catalog) youtubePageLyrics(ctx context.Context, track media.Track, q lyrics.Query) (lyrics.Lyrics, error) {
	cands, err := c.searchYouTubePage(ctx, track, q, false)
	if err != nil || len(cands) == 0 {
		if err == nil {
			err = lyrics.ErrNotFound
		}
		return lyrics.Lyrics{}, err
	}
	return cands[0].Lyrics, nil
}

// youtubePageSearch returns all distinct plain lyric candidates found in
// matching YouTube video descriptions and comments.
func (c *Catalog) youtubePageSearch(ctx context.Context, track media.Track, q lyrics.Query) ([]lyrics.Candidate, error) {
	return c.searchYouTubePage(ctx, track, q, true)
}

// youtubePageByID fetches a specific candidate ("<videoID>:desc" or
// "<videoID>:comment:<index>") from a YouTube video page.
func (c *Catalog) youtubePageByID(ctx context.Context, track media.Track, q lyrics.Query, id string) (lyrics.Lyrics, error) {
	if c.a.youtube == nil {
		return lyrics.Lyrics{}, errNotAsked
	}
	videoID, rest, ok := strings.Cut(id, ":")
	if !ok || videoID == "" {
		return lyrics.Lyrics{}, lyrics.ErrNotFound
	}
	if cached, hit := c.youtubePageCached(track, q); hit {
		for _, cand := range cached {
			if cand.ID == id {
				return cand.Lyrics, nil
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, youtubePageWait)
	defer cancel()
	info, err := c.a.youtube.VideoLyricsContext(ctx, videoID)
	if err != nil {
		return lyrics.Lyrics{}, err
	}
	for _, sc := range c.extractYouTubePage(info, 0) {
		if sc.cand.ID == id {
			return sc.cand.Lyrics, nil
		}
	}
	// If the ID wasn't found directly, fall back to a full search in case
	// ordering changed.
	_ = rest
	cands, err := c.youtubePageSearch(ctx, track, q)
	if err != nil {
		return lyrics.Lyrics{}, err
	}
	for _, cand := range cands {
		if cand.ID == id {
			return cand.Lyrics, nil
		}
	}
	return lyrics.Lyrics{}, lyrics.ErrNotFound
}

func youtubePageMemoKey(track media.Track, q lyrics.Query) string {
	return track.ID + "\x00" + q.VideoID + "\x00" + strings.ToLower(strings.TrimSpace(q.Title)) + "\x00" + strings.ToLower(strings.TrimSpace(q.Artist))
}

func (c *Catalog) youtubePageCached(track media.Track, q lyrics.Query) ([]lyrics.Candidate, bool) {
	if c.a.youtubePageMemo == nil {
		return nil, false
	}
	return c.a.youtubePageMemo.get(youtubePageMemoKey(track, q))
}

type ytPageScored struct {
	cand  lyrics.Candidate
	score int
	norm  string
}

func (c *Catalog) searchYouTubePage(ctx context.Context, track media.Track, q lyrics.Query, alwaysSearchWeb bool) ([]lyrics.Candidate, error) {
	if c.a.youtube == nil || strings.TrimSpace(q.Title) == "" {
		return nil, errNotAsked
	}
	fetchCtx, fetchCancel := context.WithTimeout(ctx, youtubePageWait)
	defer fetchCancel()

	var infos []youtube.VideoLyricsContext
	seenVID := map[string]bool{}
	var directErr error

	// 1. Check the track's own YouTube video first when known.
	if vid := strings.TrimSpace(q.VideoID); vid != "" {
		seenVID[vid] = true
		info, err := c.a.youtube.VideoLyricsContext(fetchCtx, vid)
		if err != nil {
			directErr = err
		} else {
			infos = append(infos, info)
			if !alwaysSearchWeb && !lyrics.IsAutoTopicDescription(info.Description) {
				if extracted := c.extractYouTubePage(info, 25); len(extracted) > 0 {
					return finalizeYouTubePage(extracted), nil
				}
			}
		}
	}

	// 2. Search YouTube for the official upload or lyric videos (essential when
	// playing an auto-generated Topic track or Spotify track). Skip web search
	// for DJ sets / long mixes (>15m) where unrelated track results mislead.
	query := strings.TrimSpace(q.Artist + " " + q.Title)
	var webErr error
	if query != "" && (q.Duration == 0 || q.Duration <= maxVideoDuration) {
		res, err := c.a.youtube.SearchWeb(fetchCtx, query)
		if err != nil {
			webErr = err
		} else {
			top := rankYouTubePageVideos(res.Videos, q, seenVID)
			extra := make([]youtube.VideoLyricsContext, len(top))
			extraOK := make([]bool, len(top))
			var wg sync.WaitGroup
			for i, item := range top {
				wg.Go(func() {
					info, err := c.a.youtube.VideoLyricsContext(fetchCtx, item.vid)
					if err == nil {
						extra[i] = info
						extraOK[i] = true
					}
				})
			}
			wg.Wait()
			for i, ok := range extraOK {
				if ok {
					infos = append(infos, extra[i])
				}
			}
		}
	}

	if len(infos) == 0 {
		if directErr != nil {
			return nil, directErr
		}
		if webErr != nil {
			return nil, webErr
		}
		return nil, lyrics.ErrNotFound
	}

	var all []ytPageScored
	for _, info := range infos {
		isDirect := q.VideoID != "" && info.VideoID == q.VideoID
		isOfficial := q.Artist != "" && lyrics.ArtistAffinity(q.Artist, info.Author) >= 25
		if !isDirect && !isOfficial && isDerivativeVideoDescription(q.Title, info.Description) {
			continue
		}
		bonus := 0
		if isDirect {
			bonus = 25
		} else if isOfficial {
			bonus = 20
		}
		all = append(all, c.extractYouTubePage(info, bonus)...)
	}
	if len(all) == 0 {
		return nil, lyrics.ErrNotFound
	}
	out := finalizeYouTubePage(all)
	if c.a.youtubePageMemo != nil {
		c.a.youtubePageMemo.put(youtubePageMemoKey(track, q), out)
	}
	return out, nil
}

type rankedYTVideo struct {
	vid   string
	score float64
}

func rankYouTubePageVideos(videos []media.Track, q lyrics.Query, seen map[string]bool) []rankedYTVideo {
	var ranked []rankedYTVideo
	for _, v := range videos {
		_, vid, err := media.ParseID(v.ID)
		if err != nil || vid == "" || seen[vid] {
			continue
		}
		if v.DurationMs > int(maxVideoDuration.Milliseconds()) {
			continue
		}
		ta := lyrics.TitleAffinity(q.Title, v.Title)
		if ta == 0 {
			continue
		}
		var owner string
		if len(v.Artists) > 0 {
			owner = v.Artists[0].Name
		}
		if strings.HasSuffix(owner, " - Topic") {
			continue
		}
		isOfficialChannel := strings.TrimSpace(q.Artist) != "" && lyrics.ArtistAffinity(q.Artist, owner) >= 25
		if isUnrequestedDerivativeVideo(q.Title, v.Title, isOfficialChannel) {
			continue
		}
		aa := 1.0
		if strings.TrimSpace(q.Artist) != "" {
			aa = max(lyrics.ArtistAffinity(q.Artist, owner), lyrics.ArtistAffinity(q.Artist, v.Title))
			if aa == 0 {
				continue
			}
		}
		sc := ta + aa
		if isOfficialChannel {
			sc += 30
		}
		low := strings.ToLower(v.Title)
		if strings.Contains(low, "lyric") || strings.Contains(low, "official") {
			sc += 15
		}
		if q.Duration > 0 && v.DurationMs > 0 {
			diff := (time.Duration(v.DurationMs)*time.Millisecond - q.Duration).Abs()
			if diff > 20*time.Second {
				sc -= 25
			}
		}
		seen[vid] = true
		ranked = append(ranked, rankedYTVideo{vid: vid, score: sc})
	}
	slices.SortStableFunc(ranked, func(a, b rankedYTVideo) int {
		if a.score > b.score {
			return -1
		}
		if a.score < b.score {
			return 1
		}
		return 0
	})
	if len(ranked) > maxYouTubeVideos {
		ranked = ranked[:maxYouTubeVideos]
	}
	return ranked
}

func (c *Catalog) extractYouTubePage(info youtube.VideoLyricsContext, bonus int) []ytPageScored {
	var out []ytPageScored
	watchURL := "https://www.youtube.com/watch?v=" + info.VideoID
	for i, cm := range info.Comments {
		text, sc, ok := lyrics.ExtractFromComment(cm.Text, cm.IsCreator, cm.IsPinned)
		if !ok {
			continue
		}
		kind := "Comment"
		switch {
		case cm.IsPinned && cm.IsCreator:
			kind = "Pinned comment"
		case cm.IsPinned:
			kind = "Pinned comment"
		case cm.IsCreator:
			kind = "Creator comment"
		}
		label := kind
		if cm.Author != "" {
			label = fmt.Sprintf("%s · %s", kind, cm.Author)
		}
		ly := lyrics.PlainBilingual(text, YouTubePageSource)
		ly.SourceURL = watchURL
		if cm.Author != "" {
			ly.By = cm.Author
		}
		out = append(out, ytPageScored{
			cand: lyrics.Candidate{
				ID:       fmt.Sprintf("%s:comment:%d", info.VideoID, i),
				Title:    info.Title,
				Artist:   label,
				Duration: info.Duration,
				Lyrics:   ly,
				Loaded:   true,
				SyncHint: "plain",
			},
			score: sc + bonus,
			norm:  lyrics.NormMatch(text),
		})
	}
	if text, sc, ok := lyrics.ExtractFromDescription(info.Description); ok {
		label := "Description"
		if info.Author != "" {
			label = fmt.Sprintf("Description · %s", info.Author)
		}
		ly := lyrics.PlainBilingual(text, YouTubePageSource)
		ly.SourceURL = watchURL
		if info.Author != "" {
			ly.By = info.Author
		}
		out = append(out, ytPageScored{
			cand: lyrics.Candidate{
				ID:       info.VideoID + ":desc",
				Title:    info.Title,
				Artist:   label,
				Duration: info.Duration,
				Lyrics:   ly,
				Loaded:   true,
				SyncHint: "plain",
			},
			score: sc + bonus,
			norm:  lyrics.NormMatch(text),
		})
	}
	return out
}

var (
	derivVocalRe     = regexp.MustCompile(`\bvocal\s+(?:cover|interpretation|version|edition|remix|arrangement)\b`)
	derivAddVocalsRe = regexp.MustCompile(`\b(?:with|adding|added)\s+vocals?\b`)
	derivFanLyricsRe = regexp.MustCompile(`\bfan[\s-]*(?:made\s+)?lyrics\b`)
	derivTypedCovRe  = regexp.MustCompile(`\b(?:english|spanish|french|german|japanese|korean|russian|portuguese|italian|acoustic|metal|rock|piano|guitar|drum|bass|ai|orchestral|violin|cello|choir|acapella|a\s+cappella|male|female|band)\s+cover\b`)
	derivLangVerRe   = regexp.MustCompile(`\b(?:english|eng)\s+(?:ver\.?|version|dub|adaptation|fandub)\b`)
	derivOtherRe     = regexp.MustCompile(`\b(?:fandub|parody|mashup|medley|reaction|bootleg)\b`)
	derivInterpRe    = regexp.MustCompile(`\binterpretation\b`)
	derivCoverArtRe  = regexp.MustCompile(`\b(?:album|single|track)?\s*cover\s+art\b`)
	derivBareCoverRe = regexp.MustCompile(`\bcover\b`)
	derivRemixRe     = regexp.MustCompile(`\bremix\b`)
)

// isUnrequestedDerivativeVideo reports whether videoTitle is a fan cover,
// vocal interpretation, parody, or third-party remix that was not requested by
// trackTitle (so fan-written lyrics for an instrumental track are skipped).
func isUnrequestedDerivativeVideo(trackTitle, videoTitle string, isOfficialChannel bool) bool {
	trackLow := strings.ToLower(trackTitle)
	vidLow := strings.ToLower(videoTitle)

	if derivVocalRe.MatchString(vidLow) && !derivVocalRe.MatchString(trackLow) {
		return true
	}
	if derivAddVocalsRe.MatchString(vidLow) && !derivAddVocalsRe.MatchString(trackLow) {
		return true
	}
	if derivFanLyricsRe.MatchString(vidLow) && !derivFanLyricsRe.MatchString(trackLow) {
		return true
	}
	if derivTypedCovRe.MatchString(vidLow) && !derivTypedCovRe.MatchString(trackLow) {
		return true
	}
	if derivLangVerRe.MatchString(vidLow) && !derivLangVerRe.MatchString(trackLow) {
		return true
	}
	if derivOtherRe.MatchString(vidLow) && !derivOtherRe.MatchString(trackLow) {
		return true
	}
	if derivInterpRe.MatchString(vidLow) && !strings.Contains(trackLow, "interpretation") {
		return true
	}

	if !isOfficialChannel {
		vidSansArt := derivCoverArtRe.ReplaceAllString(vidLow, "")
		if derivBareCoverRe.MatchString(vidSansArt) && !strings.Contains(trackLow, "cover") {
			return true
		}
		if derivRemixRe.MatchString(vidLow) && !strings.Contains(trackLow, "remix") {
			return true
		}
	}
	return false
}

// isDerivativeVideoDescription checks a non-official video's description preamble
// for fan-cover or vocal-interpretation disclaimers before extracting lyrics.
func isDerivativeVideoDescription(trackTitle, desc string) bool {
	trackLow := strings.ToLower(trackTitle)
	if strings.Contains(trackLow, "cover") || strings.Contains(trackLow, "vocal") || strings.Contains(trackLow, "interpretation") {
		return false
	}
	low := strings.ToLower(desc)
	if idx := strings.Index(low, "lyrics:"); idx >= 0 && idx < 2000 {
		low = low[:idx]
	} else if len(low) > 1500 {
		low = low[:1500]
	}
	disclaimers := []string{
		"original instrumental",
		"vocal interpretation",
		"vocal cover",
		"vocals would sound like",
		"write some vocals",
		"wrote lyrics for",
		"my own lyrics",
		"fan-made lyrics",
		"fan lyrics",
	}
	for _, phrase := range disclaimers {
		if strings.Contains(low, phrase) {
			return true
		}
	}
	return false
}

func finalizeYouTubePage(items []ytPageScored) []lyrics.Candidate {
	slices.SortStableFunc(items, func(a, b ytPageScored) int { return b.score - a.score })
	seen := map[string]bool{}
	out := make([]lyrics.Candidate, 0, len(items))
	for _, item := range items {
		if item.norm == "" || seen[item.norm] {
			continue
		}
		seen[item.norm] = true
		out = append(out, item.cand)
	}
	return out
}
