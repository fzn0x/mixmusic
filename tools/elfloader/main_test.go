package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// program is a minimal 64-bit ELF program for machine, asking for loader,
// or for none when loader is empty.
func program(machine elf.Machine, loader string) []byte {
	const ehsize, phsize = 64, 56
	var b bytes.Buffer
	le := binary.LittleEndian
	ident := [16]byte{0x7f, 'E', 'L', 'F', byte(elf.ELFCLASS64), byte(elf.ELFDATA2LSB), byte(elf.EV_CURRENT)}
	b.Write(ident[:])
	phnum := uint16(0)
	if loader != "" {
		phnum = 1
	}
	binary.Write(&b, le, struct {
		Type, Machine                                        uint16
		Version                                              uint32
		Entry, Phoff, Shoff                                  uint64
		Flags                                                uint32
		Ehsize, Phentsize, Phnum, Shentsize, Shnum, Shstrndx uint16
	}{uint16(elf.ET_EXEC), uint16(machine), uint32(elf.EV_CURRENT), 0, ehsize, 0, 0, ehsize, phsize, phnum, 64, 0, 0})
	if loader != "" {
		s := loader + "\x00"
		binary.Write(&b, le, elf.Prog64{
			Type: uint32(elf.PT_INTERP), Flags: uint32(elf.PF_R),
			Off: ehsize + phsize, Filesz: uint64(len(s)), Memsz: uint64(len(s)), Align: 1,
		})
		b.WriteString(s)
	}
	return b.Bytes()
}

const nixLoader = "/nix/store/pqyiidz5mi73wv5f91d48s9plcqyj5bz-glibc-2.44-57/lib/ld-linux-x86-64.so.2"

// TestCheck takes the loaders distributions have, and turns away Nix's,
// which 0.5.6's x64 builds asked for, and a program asking for none.
func TestCheck(t *testing.T) {
	for _, c := range []struct {
		name    string
		data    []byte
		problem string
	}{
		{"x64", program(elf.EM_X86_64, "/lib64/ld-linux-x86-64.so.2"), ""},
		{"arm64", program(elf.EM_AARCH64, "/lib/ld-linux-aarch64.so.1"), ""},
		{"nix", program(elf.EM_X86_64, nixLoader), "asks for the loader /nix/store/"},
		{"arm64 with x64's", program(elf.EM_AARCH64, "/lib64/ld-linux-x86-64.so.2"), "not /lib/ld-linux-aarch64.so.1"},
		{"none", program(elf.EM_X86_64, ""), "asks for no loader"},
		{"another machine", program(elf.EM_RISCV, "/lib/ld-linux-riscv64-lp64d.so.1"), "isn't built for"},
	} {
		_, err := check(c.data)
		switch {
		case c.problem == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.problem != "" && (err == nil || !strings.Contains(err.Error(), c.problem)):
			t.Errorf("%s: got %v, want an error saying %q", c.name, err, c.problem)
		}
	}
}

// TestPrograms finds the programs in a build's folder and in its archive,
// as mygo build leaves them, and nothing else.
func TestPrograms(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("mixmusic", program(elf.EM_X86_64, "/lib64/ld-linux-x86-64.so.2"))
	write("install.sh", []byte("#!/bin/sh\n"))
	write("update-linux-amd64.json", []byte("{}"))

	var gz bytes.Buffer
	z := gzip.NewWriter(&gz)
	tw := tar.NewWriter(z)
	for name, data := range map[string][]byte{
		"mixmusic":       program(elf.EM_X86_64, nixLoader),
		"mixmusic.png":   []byte("\x89PNG"),
		"share/a/b.desk": []byte("[Desktop Entry]"),
	} {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg})
		tw.Write(data)
	}
	tw.Close()
	z.Close()
	write("mixmusic-0.5.6-linux-amd64.tar.gz", gz.Bytes())

	found := map[string]error{}
	if err := programs(dir, func(name string, data []byte) {
		_, err := check(data)
		found[filepath.Base(name)] = err
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("found %v, want the program and the archive's", found)
	}
	if err := found["mixmusic"]; err != nil {
		t.Errorf("the program: %v", err)
	}
	if err := found["mixmusic-0.5.6-linux-amd64.tar.gz!mixmusic"]; err == nil {
		t.Error("the archive's program asks for Nix's loader, and passed")
	}
}
