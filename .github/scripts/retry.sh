#!/usr/bin/env bash
# Runs a command until it succeeds, for up to a number of minutes, 45 when
# not given: a download or an install that fails, as when a server or a
# mirror is out of reach for a while, tries again rather than fail the
# job. It waits 10 seconds after the first failure, then twice as long
# after each, up to 5 minutes. Each failure, and the wait, is a warning in
# the log. A command that still fails when the minutes are spent fails the
# step, with its last exit status.
#
#   bash .github/scripts/retry.sh 45 bun install --frozen-lockfile
#
# A command that fails every time, as one whose lockfile is out of date,
# fails once its minutes are spent.
set -uo pipefail

budget=45
if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
	budget="$1"
	shift
fi
if [ $# -eq 0 ]; then
	echo "usage: retry.sh [minutes] command [args...]" >&2
	exit 2
fi

end=$((SECONDS + budget * 60))
wait=10
try=1
while true; do
	"$@"
	status=$?
	if [ "$status" -eq 0 ]; then
		exit 0
	fi
	left=$((end - SECONDS))
	if [ "$left" -le 0 ]; then
		echo "::error::$* failed (exit status $status, try $try), and its $budget minutes are spent: giving up." >&2
		exit "$status"
	fi
	if [ "$wait" -gt "$left" ]; then
		wait=$left
	fi
	echo "::warning::$* failed (exit status $status, try $try): trying again in $wait s." >&2
	sleep "$wait"
	try=$((try + 1))
	wait=$((wait * 2))
	if [ "$wait" -gt 300 ]; then
		wait=300
	fi
done
