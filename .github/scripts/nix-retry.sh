#!/usr/bin/env bash
# Runs a Nix command, and runs it again when a download from the binary
# cache broke off, up to twice, 15 seconds apart. Any other failure fails
# at once, as the command would.
#
#   bash .github/scripts/nix-retry.sh nix build --print-build-logs
#
# A download that breaks partway through makes Nix fail the whole build
# with "unable to download" and "is required, but there is no substituter
# that can build it", rather than retrying the file. Running the command
# again fetches only what's missing.
set -uo pipefail

log="$(mktemp)"
trap 'rm -f "$log"' EXIT

for try in 1 2 3; do
	"$@" 2>&1 | tee "$log"
	status=${PIPESTATUS[0]}
	if [ "$status" -eq 0 ]; then
		exit 0
	fi
	if [ "$try" -eq 3 ] || ! grep -q -E "unable to download|usually happens due to networking issues" "$log"; then
		exit "$status"
	fi
	echo "::warning::A download from Nix's binary cache broke off: trying again (try $((try + 1)) of 3)."
	sleep 15
done
