#!/usr/bin/env bash
# Tells, as the step output newest, whether the commit a push to main runs
# for is still main's newest. When it isn't, a newer push's run deploys
# instead, so this one has nothing to deploy: it ends, rather than wait in
# a line from which GitHub would cancel it, and show the commit as failed.
#
#   - id: newest
#     run: bash .github/scripts/newest.sh
#   - if: steps.newest.outputs.newest == 'true'
#
# A run started by hand, or for a pull request, always goes on, and so
# does one that can't read main, three times 5 seconds apart, as all did
# before.
set -uo pipefail

newest=true
if [ "${GITHUB_EVENT_NAME:-}" = push ]; then
	head=""
	for try in 1 2 3; do
		[ "$try" -gt 1 ] && sleep 5
		head="$(git ls-remote "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY" refs/heads/main | cut -f1)" && [ -n "$head" ] && break
		head=""
	done
	if [ -z "$head" ]; then
		echo "::warning::Couldn't read main's newest commit: this run goes on."
	elif [ "$head" != "$GITHUB_SHA" ]; then
		newest=false
		echo "::notice::main moved on to ${head:0:8}, whose run deploys instead."
	fi
fi
echo "newest=$newest" >>"$GITHUB_OUTPUT"
