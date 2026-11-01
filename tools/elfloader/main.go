// Command elfloader checks that MixMusic's Linux builds ask for the
// system's own loader, at the path every distribution has it, so that they
// start outside NixOS too (#166). It reads the ELF programs in the folders
// given, and those in their .tar.gz archives, and fails on a program that
// asks for another loader, or for none.
//
//	go run ./tools/elfloader build/linux-amd64 build/linux-arm64
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// loaders are the loaders of the distributions, by machine.
var loaders = map[elf.Machine]string{
	elf.EM_X86_64:  "/lib64/ld-linux-x86-64.so.2",
	elf.EM_AARCH64: "/lib/ld-linux-aarch64.so.1",
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("usage: elfloader <folder>...")
	}
	n, bad := 0, 0
	for _, dir := range os.Args[1:] {
		err := programs(dir, func(name string, data []byte) {
			n++
			loader, err := check(data)
			if err != nil {
				bad++
				fmt.Printf("%s: %v\n", name, err)
				return
			}
			fmt.Printf("%s: %s\n", name, loader)
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	switch {
	case n == 0:
		log.Fatal("elfloader: no Linux program found")
	case bad > 0:
		log.Fatalf("elfloader: %d of %d programs ask for a loader other Linux systems don't have", bad, n)
	}
}

// programs calls found with each ELF program in dir, and in its .tar.gz
// archives, named by their path.
func programs(dir string, found func(name string, data []byte)) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		switch {
		case isELF(data):
			found(path, data)
		case strings.HasSuffix(e.Name(), ".tar.gz"):
			if err := archived(path, data, found); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
		}
	}
	return nil
}

// archived calls found with each ELF program in the .tar.gz data.
func archived(path string, data []byte, found func(name string, data []byte)) error {
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	t := tar.NewReader(z)
	for {
		h, err := t.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(t)
		if err != nil {
			return err
		}
		if isELF(b) {
			found(path+"!"+h.Name, b)
		}
	}
}

func isELF(b []byte) bool { return bytes.HasPrefix(b, []byte(elf.ELFMAG)) }

// check returns the loader the program asks for, or why it isn't the one
// its machine's distributions have.
func check(data []byte) (string, error) {
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	want, ok := loaders[f.Machine]
	if !ok {
		return "", fmt.Errorf("a program for %v, which MixMusic isn't built for", f.Machine)
	}
	for _, p := range f.Progs {
		if p.Type != elf.PT_INTERP {
			continue
		}
		b, err := io.ReadAll(p.Open())
		if err != nil {
			return "", err
		}
		got := strings.TrimRight(string(b), "\x00")
		if got != want {
			return "", fmt.Errorf("asks for the loader %s, not %s", got, want)
		}
		return got, nil
	}
	return "", errors.New("asks for no loader, so it can't load the system's libraries")
}
