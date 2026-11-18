package app

// Importing playlists (#613): from CSV and TSV files, M3U and M3U8 lists,
// text, JSON, and the archives of Spotify's and Google's data downloads,
// which internal/playlistio reads. Each song becomes a track as its IDs
// say, else as the library has it, else as YouTube Music has it
// (match.Find), at a pace that leaves playback room
// (playlistimport_match.go). The user reviews what was found, and the
// songs go to a new mix of each list, to a mix of theirs, or to Liked
// songs. An import under way is kept in import.json, and goes on after a
// restart; the mixes the lists went to are kept there too, so that
// importing a list again adds only what is new.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/playlistio"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/locales"
)

// importFileName is the file an import is kept in, in the data folder.
const importFileName = "import.json"

// ImportStep is where an import is.
type ImportStep string

// The steps of an import.
const (
	// ImportNone: no import is under way.
	ImportNone ImportStep = ""
	// ImportColumns: a table waits for the user to name its columns.
	ImportColumns ImportStep = "columns"
	// ImportPick: the lists read wait for the user to pick those to
	// import.
	ImportPick ImportStep = "pick"
	// ImportMatching: the songs are being looked for.
	ImportMatching ImportStep = "matching"
	// ImportHalted: the looking stopped, on a refusal of the service or
	// errors, until the user tries again.
	ImportHalted ImportStep = "halted"
	// ImportReview: what was found waits for the user to review it.
	ImportReview ImportStep = "review"
)

// ImportState is what became of a song an import looked for.
type ImportState int

// The states of a song of an import.
const (
	// ImportPending: not looked for yet.
	ImportPending ImportState = iota
	// ImportSure: found, sure.
	ImportSure
	// ImportCheck: something close found, for the user to check.
	ImportCheck
	// ImportMissing: not found.
	ImportMissing
)

// ImportRow is a song of an import: what its file said, and what was
// found of it.
type ImportRow struct {
	Entry playlistio.Entry `json:"entry"`
	State ImportState      `json:"state"`
	// Track is the track found: the song when sure, the closest when to
	// check.
	Track *media.Track `json:"track,omitempty"`
	// InLibrary is set when the library has the track already.
	InLibrary bool `json:"inLibrary,omitempty"`
	// Ticked is set when the song is to be imported.
	Ticked bool `json:"ticked,omitempty"`
}

// ImportList is a list an import read.
type ImportList struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
	// Source names where the list came from, which the mix it goes to
	// is remembered by.
	Source string `json:"source"`
	// Own is set for a list MixMusic wrote, whose songs come as they are.
	Own bool `json:"own,omitempty"`
	// Picked is set when the list is to be imported.
	Picked bool        `json:"picked"`
	Rows   []ImportRow `json:"rows"`
}

// importTable is a table that waits for the user to name its columns.
type importTable struct {
	Source string            `json:"source"`
	Table  *playlistio.Table `json:"table"`
}

// importJob is an import under way.
type importJob struct {
	Step   ImportStep    `json:"step"`
	Lists  []ImportList  `json:"lists"`
	Tables []importTable `json:"tables,omitempty"`
	// Halt is why the looking stopped: haltRate or haltErrors; Cause
	// the error, in English.
	Halt  string `json:"halt,omitempty"`
	Cause string `json:"cause,omitempty"`
	// Cut is set when the files held more songs than an import takes.
	Cut bool `json:"cut,omitempty"`
}

// importFile is what import.json keeps.
type importFile struct {
	Job *importJob `json:"job,omitempty"`
	// Sources are the mixes the lists imported went to, by their sources:
	// a mix's ID, or importLiked.
	Sources map[string]string `json:"sources,omitempty"`
}

// importLiked is the source's mix of a list that went to Liked songs.
const importLiked = "liked"

// The reasons the looking stopped.
const (
	haltRate   = "rate"
	haltErrors = "errors"
)

// importState is the import of the app, and what runs it.
type importState struct {
	mu      sync.Mutex
	opened  bool
	data    importFile
	version uint64
	// run is the looking under way, if any, and stopping why it is told
	// to stop: "stop", "cancel" or "quit".
	run      *importRun
	stopping string
	saved    time.Time
	cache    *importCache
	watchers []*func()
	// services, when set, look for songs in place of the app's: tests set
	// it (UseServices).
	services *ImportServices
	// finishing is set while Finish adds the songs.
	finishing bool
}

