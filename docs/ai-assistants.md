# AI assistants
<!-- docs: group=using order=77 -->

An AI assistant on your computer that speaks MCP, the Model Context
Protocol, can control MixMusic when you ask it in plain words: "what's
this song?", "play my liked songs", "skip the next two", "a bit quieter".
It is off at first, and only the assistants of this computer can reach
it.

## Turn it on

Open Settings › Desktop, and turn on **Let AI assistants control
MixMusic**, under **AI assistants**. Under it MixMusic shows:

- **Command**: the command an assistant starts, MixMusic's own program
  with `--mcp`, and **copy** beside it.
- **Configuration**: the same as the JSON most assistants take, with
  **copy**.
- **Connected**: the assistants connected now, by the names they give.

## Add MixMusic to an assistant

An assistant starts the command as a server of MCP that runs a command,
which its settings may call a "stdio" or "local" server. The command
passes the assistant's requests to the MixMusic that runs, and its
answers back; it opens no window. Most assistants keep their servers in
a JSON file, under `mcpServers`:

```json
{
  "mcpServers": {
    "mixmusic": {
      "command": "C:\\Users\\you\\AppData\\Local\\Programs\\MixMusic\\MixMusic.exe",
      "args": ["--mcp"]
    }
  }
}
```

Copy yours from the settings page, as the program's place differs from
one computer to another: `/Applications/MixMusic.app/Contents/MacOS/MixMusic`
on a Mac, the AppImage itself, or `flatpak run` with MixMusic's ID in a
Flatpak. A portable copy adds the `env` of its `MIXMUSIC_DATA_DIR`, for
the command to find it.

Where the servers go, as the assistants' own documentation tells:

- **Claude Desktop**: Settings › Developer › Edit Config opens
  `claude_desktop_config.json`. Add `mixmusic` to its `mcpServers`, and
  start Claude again.
- **Claude Code**: `claude mcp add mixmusic -- "<the program>" --mcp`,
  with the program the command names.
- **Cursor**: `mcp.json` in the `.cursor` folder of your home folder, under
  `mcpServers`.
- **VS Code**: `mcp.json`, under `servers`, where the entry also takes
  `"type": "stdio"`.

Other assistants take the same command and argument in their own words.

## What an assistant can do

| Tool | What it does |
|---|---|
| `now_playing` | Tells what plays: the track, its artists, album and service, whether it plays, where it is, the volume, shuffle and repeat. |
| `get_queue` | Lists the next 20 tracks of the queue, and how many come after them. |
| `play`, `pause` | Play and pause, as the player bar's button. |
| `next_track` | Skips to the next track, or past several: "skip the next two" plays the third. |
| `previous_track` | Goes back, as the previous button does: a track past its first 3 seconds starts again. |
| `seek` | Moves the track to a time, or forward or back by some seconds. |
| `set_volume` | Sets the volume, or turns it up or down by some percent. |
| `search` | Searches YouTube Music, and Spotify once you signed in, for songs, music videos, albums, artists and playlists. While explicit content is hidden, it finds none. |
| `play_result` | Plays a song, an album, an artist's top tracks or a playlist that a search found, or puts it next, or at the end of the queue. |
| `list_playlists` | Lists your liked songs, your mixes, and your playlists on YouTube Music and Spotify. |
| `play_playlist` | Plays one of them, by its name or its ID, shuffled if you like. |

An assistant can't like or dislike, change a mix or a playlist, download,
change your settings, or read the paths of your music files.

## What you see

Each thing an assistant does shows a toast in MixMusic's window, such as
"An AI assistant paused “Song” by Artist", or "An AI assistant played
your liked songs". Reading what plays, the queue, a search or your
playlists shows none. Settings › Desktop › AI assistants lists the
assistants connected.

Turning the switch off ends every assistant's session at once: the
assistant shows MixMusic's server as stopped. After you turn it on again,
start the server again in the assistant, or the assistant itself.

## What goes where

MixMusic listens for the command on `127.0.0.1`, this computer's own
address, so that no other computer reaches it, and the system's firewall
never asks. The command proves it is yours with a key MixMusic writes,
while it's on, in `mcp.json` in its data folder, which your account alone
reads. Each time it turns on, the key is new.

What the tools tell goes to the assistant, and from it to the AI model it
uses, which may run on the internet: the track that plays and the queue,
the results of its searches, and the names of your playlists and mixes.
Turn it off when you don't use it.

## When it doesn't work

- **The assistant says the server failed, with "MixMusic isn't open, or
  Let AI assistants control MixMusic is off"** in its log: open MixMusic,
  turn the switch on, then start the server again in the assistant.
- **A tool says "MixMusic's player isn't ready yet"**: MixMusic is still
  starting. Ask again in a moment.
- **The assistant lists no tools of MixMusic**: check that its
  configuration names the program the settings page shows, which moves
  when you install MixMusic elsewhere.

[Settings](settings.md#desktop) lists the other rows of Settings ›
Desktop.
