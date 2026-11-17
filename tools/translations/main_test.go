package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// catalogs writes the catalogs, by file name, into a new folder.
func catalogs(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func output(t *testing.T, dir string, actions bool) string {
	t.Helper()
	var b strings.Builder
	if err := run(&b, dir, actions); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

const en = `{
  "@playButton": {"description": "a button"},
  "playButton": "Play",
  "queueTitle": "Queue",
  "songsCount": "{count, plural, one {# song} other {# songs}}"
}`

func TestLacksAreSaidNotFailed(t *testing.T) {
	dir := catalogs(t, map[string]string{
		"en.json":      en,
		"fr.json":      `{"playButton": "Lire", "oldKey": "Ancien", "mygo.appName": "MixMusic"}`,
		"zh-Hans.json": `{"playButton": "播放", "queueTitle": "队列", "songsCount": "{count, plural, other {# 首歌}}"}`,
	})
	got := output(t, dir, false)
	for _, want := range []string{
		"fr.json lacks 2 of en.json's 3 messages, which show in English: queueTitle, songsCount\n",
		"fr.json has 1 message en.json no longer has, which nothing shows: oldKey\n",
		"zh-Hans.json translates every message\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "mygo.appName") || strings.Contains(got, "@playButton") {
		t.Errorf("MyGo's messages and descriptions are no messages to translate:\n%s", got)
	}
}

func TestWarningsInActions(t *testing.T) {
	dir := catalogs(t, map[string]string{"en.json": en, "fr.json": `{"playButton": "Lire"}`})
	got := output(t, dir, true)
	if !strings.HasPrefix(got, "::warning file=") || !strings.Contains(got, "fr.json,title=Translations::fr.json lacks 2 of") {
		t.Errorf("not a warning of the run: %q", got)
	}
}

// TestTheMoreFolder reads the translations of locales/more, which the app
// adds as it starts, beside those of locales itself.
func TestTheMoreFolder(t *testing.T) {
	dir := catalogs(t, map[string]string{"en.json": en, "fr.json": `{"playButton": "Lire"}`})
	if err := os.Mkdir(filepath.Join(dir, "more"), 0o755); err != nil {
		t.Fatal(err)
	}
	de := `{"@@locale": "de", "@@x-machine-translated": true, "playButton": "Abspielen", "queueTitle": "Warteschlange"}`
	if err := os.WriteFile(filepath.Join(dir, "more", "de.json"), []byte(de), 0o644); err != nil {
		t.Fatal(err)
	}
	got := output(t, dir, false)
	for _, want := range []string{
		"fr.json lacks 2 of en.json's 3 messages",
		"more/de.json lacks 1 of en.json's 3 messages, which show in English: songsCount\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "@@x-machine-translated") {
		t.Errorf("a catalog's marks are no messages:\n%s", got)
	}
	got = output(t, dir, true)
	if !strings.Contains(got, "::warning file="+filepath.ToSlash(filepath.Join(dir, "more", "de.json"))+",title=Translations::more/de.json lacks") {
		t.Errorf("more/de.json's warning: %q", got)
	}
}

func TestEnglishAlone(t *testing.T) {
	if got := output(t, catalogs(t, map[string]string{"en.json": en}), true); got != "No translations yet: en.json alone\n" {
		t.Errorf("got %q", got)
	}
}

func TestUnreadableCatalogFails(t *testing.T) {
	dir := catalogs(t, map[string]string{"en.json": en, "fr.json": `{"playButton": `})
	var b strings.Builder
	if err := run(&b, dir, false); err == nil || !strings.Contains(err.Error(), "fr.json") {
		t.Errorf("a broken catalog: got %v, want an error naming it", err)
	}
}

// TestTheRepositorysCatalogs reads the catalogs the app ships.
func TestTheRepositorysCatalogs(t *testing.T) {
	var b strings.Builder
	if err := run(&b, filepath.Join("..", "..", "locales"), false); err != nil {
		t.Fatal(err)
	}
}