// importRun is a looking under way.
type importRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Imports imports playlists from files and text, and exports them.
type Imports struct{ a *App }

// Imports returns the imports of playlists.
func (c *Collection) Imports() *Imports { return &Imports{c.a} }

// ImportListInfo is a list an import read, as the user picks it.
type ImportListInfo struct {
	Name   string
	Kind   string
	Songs  int
	Picked bool
}

// ImportCounts count the songs of the lists picked: found and sure, to
// check, of the library already, not found, and ticked.
type ImportCounts struct {
	Found, Check, InLibrary, Missing, Ticked int
}

// ImportStatus is where the import is, for the windows.
type ImportStatus struct {
	Step ImportStep
	// Version counts the import's changes.
	Version uint64
	Lists   []ImportListInfo
	// Table is the table whose columns the user names, at
	// ImportColumns, and TableSource the file it came from.
	Table *playlistio.Table
	// Done and Total count the songs looked for of those to look for.
	Done, Total int
	// Halt is why the looking stopped, at ImportHalted.
	Halt   error
	Counts ImportCounts
	// Cut is set when the files held more than playlistio.MaxEntries
	// songs: those after went.
	Cut bool
}

// ImportFailure is a file an import couldn't read, and why.
type ImportFailure struct {
	Name string
	Err  error
}

// ImportOpened is what Open read: how many songs, and the files it
// couldn't read.
type ImportOpened struct {
	Songs  int
	Failed []ImportFailure
}

// importLoad reads import.json once. It holds im.mu.
func (a *App) importLoad() *importState {
	im := &a.imports
	if im.opened {
		return im
	}
	im.opened = true
	data, err := store.ReadFile(filepath.Join(a.opts.DataDir, importFileName))
	if err == nil {
		if err := json.Unmarshal(data, &im.data); err != nil {
			log.Printf("mixmusic: reading %s, which starts anew: %v", importFileName, err)
			im.data = importFile{}
		}
	}
	return im
}

// importSave writes import.json. It holds im.mu.
func (a *App) importSave() {
	im := &a.imports
	im.saved = time.Now()
	data, err := json.Marshal(im.data)
	if err == nil {
		err = store.WriteFile(filepath.Join(a.opts.DataDir, importFileName), data)
	}
	if err != nil {
		log.Printf("mixmusic: saving %s: %v", importFileName, err)
	}
}

// importChanged counts a change, and tells the watchers, beside im.mu.
func (a *App) importChanged() {
	im := &a.imports
	im.version++
	fns := im.watchers
	go func() {
		for _, fn := range fns {
			(*fn)()
		}
	}()
}

// Watch has fn told, from any goroutine, when the import changes, until
// cancel.
func (i *Imports) Watch(fn func()) (cancel func()) {
	im := &i.a.imports
	f := &fn
	im.mu.Lock()
	im.watchers = append(slices.Clone(im.watchers), f)
	im.mu.Unlock()
	return func() {
		im.mu.Lock()
		im.watchers = slices.DeleteFunc(slices.Clone(im.watchers), func(g *func()) bool { return g == f })
		im.mu.Unlock()
	}
}

// Status returns where the import is.
func (i *Imports) Status() ImportStatus {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	st := ImportStatus{Version: im.version}
	j := im.data.Job
	if j == nil {
		return st
	}
	st.Step, st.Cut = j.Step, j.Cut
	if st.Step == ImportMatching && im.run == nil {
		// Kept as it went, and not going on yet.
		st.Step = ImportHalted
	}
	if len(j.Tables) > 0 {
		st.Table = j.Tables[0].Table
	}
	for _, l := range j.Lists {
		st.Lists = append(st.Lists, ImportListInfo{Name: l.Name, Kind: l.Kind, Songs: len(l.Rows), Picked: l.Picked})
		if !l.Picked {
			continue
		}
		for _, r := range l.Rows {
			st.Total++
			switch {
			case r.State == ImportPending:
				st.Counts.Missing++
				continue
			case r.State == ImportMissing:
				st.Counts.Missing++
			case r.State == ImportCheck:
				st.Counts.Check++
			case r.InLibrary:
				st.Counts.InLibrary++
			default:
				st.Counts.Found++
			}
			st.Done++
			if r.Ticked {
				st.Counts.Ticked++
			}
		}
	}
	if j.Halt != "" || st.Step == ImportHalted {
		st.Halt = haltError(j.Halt, j.Cause)
	}
	return st
}

