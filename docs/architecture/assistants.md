# AI assistants

Settings › Desktop › AI assistants lets the AI assistants of this computer
control MixMusic over MCP, the Model Context Protocol (#477). It is off at
first.

## How an assistant connects

The assistant starts a command, `MixMusic --mcp`, and speaks MCP's stdio
transport with it: one JSON-RPC message per line on its standard input
and output. The command passes those lines, as they are, to the MixMusic
that runs, over a TCP connection to `127.0.0.1`, and its answers back, and
ends as either side closes. MCP's stdio framing works unchanged over a
TCP connection, as the protocol says, so MixMusic serves the connection as
it would the assistant's own pipes.

Why a command, and not MCP's Streamable HTTP at an address with a key:

- Every desktop assistant takes a command, as MCP asks its clients to
  take stdio wherever they can; an address with a key works only in those
  that take HTTP with headers of their own, and some file of servers of a
  desktop assistant takes commands alone.
- The configuration holds no secret: the command reads the key from the
  data folder, where only the user reads it, so the key changes each time
  the switch turns on, and no file of an assistant goes stale.
- Turning the switch off ends the sessions at once, and the assistants see
  it: the command ends with the connection, and the assistant sees its
  server stop. An HTTP client would learn it only at its next request.
- Nothing speaks HTTP, so a web page can't reach it either: a request of
  a browser, which may go to any port of `127.0.0.1`, isn't the key's
  line, and gets refused.

`main` runs the command before the single instance's lock, so that it
doesn't hand over to the MixMusic that runs, as a second start does, and
opens no window. The settings page shows the command for this copy:
MixMusic's program, the AppImage rather than the folder it mounts, or
`flatpak run` with the Flatpak's ID, with `MIXMUSIC_DATA_DIR` for a
portable copy; and the JSON of `mcpServers`, which most assistants take.

## The parts

- **`internal/mcp`** is the protocol, apart from the app. `Server` listens
  on `127.0.0.1` alone, at a port the system picks, and refuses a listener
  elsewhere; while it listens, `mcp.json` in the data folder holds the
  port and a key of 256 random bits, made at each start, written
  `0600` (the data folder's access lists keep it on Windows). A connection
  first gives the key, on a line of JSON, in 5 s, compared in constant
  time; then it is a session. `Stop` closes the listener and every
  session, and removes the file. `Relay` is the command's side.
- **The app** (`internal/app/assistant.go`) starts and stops the server as
  the setting `assistantControl` (`settings.Desktop`) says, from `apply`,
  and at quit; the file of a run that crashed goes at the next start. Its
  status is `System.AssistantStatus`, with the command, its JSON, and the
  assistants connected, by the names they give.
- **The tools** (`internal/app/assistanttools.go`) don't make a second
  player API: they read what plays and the queue as remote control tells
  them (`remoteStateOf`), with each track's service, and send play, pause,
  next, previous, seeking, the volume and the queue's entries through
  remote control's commands (`remoteHost.Do`), on the player remote
  control has (`remoteHost.player`). What they find plays as the windows
  play it: a song with `PlayWithRadio`, a collection with `PlayAll`, and
  `PlayNext` and `Enqueue` for next and last. Search goes through the
  catalog without keeping the query among the user's recent searches, and
  leaves out explicit content while the settings hide it. The results and
  the playlists are kept by ID (1,000 at most), for `play_result` and
  `play_playlist`, as MCP keeps no state between requests.
- **The toasts.** Each change a tool makes calls `Player.Tell`, which has
  the windows show a notice, as the player's own, after what the tool
  asked of the player. Reading shows none.
- **The window**: `settings_assistants.go` registers the group with
  `addSettings`; `assistantcommand.go` is the command.

## The protocol

The server speaks both eras of MCP:

- **Revision 2026-07-28**, whose requests each carry their version, the
  client's capabilities and its name in `_meta`, and which keeps no
  session: each request is served on its own. `server/discover` tells the
  versions; a version it doesn't speak gets `-32022` with those it does; a
  request without the client's capabilities gets `-32602`. Results carry
  `resultType` and the server's name in `_meta`, and the lists `ttlMs`
  and `cacheScope`. `subscriptions/listen` is acknowledged with nothing
  to tell, as the tools don't change while MixMusic runs, and gets its
  last answer as the session ends.
- **The revisions before, from 2024-11-05 to 2025-11-25**, whose sessions
  start with `initialize`: the server agrees on the version the assistant
  asks for, or the newest of those, and serves the session as that
  revision has it.

Requests run as they come, a tool beside the reading, so that a slow
search doesn't hold up a cancellation (`notifications/cancelled`, which
stops the tool and drops its answer). A session runs 4 tools at once at
most, and makes a burst of 10 calls, then one each 200 ms: past that, a
call gets an error the assistant reads. A tool's failure, as an argument
out of range or nothing to play, is a result with `isError`, which the
assistant reads to try again; an unknown tool or a malformed request is a
JSON-RPC error. A tool runs a minute at most, and a panic in one becomes
its error.

The server sends no requests of its own, and offers no resources and no
prompts.

## Dependencies

None. MCP's official Go SDK (v1.8.0) is pure Go, under the Apache
License 2.0 and the MIT License while it changes licence, but it would
bring ten modules MixMusic doesn't have, for OAuth, JWTs, JSON Schema and
HTTP transports, which a server of a dozen tools over a line protocol
doesn't use. The JSON-RPC MixMusic needs is in `internal/mcp`.

## Tests

`internal/mcp`'s tests speak both eras through `Relay`, as an assistant
would, against tools of their own, and check the errors, the
cancellation, the pace, the key, that the server listens on `127.0.0.1`
alone, and that stopping ends the sessions and removes the file.
`internal/app`'s tests call each tool through the command, on the quiet
player of the tests and a fake catalog. The settings page's test turns
it on, copies the command, has an assistant connect and pause, and sees
the toast. They all listen on `127.0.0.1` alone.
