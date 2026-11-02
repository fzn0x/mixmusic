package settings

import "slices"

// Tools are the tools MixMusic runs: Settings › Playback › Tools.
type Tools struct {
	// YTDLPPath is a yt-dlp of the user's own, which MixMusic runs in place
	// of the one it finds or installs; "" for none.
	YTDLPPath string `json:"ytdlpPath"`
	// JSRuntime is the JavaScript runtime yt-dlp uses: "deno", "node",
	// "bun" or "quickjs"; "" finds one.
	JSRuntime string `json:"jsRuntime"`
}

// clean is t without a JavaScript runtime MixMusic doesn't know.
func (t Tools) clean() Tools {
	if !slices.Contains([]string{"", "deno", "node", "bun", "quickjs"}, t.JSRuntime) {
		t.JSRuntime = ""
	}
	return t
}
