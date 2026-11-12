#!/usr/bin/env bash
# Tells, as the step output newest, whether the commit a push to main runs
# for is still main's newest, of those this workflow runs for. When main
# moved on with a commit that changes a path of the workflow's on.push.paths,
# that push's run deploys instead, so this one has nothing to deploy: it
# ends, rather than wait in a line from which GitHub would cancel it, and
# show the commit as failed. When the commits after this one change none of
# those paths, as a fix of the app after a change of the site, no newer run
# of the workflow started, and this one deploys.
#
#   - id: newest
#     run: bash .github/scripts/newest.sh
#   - if: steps.newest.outputs.newest == 'true'
#
# The job needs the workflow's file, as a sparse checkout of
# .github/workflows gives it. A run started by hand, or for a pull request,
# always goes on, and so does one that can't read main, three times 5
# seconds apart, as all did before. One that can't tell what the commits
# after it change leaves the deploy to the newest, as all did before.
set -uo pipefail

# pushPaths prints the paths of the workflow's on.push.paths, one a line,
# as file, the workflow's YAML, lists them: nothing for a workflow that runs
# for every push.
pushPaths() {
	awk '
		/^  push:/ { push = 1; next }
		push && /^  [A-Za-z_]+:/ { push = 0 }
		push && /^    paths:/ { paths = 1; next }
		paths && /^      - / { p = $0; sub(/^      - /, "", p); gsub(/["'"'"']/, "", p); if (p !~ /^!/) print p; next }
		paths { paths = 0 }
	' "$1"
}

# laterChanges tells whether the commits after this run's, up to head,
# change a path the workflow's pushes run for: then a push of them started
# a run of it, which deploys instead. It tells so when it can't tell.
laterChanges() {
	local head=$1 file
	file="${GITHUB_WORKFLOW_REF#*/*/}"
	file="${file%@*}"
	[ -f "$file" ] || return 0
	local -a paths=()
	mapfile -t paths < <(pushPaths "$file")
	[ "${#paths[@]}" -gt 0 ] || return 0
	# The tree of main's newest commit, without the files' contents: the
	# names of what changed are all it takes.
	git fetch --quiet --no-tags --depth=1 --filter=blob:none origin "$head" 2>/dev/null || return 0
	local -a specs=()
	local p
	for p in "${paths[@]}"; do specs+=(":(glob)$p"); done
	local changed
	changed="$(git diff --name-only --no-renames "$GITHUB_SHA" "$head" -- "${specs[@]}" 2>/dev/null)" || return 0
	[ -n "$changed" ]
}

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
		if laterChanges "$head"; then
			newest=false
			echo "::notice::main moved on to ${head:0:8}, whose run deploys instead."
		else
			echo "::notice::main moved on to ${head:0:8}, which changes nothing this workflow runs for: this run deploys."
		fi
	fi
fi
echo "newest=$newest" >>"$GITHUB_OUTPUT"
