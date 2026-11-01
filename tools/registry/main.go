// Command registry builds the indexes of the community's themes and
// plugins that MixMusic's marketplace reads, and checks every add-on on
// the way: community/registry-2.json, of community/themes and
// community/plugins, for MixMusic's own interface, and
// community/registry.json, of community/web, for the web interface of the
// versions before it, which keep reading it there.
//
//	go run ./tools/registry          # write the registries
//	go run ./tools/registry -check   # fail when one is out of date or an add-on is invalid
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/addons"
)

// baseURL is where the app downloads the add-ons listed.
const baseURL = "https://raw.githubusercontent.com/fzn0x/mixmusic/main/community/"

// maxAddonSize bounds the files of one add-on, in bytes.
const maxAddonSize = 4 << 20

// registries are those the command writes: their file, their version, the
// folder of their add-ons under community, and the plugin API their
// plugins are made for.
var registries = []struct {
	file    string
	version int
	folder  string
	api     int
}{
	{"registry-2.json", 2, "", addons.PluginAPI},
	{"registry.json", 1, "web", 1},
}

func main() {
	check := flag.Bool("check", false, "check that the registries are up to date instead of writing them")
	root := flag.String("root", ".", "the repository's root")
	flag.Parse()
	community := filepath.Join(*root, "community")
	builtin, err := builtinIDs(filepath.Join(*root, "builtin"))
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range registries {
		dir := filepath.Join(community, r.folder)
		base := baseURL
		if r.folder != "" {
			base += r.folder + "/"
		}
		reg := addons.Registry{Version: r.version, BaseURL: base}
		themes, err := collect(dir, addons.KindTheme, builtin, 0)
		plugins, err2 := collect(dir, addons.KindPlugin, builtin, r.api)
		reg.Themes, reg.Plugins = themes, plugins
		if err := errors.Join(err, err2, reg.Validate()); err != nil {
			log.Fatalf("invalid add-ons:\n%v", err)
		}
		data, err := json.MarshalIndent(reg, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		data = append(data, '\n')
		out := filepath.Join(community, r.file)
		if *check {
			current, _ := os.ReadFile(out)
			if !bytes.Equal(bytes.ReplaceAll(current, []byte("\r\n"), []byte("\n")), data) {
				log.Fatalf("community/%s is out of date: run go run ./tools/registry", r.file)
			}
			fmt.Printf("ok: %s, %d themes, %d plugins\n", r.file, len(reg.Themes), len(reg.Plugins))
			continue
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("wrote %s: %d themes, %d plugins\n", out, len(reg.Themes), len(reg.Plugins))
	}
}

// builtinIDs returns the IDs of the themes MixMusic ships with, which
// community themes may not take.
func builtinIDs(dir string) (map[string]bool, error) {
	ids := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(dir, "themes"))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			ids[e.Name()] = true
		}
	}
	return ids, nil
}

// collect reads and checks the add-ons of a kind in a folder of the
// community's; plugins must be made for the plugin API api.
func collect(community string, kind addons.Kind, builtin map[string]bool, api int) ([]addons.Entry, error) {
	folder := string(kind) + "s"
	dirs, err := os.ReadDir(filepath.Join(community, folder))
	if errors.Is(err, fs.ErrNotExist) {
		return []addons.Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	entries := []addons.Entry{}
	var errs []error
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dir := filepath.Join(community, folder, d.Name())
		e, err := entry(dir, kind, d.Name())
		switch {
		case err != nil:
		case kind == addons.KindTheme && builtin[e.ID]:
			err = errors.New("a theme MixMusic ships with has this id")
		case kind == addons.KindPlugin && max(e.API, 1) != api:
			err = fmt.Errorf(`plugins here are made for the plugin API %d: "api": %d`, api, api)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s/%s: %w", folder, d.Name(), err))
			continue
		}
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b addons.Entry) int { return strings.Compare(a.ID, b.ID) })
	return entries, errors.Join(errs...)
}

// entry reads an add-on's manifest and checks its files.
func entry(dir string, kind addons.Kind, name string) (addons.Entry, error) {
	data, err := os.ReadFile(filepath.Join(dir, string(kind)+".json"))
	if err != nil {
		return addons.Entry{}, err
	}
	var e addons.Entry
	var files []string
	switch kind {
	case addons.KindTheme:
		t, err := addons.ParseTheme(data)
		if err != nil {
			return e, err
		}
		e = addons.Entry{ID: t.ID, Name: t.Name, Version: t.Version, Author: t.Author, Description: t.Description, Preview: t.Preview, Colors: t.Colors}
		files = t.Files
	default:
		p, err := addons.ParsePlugin(data)
		if err != nil {
			return e, err
		}
		e = addons.Entry{ID: p.ID, Name: p.Name, Version: p.Version, Author: p.Author, Description: p.Description, Permissions: p.Permissions, API: p.API}
		files = p.Files
	}
	if e.ID != name {
		return e, fmt.Errorf("the folder must be named after the id, %s", e.ID)
	}
	if strings.TrimSpace(e.Description) == "" {
		return e, errors.New("a description is required")
	}
	if len(files) == 0 {
		return e, errors.New(`"files" must list every file of the add-on`)
	}
	onDisk, size, err := walk(dir)
	if err != nil {
		return e, err
	}
	if size > maxAddonSize {
		return e, fmt.Errorf("the add-on weighs %d KB; keep it under %d KB", size>>10, maxAddonSize>>10)
	}
	listed := slices.Sorted(slices.Values(files))
	if !slices.Equal(listed, onDisk) {
		return e, fmt.Errorf(`"files" lists %v, but the folder holds %v`, listed, onDisk)
	}
	e.Path = string(kind) + "s/" + e.ID
	e.Files = listed
	return e, nil
}

// walk lists the files of a folder, as slash paths, and their total size:
// the files the add-on ships, without the TypeScript a plugin is written
// in, which compiles to the JavaScript beside it.
func walk(dir string) ([]string, int64, error) {
	sizes := map[string]int64{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sizes[path.Clean(filepath.ToSlash(rel))] = info.Size()
		return nil
	})
	var files []string
	var size int64
	for f, n := range sizes {
		if _, compiled := sizes[strings.TrimSuffix(f, ".ts")+".js"]; compiled && strings.HasSuffix(f, ".ts") && !strings.HasSuffix(f, ".d.ts") {
			continue
		}
		files = append(files, f)
		size += n
	}
	slices.Sort(files)
	return files, size, err
}