// Lists returns the lists of the import with their songs, for the review.
func (i *Imports) Lists() []ImportList {
	im := &i.a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	i.a.importLoad()
	if im.data.Job == nil {
		return nil
	}
	out := slices.Clone(im.data.Job.Lists)
	for k := range out {
		out[k].Rows = slices.Clone(out[k].Rows)
	}
	return out
}

// Open reads the lists of files, at most playlistio.MaxFiles of them, and
// starts an import of them in place of one under way: next, the user
// names the columns of a table that needs it, or picks the lists of
// several; else the looking starts. It tells the files it couldn't read,
// and fails when it read none.
func (i *Imports) Open(paths []string) (ImportOpened, error) {
	if len(paths) > playlistio.MaxFiles {
		return ImportOpened{}, wordedLater(fmt.Sprintf("MixMusic imports %d files at once at most", playlistio.MaxFiles),
			func() string { return locales.PlaylistImportTooManyFiles(playlistio.MaxFiles) }, nil)
	}
	var out ImportOpened
	job := &importJob{}
	text := int64(0)
	for _, p := range paths {
		name := filepath.Base(p)
		if info, err := os.Stat(p); err == nil && !strings.EqualFold(filepath.Ext(p), ".zip") {
			if text += info.Size(); text > playlistio.MaxText {
				out.Failed = append(out.Failed, ImportFailure{name, importReadError(playlistio.ErrTooBig)})
				continue
			}
		}
		src, err := playlistio.ReadFile(p)
		if err != nil {
			out.Failed = append(out.Failed, ImportFailure{name, importReadError(err)})
			continue
		}
		i.a.addSource(job, src, sourceKey(src, name))
	}
	return out, i.start(job, &out)
}

// Paste starts an import of text the user pasted, as Open does of files.
func (i *Imports) Paste(text string) (ImportOpened, error) {
	var out ImportOpened
	src, err := playlistio.ReadText(locales.PlaylistImportPastedName(), text)
	if err != nil {
		return out, importReadError(err)
	}
	job := &importJob{}
	sum := sha256.Sum256([]byte(text))
	i.a.addSource(job, src, "text:"+hex.EncodeToString(sum[:8]))
	return out, i.start(job, &out)
}

// sourceKey names where a file's lists come from: the data download, or
// the file's name.
func sourceKey(src playlistio.Source, name string) string {
	if src.Archive != "" {
		return "archive:" + src.Archive
	}
	return "file:" + strings.ToLower(name)
}

// addSource adds the lists and the tables of src to job, as many songs
// as an import takes.
func (a *App) addSource(job *importJob, src playlistio.Source, key string) {
	count := 0
	for _, l := range job.Lists {
		count += len(l.Rows)
	}
	for _, l := range src.Lists {
		if count >= playlistio.MaxEntries {
			job.Cut = true
			break
		}
		if len(l.Entries) > playlistio.MaxEntries-count {
			l.Entries, job.Cut = l.Entries[:playlistio.MaxEntries-count], true
		}
		count += len(l.Entries)
		job.Lists = append(job.Lists, importListOf(l, key))
	}
	for _, t := range src.Tables {
		job.Tables = append(job.Tables, importTable{Source: key, Table: t})
	}
}

// importListOf is the list of an import of l, from the source key.
func importListOf(l playlistio.List, key string) ImportList {
	name := l.Name
	switch l.Kind {
	case playlistio.KindLibrary:
		name = locales.PlaylistImportYourLibrary()
	case playlistio.KindHistory:
		name = locales.PlaylistImportHistory()
	}
	if strings.TrimSpace(name) == "" {
		name = locales.PlaylistImportPastedName()
	}
	out := ImportList{Name: name, Kind: l.Kind, Source: key + "\x00" + l.Kind + "\x00" + l.Name, Own: l.Own, Picked: true}
	out.Rows = make([]ImportRow, len(l.Entries))
	for k, e := range l.Entries {
		out.Rows[k] = ImportRow{Entry: e}
	}
	return out
}

