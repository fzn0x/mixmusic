//go:build windows

package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// posixRename renames from over to as newer programs do, with POSIX
// semantics: through SetFileInformationByHandle's FileRenameInfoEx, which
// replaces a file open with FILE_SHARE_DELETE.
func posixRename(from, to string) error {
	src, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	const deleteAccess = 0x00010000
	h, err := syscall.CreateFile(src, deleteAccess, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	name, err := syscall.UTF16FromString(to)
	if err != nil {
		return err
	}
	// FILE_RENAME_INFO, whose name starts right after FileNameLength.
	type renameInfo struct {
		Flags          uint32
		RootDirectory  syscall.Handle
		FileNameLength uint32
	}
	at := unsafe.Offsetof(renameInfo{}.FileNameLength) + 4
	buf := make([]byte, int(at)+len(name)*2+8)
	info := (*renameInfo)(unsafe.Pointer(&buf[0]))
	const replaceIfExists, posixSemantics, fileRenameInfoEx = 1, 2, 22
	info.Flags = replaceIfExists | posixSemantics
	info.FileNameLength = uint32((len(name) - 1) * 2)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[at])), len(name)), name)
	set := syscall.NewLazyDLL("kernel32.dll").NewProc("SetFileInformationByHandle")
	if r, _, err := set.Call(uintptr(h), fileRenameInfoEx, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r == 0 {
		return err
	}
	return nil
}

// TestReadLetsAnEditorSave saves settings.json, in each way editors do,
// while MixMusic has it open to read: through os.Open's handle, which
// shares no deletes, the saves fail; through openShared's, they go
// through. Only a rename over the file without POSIX semantics, which
// MoveFileEx does, fails either way: Windows doesn't replace a file that
// is open at all, so it can fail during the moment a read takes.
func TestReadLetsAnEditorSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	saved := filepath.Join(dir, "saved.tmp")
	saves := []struct {
		name string
		save func() error
	}{
		// As programs that rename with POSIX semantics replace a file.
		{"replace", func() error { return posixRename(saved, path) }},
		// As editors that keep the file they replace as a backup save.
		{"rename away", func() error { return os.Rename(path, path+".bak") }},
		{"delete", func() error { return os.Remove(path) }},
	}
	try := func(open func(string) (*os.File, error), save func() error) error {
		writeText(t, path, `{"theme": "mix"}`)
		writeText(t, saved, `{"theme": "aurora"}`)
		os.Remove(path + ".bak")
		f, err := open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		return save()
	}
	for _, c := range saves {
		if err := try(os.Open, c.save); err == nil {
			t.Errorf("%s went through os.Open's handle too: openShared is no longer needed for it", c.name)
		}
		if err := try(openShared, c.save); err != nil {
			t.Errorf("%s while MixMusic reads the file: %v", c.name, err)
		}
	}
	if err := try(openShared, func() error { return os.Rename(saved, path) }); err != nil {
		t.Logf("MoveFileEx over a file open to read: %v", err)
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadFile(filepath.Join(dir, "missing.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("reading a missing file: %v, want fs.ErrNotExist", err)
	}
	if _, err := ReadFile(filepath.Join(dir, "no", "folder.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("reading in a missing folder: %v, want fs.ErrNotExist", err)
	}
	path := filepath.Join(dir, "settings.json")
	writeText(t, path, "\xEF\xBB\xBF{}")
	if got, err := ReadFile(path); err != nil || string(got) != "\xEF\xBB\xBF{}" {
		t.Errorf("ReadFile = %q, %v", got, err)
	}
	if _, err := ReadFile(dir); err == nil {
		t.Error("a folder read as a file")
	}
}

// TestWriteWaitsForAnEditor saves settings.json while another program has
// it open without sharing deletes, for a moment: the save goes through
// once it closes it.
func TestWriteWaitsForAnEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	writeText(t, path, `{"theme": "mix"}`)
	held, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// What MixMusic's write did before: it fails while the file is open.
	other := filepath.Join(dir, "other.tmp")
	writeText(t, other, "{}")
	if err := os.Rename(other, path); err == nil {
		held.Close()
		t.Skip("os.Open shares deletes here: a save never waits")
	} else if !busy(err) {
		t.Fatalf("replacing an open file: %v, want a sharing error", err)
	}
	closed := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		held.Close()
		close(closed)
	}()
	if err := WriteFile(path, []byte(`{"theme": "aurora"}`)); err != nil {
		t.Fatalf("WriteFile while the file was open a moment: %v", err)
	}
	<-closed
	if got := readText(t, path); got != `{"theme": "aurora"}` {
		t.Errorf("the file holds %q", got)
	}
}

// TestWriteGivesUp saves while another program keeps the file open: the
// save fails after a moment, and leaves no temporary file.
func TestWriteGivesUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	writeText(t, path, `{"theme": "mix"}`)
	held, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	start := time.Now()
	err = WriteFile(path, []byte(`{"theme": "aurora"}`))
	if err == nil {
		t.Skip("os.Open shares deletes here: a save never waits")
	}
	if !busy(err) {
		t.Errorf("WriteFile = %v, want a sharing error", err)
	}
	if took := time.Since(start); took < 80*time.Millisecond || took > 2*time.Second {
		t.Errorf("WriteFile gave up after %v, want about 100 ms", took)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("the folder holds %d files, want settings.json alone", len(entries))
	}
}
