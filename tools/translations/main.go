// Command translations says which messages of locales/en.json each
// translation lacks, those beside it and those of locales/more, which the
// app adds as it starts (#776). The app shows those in English, so a
// release ships with them: the release workflow runs it, and its warnings
// tell what is left to translate. It fails only on a catalog it can't read.
//
//	go run ./tools/translations                 # the messages each language lacks
//	go run ./tools/translations -dir locales    # the catalogs of another folder
//
// In GitHub Actions (GITHUB_ACTIONS=true) each language's line is a
// warning of the run.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func main() {
	log.SetFlags(0)
	dir := flag.String("dir", "locales", "the folder of the catalogs, en.json among them")
	flag.Parse()
	if err := run(os.Stdout, *dir, os.Getenv("GITHUB_ACTIONS") == "true"); err != nil {
		log.Fatal(err)
	}
}

// run writes, for each catalog of dir and of dir/more other than en.json,
// the messages it lacks and those en.json no longer has; as warnings of a
// GitHub Actions run when actions is set.
func run(w io.Writer, dir string, actions bool) error {
	en, err := messages(filepath.Join(dir, "en.json"))
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	more, err := filepath.Glob(filepath.Join(dir, "more", "*.json"))
	if err != nil {
		return err
	}
	slices.Sort(files)
	slices.Sort(more)
	translations := 0
	for _, f := range append(files, more...) {
		if f == filepath.Join(dir, "en.json") {
			continue
		}
		// more/de.json, as it sits in the folder.
		base, err := filepath.Rel(dir, f)
		if err != nil {
			return err
		}
		base = filepath.ToSlash(base)
		translations++
		tr, err := messages(f)
		if err != nil {
			return err
		}
		var lacks, extra []string
		for k := range en {
			if !tr[k] {
				lacks = append(lacks, k)
			}
		}
		for k := range tr {
			// MyGo's own messages, as mygo.appName, are the translations'.
			if !en[k] && !strings.HasPrefix(k, "mygo.") {
				extra = append(extra, k)
			}
		}
		slices.Sort(lacks)
		slices.Sort(extra)
		name := filepath.ToSlash(f)
		if len(lacks) > 0 {
			say(w, actions, name, fmt.Sprintf("%s lacks %d of en.json's %d messages, which show in English: %s",
				base, len(lacks), len(en), strings.Join(lacks, ", ")))
		}
		if len(extra) > 0 {
			say(w, actions, name, fmt.Sprintf("%s has %s en.json no longer has, which nothing shows: %s",
				base, count(len(extra), "message"), strings.Join(extra, ", ")))
		}
		if len(lacks) == 0 && len(extra) == 0 {
			fmt.Fprintf(w, "%s translates every message\n", base)
		}
	}
	if translations == 0 {
		fmt.Fprintln(w, "No translations yet: en.json alone")
	}
	return nil
}

// count says n of a thing, as "1 message" or "2 messages".
func count(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// say writes a line, as a warning on file in GitHub Actions.
func say(w io.Writer, actions bool, file, line string) {
	if actions {
		fmt.Fprintf(w, "::warning file=%s,title=Translations::%s\n", file, line)
		return
	}
	fmt.Fprintln(w, line)
}

// messages reads the keys of a catalog's messages, without the
// descriptions under "@" keys.
func messages(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	keys := map[string]bool{}
	for k := range m {
		if !strings.HasPrefix(k, "@") {
			keys[k] = true
		}
	}
	return keys, nil
}
