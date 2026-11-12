// Command sitefinish finishes the website Zola built in site/public:
//
//   - the landing page in each language of languages.go, at
//     /<language>/, from the English one and site/i18n/<language>.json,
//     the translations of its pieces keyed by their English;
//   - on every landing page, its language, the alternates search engines
//     read (hreflang), its canonical address and Open Graph locale, and
//     the language menus;
//   - the sitemap, with those pages and their alternates;
//   - llms.txt and llms-full.txt, for AI agents: what MixMusic is, and its
//     docs.
//
// A piece of the page without a translation shows in English, and the
// command says how many each language lacks. With -extract, it writes
// site/i18n/strings.json, every piece of the English page with what it is
// and where, for translators: run it after changing the page's words.
//
//	go run ./tools/sitedocs && zola --root site build && go run ./tools/sitefinish
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

func main() {
	site := flag.String("site", "site", "the site's folder, with zola.toml, i18n and the built public")
	extractOnly := flag.Bool("extract", false, "write i18n/strings.json from the built English page, and stop")
	flag.Parse()
	if err := run(*site, *extractOnly); err != nil {
		fmt.Fprintln(os.Stderr, "sitefinish:", err)
		os.Exit(1)
	}
}

// config is what sitefinish reads of zola.toml: the site's address for
// search engines, and its description.
type config struct {
	canonical, description string
}

var (
	canonicalLine   = regexp.MustCompile(`(?m)^canonical_url\s*=\s*"([^"]+)"`)
	descriptionLine = regexp.MustCompile(`(?m)^description\s*=\s*"((?:[^"\\]|\\.)*)"`)
)

func readConfig(path string) (config, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var c config
	if m := canonicalLine.FindSubmatch(src); m != nil {
		c.canonical = string(m[1])
	}
	if m := descriptionLine.FindSubmatch(src); m != nil {
		c.description = tomlValue(`"` + string(m[1]) + `"`)
	}
	if c.canonical == "" {
		return c, fmt.Errorf("%s: no canonical_url", path)
	}
	return c, nil
}

// warn tells of something that doesn't fail the build: as a warning of
// GitHub Actions' when it runs there.
func warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Println("::warning title=Site translations::" + msg)
		return
	}
	fmt.Println("sitefinish:", msg)
}

func run(site string, extractOnly bool) error {
	public := filepath.Join(site, "public")
	cfg, err := readConfig(filepath.Join(site, "zola.toml"))
	if err != nil {
		return err
	}
	src, err := os.ReadFile(filepath.Join(public, "index.html"))
	if err != nil {
		return fmt.Errorf("build the site first: %w", err)
	}
	doc, err := parse(src)
	if err != nil {
		return err
	}
	// The pieces as the canonical copy has them, whichever copy was built:
	// the translations are keyed by those.
	units := extract(doc)
	built, home := builtFor(doc), slash(cfg.canonical)
	for i := range units {
		units[i].Text = rebase(units[i].Text, built, home)
	}
	stringsPath := filepath.Join(site, "i18n", "strings.json")
	if extractOnly {
		out, err := json.MarshalIndent(units, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(stringsPath, append(out, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Printf("sitefinish: %d pieces in %s\n", len(units), stringsPath)
		return nil
	}
	if old, err := os.ReadFile(stringsPath); err == nil {
		var listed []unit
		if json.Unmarshal(old, &listed) == nil && !sameUnits(listed, units) {
			warn("%s isn't the English page's: run go run ./tools/sitefinish -extract, and translate what changed", filepath.ToSlash(stringsPath))
		}
	}
	for _, l := range languages {
		var table map[string]string
		if l.Dir != "" {
			table, err = readTable(filepath.Join(site, "i18n", l.Dir+".json"))
			if err != nil {
				return err
			}
		}
		out, missing, err := page(src, l, table, cfg.canonical)
		if err != nil {
			return fmt.Errorf("%s: %w", l.Tag, err)
		}
		if len(missing) > 0 {
			warn("%s: %d of the page's %d pieces have no translation, and show in English; the first: %q", l.Tag, len(missing), len(units), missing[0])
		}
		dir := filepath.Join(public, l.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), out, 0o644); err != nil {
			return err
		}
	}
	sm, err := os.ReadFile(filepath.Join(public, "sitemap.xml"))
	if err != nil {
		return err
	}
	if sm, err = sitemap(sm, cfg.canonical); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(public, "sitemap.xml"), sm, 0o644); err != nil {
		return err
	}
	pages, err := readDocs(filepath.Join(site, "content", "docs"))
	if err != nil {
		return err
	}
	index, full := llms(pages, cfg.canonical, cfg.description)
	if err := os.WriteFile(filepath.Join(public, "llms.txt"), []byte(index), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(public, "llms-full.txt"), []byte(full), 0o644); err != nil {
		return err
	}
	fmt.Printf("sitefinish: %d languages, %d docs pages in llms.txt\n", len(languages), len(pages))
	return nil
}

// readTable reads a language's translations, keyed by their English; a
// language without its file yet has none.
func readTable(path string) (map[string]string, error) {
	src, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var table map[string]string
	if err := json.Unmarshal(src, &table); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return table, nil
}

// sameUnits reports whether a and b list the same pieces, in any order.
func sameUnits(a, b []unit) bool {
	key := func(us []unit) []string {
		out := make([]string, len(us))
		for i, u := range us {
			out[i] = u.Kind + "\x00" + u.Text
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(key(a), key(b))
}
