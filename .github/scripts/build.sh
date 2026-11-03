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
# per platform.
#
# On a Mac, Nix's shell has an xcrun of its own, which finds tools in the
# SDK of nixpkgs (its DEVELOPER_DIR), without notarytool or stapler. When
# release.yml signs, APPLE_TOOLS holds the Mac's own xcrun and codesign,
# which mygo build signs and notarizes with: they come first.
set -euo pipefail

if [ -n "${APPLE_TOOLS:-}" ]; then
	PATH="$APPLE_TOOLS:$PATH"
fi

IFS=, read -r -a platforms <<<"${1:?usage: build.sh <platform>[,<platform>...]}"
for p in "${platforms[@]}"; do
	case "$p" in
	linux/amd64) loader=/lib64/ld-linux-x86-64.so.2 ;;
	linux/arm64) loader=/lib/ld-linux-aarch64.so.1 ;;
	*) loader= ;;
	esac
	MYGO_LDFLAGS="${MYGO_LDFLAGS:-}${loader:+ -I $loader}" bun run build -- -platform "$p"
done
