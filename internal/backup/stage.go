package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
)

// RestoreDir is the folder of the data folder a restore stages in, and
// keeps what it replaced: the only one of a backup's making.
const RestoreDir = "restore"

// The folders of RestoreDir: next holds the restore staged, which the next
// start swaps in; undo what the last restore replaced; aside what a swap
// moves away, until it is done.
const (
	nextDir  = "next"
	undoDir  = "undo"
	asideDir = "aside"
	planName = "plan.json"
	filesDir = "files"
)

// Choice is what the user chose to restore of a backup.
type Choice struct {
	// Parts are the parts to restore, of those the backup holds.
	Parts []Part
	// Merge adds the backup's library to the one here, rather than
	// replacing it.
	Merge bool
	// Folders match the backup's folders of music to those here: a folder
	// it came from to the one here that holds the same music, or "" for
	// none, whose songs show as missing.
	Folders map[string]string
	// Download downloads the songs of the Downloads part again.
	Download bool
}

// plan is a restore staged, or what undoes one.
type plan struct {
	Format int `json:"format"`
	// Undo is set for a plan that puts back what a restore replaced.
	Undo bool `json:"undo,omitempty"`
	// Paths are the files and folders of the data folder it replaces,
	// with forward slashes: those it holds take their place, and the
	// others go.
	Paths []string `json:"paths"`
	// Backup is when the backup restored was made, and App the version of
	// MixMusic that made it; Restored when it was restored.
	Backup   time.Time `json:"backup"`
	App      string    `json:"app,omitempty"`
	Restored time.Time `json:"restored,omitzero"`
}

// Stage stages the restore of what c chose of the backup in the data
// folder dataDir, for the next start to swap in: its files, of the music
// on this computer where c.Folders moves them, and with those the settings
// of this computer keep, and plugins switched off; or, with c.Merge, the
// library here with the backup's added. What the backup unpacked moves
// there: the backup is spent, and only Close remains.
func (b *Backup) Stage(dataDir string, c Choice) (err error) {
	next := filepath.Join(dataDir, RestoreDir, nextDir)
	if err := os.RemoveAll(next); err != nil {
		return err
	}
	files := filepath.Join(next, filesDir)
	if err := os.MkdirAll(files, 0o755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(next)
		}
	}()
	pl := plan{Format: Format, Backup: b.Manifest.Created, App: b.Manifest.App}
	chosen := func(p Part) bool { return slices.Contains(c.Parts, p) && b.Manifest.Has(p) }
	for _, p := range Parts {
		if p == Downloads || !chosen(p) {
			continue
		}
		pl.Paths = append(pl.Paths, p.paths()...)
		for _, e := range b.Manifest.Entries {
			if e.Part != p {
				continue
			}
			if err := moveFile(b.file(e.Path), filepath.Join(files, filepath.FromSlash(e.Path))); err != nil {
				return err
			}
		}
	}
	if move := mover(b.Manifest.System, c.Folders); move != nil && (chosen(Library) || chosen(Queue)) {
		if err := moveSongs(files, move); err != nil {
			return err
		}
	}
	if chosen(Library) && c.Merge {
		if err := mergeHere(dataDir, files); err != nil {
			return err
		}
	}
	if chosen(Settings) || chosen(Addons) {
		staged, err := settingsHere(dataDir, files, chosen(Settings), chosen(Addons))
		if err != nil {
			return err
		}
		if staged && !slices.Contains(pl.Paths, "settings.json") {
			pl.Paths = append(pl.Paths, "settings.json")
		}
	}
	if c.Download && chosen(Downloads) {
		if err := moveFile(b.file(DownloadsName), filepath.Join(next, DownloadsName)); err != nil {
			return err
		}
	}
	if len(pl.Paths) == 0 && !(c.Download && chosen(Downloads)) {
		return errors.New("nothing to restore")
	}
	return writePlan(next, pl)
}

// writePlan writes pl as the plan of the folder dir: last, once what it
// tells of is there.
func writePlan(dir string, pl plan) error {
	data, err := json.MarshalIndent(pl, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteFile(filepath.Join(dir, planName), append(data, '\n'))
}

// readPlan reads the plan of the folder dir.
func readPlan(dir string) (plan, error) {
	var pl plan
	err := readJSON(filepath.Join(dir, planName), &pl)
	return pl, err
}

// moveFile moves the file at from to to, making to's folder: by renaming
// it, or else by copying it, as across drives.
func moveFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	if err := copyFile(from, to); err != nil {
		return err
	}
	return os.Remove(from)
}

