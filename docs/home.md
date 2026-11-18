# Home
<!-- docs: group=using order=15 -->

Home is where MixMusic opens: your own tiles first, then shelves of what you played and made, and those of YouTube Music and Spotify. You choose which shelves it shows, in which order, and add shelves of your own.

## What it shows

- **Your tiles.** Jump back in plays the track you played last; Liked songs and Your mixes open them; Add Spotify signs you in while you aren't.
- **Your shelves.** Recently played, Your mixes, and [New from artists you follow](library.md#new-releases).
- **Spotify's**, once you [signed in](accounts.md): what you played there lately, your top tracks, your playlists and your saved albums.
- **YouTube Music's** home: its own shelves, as Quick picks, mixes and new releases, which are yours once you signed in.

A shelf of songs, on Home, Explore and the now playing page, has **Play** and **Shuffle** buttons in its title row, before the arrows that scroll it. Play queues the shelf's songs in the order shown and starts at the first one the queue doesn't pass over; Shuffle queues the same songs with shuffle on, from one at random. Shuffle shows from two songs on. Songs that Hide explicit content leaves out stay out. A shelf that mixes songs with albums, playlists, artists or podcasts plays only its songs, and a shelf with no songs has no buttons; its tiles keep their own play circle.

The pivots at the top keep Everything, only Yours, only Spotify's or only YouTube Music's.

Jump back in, Recently played and Your mixes follow what you play and the mixes you make or delete, so they show the history as it is now, not as it was when Home first showed. Before there's any history, Jump back in is the track that's current.

Your own shelves show at once. A service's shelves that are still coming show a spinner, and the shelves placed after them wait, so that none jumps when they arrive. When YouTube Music or Spotify can't load its shelves, Home says so once, where that service's shelves would be, in a box with the reason and **try again**, which asks again. A shelf you added that couldn't come shows its title and the same box, where you placed it. While you're offline and your downloads hold tracks, an **open downloads** button shows under the first box and opens [Library › Downloads](library.md#downloads).

## Arrange Home

**Edit home**, at the top of Home, lists its shelves in the order Home shows them, and keeps each change at once. **Done** shows Home again.

- **Move a shelf:** drag it onto another, which it takes the place of. Or click its row, or reach it with Tab, and press Alt+Up or Alt+Down (⌥↑ and ⌥↓ on macOS) to move it a place up or down.
- **Hide a shelf:** its × hides it. The shelves you hid wait under the others, under Hidden; **Show** brings one back where it was.
- **Add a shelf:** Add a shelf adds one of these, first, where you see it:
  - one of your [mixes](library.md#mixes);
  - a playlist of your library on YouTube Music or Spotify, once you signed in to it;
  - an artist's new releases: the newest albums and singles of an artist you follow;
  - Recently played, if you hid it;
  - Most played this month: the tracks you played most these 30 days, as [Stats](library.md#stats) count them;
  - your [downloads](library.md#downloads), the last downloaded first.

  The menu checks those Home shows already; choosing one again takes it off. A shelf you added has a bin on its row instead of the ×, which removes it.
- **Reset home** puts Home back as it comes, once you said so: its shelves in their own order, those you hid shown, and those you added taken off.

## How Home keeps your order

- **Shelves keep their places as the services change theirs.** YouTube Music and Spotify change their shelves from day to day, and name them in your language. MixMusic tells a shelf of YouTube Music by the page its title or its More button opens, Spotify's and your own by what they hold, and only a shelf that opens no page, as Quick picks, by its title, so that a shelf renamed is a new one.
- **A service's new shelves** come after the ones you placed, in the service's order.
- **A shelf the service no longer has** shows nothing, and leaves your order the next time you edit Home. One you hid stays hidden, in case it comes back.
- **It's kept in settings.json,** as `homeShelves`: each shelf's key, its title as you last arranged Home, and whether you hid it. It goes where your settings go, as when you [bring them to another copy](data-and-privacy.md#clear-move-or-reset).

## Explore

Explore is YouTube Music's own pages of what's new and what's popular. It's YouTube Music's alone, and it works without signing in. Open it from the sidebar, with Alt+5 (⌥5 on macOS), or with the link `mixmusic://explore`. Three pivots sit under its title. Back and Forward come back to the one you chose, and `mixmusic://explore/charts` or `mixmusic://explore/moods` opens on one; the sidebar, Alt+5 and `mixmusic://explore` open New:

- **New:** the shelves of YouTube Music's explore page: new albums and singles, and trending songs.
- **Charts:** the shelves of YouTube Music's charts.
- **Moods & genres:** a grid of tiles, by YouTube Music's groups. A tile opens a page of that mood's or genre's playlists, headed with its name, in YouTube Music's own shelves.

[Hide explicit content](playing-music.md#hiding-explicit-content) keeps explicit songs out of its shelves. When YouTube Music has nothing to show, the page says "YouTube Music shows nothing here right now." When it can't be reached, the page shows the reason with a way to try again, and Refresh this page (F5, ⇧⌘R on macOS) asks again.
