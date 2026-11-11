#!/usr/bin/env bash
# Runs a Nix command, and runs it again when a download from the binary
# cache broke off, for up to 45 minutes: 10 seconds after the first, then
# twice as long after each, up to 5 minutes, as retry.sh waits. Any other
# failure fails at once, as the command would.
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

end=$((SECONDS + 45 * 60))
wait=10
try=1
while true; do
	"$@" 2>&1 | tee "$log"
	status=${PIPESTATUS[0]}
	if [ "$status" -eq 0 ]; then
		exit 0
	fi
	left=$((end - SECONDS))
	if [ "$left" -le 0 ] || ! grep -q -E "unable to download|usually happens due to networking issues" "$log"; then
		exit "$status"
	fi
	if [ "$wait" -gt "$left" ]; then
		wait=$left
	fi
	echo "::warning::A download from Nix's binary cache broke off (try $try): trying again in $wait s."
	sleep "$wait"
	try=$((try + 1))
	wait=$((wait * 2))
	if [ "$wait" -gt 300 ]; then
		wait=300
	fi
done
