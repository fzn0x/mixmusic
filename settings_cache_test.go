package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

func TestParseCacheSizeMB(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"2500", 2500, true},
		{"2500 MB", 2500, true},
		{"2500 mib", 2500, true},
		{"2.5 GB", 2560, true},
		{"2,5 gb", 2560, true},
		{"1 TB", settings.MaxCacheSizeMB, true},
		{"", 0, false},
		{"0", 0, false},
		{"-10", 0, false},
		{"abc", 0, false},
		{"2 TB", 0, false},
	} {
		got, ok := parseCacheSizeMB(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseCacheSizeMB(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestCacheUnlimitedAndCustomUI(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")

	// Select Unlimited.
	click(t, s, tt, "Cache size")
	click(t, s, tt, "Unlimited")
	until(t, s, tt, "unlimited saved", func() bool {
		return s.svc.Preferences.Get().CacheSizeMB == settings.UnlimitedCache
	})
	if !tt.HasText("Unlimited: every track that played stays.") {
		t.Errorf("unlimited description missing: %q", tt.Texts())
	}

	// Select Custom and enter 2.5 GB, then 3000 MB, then invalid 0.
	click(t, s, tt, "Cache size")
	click(t, s, tt, "Custom")
	until(t, s, tt, "custom input shown", func() bool {
		return tt.HasText("Custom cache size")
	})

	clickRole(t, s, tt, ui.RoleTextField, "Custom cache size")
	tt.Type("2.5 GB")
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().CacheSizeMB; got != 2560 {
		t.Fatalf("after 2.5 GB: CacheSizeMB = %d, want 2560", got)
	}

	clickRole(t, s, tt, ui.RoleTextField, "Custom cache size")
	tt.Key(0, ui.KeyEnd)
	for range 16 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("3000 MB")
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().CacheSizeMB; got != 3000 {
		t.Fatalf("after 3000 MB: CacheSizeMB = %d, want 3000", got)
	}

	clickRole(t, s, tt, ui.RoleTextField, "Custom cache size")
	tt.Key(0, ui.KeyEnd)
	for range 16 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("0")
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().CacheSizeMB; got != 3000 {
		t.Errorf("invalid input changed CacheSizeMB to %d", got)
	}
	if !tt.HasText("Enter a cache size between 1 MB and 1 TB") {
		t.Errorf("range error toast missing: %q", tt.Texts())
	}
}

func TestCacheMergeSettingsUI(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")

	clickRole(t, s, tt, ui.RoleSwitch, "Merge with downloads")
	until(t, s, tt, "merge enabled", func() bool {
		return s.svc.Preferences.Get().CacheWithDownloads
	})
	wantFolder := filepath.Join(s.svc.Downloads.Folder(), "audio")
	if got := s.svc.System.CacheFolder(); !samePath(got, wantFolder) {
		t.Errorf("merged CacheFolder = %q, want %q", got, wantFolder)
	}

	clickRole(t, s, tt, ui.RoleSwitch, "Merge with downloads")
	until(t, s, tt, "merge disabled", func() bool {
		return !s.svc.Preferences.Get().CacheWithDownloads
	})
	if got, want := s.svc.System.CacheFolder(), s.svc.System.DefaultCacheFolder(); !samePath(got, want) {
		t.Errorf("unmerged CacheFolder = %q, want %q", got, want)
	}
}

func TestPseudoLocaleCacheSettings(t *testing.T) {
	usePseudoLocale(t)
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.CacheSizeMB = 2560
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt := testerOf(t, s, 1280, 4000)
	tt.SetLocale("en-XA")
	show(s, tt, "/settings/playback")
	// Verify cache folder path (if shown) is allowed as non-localized text.
	var paths []string
	if dir := s.svc.System.CacheFolder(); dir != "" {
		paths = append(paths, dir)
	}
	if dir := s.svc.Downloads.Folder(); dir != "" {
		paths = append(paths, dir)
	}
	// Only check rows when no system path leaks into non-path texts.
	for _, txt := range tt.Texts() {
		if strings.Contains(txt, string(os.PathSeparator)) {
			paths = append(paths, txt)
		}
	}
	checkPseudo(t, "Settings › Playback (cache)", tt.Texts(), paths...)
}