// start makes job the import, in place of one under way, at its first
// step, and fails as Open does when it holds nothing.
func (i *Imports) start(job *importJob, out *ImportOpened) error {
	for _, l := range job.Lists {
		out.Songs += len(l.Rows)
	}
	if len(job.Lists) == 0 && len(job.Tables) == 0 {
		if len(out.Failed) > 0 {
			return out.Failed[0].Err
		}
		return importReadError(playlistio.ErrNoEntries)
	}
	a := i.a
	a.stopImportRun("cancel")
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	im.data.Job = job
	a.importNext()
	return nil
}

// importNext moves the import to its next step: a table to name the
// columns of, the lists to pick of several, else the looking. It holds
// im.mu.
func (a *App) importNext() {
	im := &a.imports
	j := im.data.Job
	switch {
	case len(j.Tables) > 0:
		j.Step = ImportColumns
	case len(j.Lists) > 1 && j.Step != ImportPick && j.Step != ImportMatching:
		j.Step = ImportPick
	case len(j.Lists) == 0:
		im.data.Job = nil
	default:
		j.Step = ImportMatching
		a.startImportRun()
	}
	a.importSave()
	a.importChanged()
}

// SetColumns names the columns of the table that waits: fields holds what
// each holds, and header tells whether its first row names them. The
// import moves on.
func (i *Imports) SetColumns(fields []playlistio.Field, header bool) {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	j := im.data.Job
	if j == nil || len(j.Tables) == 0 {
		return
	}
	t := j.Tables[0]
	j.Tables = j.Tables[1:]
	row := -1
	if header {
		row = 0
	}
	for _, l := range t.Table.Lists(fields, row) {
		if len(l.Entries) > 0 {
			j.Lists = append(j.Lists, importListOf(l, t.Source))
		}
	}
	a.importNext()
}

// SkipTable leaves out the table that waits, and moves the import on.
func (i *Imports) SkipTable() {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	if j := im.data.Job; j != nil && len(j.Tables) > 0 {
		j.Tables = j.Tables[1:]
		a.importNext()
	}
}

// Pick picks the lists to import, picked[k] of the k-th, and starts
// looking for their songs.
func (i *Imports) Pick(picked []bool) {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	j := im.data.Job
	if j == nil || j.Step != ImportPick {
		return
	}
	any := false
	for k := range j.Lists {
		j.Lists[k].Picked = k < len(picked) && picked[k]
		any = any || j.Lists[k].Picked
	}
	if !any {
		return
	}
	j.Step = ImportMatching
	a.startImportRun()
	a.importSave()
	a.importChanged()
}

// Stop stops looking, for the user to review what was found, as after a
// halt too: the songs not looked for yet count as not found, and
// TryAgain looks for them.
func (i *Imports) Stop() {
	a := i.a
	a.stopImportRun("stop")
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	if j := im.data.Job; j != nil && im.run == nil && (j.Step == ImportMatching || j.Step == ImportHalted) {
		j.Step, j.Halt, j.Cause = ImportReview, "", ""
		a.importSave()
		a.importChanged()
	}
}

// TryAgain looks again for the songs not looked for yet, as after a halt.
func (i *Imports) TryAgain() {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	j := im.data.Job
	if j == nil || im.run != nil || (j.Step != ImportMatching && j.Step != ImportHalted && j.Step != ImportReview) {
		return
	}
	j.Step, j.Halt, j.Cause = ImportMatching, "", ""
	a.startImportRun()
	a.importSave()
	a.importChanged()
}

// Cancel drops the import.
func (i *Imports) Cancel() {
	a := i.a
	a.stopImportRun("cancel")
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	if im.data.Job != nil {
		im.data.Job = nil
		a.importSave()
		a.importChanged()
	}
}

// SetTicked ticks the row-th song of the list-th list, or unticks it.
func (i *Imports) SetTicked(list, row int, on bool) {
	i.editRow(list, row, func(r *ImportRow) { r.Ticked = on && r.Track != nil })
}

// TickAll ticks every song found of the lists picked, or unticks them.
func (i *Imports) TickAll(on bool) {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	if j := im.data.Job; j != nil {
		for k := range j.Lists {
			for n := range j.Lists[k].Rows {
				r := &j.Lists[k].Rows[n]
				r.Ticked = on && r.Track != nil
			}
		}
		a.importChanged()
	}
}

