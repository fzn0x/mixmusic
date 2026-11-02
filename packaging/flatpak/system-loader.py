#!/usr/bin/env python3
"""Points a program at the system's dynamic loader, in place.

    python3 system-loader.py <program>

The Go toolchain of Nix's shell, which the release and CI build MixMusic
with, links Nix's own loader, /nix/store/…/ld-linux-x86-64.so.2, into a
program that loads C libraries at run time, as MixMusic does through
purego. The Flatpak's runtime has no /nix: this sets the program's ELF
interpreter to the loader every glibc system has, which is shorter, and
leaves a program that has it already, or none, as it is.
"""

import struct
import sys

# The loaders of glibc, by ELF machine.
LOADERS = {
    62: "/lib64/ld-linux-x86-64.so.2",  # x86-64
    183: "/lib/ld-linux-aarch64.so.1",  # Arm64
}
PT_INTERP = 3


def main(path):
    with open(path, "r+b") as f:
        elf = bytearray(f.read())
        if elf[:4] != b"\x7fELF" or elf[4] != 2 or elf[5] != 1:
            sys.exit(f"{path}: not a 64-bit little-endian ELF program")
        machine = struct.unpack_from("<H", elf, 0x12)[0]
        loader = LOADERS.get(machine)
        if loader is None:
            sys.exit(f"{path}: no loader known for ELF machine {machine}")
        phoff = struct.unpack_from("<Q", elf, 0x20)[0]
        phentsize, phnum = struct.unpack_from("<HH", elf, 0x36)
        for i in range(phnum):
            kind, _, offset, _, _, size = struct.unpack_from("<IIQQQQ", elf, phoff + i * phentsize)
            if kind != PT_INTERP:
                continue
            old = bytes(elf[offset : offset + size]).split(b"\0")[0].decode()
            if old == loader:
                print(f"{path} runs on {loader}")
                return
            new = loader.encode()
            # The kernel reads the segment to its end, which must be a NUL,
            # and takes the path up to the first.
            if len(new) >= size:
                sys.exit(f"{path}: {loader} doesn't fit where {old} is")
            elf[offset : offset + size] = new + b"\0" * (size - len(new))
            f.seek(0)
            f.write(elf)
            print(f"{path} ran on {old}, now on {loader}")
            return
    print(f"{path} has no interpreter")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    main(sys.argv[1])
