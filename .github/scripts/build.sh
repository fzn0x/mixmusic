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
set -euo pipefail

IFS=, read -r -a platforms <<<"${1:?usage: build.sh <platform>[,<platform>...]}"
for p in "${platforms[@]}"; do
	case "$p" in
	linux/amd64) loader=/lib64/ld-linux-x86-64.so.2 ;;
	linux/arm64) loader=/lib/ld-linux-aarch64.so.1 ;;
	*) loader= ;;
	esac
	MYGO_LDFLAGS="${MYGO_LDFLAGS:-}${loader:+ -I $loader}" bun run build -- -platform "$p"
done
