#!/usr/bin/env bash
# Builds MixMusic for each platform given, as `bun run build -- -platform`
# does, one platform at a time.
#
#   .github/scripts/build.sh linux/amd64,linux/arm64
#
# A Linux build asks for the system's own loader, at the path every
# distribution has it. Nix's Go links a program for the machine it runs on
# with Nix's loader (/nix/store/…-glibc-…/lib/ld-linux-x86-64.so.2), which
# no other Linux has, so 0.5.6's x64 builds didn't start outside NixOS
# (#166). MYGO_LDFLAGS applies to every target of a build: hence a build
# per platform. After each, the log lists what its archive holds, and the
# script stops if it ships Go source (#303).
#
# On a Mac, Nix's shell has an xcrun of its own, which finds tools in the
# SDK of nixpkgs (its DEVELOPER_DIR), without notarytool or stapler. When
# release.yml signs, APPLE_TOOLS holds the Mac's own xcrun and codesign,
# which mygo build signs and notarizes with: they come first.
set -euo pipefail

if [ -n "${APPLE_TOOLS:-}" ]; then
	PATH="$APPLE_TOOLS:$PATH"
fi

# shipped lists what the archive of the build in dir holds, and fails if
# the build ships Go source, beside the app or in its archive: `mygo
# build` ships every file of resources/ with the app, beside it or in a
# Mac's Contents/Resources, as 0.5.9's did resources.go (#303).
shipped() {
	local dir="$1" archive found
	for archive in "$dir"/*.tar.gz; do
		if [ -f "$archive" ]; then
			echo "$archive holds:"
			tar -tzf "$archive" | sed 's/^/  /'
		fi
	done
	found="$(
		find "$dir" -name '*.go'
		for archive in "$dir"/*.tar.gz; do
			if [ -f "$archive" ]; then
				tar -tzf "$archive" | awk -v a="$archive" '/\.go$/ { print a ": " $0 }'
			fi
		done
	)"
	if [ -n "$found" ]; then
		echo "build.sh: the build ships Go source, which the app never reads:" >&2
		echo "$found" >&2
		echo "mygo build ships every file of resources/ with the app: keep files the app doesn't read as it runs out of it." >&2
		exit 1
	fi
}

IFS=, read -r -a platforms <<<"${1:?usage: build.sh <platform>[,<platform>...]}"
for p in "${platforms[@]}"; do
	case "$p" in
	linux/amd64) loader=/lib64/ld-linux-x86-64.so.2 ;;
	linux/arm64) loader=/lib/ld-linux-aarch64.so.1 ;;
	*) loader= ;;
	esac
	MYGO_LDFLAGS="${MYGO_LDFLAGS:-}${loader:+ -I $loader}" bun run build -- -platform "$p"
	shipped "build/${p/\//-}"
done
