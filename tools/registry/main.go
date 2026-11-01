// Command registry builds the index of the community's themes and plugins
// that MixMusic's marketplace reads, community/registry-2.json, of
// community/themes and community/plugins, and checks every add-on on the
// way. Apps download the add-ons from the registry branch, where the
// Registry workflow publishes them: the plugins' JavaScript, compiled from
// their TypeScript by `bun run build:plugins`, which main doesn't keep.
//
// community/registry.json is the registry of the web interface of MixMusic
// 0.4 and before, which keep reading it: it stays as it is, with its
// add-ons in web/ of the registry branch.
//
//	go run ./tools/registry                 # write registry-2.json
//	go run ./tools/registry -check          # fail when it is out of date or an add-on is invalid
//	go run ./tools/registry -publish <dir>  # copy the files apps download into dir
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

// baseURL is where the app downloads the add-ons listed: the registry
// branch, which the Registry workflow publishes them to.
const baseURL = addons.CommunityFilesURL

// webBaseURL is where MixMusic 0.4 and before download the add-ons of
// registry.json.
const webBaseURL = baseURL + "web/"

// maxAddonSize bounds the files of one add-on, in bytes.
const maxAddonSize = 4 << 20

func main() {
	check := flag.Bool("check", false, "check that the registry is up to date instead of writing it")
	publish := flag.String("publish", "", "copy the files apps download into this folder instead of writing the registry")
	root := flag.String("root", ".", "the repository's root")
	flag.Parse()
	community := filepath.Join(*root, "community")
	builtin, err := builtinIDs(filepath.Join(*root, "builtin"))
	if err != nil {
		log.Fatal(err)
	}
	reg := addons.Registry{Version: 2, BaseURL: baseURL}
	themes, err := collect(community, addons.KindTheme, builtin, 0)
	plugins, err2 := collect(community, addons.KindPlugin, builtin, addons.PluginAPI)
	reg.Themes, reg.Plugins = themes, plugins
	if err := errors.Join(err, err2, reg.Validate(), checkWeb(community)); err != nil {
		log.Fatalf("invalid add-ons:\n%v", err)
	}
	if *publish != "" {
		n, err := stage(community, *publish, slices.Concat(reg.Themes, reg.Plugins))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("copied %d files of %d themes and %d plugins to %s\n", n, len(reg.Themes), len(reg.Plugins), *publish)
		return
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	data = append(data, '\n')
	out := filepath.Join(community, "registry-2.json")
	if *check {
		current, _ := os.ReadFile(out)
		if !bytes.Equal(bytes.ReplaceAll(current, []byte("\r\n"), []byte("\n")), data) {
			log.Fatal("community/registry-2.json is out of date: run go run ./tools/registry")
		}
		fmt.Printf("ok: registry-2.json, %d themes, %d plugins\n", len(reg.Themes), len(reg.Plugins))
		return
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s: %d themes, %d plugins\n", out, len(reg.Themes), len(reg.Plugins))
}

// checkWeb checks registry.json, the frozen registry of MixMusic 0.4 and
// before: that it reads, and sends them to web/ of the registry branch.
func checkWeb(community string) error {
	data, err := os.ReadFile(filepath.Join(community, "registry.json"))
	if err != nil {
		return err
	}
	var reg addons.Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return fmt.Errorf("registry.json: %w", err)
	}
	if reg.BaseURL != webBaseURL {
		return fmt.Errorf("registry.json: its baseUrl is %s, not %s", reg.BaseURL, webBaseURL)
	}
	return nil
}

// stage copies the files of entries, as apps download them, from
// community into dir, under the entries' paths.
func stage(community, dir string, entries []addons.Entry) (int, error) {
	n := 0
	for _, e := range entries {
		for _, f := range e.Files {
			data, err := os.ReadFile(filepath.Join(community, filepath.FromSlash(e.Path), filepath.FromSlash(f)))
			if err != nil {
				return n, err
			}
			out := filepath.Join(dir, filepath.FromSlash(e.Path), filepath.FromSlash(f))
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return n, err
			}
			if err := os.WriteFile(out, data, 0o644); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
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
	for _, f := range listed {
		if strings.HasSuffix(f, ".js") && !slices.Contains(onDisk, f) && fileExists(filepath.Join(dir, strings.TrimSuffix(f, ".js")+".ts")) {
			return e, fmt.Errorf("%s isn't compiled from its TypeScript yet: run bun run build:plugins", f)
		}
	}
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

// fileExists tells whether a file is there.
func fileExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}
