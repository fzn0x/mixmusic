# Changelog fragments

A change people will notice adds its entry to the changelog here, as a
file, rather than to [CHANGELOG.md](../CHANGELOG.md). Pull requests open at
the same time then never change the same lines, and never conflict over
the changelog.

Put the file in the folder of its section, as
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) names them:
`added`, `changed`, `deprecated`, `removed`, `fixed` or `security`. Name it
after the issue it closes, such as `fixed/101-stream-retry.md`; the issue's
number orders the entries. It holds the entry as CHANGELOG.md has them, a
Markdown list item, or a few, wrapped near 74 columns:

```md
- A song whose audio stops arriving partway through loads again where it
  was, rather than skipping to the next song (#101).
```

`go run ./tools/changelog -print` shows `## [Unreleased]` as the next
release will have it, and `go run ./tools/changelog -check`, which CI runs,
checks the fragments. A release folds them into CHANGELOG.md with
`go run ./tools/changelog`, which deletes them.