// Replace makes t the track of the row-th song of the list-th list, as
// the user chose it: sure, and ticked.
func (i *Imports) Replace(list, row int, t media.Track) {
	ids := i.a.library.TrackIDs()
	t = i.a.asSong(t)
	i.editRow(list, row, func(r *ImportRow) {
		r.Track, r.State, r.Ticked = &t, ImportSure, true
		r.InLibrary = ids[t.ID] || t.Source == media.SourceFile
	})
}

// editRow changes a row of the import with fn.
func (i *Imports) editRow(list, row int, fn func(r *ImportRow)) {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	defer im.mu.Unlock()
	a.importLoad()
	j := im.data.Job
	if j == nil || list < 0 || list >= len(j.Lists) || row < 0 || row >= len(j.Lists[list].Rows) {
		return
	}
	fn(&j.Lists[list].Rows[row])
	if im.run == nil {
		a.importSave()
	}
	a.importChanged()
}

// Misses are the songs of the lists picked that weren't found, or looked
// for, as the lists they were in, for the user to keep.
func (i *Imports) Misses() []playlistio.ExportList {
	var out []playlistio.ExportList
	for _, l := range i.Lists() {
		if !l.Picked {
			continue
		}
		el := playlistio.ExportList{Name: l.Name}
		for _, r := range l.Rows {
			if r.State == ImportMissing || r.State == ImportPending {
				el.Rows = append(el.Rows, playlistio.EntryRow(r.Entry))
			}
		}
		if len(el.Rows) > 0 {
			out = append(out, el)
		}
	}
	return out
}

// SaveMisses writes the songs not found to path, as CSV.
func (i *Imports) SaveMisses(path string) error {
	return writeLists(path, playlistio.FormatCSV, i.Misses())
}

// ImportTarget is where an import's songs go.
type ImportTarget struct {
	// Mix is the ID of the user's mix they go to, "" for a new mix of
	// each list.
	Mix string
	// Liked has them go to Liked songs.
	Liked bool
	// Name names the new mix of a list alone, "" for the list's own name.
	Name string
}

// ImportResult is what an import came to: the mixes the songs went to,
// and how many went.
type ImportResult struct {
	Mixes []media.Playlist
	Added int
	Liked bool
}

// errImportGone: the import was finished, or dropped, meanwhile.
var errImportGone = errors.New("the import isn't under way any more")

// Finish adds the songs ticked to target, and ends the import: to Liked
// songs, liked on their service, as any like is; to a mix of the user's;
// or to a new mix of each list, named as the list, in its order. A list
// imported before goes to the mix it went to, if it's there still, and
// any mix gets only the songs it hasn't.
func (i *Imports) Finish(ctx context.Context, target ImportTarget) (ImportResult, error) {
	a := i.a
	im := &a.imports
	im.mu.Lock()
	a.importLoad()
	j := im.data.Job
	if j == nil || im.finishing || im.run != nil {
		im.mu.Unlock()
		return ImportResult{}, errImportGone
	}
	im.finishing = true
	lists := slices.Clone(j.Lists)
	sources := im.data.Sources
	im.mu.Unlock()
	defer func() {
		im.mu.Lock()
		im.finishing = false
		im.mu.Unlock()
	}()

	type picked struct {
		list   ImportList
		tracks []media.Track
	}
	var all []picked
	for _, l := range lists {
		if !l.Picked {
			continue
		}
		seen := map[string]bool{}
		var tracks []media.Track
		for _, r := range l.Rows {
			if r.Ticked && r.Track != nil && !seen[r.Track.ID] {
				seen[r.Track.ID] = true
				tracks = append(tracks, *r.Track)
			}
		}
		all = append(all, picked{l, a.asSongs(tracks)})
	}
	var res ImportResult
	went := map[string]string{}
	switch {
	case target.Liked:
		var tracks []media.Track
		for _, p := range all {
			tracks = append(tracks, p.tracks...)
			went[p.list.Source] = importLiked
		}
		added, err := a.library.LikeAll(tracks)
		if err != nil {
			return res, err
		}
		res.Liked, res.Added = true, len(added)
		go a.syncImportedLikes(added)
	case target.Mix != "":
		var tracks []media.Track
		for _, p := range all {
			tracks = append(tracks, p.tracks...)
			went[p.list.Source] = target.Mix
		}
		mix, n, err := a.addToMix(target.Mix, tracks)
		if err != nil {
			return res, err
		}
		res.Mixes, res.Added = append(res.Mixes, mix), n
	default:
		for _, p := range all {
			if len(p.tracks) == 0 {
				continue
			}
			if id := sources[p.list.Source]; id != "" && id != importLiked {
				if mix, n, err := a.addToMix(id, p.tracks); err == nil {
					res.Mixes, res.Added = append(res.Mixes, mix), res.Added+n
					went[p.list.Source] = id
					continue
				}
			}
			name := p.list.Name
			if target.Name != "" && len(all) == 1 {
				name = target.Name
			}
			mix, err := a.library.CreatePlaylist(name, "", p.tracks)
			if err != nil {
				return res, err
			}
			res.Mixes, res.Added = append(res.Mixes, mix), res.Added+len(p.tracks)
			went[p.list.Source] = mix.ID
		}
	}
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.data.Sources == nil {
		im.data.Sources = map[string]string{}
	}
	for source, id := range went {
		im.data.Sources[source] = id
	}
	im.data.Job = nil
	a.importSave()
	a.importChanged()
	return res, nil
}