// copyFile copies the file at from to to, making to's folder.
func copyFile(from, to string) error {
	in, err := store.OpenShared(from)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeJSON writes v to path as the data folder's files keep it.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteFile(path, append(data, '\n'))
}

// moveSongs gives the tracks of the music files a backup holds the IDs of
// where move tells they are now: in the library, the play log, the queue
// and the songs with copies, of the folder files.
func moveSongs(files string, move func(string) (string, bool)) error {
	moveID := func(id string) (string, bool) { return localfiles.MoveID(id, move) }
	if _, err := os.Stat(filepath.Join(files, "library.json")); err == nil {
		lib, err := library.Open(filepath.Join(files, "library.json"))
		if err != nil {
			return &Error{Problem: Damaged, Entry: "library.json", Err: err}
		}
		if err := lib.Move(moveID); err != nil {
			return err
		}
	}
	if path := filepath.Join(files, "queue.json"); exists(path) {
		var q map[string]json.RawMessage
		if err := readJSON(path, &q); err != nil {
			return &Error{Problem: Damaged, Entry: "queue.json", Err: err}
		}
		var tracks []media.Track
		if err := json.Unmarshal(q["tracks"], &tracks); err == nil && len(tracks) > 0 {
			for i, t := range tracks {
				if id, ok := moveID(t.ID); ok {
					tracks[i].ID = id
				}
			}
			if q["tracks"], err = json.Marshal(tracks); err != nil {
				return err
			}
			if err := writeJSON(path, q); err != nil {
				return err
			}
		}
	}
	if path := filepath.Join(files, "local-copies.json"); exists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		moved, err := localfiles.MoveLinks(data, move)
		if err != nil {
			return &Error{Problem: Damaged, Entry: "local-copies.json", Err: err}
		}
		if err := store.WriteFile(path, moved); err != nil {
			return err
		}
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// mergeHere makes the library of the folder files, a backup's, the
// library of the data folder dataDir with the backup's added: the likes,
// mixes, follows, history and searches as library.MergeData merges them,
// every play of both logs, the covers of both, and the songs with copies
// of this computer, if it has any.
func mergeHere(dataDir, files string) error {
	var here, theirs library.Data
	if data, err := store.ReadFile(filepath.Join(dataDir, "library.json")); err == nil {
		if err := json.Unmarshal(data, &here); err != nil {
			return fmt.Errorf("reading the library here: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if exists(filepath.Join(files, "library.json")) {
		if err := readJSON(filepath.Join(files, "library.json"), &theirs); err != nil {
			return &Error{Problem: Damaged, Entry: "library.json", Err: err}
		}
	}
	merged, _ := library.MergeData(here, theirs)
	if err := writeJSON(filepath.Join(files, "library.json"), merged); err != nil {
		return err
	}
	out := filepath.Join(files, "plays.merged")
	if _, err := library.MergePlays(filepath.Join(dataDir, "plays.jsonl"), filepath.Join(files, "plays.jsonl"), out); err != nil {
		return err
	}
	if err := os.Rename(out, filepath.Join(files, "plays.jsonl")); err != nil {
		return err
	}
	covers := filepath.Join(dataDir, "mix-covers")
	err := filepath.WalkDir(covers, func(abs string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist) && abs == covers:
			return nil
		case err != nil:
			return err
		case d.IsDir() || !d.Type().IsRegular():
			return nil
		}
		rel, err := filepath.Rel(dataDir, abs)
		if err != nil {
			return err
		}
		if to := filepath.Join(files, rel); !exists(to) {
			return copyFile(abs, to)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if links := filepath.Join(dataDir, "local-copies.json"); exists(links) {
		return copyFile(links, filepath.Join(files, "local-copies.json"))
	}
	return nil
}

// settingsHere makes the settings of the folder files, a backup's, keep
// the values of this computer's settings, of the data folder dataDir,
// that belong to it, with the plugins here switched on, or none once the
// add-ons are restored: plugins come back switched off, to be trusted
// again. With the add-ons restored but not the settings, it stages the
// settings here with no plugin on. It tells whether it staged settings.
func settingsHere(dataDir, files string, withSettings, withAddons bool) (bool, error) {
	here := settings.Default()
	data, err := store.ReadFile(filepath.Join(dataDir, "settings.json"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err == nil {
		s, err := settings.Decode(data)
		switch {
		case err == nil:
			here = s
		case !withSettings:
			// A file with a mistake stays as the user left it, with no
			// plugin on that MixMusic reads.
			return false, nil
		}
	} else if !withSettings {
		// No settings here, so no plugin on: nothing to change.
		return false, nil
	}
	path := filepath.Join(files, "settings.json")
	s := here
	if withSettings {
		data, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		if s, err = settings.Decode(data); err != nil {
			return false, &Error{Problem: Damaged, Entry: "settings.json", Err: err}
		}
		s = KeepLocal(s, here)
	}
	if withAddons {
		s.EnabledPlugins = []string{}
	}
	return true, writeJSON(path, s)
}

// KeepLocal returns s, settings from a backup, with the values of here,
// this computer's settings, of those that belong to it: the folders of
// downloads, music and backups, yt-dlp and the JavaScript runtime, the
// colour file, the output device and the Spotify device, and the plugins
// switched on.
func KeepLocal(s, here settings.Settings) settings.Settings {
	s.DownloadsFolder = here.DownloadsFolder
	s.MusicFolders = slices.Clone(here.MusicFolders)
	s.BackupFolder = here.BackupFolder
	s.YTDLPPath, s.JSRuntime = here.YTDLPPath, here.JSRuntime
	s.ColorSource, s.ColorFile = here.ColorSource, here.ColorFile
	s.OutputDevice = here.OutputDevice
	s.SpotifyDevice = here.SpotifyDevice
	s.EnabledPlugins = slices.Clone(here.EnabledPlugins)
	return s
}

// Added counts what restoring the library of the backup with Merge would
// add to the library of the data folder dataDir, of its songs on this
// computer where folders moves them.
func (b *Backup) Added(dataDir string, folders map[string]string) (Counts, error) {
	var c Counts
	theirs, ok := b.library()
	if !ok {
		return c, nil
	}
	var here library.Data
	if data, err := store.ReadFile(filepath.Join(dataDir, "library.json")); err == nil {
		if err := json.Unmarshal(data, &here); err != nil {
			return c, err
		}
	}
	plays := b.file("plays.jsonl")
	if move := mover(b.Manifest.System, folders); move != nil {
		// What the moves make of the backup's library, in a folder of its
		// own.
		dir, err := os.MkdirTemp(filepath.Dir(b.dir), "open-")
		if err != nil {
			return c, err
		}
		defer os.RemoveAll(dir)
		for _, name := range []string{"library.json", "plays.jsonl"} {
			if b.has(name) {
				if err := copyFile(b.file(name), filepath.Join(dir, name)); err != nil {
					return c, err
				}
			}
		}
		if err := moveSongs(dir, move); err != nil {
			return c, err
		}
		if err := readJSON(filepath.Join(dir, "library.json"), &theirs); err != nil {
			return c, err
		}
		plays = filepath.Join(dir, "plays.jsonl")
	}
	_, a := library.MergeData(here, theirs)
	c.Liked, c.Disliked, c.Mixes, c.Artists, c.Podcasts = a.Liked, a.Disliked, a.Mixes, a.Artists, a.Podcasts
	n, err := library.PlaysAdded(filepath.Join(dataDir, "plays.jsonl"), plays)
	if err != nil {
		return c, err
	}
	c.Plays = n
	return c, nil
}

// Applied is what the start found staged, and did.
type Applied struct {
	// Restored is set once a restore swapped in, and Undone once an undo
	// did; Err tells why neither did, and every file went back.
	Restored, Undone bool
	Err              error
	// Backup is when the backup restored was made.
	Backup time.Time
	// Downloads are the downloads to download again, if any.
	Downloads *DownloadList
}

// Apply swaps in the restore staged in the data folder dataDir, if any:
// it runs as MixMusic starts, before anything opens what it replaces. The
// files and folders it replaces move aside, and those staged take their
// place; once all did, those aside are kept, for Undo to put back; should
// one fail, all go back. An undo staged is swapped in likewise, and what
// it replaced goes.
func Apply(dataDir string) Applied {
	restore := filepath.Join(dataDir, RestoreDir)
	// What a preview unpacked and never staged.
	if found, _ := filepath.Glob(filepath.Join(restore, "open-*")); len(found) > 0 {
		for _, dir := range found {
			os.RemoveAll(dir)
		}
	}
	next := filepath.Join(restore, nextDir)
	pl, err := readPlan(next)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		os.RemoveAll(next)
		return Applied{}
	case err != nil:
		os.RemoveAll(next)
		return Applied{Err: err}
	}
	out := Applied{Backup: pl.Backup}
	aside := filepath.Join(restore, asideDir)
	if err := swap(dataDir, next, aside, pl.Paths); err != nil {
		os.RemoveAll(next)
		out.Err = err
		return out
	}
	if pl.Undo {
		out.Undone = true
		os.RemoveAll(aside)
	} else {
		out.Restored = true
		var l DownloadList
		if readJSON(filepath.Join(next, DownloadsName), &l) == nil {
			out.Downloads = &l
		}
		undo := plan{Format: Format, Undo: true, Paths: pl.Paths, Backup: pl.Backup, App: pl.App, Restored: time.Now().Round(time.Second)}
		err := writePlan(aside, undo)
		if err == nil {
			err = os.RemoveAll(filepath.Join(restore, undoDir))
		}
		if err == nil {
			err = renameRetry(aside, filepath.Join(restore, undoDir))
		}
		if err != nil {
			log.Printf("mixmusic: keeping what the restore replaced, for its undo: %v", err)
		}
	}
	os.RemoveAll(next)
	return out
}

// swapRename renames a file or a folder as a swap moves it: tests make it
// fail.
var swapRename = renameRetry

// swap moves the files and folders of paths in the data folder dataDir
// into aside's files, and those of next's files into their place. Should
// one fail, those it moved go back, and it returns why.
func swap(dataDir, next, aside string, paths []string) error {
	if err := os.RemoveAll(aside); err != nil {
		return err
	}
	type moved struct{ from, to string }
	var done []moved
	back := func() {
		for _, m := range slices.Backward(done) {
			if err := swapRename(m.to, m.from); err != nil {
				log.Printf("mixmusic: putting %s back after a restore failed: %v", m.from, err)
			}
		}
	}
	move := func(from, to string) error {
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := swapRename(from, to); err != nil {
			return err
		}
		done = append(done, moved{from, to})
		return nil
	}
	for _, p := range paths {
		from := filepath.Join(dataDir, filepath.FromSlash(p))
		if _, err := os.Lstat(from); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := move(from, filepath.Join(aside, filesDir, filepath.FromSlash(p))); err != nil {
			back()
			return err
		}
	}
	for _, p := range paths {
		from := filepath.Join(next, filesDir, filepath.FromSlash(p))
		if _, err := os.Lstat(from); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := move(from, filepath.Join(dataDir, filepath.FromSlash(p))); err != nil {
			back()
			return err
		}
	}
	return nil
}

// Undo is the last restore, which StageUndo undoes.
type Undo struct {
	// Backup is when the backup restored was made, and Restored when it
	// was restored.
	Backup, Restored time.Time
}

// LastRestore returns the last restore of the data folder dataDir, while
// it can be undone.
func LastRestore(dataDir string) (Undo, bool) {
	pl, err := readPlan(filepath.Join(dataDir, RestoreDir, undoDir))
	if err != nil || !pl.Undo {
		return Undo{}, false
	}
	return Undo{Backup: pl.Backup, Restored: pl.Restored}, true
}

// StageUndo stages the undo of the last restore of the data folder
// dataDir, for the next start to swap in: the files it replaced take the
// place of those it brought, which go.
func StageUndo(dataDir string) error {
	restore := filepath.Join(dataDir, RestoreDir)
	if _, ok := LastRestore(dataDir); !ok {
		return errors.New("no restore to undo")
	}
	next := filepath.Join(restore, nextDir)
	if err := os.RemoveAll(next); err != nil {
		return err
	}
	return renameRetry(filepath.Join(restore, undoDir), next)
}

// Staged reports whether a restore or an undo waits in the data folder
// dataDir for the next start.
func Staged(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, RestoreDir, nextDir, planName))
	return err == nil
}

// Work is the folder of the data folder dataDir that a backup opens in,
// for its files to move to the restore staged at once.
func Work(dataDir string) string { return filepath.Join(dataDir, RestoreDir) }

// mover returns what moves the paths of the music files of the system
// that made a backup, by folders, which match the folders of music it came
// from to those here: a path in a folder moves to the same place in its
// match. It returns nil when none moves.
func mover(system string, folders map[string]string) func(string) (string, bool) {
	type pair struct{ from, to string }
	var pairs []pair
	for from, to := range folders {
		if from = trimSep(system, from); from != "" && to != "" && !samePath(system, from, to) {
			pairs = append(pairs, pair{from, filepath.Clean(to)})
		}
	}
	if len(pairs) == 0 {
		return nil
	}
	// A folder in another moves by its own match.
	slices.SortFunc(pairs, func(a, b pair) int { return len(b.from) - len(a.from) })
	return func(p string) (string, bool) {
		for _, pr := range pairs {
			rest, ok := under(system, p, pr.from)
			if !ok {
				continue
			}
			parts := splitPath(system, rest)
			if slices.Contains(parts, "..") {
				return "", false
			}
			return filepath.Join(append([]string{pr.to}, parts...)...), true
		}
		return "", false
	}
}

// folds reports whether the paths of system ignore case: Windows' and
// macOS's do.
func folds(system string) bool { return system == "windows" || system == "darwin" }

// seps are the separators of the paths of system.
func seps(system string) string {
	if system == "windows" {
		return `\/`
	}
	return "/"
}

// trimSep is p without the separators at its end.
func trimSep(system, p string) string { return strings.TrimRight(p, seps(system)) }

// splitPath splits p, a path of system, into its names.
func splitPath(system, p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool { return strings.ContainsRune(seps(system), r) })
}

// under reports whether p, a path of system, is in the folder dir, and
// returns the rest of it then.
func under(system, p, dir string) (string, bool) {
	if len(p) < len(dir) {
		return "", false
	}
	head, rest := p[:len(dir)], p[len(dir):]
	if system == "windows" {
		head, dir = strings.ReplaceAll(head, "/", `\`), strings.ReplaceAll(dir, "/", `\`)
	}
	if head != dir && !(folds(system) && strings.EqualFold(head, dir)) {
		return "", false
	}
	if rest != "" && !strings.ContainsRune(seps(system), rune(rest[0])) {
		return "", false
	}
	return rest, true
}

// samePath reports whether a, a path of system, is b, a path of this
// computer: of the same system, and alike, but for case where both ignore
// it.
func samePath(system, a, b string) bool {
	if system != runtimeGOOS {
		return false
	}
	rest, ok := under(system, trimSep(system, filepath.Clean(b)), trimSep(system, a))
	return ok && rest == ""
}

// baseName is the last name of p, a path of system.
func baseName(system, p string) string {
	parts := splitPath(system, p)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// Suggest matches each folder of music a backup made on system came from
// to one of those here: the same folder, or else one of the same name,
// whatever its case where either system ignores it; or "" when none is.
func Suggest(system string, from, here []string) map[string]string {
	out := map[string]string{}
	for _, f := range from {
		out[f] = ""
		for _, h := range here {
			if samePath(system, f, h) {
				out[f] = h
				break
			}
		}
		if out[f] != "" {
			continue
		}
		name := baseName(system, f)
		for _, h := range here {
			hn := filepath.Base(h)
			if name != "" && (hn == name || (folds(system) || folds(runtimeGOOS)) && strings.EqualFold(hn, name)) {
				out[f] = h
				break
			}
		}
	}
	return out
}

// Moved lists the folders of music a backup came from that aren't among
// those here: the folders a restore asks for a match of.
func Moved(system string, from, here []string) []string {
	var out []string
	for _, f := range from {
		if !slices.ContainsFunc(here, func(h string) bool { return samePath(system, f, h) }) {
			out = append(out, f)
		}
	}
	return out
}
