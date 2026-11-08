# Home
<!-- docs: group=using order=15 -->

Home is where MixMusic opens: your own tiles first, then shelves of what you played and made, and those of YouTube Music and Spotify. You choose which shelves it shows, in which order, and add shelves of your own.

## What it shows

- **Your tiles.** Jump back in plays the track you played last; Liked songs and Your mixes open them; Add Spotify signs you in while you aren't.
- **Your shelves.** Recently played, Your mixes, and [New from artists you follow](library.md#new-releases).
- **Spotify's**, once you [signed in](accounts.md): what you played there lately, your top tracks, your playlists and your saved albums.
- **YouTube Music's** home: its own shelves, as Quick picks, mixes and new releases, which are yours once you signed in.

The pivots at the top keep Everything, only Yours, only Spotify's or only YouTube Music's.

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