// addToMix adds to the mix of id those of tracks it hasn't, and returns
// it and how many it added.
func (a *App) addToMix(id string, tracks []media.Track) (media.Playlist, int, error) {
	native, err := localID(id)
	if err != nil {
		return media.Playlist{}, 0, err
	}
	have, err := a.library.PlaylistTrackIDs(native)
	if err != nil {
		return media.Playlist{}, 0, err
	}
	var add []media.Track
	for _, t := range tracks {
		if !have[t.ID] {
			add = append(add, t)
		}
	}
	if len(add) > 0 {
		if err := a.library.AddToPlaylist(native, add); err != nil {
			return media.Playlist{}, 0, err
		}
	}
	mix, err := a.library.Playlist(native)
	return mix, len(add), err
}

// syncImportedLikes likes the tracks an import liked on their services,
// and has the scrobbling services hear of it, as SetLiked does of a like.
func (a *App) syncImportedLikes(tracks []media.Track) {
	ctx := context.Background()
	for _, t := range tracks {
		a.syncYouTubeLike(ctx, t, true)
		a.syncSpotifyLike(ctx, t, true)
		a.sendLoves(ctx, t, true)
	}
}

// writeLists writes lists to path in format f, safely.
func writeLists(path string, f playlistio.Format, lists []playlistio.ExportList) error {
	return store.Replace(path, 0o644, func(out *os.File) error { return playlistio.Write(out, f, lists) })
}

// importReadError is why a file couldn't be read, in the words of the
// interface: a piece of the message that names the file.
func importReadError(err error) error {
	words := map[error]func() string{
		playlistio.ErrTooBig:         locales.PlaylistImportErrTooBig,
		playlistio.ErrArchiveTooBig:  locales.PlaylistImportErrArchiveTooBig,
		playlistio.ErrArchiveFiles:   locales.PlaylistImportErrArchiveFiles,
		playlistio.ErrNoEntries:      locales.PlaylistImportErrNoSongs,
		playlistio.ErrNotAList:       locales.PlaylistImportErrNotAList,
		playlistio.ErrNoDataDownload: locales.PlaylistImportErrNotDataDownload,
	}
	for known, say := range words {
		if errors.Is(err, known) {
			return wordedLater(known.Error(), say, err)
		}
	}
	return err
}

// haltError is why the looking stopped, in the words of the interface.
func haltError(halt, cause string) error {
	switch halt {
	case haltRate:
		return wordedLater("YouTube Music asks MixMusic to slow down: try again in a few minutes",
			locales.PlaylistImportHaltRate, errors.New(cause))
	case haltErrors:
		return wordedLater("3 searches in a row failed: "+cause,
			func() string { return locales.PlaylistImportHaltErrors(cause) }, errors.New(cause))
	}
	return wordedLater("MixMusic quit while it looked for the songs", locales.PlaylistImportHaltQuit, nil)
}
